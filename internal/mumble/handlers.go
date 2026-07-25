package mumble

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dchote/go-mumble-server/internal/acl"
	"github.com/dchote/go-mumble-server/internal/audio"
	"github.com/dchote/go-mumble-server/internal/auth"
	"github.com/dchote/go-mumble-server/internal/ban"
	"github.com/dchote/go-mumble-server/internal/channel"
	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/internal/user"
	"github.com/dchote/go-mumble-server/pkg/mumble"
	mumbleaudio "github.com/dchote/go-mumble-server/pkg/mumble/audio"
	"github.com/dchote/go-mumble-server/pkg/mumble/crypto"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol"
	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
	"gorm.io/gorm"
)

// Server holds Mumble server state and builds the handler table.
type Server struct {
	cfg             *config.Config
	db              *gorm.DB
	users           *user.Manager
	chans           *channel.Manager
	bans            *ban.Manager
	acl             *acl.Evaluator
	table           protocol.HandlerTable
	connMu          sync.RWMutex
	conns           map[uint32]*connection.Conn
	addrBySession   sync.Map // session -> net.Addr
	sessionByAddr   sync.Map // addr.String() -> uint32 sessionID
	voiceTargets    sync.Map // session -> map[targetID][]session
	textRateLimiter sync.Map // session -> *rateWindow
	// userStateRateLimiter throttles self-targeted UserState, which murmur also
	// rate-limits because each accepted message fans out to every client.
	userStateRateLimiter sync.Map // session -> *rateWindow
	channelCryptoMu      sync.RWMutex
	channelCrypto        map[uint32]string // channelID -> "legacy"|"lite"|"secure"|"mixed"|""
	router               *audio.Router
	udpConn              net.PacketConn
	voiceDebug           atomic.Bool
	// Content policy, held separately from cfg so REST edits take effect without a
	// restart and without racing the handler goroutines that read them.
	allowRecording atomic.Bool
	maxTextBytes   atomic.Int64
	maxImageBytes  atomic.Int64
}

// rateWindow is a fixed one-second window counter guarding a per-session message
// budget.
type rateWindow struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
}

func (r *rateWindow) allow(limit int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.windowAt) > time.Second {
		r.windowAt = now
		r.count = 0
	}
	r.count++
	return r.count <= limit
}

func rateLimitAllows(limiter *sync.Map, sessionID uint32, perSecond int) bool {
	v, _ := limiter.LoadOrStore(sessionID, &rateWindow{})
	return v.(*rateWindow).allow(perSecond)
}

// HandleUDP processes an incoming UDP voice or ping packet.
func (s *Server) HandleUDP(addr net.Addr, data []byte) {
	if s.udpConn == nil {
		return
	}
	voiceDebug := s.voiceDebug.Load()
	plain := make([]byte, len(data)+256)
	var senderSession uint32
	var senderCrypt *crypto.CryptState
	var fromCache bool

	addrKey := addr.String()

	// Primary path: look up session by cached address (set after first successful identification).
	if cached, ok := s.sessionByAddr.Load(addrKey); ok {
		sid := cached.(uint32)
		s.connMu.RLock()
		c, cOk := s.conns[sid]
		s.connMu.RUnlock()
		if cOk && c.Crypt != nil && c.State() == connection.StateActive {
			if err := c.Crypt.Decrypt(plain, data); err == nil {
				senderSession = sid
				senderCrypt = c.Crypt
				plain = plain[:len(data)-c.Crypt.Overhead()]
				fromCache = true
			} else {
				// Cached address no longer valid (NAT rebinding); clear and fall through.
				if voiceDebug {
					slog.Info("[VOICE-DEBUG] HandleUDP: cache decrypt failed (NAT rebinding?), clearing",
						"addr", addrKey, "cached_session", sid)
				}
				s.sessionByAddr.Delete(addrKey)
			}
		}
	}

	// Fallback: trial decrypt for unmapped addresses (first packet from a new client).
	if senderSession == 0 {
		s.connMu.RLock()
		var tryCount, connCount int
		var lastErr error
		connCount = len(s.conns)
		for _, mode := range []crypto.Mode{crypto.ModeSecure, crypto.ModeLegacy, crypto.ModeLite} {
			for sid, c := range s.conns {
				if c.Crypt == nil || c.State() != connection.StateActive || c.Crypt.Mode() != mode {
					continue
				}
				tryCount++
				if err := c.Crypt.Decrypt(plain, data); err == nil {
					senderSession = sid
					senderCrypt = c.Crypt
					plain = plain[:len(data)-c.Crypt.Overhead()]
					goto found
				} else {
					lastErr = err
				}
			}
		}
	found:
		s.connMu.RUnlock()

		if senderSession == 0 && voiceDebug {
			firstBytes := 16
			if len(data) < firstBytes {
				firstBytes = len(data)
			}
			errStr := ""
			if lastErr != nil {
				errStr = lastErr.Error()
			}
			// Down-level to Debug when another port from the same host is already mapped
			// (e.g. Mumble client's probe port vs voice port) to reduce log noise.
			downLevel := false
			if host, _, err := net.SplitHostPort(addrKey); err == nil {
				s.addrBySession.Range(func(_, v interface{}) bool {
					if a, ok := v.(net.Addr); ok {
						if h, _, e := net.SplitHostPort(a.String()); e == nil && h == host {
							downLevel = true
							return false // stop iteration
						}
					}
					return true
				})
			}
			args := []interface{}{
				"addr", addrKey, "data_len", len(data),
				"first_bytes", hex.EncodeToString(data[:firstBytes]),
				"conn_count", connCount, "tries", tryCount, "last_err", errStr}
			if downLevel {
				slog.Debug("[VOICE-DEBUG] HandleUDP: trial decrypt failed, no matching session (same host, different port)", args...)
			} else {
				slog.Warn("[VOICE-DEBUG] HandleUDP: trial decrypt failed, no matching session", args...)
			}
		}
	}

	if senderSession == 0 {
		return
	}

	if voiceDebug && !fromCache {
		slog.Info("[VOICE-DEBUG] HandleUDP: session identified via trial-decrypt",
			"addr", addrKey, "session", senderSession)
	}

	// Cache both directions of the address<->session mapping.
	s.addrBySession.Store(senderSession, addr)
	s.sessionByAddr.Store(addrKey, senderSession)

	if len(plain) < 1 {
		return
	}

	codecType := (plain[0] >> 5) & 0x7

	// Type 1 = UDP ping: echo back to sender for connectivity confirmation,
	// but suppress the echo if the sender's channel has mixed crypto modes
	// (this forces the client to fall back to TCP tunnel).
	if codecType == 1 {
		cid, ok := s.users.ChannelID(senderSession)
		mixed := ok && s.channelHasMixedCrypto(cid)
		if mixed {
			if voiceDebug {
				slog.Info("[VOICE-DEBUG] HandleUDP: ping suppress (mixed_crypto channel)",
					"session", senderSession, "channel", cid)
			}
			return
		}
		enc := make([]byte, len(plain)+senderCrypt.Overhead())
		if err := senderCrypt.Encrypt(enc, plain); err == nil {
			s.udpConn.WriteTo(enc, addr)
			if voiceDebug {
				slog.Info("[VOICE-DEBUG] HandleUDP: ping echo sent", "session", senderSession, "addr", addrKey)
			}
		}
		return
	}

	if s.router == nil {
		return
	}
	target := plain[0] & 0x1F
	outgoing := rewriteAudioPacket(senderSession, plain)
	_ = s.router.Route(senderSession, target, outgoing)
}

// rewriteAudioPacket converts a client-to-server audio packet into a
// server-to-client packet by inserting the sender's session ID varint
// between the header byte and the rest of the payload.
// Client→server: [header] [sequence] [payload_len] [data...]
// Server→client: [header] [session]  [sequence]    [payload_len] [data...]
func rewriteAudioPacket(senderSession uint32, clientPacket []byte) []byte {
	if len(clientPacket) < 1 {
		return clientPacket
	}
	var sessionBuf [mumbleaudio.MaxVarintLen]byte
	n := mumbleaudio.EncodeVarint(sessionBuf[:], int64(senderSession))
	out := make([]byte, 1+n+len(clientPacket)-1)
	out[0] = clientPacket[0]
	copy(out[1:], sessionBuf[:n])
	copy(out[1+n:], clientPacket[1:])
	return out
}

// SendAudio implements audio.RecipientSender. Encrypts with recipient's key and sends via UDP, or TCP fallback.
// When the recipient's channel has mixed crypto modes, UDP is skipped to force TCP tunnel relay.
func (s *Server) SendAudio(sessionID uint32, packet []byte) error {
	s.connMu.RLock()
	c, ok := s.conns[sessionID]
	recipientAddr, _ := s.addrBySession.Load(sessionID)
	s.connMu.RUnlock()
	if !ok || c == nil {
		if s.voiceDebug.Load() {
			slog.Warn("[VOICE-DEBUG] SendAudio: recipient conn not found", "recipient", sessionID)
		}
		return nil
	}

	voiceDebug := s.voiceDebug.Load()

	// Try UDP first if recipient has a known UDP address,
	// but force TCP tunnel when the channel has mixed crypto modes.
	if recipientAddr != nil && c.Crypt != nil && s.udpConn != nil {
		cid, uOk := s.users.ChannelID(sessionID)
		mixed := uOk && s.channelHasMixedCrypto(cid)
		if !uOk || !mixed {
			addr := recipientAddr.(net.Addr)
			overhead := c.Crypt.Overhead()
			enc := make([]byte, len(packet)+overhead)
			encErr := c.Crypt.Encrypt(enc, packet)
			if encErr == nil {
				_, writeErr := s.udpConn.WriteTo(enc, addr)
				if voiceDebug {
					slog.Info("[VOICE-DEBUG] SendAudio via UDP",
						"recipient", sessionID, "addr", addr.String(),
						"pkt_len", len(packet), "enc_len", len(enc), "err", writeErr)
				}
				return writeErr
			}
			if voiceDebug {
				slog.Error("[VOICE-DEBUG] SendAudio encrypt failed", "recipient", sessionID, "err", encErr)
			}
		} else if voiceDebug {
			slog.Info("[VOICE-DEBUG] SendAudio skipping UDP (mixed_crypto)",
				"recipient", sessionID, "mixed", mixed)
		}
	} else if voiceDebug {
		slog.Info("[VOICE-DEBUG] SendAudio no UDP path",
			"recipient", sessionID,
			"has_addr", recipientAddr != nil,
			"has_crypt", c.Crypt != nil,
			"has_udp_conn", s.udpConn != nil)
	}

	// TCP fallback: wrap as UDPTunnel message
	err := c.WriteRaw(protocol.MessageUDPTunnel, packet)
	if voiceDebug {
		slog.Info("[VOICE-DEBUG] SendAudio via TCP fallback",
			"recipient", sessionID, "pkt_len", len(packet), "err", err)
	}
	return err
}

func (s *Server) canSenderSpeak(sessionID uint32) bool {
	g, ok := s.users.SpeakGateFor(sessionID)
	if !ok {
		return false
	}
	if g.Mute || g.Suppress || g.SelfMute {
		return false
	}
	return s.aclCheck(acl.Subject{SessionID: sessionID, UserID: g.UserID}, g.ChannelID, mumble.PermissionSpeak)
}

func (s *Server) audioFilterRecipient(senderSessionID, recipientSessionID uint32) bool {
	vs, ok := s.users.VoiceState(recipientSessionID)
	if !ok {
		return false
	}
	return !vs.Deaf && !vs.SelfDeaf
}

func (s *Server) getVoiceTargetRecipients(sessionID uint32, targetID uint8) []uint32 {
	v, ok := s.voiceTargets.Load(sessionID)
	if !ok {
		return nil
	}
	m, ok := v.(map[uint8][]uint32)
	if !ok {
		return nil
	}
	return m[targetID]
}

// UpdateChannelCrypto recomputes the aggregate crypto mode string for a channel.
// Must be called whenever the channel's user set changes (join, leave, move, disconnect).
func (s *Server) UpdateChannelCrypto(channelID uint32) {
	sessions := s.users.SessionIDsInChannel(channelID)
	modes := make(map[crypto.Mode]bool)
	s.connMu.RLock()
	for _, sid := range sessions {
		if c, ok := s.conns[sid]; ok && c.Crypt != nil {
			modes[c.Crypt.Mode()] = true
		}
	}
	s.connMu.RUnlock()

	var mode string
	switch len(modes) {
	case 0:
		mode = ""
	case 1:
		for m := range modes {
			mode = cryptoModeString(m)
		}
	default:
		mode = "mixed"
	}

	s.channelCryptoMu.Lock()
	if mode == "" {
		delete(s.channelCrypto, channelID)
	} else {
		s.channelCrypto[channelID] = mode
	}
	s.channelCryptoMu.Unlock()
}

// channelHasMixedCrypto returns true if the channel (or any of its linked channels)
// has clients using different crypto modes.
func (s *Server) channelHasMixedCrypto(channelID uint32) bool {
	allModes := make(map[string]bool)

	channelIDs := []uint32{channelID}
	if linked := s.chans.LinkedChannelIDs(channelID); len(linked) > 1 {
		channelIDs = append(channelIDs, linked[1:]...)
	}

	s.channelCryptoMu.RLock()
	for _, cid := range channelIDs {
		if m, ok := s.channelCrypto[cid]; ok && m != "" {
			if m == "mixed" {
				s.channelCryptoMu.RUnlock()
				return true
			}
			allModes[m] = true
		}
	}
	s.channelCryptoMu.RUnlock()
	return len(allModes) > 1
}

// ChannelCryptoMode returns the aggregate crypto mode string for a channel.
// Returns "legacy", "lite", "secure", "mixed", or "" (no active users).
func (s *Server) ChannelCryptoMode(channelID uint32) string {
	s.channelCryptoMu.RLock()
	defer s.channelCryptoMu.RUnlock()
	return s.channelCrypto[channelID]
}

// AllChannelCryptoModes returns a snapshot of all channel crypto modes.
func (s *Server) AllChannelCryptoModes() map[uint32]string {
	s.channelCryptoMu.RLock()
	defer s.channelCryptoMu.RUnlock()
	out := make(map[uint32]string, len(s.channelCrypto))
	for k, v := range s.channelCrypto {
		out[k] = v
	}
	return out
}

// NewServer creates a Mumble protocol server.
func NewServer(cfg *config.Config, db *gorm.DB, serverID uint, udpConn net.PacketConn) *Server {
	users := user.NewManager(db, cfg.MaxUsers)
	chans := channel.NewManager(db, serverID)
	_ = acl.EnsureDefaultRootACLs(db, serverID)
	s := &Server{
		cfg:           cfg,
		db:            db,
		users:         users,
		chans:         chans,
		bans:          ban.NewManager(db, serverID),
		acl:           acl.NewEvaluator(db, chans, users),
		table:         protocol.NewHandlerTable(),
		conns:         make(map[uint32]*connection.Conn),
		channelCrypto: make(map[uint32]string),
		udpConn:       udpConn,
	}
	voiceDebug := cfg.VoiceDebug
	s.voiceDebug.Store(voiceDebug)
	s.SetContentPolicy(cfg.AllowRecording, cfg.MaxTextMessageLength, cfg.MaxImageMessageLength)
	s.router = audio.NewRouterWithConfig(audio.RouterConfig{
		Sender: s,
		GetChan: func(sid uint32) uint32 {
			cid, ok := s.users.ChannelID(sid)
			if !ok {
				return 0
			}
			return cid
		},
		GetUsersInChan: s.users.SessionIDsInChannel,
		GetVoiceTarget: s.getVoiceTargetRecipients,
		GetLinkedChans: func(cid uint32) []uint32 {
			ids := s.chans.LinkedChannelIDs(cid)
			if len(ids) <= 1 {
				return nil
			}
			return ids[1:]
		},
		FilterRecipient: s.audioFilterRecipient,
		CanSenderSpeak:  s.canSenderSpeak,
		VoiceDebug:      voiceDebug,
	})
	s.registerHandlers()
	return s
}

func (s *Server) registerHandlers() {
	s.table[protocol.MessageVersion] = s.handleVersion
	s.table[protocol.MessageAuthenticate] = s.handleAuthenticate
	s.table[protocol.MessagePing] = s.handlePing
	s.table[protocol.MessageUserRemove] = s.handleUserRemove
	s.table[protocol.MessageUserState] = s.handleUserState
	s.table[protocol.MessageCryptSetup] = s.handleCryptSetup
	s.table[protocol.MessageChannelState] = s.handleChannelState
	s.table[protocol.MessageChannelRemove] = s.handleChannelRemove
	s.table[protocol.MessageTextMessage] = s.handleTextMessage
	s.table[protocol.MessageVoiceTarget] = s.handleVoiceTarget
	s.table[protocol.MessageUDPTunnel] = s.handleUDPTunnel
	s.table[protocol.MessageBanList] = s.handleBanList
	s.table[protocol.MessageACL] = s.handleACL
	s.table[protocol.MessagePermissionQuery] = s.handlePermissionQuery
	s.table[protocol.MessageRequestBlob] = s.handleRequestBlob
	s.table[protocol.MessageUserStats] = s.handleUserStats
	s.table[protocol.MessageQueryUsers] = s.handleQueryUsers
	s.table[protocol.MessageUserList] = s.handleUserList
	s.table[protocol.MessageContextActionModify] = s.handleContextActionModify
	s.table[protocol.MessageContextAction] = s.handleContextAction
	s.table[protocol.MessagePluginDataTransmission] = s.handlePluginDataTransmission
}

// HandlerTable returns the handler table.
func (s *Server) HandlerTable() protocol.HandlerTable {
	return s.table
}

// UserManager returns the user manager.
// ChanManager returns the channel manager for REST API channel CRUD.
func (s *Server) ChanManager() *channel.Manager {
	return s.chans
}

func (s *Server) UserManager() *user.Manager {
	return s.users
}

// ACLEvaluator returns the ACL evaluator for cache invalidation.
func (s *Server) ACLEvaluator() *acl.Evaluator {
	return s.acl
}

// RefreshSuppressStates recomputes Suppress from Speak ACL for every connected user
// and broadcasts any changes. Call after ACL edits so clients (including Mumla/Plumble)
// see the correct suppressed indicator without rejoining. Sends a minimal
// {session, suppress} delta per murmur's clearACLCache (Server.cpp:2187-2199), with
// the real suppress value rather than upstream's hardcoded true, which is a known
// upstream bug.
//
// Note: Humla's AudioHandler treats a suppress-only update as authoritative for the
// mute OR of mute/self_mute/suppress (absent fields read as false). That is a client
// bug; we still match murmur's minimal delta rather than over-broadcasting self_mute.
// See docs/features/0007-userstate-field-presence.md.
func (s *Server) RefreshSuppressStates() {
	if s.acl == nil {
		return
	}
	for _, u := range s.users.SnapshotAll() {
		sid := u.SessionID
		// Resolved before UpdateUser — see maySpeak.
		maySpeak := s.maySpeak(acl.SubjectOf(u), u.ChannelID)
		var changed bool
		updated, ok := s.users.UpdateUser(sid, func(live *mumble.User) {
			changed = applySuppressFromSpeak(live, maySpeak)
		})
		if !ok || !changed {
			continue
		}
		s.Broadcast(0, protocol.MessageUserState, &messages.UserState{
			Session:   sid,
			Suppress:  updated.Suppress,
			SetFields: messages.UserStateSetSession | messages.UserStateSetSuppress,
		})
	}
}

// baselinePermissions is what this server answers with when it has no ACL
// evaluator at all, so an evaluator-less server is neither wide open nor completely
// locked down: ordinary participation is allowed, administration is not.
const baselinePermissions = acl.DefaultPermissions

// aclCheck resolves a permission for a subject in channelID. Every permission gate
// in this package goes through here rather than touching s.acl directly, so the
// no-evaluator case is answered in exactly one place.
//
// Lock ordering: resolve ACL questions BEFORE taking the user manager's lock. The
// evaluator can look up connected users for group membership, so calling it from
// inside UpdateUser would deadlock against that same lock.
func (s *Server) aclCheck(subject acl.Subject, channelID uint32, perm mumble.Permission) bool {
	if s == nil || s.acl == nil {
		return baselinePermissions&perm == perm
	}
	return s.acl.Check(subject, channelID, perm)
}

// aclPermissions returns the effective permission mask for a subject in channelID.
func (s *Server) aclPermissions(subject acl.Subject, channelID uint32) uint32 {
	if s == nil || s.acl == nil {
		return uint32(baselinePermissions)
	}
	return s.acl.EffectivePermissions(subject, channelID)
}

// invalidateACLCache drops cached permissions. Group membership depends on where a
// user currently is (@in/@out) and on the tokens they presented, so every change to
// either has to go through here.
func (s *Server) invalidateACLCache() {
	if s == nil || s.acl == nil {
		return
	}
	s.acl.InvalidateCache()
}

// maySpeak reports whether the subject may Speak in channelID.
func (s *Server) maySpeak(subject acl.Subject, channelID uint32) bool {
	return s.aclCheck(subject, channelID, mumble.PermissionSpeak)
}

// channelIsFull reports whether a channel has reached its user limit for the
// subject, mirroring murmur's Server::isChannelFull (Server.cpp:2450): a Write
// holder is never blocked, and a limit of 0 means unlimited.
func (s *Server) channelIsFull(channelID uint32, subject acl.Subject) bool {
	ch, ok := s.chans.GetChannel(channelID)
	if !ok || ch.MaxUsers == 0 {
		return false
	}
	if s.aclCheck(subject, channelID, mumble.PermissionWrite) {
		return false
	}
	return uint32(s.users.CountInChannel(channelID)) >= ch.MaxUsers
}

// firstPermanentChannel walks up from channelID to the nearest ancestor that is not
// a temporary channel, mirroring the loop murmur uses before honouring a mute in a
// temporary channel (Messages.cpp:815-833). Reports false when the whole chain is
// temporary or the channel is unknown.
func (s *Server) firstPermanentChannel(channelID uint32) (uint32, bool) {
	if s.chans == nil {
		return channelID, true
	}
	for _, cid := range s.chans.AncestorChain(channelID) {
		ch, ok := s.chans.GetChannel(cid)
		if !ok {
			return 0, false
		}
		if !ch.IsTemporary {
			return cid, true
		}
	}
	return 0, false
}

// SetContentPolicy updates the recording policy and content size limits. Callable
// at runtime when server config is updated via REST.
func (s *Server) SetContentPolicy(allowRecording bool, maxTextBytes, maxImageBytes int) {
	s.allowRecording.Store(allowRecording)
	s.maxTextBytes.Store(int64(maxTextBytes))
	s.maxImageBytes.Store(int64(maxImageBytes))
}

// recordingAllowed reports whether clients may announce that they are recording.
func (s *Server) recordingAllowed() bool {
	return s.allowRecording.Load()
}

// maxTextLength returns the cap on user-supplied text (chat messages, comments).
// 0 means unlimited.
func (s *Server) maxTextLength() int {
	return int(s.maxTextBytes.Load())
}

// maxImageLength returns the cap on user-supplied image payloads (avatar textures).
// 0 means unlimited.
func (s *Server) maxImageLength() int {
	return int(s.maxImageBytes.Load())
}

// BanManager returns the ban manager (for cache invalidation when bans change via REST).
func (s *Server) BanManager() *ban.Manager {
	return s.bans
}

// HasUDPAddress returns true if the session has sent at least one UDP packet (voice or ping),
// indicating the client is using native UDP transport rather than TCP tunnel.
func (s *Server) HasUDPAddress(sessionID uint32) bool {
	_, ok := s.addrBySession.Load(sessionID)
	return ok
}

// SetVoiceDebug enables or disables voice path debug logging (UDP, TCP tunnel, routing).
// Callable at runtime when server config is updated via REST.
func (s *Server) SetVoiceDebug(enabled bool) {
	s.voiceDebug.Store(enabled)
	if s.router != nil {
		s.router.SetVoiceDebug(enabled)
	}
}

// BroadcastChannelState broadcasts a channel's state to all connected clients.
// Used when channels are created/updated via REST so Mumble clients see the changes.
func (s *Server) BroadcastChannelState(ch *mumble.Channel) {
	if ch != nil {
		s.Broadcast(0, protocol.MessageChannelState, channelToState(ch))
	}
}

// BroadcastChannelRemove broadcasts a channel removal to all connected clients.
func (s *Server) BroadcastChannelRemove(channelID uint32) {
	s.Broadcast(0, protocol.MessageChannelRemove, &messages.ChannelRemove{ChannelID: channelID})
}

// RegisterConn registers a connection for broadcasting.
func (s *Server) RegisterConn(sessionID uint32, c *connection.Conn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	s.conns[sessionID] = c
}

// UnregisterConn removes a connection. Callers must call UpdateChannelCrypto for the
// user's channel after removing the user (UnregisterConn cannot see the user if
// callers remove first).
func (s *Server) UnregisterConn(sessionID uint32) {
	if a, ok := s.addrBySession.Load(sessionID); ok {
		s.sessionByAddr.Delete(a.(net.Addr).String())
	}
	s.connMu.Lock()
	delete(s.conns, sessionID)
	s.connMu.Unlock()
	s.voiceTargets.Delete(sessionID)
	s.addrBySession.Delete(sessionID)
	s.textRateLimiter.Delete(sessionID)
	s.userStateRateLimiter.Delete(sessionID)
	s.chans.CleanEmptyTempChannels(func(cid uint32) bool {
		return s.users.CountInChannel(cid) > 0
	})
}

// KickSession kicks a user (server-initiated, e.g. from REST API). Actor 0 = server.
func (s *Server) KickSession(sessionID uint32, reason string) bool {
	u, ok := s.users.Snapshot(sessionID)
	if !ok {
		return false
	}
	channelID := u.ChannelID
	targetConn := s.conn(sessionID)
	ur := &messages.UserRemove{Session: sessionID, Actor: 0, Reason: reason, Ban: false}
	s.Broadcast(0, protocol.MessageUserRemove, ur)
	s.users.Remove(sessionID)
	s.UnregisterConn(sessionID)
	if channelID != 0 {
		s.UpdateChannelCrypto(channelID)
	}
	if targetConn != nil {
		targetConn.CloseAfterFlush()
	}
	slog.Info("User kicked via REST", "session", sessionID, "name", u.Name, "reason", reason)
	return true
}

// MuteSession sets server mute state for a user. Routed through the same helper as
// the protocol handler so the "un-muting clears deafen" invariant holds here too.
// Broadcasts a minimal {session, actor, mute} delta, plus deaf only when the cascade
// cleared it, matching handleUserState's echo shape rather than a full snapshot.
func (s *Server) MuteSession(sessionID uint32, mute bool) bool {
	req := &messages.UserState{
		Session:   sessionID,
		Mute:      mute,
		SetFields: messages.UserStateSetSession | messages.UserStateSetMute,
	}
	updated, ok := s.users.UpdateUser(sessionID, func(u *mumble.User) {
		applyAdminVoiceState(&u.VoiceState, req)
	})
	if !ok {
		return false
	}
	// Actor 0 = server-initiated (REST), matching KickSession/BanAndKickSession. Left
	// without presence so it is omitted on the wire rather than claiming session 0 acted.
	s.Broadcast(0, protocol.MessageUserState, req)
	slog.Info("User mute changed via REST", "session", sessionID, "name", updated.Name, "mute", mute)
	return true
}

// banEntry starts a BanList entry with the shared name/reason/start fields.
func banEntry(name, reason string) messages.BanEntry {
	return messages.BanEntry{
		Name:   name,
		Reason: reason,
		Start:  time.Now().Format(time.RFC3339),
	}
}

// banEntryByCert bans a client by certificate fingerprint so the ban survives IP changes.
func banEntryByCert(name, reason, certHash string) messages.BanEntry {
	be := banEntry(name, reason)
	be.Hash = strings.ToLower(certHash)
	return be
}

// banEntryByIP bans a client by address. IPv4 uses a /32 mask; IPv6 uses /128.
func banEntryByIP(name, reason string, ip net.IP) messages.BanEntry {
	be := banEntry(name, reason)
	mask := uint32(32)
	if ip4 := ip.To4(); ip4 != nil {
		ip = ip4
	} else {
		mask = 128
	}
	be.Address = ip
	be.Mask = mask
	return be
}

// BanAndKickSession bans the user by IP and kicks them.
func (s *Server) BanAndKickSession(sessionID uint32, reason string) bool {
	u, ok := s.users.Snapshot(sessionID)
	if !ok {
		return false
	}
	targetConn := s.conn(sessionID)
	var ip net.IP
	if u.Address != "" {
		ip = net.ParseIP(u.Address)
	}
	if ip == nil && targetConn != nil {
		if addr := targetConn.RemoteAddr(); addr != nil {
			if host, _, err := net.SplitHostPort(addr.String()); err == nil {
				ip = net.ParseIP(host)
			}
		}
	}
	existing := s.bans.List()
	// Ban by cert hash when available (persists across IP changes); otherwise by IP.
	if u.CertHash != "" {
		existing = append(existing, banEntryByCert(u.Name, reason, u.CertHash))
		_ = s.bans.Replace(existing)
	} else if ip != nil {
		existing = append(existing, banEntryByIP(u.Name, reason, ip))
		_ = s.bans.Replace(existing)
	}
	channelID := u.ChannelID
	ur := &messages.UserRemove{Session: sessionID, Actor: 0, Reason: reason, Ban: true}
	s.Broadcast(0, protocol.MessageUserRemove, ur)
	s.users.Remove(sessionID)
	s.UnregisterConn(sessionID)
	if channelID != 0 {
		s.UpdateChannelCrypto(channelID)
	}
	if targetConn != nil {
		targetConn.CloseAfterFlush()
	}
	slog.Info("User banned and kicked via REST", "session", sessionID, "name", u.Name, "reason", reason)
	return true
}

// Broadcast sends a message to all connections except skipSession.
func (s *Server) Broadcast(skipSession uint32, msgType protocol.MessageType, msg messages.Message) {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	for sid, c := range s.conns {
		if sid != skipSession {
			_ = c.WriteMessage(msgType, msg)
		}
	}
}

func (s *Server) handleVersion(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if len(payload) > 0 {
		var v messages.Version
		if err := v.Unmarshal(payload); err != nil {
			return err
		}
		slog.Debug("client version", "release", v.Release, "os", v.OS)
		c.SetClientCryptoModes(v.CryptoModes)
		c.SetClientVersion(clientVersionFull(v))
	}
	return nil
}

// clientVersionFull resolves a client's Version message to Mumble's packed 64-bit
// version_v2 form, preferring version_v2 and falling back to the legacy 32-bit
// version_v1 encoding. Mirrors MumbleProto::getVersion (research/mumble/src/ProtoUtils.cpp).
func clientVersionFull(v messages.Version) uint64 {
	if v.VersionV2 != 0 {
		return v.VersionV2
	}
	if v.VersionV1 != 0 {
		major := uint64((v.VersionV1 & 0xFFFF0000) >> 16)
		minor := uint64((v.VersionV1 & 0xFF00) >> 8)
		patch := uint64(v.VersionV1 & 0xFF)
		return major<<48 | minor<<32 | patch<<16
	}
	return 0
}

func (s *Server) handleAuthenticate(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	var authMsg messages.Authenticate
	if err := authMsg.Unmarshal(payload); err != nil {
		return err
	}
	addr := c.RemoteAddr()
	certHash := c.CertificateHash()
	if s.bans.IsBanned(addr, certHash) {
		return s.sendReject(c, messages.RejectWrongServerPW, "Banned")
	}
	if authMsg.Username == "" {
		return s.sendReject(c, messages.RejectInvalidUsername, "Username required")
	}
	if _, ok := s.users.SnapshotByName(authMsg.Username); ok {
		return s.sendReject(c, messages.RejectUsernameInUse, "Username in use")
	}
	if s.users.Count() >= s.cfg.MaxUsers && s.cfg.MaxUsers > 0 {
		return s.sendReject(c, messages.RejectServerFull, "Server full")
	}
	var apiUserID uint
	if s.cfg.ServerPassword != "" {
		// Server password set: accept server password OR API user password
		if authMsg.Password != s.cfg.ServerPassword {
			id, hash, _, found := s.users.LookupAPIUser(authMsg.Username)
			if !found || !auth.ComparePassword(hash, authMsg.Password) {
				return s.sendReject(c, messages.RejectWrongServerPW, "Wrong password")
			}
			apiUserID = uint(id)
		}
	} else if authMsg.Password != "" {
		// No server password: if client sent password, validate against API users
		id, hash, _, found := s.users.LookupAPIUser(authMsg.Username)
		if found {
			if !auth.ComparePassword(hash, authMsg.Password) {
				return s.sendReject(c, messages.RejectWrongServerPW, "Wrong password")
			}
			apiUserID = uint(id)
		}
	}
	userID := uint32(0)
	if apiUserID != 0 {
		userID = acl.MakeAPIUserID(apiUserID)
	} else if uid, hash, regCertHash, found := s.lookupRegisteredUser(authMsg.Username); found {
		if !registeredUserCredentialsValid(hash, regCertHash, authMsg.Password, certHash) {
			return s.sendReject(c, messages.RejectWrongServerPW, "Wrong password")
		}
		userID = uid
	}
	// The SuperUser identity carries immunity from other admins' UserState changes,
	// so it may only be claimed by a connection that actually proved an account.
	if authMsg.Username == mumble.SuperUserName && userID == 0 {
		return s.sendReject(c, messages.RejectWrongUserPW, "SuperUser requires an account")
	}
	u := mumble.User{
		UserID:       userID,
		ChannelID:    s.joinChannelFor(userID),
		Name:         authMsg.Username,
		AccessTokens: authMsg.Tokens,
		IsSuperUser:  authMsg.Username == mumble.SuperUserName,
	}
	if addr != nil {
		if host, _, err := net.SplitHostPort(addr.String()); err == nil {
			u.Address = host
		} else {
			u.Address = addr.String()
		}
	}
	u.CertHash = certHash
	stored, ok := s.users.Add(u)
	if !ok {
		return s.sendReject(c, messages.RejectServerFull, "Server full")
	}
	c.SetSessionID(stored.SessionID)
	c.SetUserName(stored.Name)
	c.SetActive()
	// This session brings its own access tokens and channel, both of which feed
	// group resolution, so anything cached for this account is now suspect.
	s.invalidateACLCache()
	s.sendSync(c, stored)
	s.UpdateChannelCrypto(stored.ChannelID)
	s.Broadcast(stored.SessionID, protocol.MessageUserState, userToState(stored))
	slog.Info("Mumble client authenticated", "user", stored.Name, "session", stored.SessionID, "channel", stored.ChannelID)
	return nil
}

// joinChannelFor picks the channel a connecting user lands in: the configured
// default, falling back to root when that channel is full or cannot be entered.
// Murmur applies the same fallback chain when a client connects (Messages.cpp:300-315).
// The user has no session yet, so permissions resolve against the account alone.
func (s *Server) joinChannelFor(userID uint32) uint32 {
	subject := acl.SubjectForUserID(userID)
	root := uint32(s.chans.RootID())
	if s.cfg.DefaultChannel <= 0 {
		return root
	}
	target := uint32(s.cfg.DefaultChannel)
	if target == root {
		return root
	}
	if _, exists := s.chans.GetChannel(target); !exists {
		return root
	}
	if !s.aclCheck(subject, target, mumble.PermissionEnter) {
		return root
	}
	if s.channelIsFull(target, subject) {
		return root
	}
	return target
}

// lookupRegisteredUser returns UserID and credentials for a username on this server's registered_users.
func (s *Server) lookupRegisteredUser(username string) (userID uint32, passwordHash, certHash string, found bool) {
	var u struct {
		UserID       int32
		PasswordHash string
		CertHash     string
	}
	err := s.db.Table("registered_users").Where("server_id = ? AND name = ?", s.chans.ServerID(), username).
		Select("user_id", "password_hash", "cert_hash").First(&u).Error
	if err != nil {
		return 0, "", "", false
	}
	return uint32(u.UserID), u.PasswordHash, u.CertHash, true
}

func registeredUserCredentialsValid(storedPasswordHash, storedCertHash, providedPassword, presentedCertHash string) bool {
	certMatches := storedCertHash != "" && presentedCertHash != "" && strings.EqualFold(storedCertHash, presentedCertHash)
	passwordMatches := false
	if providedPassword != "" && storedPasswordHash != "" {
		// Support both Argon2id and legacy bcrypt hashes.
		passwordMatches = (strings.HasPrefix(storedPasswordHash, "$argon2id$") && auth.CompareArgon2id(storedPasswordHash, providedPassword)) ||
			(strings.HasPrefix(storedPasswordHash, "$2") && auth.ComparePassword(storedPasswordHash, providedPassword))
	}
	return certMatches || passwordMatches
}

func (s *Server) sendReject(c *connection.Conn, typ messages.RejectType, reason string) error {
	return c.WriteMessage(protocol.MessageReject, &messages.Reject{Type: typ, Reason: reason})
}

func cryptoModeString(mode crypto.Mode) string {
	switch mode {
	case crypto.ModeSecure:
		return "secure"
	case crypto.ModeLite:
		return "lite"
	default:
		return "legacy"
	}
}

// negotiateCryptoMode selects the best mutually-supported crypto mode for the connection.
func (s *Server) negotiateCryptoMode(c *connection.Conn) crypto.Mode {
	clientModes := c.ClientCryptoModes()
	if clientModes == 0 {
		clientModes = 0x02 // standard client: legacy only
	}
	mutual := clientModes & 0x07 // server supports all (lite|legacy|secure)
	tls13 := false
	hasClientCert := false
	if tc, ok := c.Conn.(*tls.Conn); ok {
		st := tc.ConnectionState()
		tls13 = st.Version == tls.VersionTLS13
		hasClientCert = len(st.PeerCertificates) > 0
	}
	if (mutual&0x04) != 0 && tls13 && hasClientCert {
		return crypto.ModeSecure
	}
	if (mutual & 0x02) != 0 {
		return crypto.ModeLegacy
	}
	if (mutual & 0x01) != 0 {
		return crypto.ModeLite
	}
	return crypto.ModeLegacy
}

func (s *Server) sendSync(c *connection.Conn, u mumble.User) {
	mode := s.negotiateCryptoMode(c)
	if updated, ok := s.users.UpdateUser(u.SessionID, func(live *mumble.User) {
		live.CryptoMode = cryptoModeString(mode)
	}); ok {
		u = updated
	}
	c.Crypt = crypto.NewCryptState(mode)
	key, encNonce, decNonce := s.generateCryptSetup(mode)
	c.Crypt.SetKey(key, encNonce, decNonce)
	_ = c.WriteMessage(protocol.MessageCryptSetup, &messages.CryptSetup{
		Key:         key,
		ClientNonce: decNonce, // server's decrypt nonce = client's encrypt nonce
		ServerNonce: encNonce, // server's encrypt nonce = client's decrypt nonce
	})
	// Register conn for UDP routing before rest of sync so HandleUDP can identify
	// the sender when the client sends its first UDP packet (ping/voice) after CryptSetup.
	s.RegisterConn(u.SessionID, c)
	_ = c.WriteMessage(protocol.MessageCodecVersion, &messages.CodecVersion{Opus: true})
	for _, ch := range s.chans.GetTree() {
		cs := channelToState(ch)
		_ = c.WriteMessage(protocol.MessageChannelState, cs)
	}
	_ = c.WriteMessage(protocol.MessageUserState, userToState(u))
	for _, ou := range s.users.SnapshotAll() {
		if ou.SessionID != u.SessionID {
			_ = c.WriteMessage(protocol.MessageUserState, userToState(ou))
		}
	}
	perms := uint64(s.aclPermissions(acl.SubjectOf(u), s.chans.RootID()))
	_ = c.WriteMessage(protocol.MessageServerSync, &messages.ServerSync{
		Session:      u.SessionID,
		MaxBandwidth: uint32(s.cfg.MaxBandwidth),
		WelcomeText:  s.cfg.WelcomeText,
		Permissions:  perms,
	})
	_ = c.WriteMessage(protocol.MessageServerConfig, &messages.ServerConfig{
		MaxBandwidth:       uint32(s.cfg.MaxBandwidth),
		WelcomeText:        s.cfg.WelcomeText,
		AllowHTML:          true,
		MessageLength:      uint32(s.maxTextLength()),
		ImageMessageLength: uint32(s.maxImageLength()),
		MaxUsers:           uint32(s.cfg.MaxUsers),
		RecordingAllowed:   s.recordingAllowed(),
	})
}

func (s *Server) generateCryptSetup(mode crypto.Mode) (key, encNonce, decNonce []byte) {
	switch mode {
	case crypto.ModeSecure:
		key = make([]byte, 32)
		rand.Read(key)
		encNonce = make([]byte, 12)
		rand.Read(encNonce)
		decNonce = make([]byte, 12)
		rand.Read(decNonce)
		return key, encNonce, decNonce
	case crypto.ModeLite:
		return []byte{}, nil, nil
	default:
		key = make([]byte, 16)
		rand.Read(key)
		encNonce = make([]byte, 16)
		rand.Read(encNonce)
		decNonce = make([]byte, 16)
		rand.Read(decNonce)
		return key, encNonce, decNonce
	}
}

func channelToState(ch *mumble.Channel) *messages.ChannelState {
	// Root (ID 0): omit parent field on wire. Murmur does the same — the proto2
	// `optional` parent field is absent for root so has_parent()=false on the client.
	hasParent := ch.ID != 0
	return &messages.ChannelState{
		ChannelID:   ch.ID,
		Parent:      ch.ParentID,
		HasParent:   hasParent,
		Name:        ch.Name,
		Description: ch.Description,
		Position:    ch.Position,
		MaxUsers:    ch.MaxUsers,
		Temporary:   ch.IsTemporary,
		Links:       ch.Links,
	}
}

// userToState builds a join/roster snapshot of a user for broadcast, mirroring
// murmur's transmission of a user's profile to a newly connecting client
// (Messages.cpp:493-530). This is a snapshot, not a delta: field presence here means
// "this flag is set", not "this flag just changed", so voice flags are emitted only
// when true. mute/deaf and self_mute/self_deaf are mutually exclusive on the wire
// (a deafened user does not also carry an explicit mute=true), matching murmur's
// `if (deaf) ... else if (mute) ...` exactly.
//
// This must never be used to build the echo for a client-initiated UserState update
// (handleUserState) or any other delta broadcast: those broadcast the mutated inbound
// message instead, per docs/architecture/protocol-encoding.md, so that presence
// continues to mean "changed" on that path. Using a snapshot there was the v0.1.4
// regression this function's shape now guards against.
//
// Name and UserID are included whenever non-empty/non-zero (typical for connected
// users) because these snapshots are meant to be self-contained. Texture and Comment
// keep omit-if-empty encoding and do not set presence bits, so a routine mute update
// never looks like a blob clear. PluginIdentity and PluginContext are never sent to
// clients (per Mumble.proto).
func userToState(u mumble.User) *messages.UserState {
	state := &messages.UserState{
		Session:   u.SessionID,
		UserID:    u.UserID,
		Name:      u.Name,
		ChannelID: u.ChannelID,
		Texture:   u.Texture,
		Comment:   u.Comment,
		SetFields: messages.UserStateSetSession | messages.UserStateSetChannelID,
	}
	if u.Deaf {
		state.Deaf = true
		state.SetFields |= messages.UserStateSetDeaf
	} else if u.Mute {
		state.Mute = true
		state.SetFields |= messages.UserStateSetMute
	}
	if u.Suppress {
		state.Suppress = true
		state.SetFields |= messages.UserStateSetSuppress
	}
	if u.PrioritySpeaker {
		state.PrioritySpeaker = true
		state.SetFields |= messages.UserStateSetPrioritySpeaker
	}
	if u.Recording {
		state.Recording = true
		state.SetFields |= messages.UserStateSetRecording
	}
	if u.SelfDeaf {
		state.SelfDeaf = true
		state.SetFields |= messages.UserStateSetSelfDeaf
	} else if u.SelfMute {
		state.SelfMute = true
		state.SetFields |= messages.UserStateSetSelfMute
	}
	return state
}

func (s *Server) handlePing(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	resp := messages.Ping{}
	if len(payload) > 0 {
		var p messages.Ping
		if err := p.Unmarshal(payload); err != nil {
			return err
		}
		resp.Timestamp = p.Timestamp
		if p.TCPPingAvg > 0 {
			s.users.SetPing(c.SessionID(), p.TCPPingAvg)
		}
	}
	if resp.Timestamp == 0 {
		resp.Timestamp = uint64(time.Now().UnixMicro())
	}
	resp.Good = c.Crypt.Good
	resp.Late = c.Crypt.Late
	resp.Lost = c.Crypt.Lost
	resp.Resync = c.Crypt.Resync
	_ = c.WriteMessage(protocol.MessagePing, &resp)
	return nil
}

func (s *Server) handleUserRemove(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var ur messages.UserRemove
	if err := ur.Unmarshal(payload); err != nil {
		return err
	}
	if ur.Session == 0 {
		return nil
	}
	sender, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	target, ok := s.users.Snapshot(ur.Session)
	if !ok {
		return nil
	}
	perm := mumble.PermissionKick
	if ur.Ban {
		perm = mumble.PermissionBan
	}
	// SuperUser can never be kicked or banned, by anybody (murmur Messages.cpp:1245).
	if target.IsSuperUser || !s.aclCheck(acl.SubjectOf(sender), s.chans.RootID(), perm) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
			Type: messages.DenyPermission, Reason: "No " + map[bool]string{false: "kick", true: "ban"}[ur.Ban] + " permission",
		})
		return nil
	}
	ur.Actor = c.SessionID()
	targetConn := s.conn(ur.Session)
	s.Broadcast(0, protocol.MessageUserRemove, &ur)
	if ur.Ban && targetConn != nil {
		var ip net.IP
		if addr := targetConn.RemoteAddr(); addr != nil {
			if host, _, err := net.SplitHostPort(addr.String()); err == nil {
				ip = net.ParseIP(host)
			}
		}
		if ip != nil {
			existing := s.bans.List()
			existing = append(existing, banEntryByIP(target.Name, ur.Reason, ip))
			_ = s.bans.Replace(existing)
		}
	}
	channelID := target.ChannelID
	s.users.Remove(ur.Session)
	s.UnregisterConn(ur.Session)
	if channelID != 0 {
		s.UpdateChannelCrypto(channelID)
	}
	if targetConn != nil {
		targetConn.CloseAfterFlush()
	}
	return nil
}

// selfOnlyUserStateFields are UserState fields a client may only set on itself.
const selfOnlyUserStateFields = messages.UserStateSetSelfMute | messages.UserStateSetSelfDeaf |
	messages.UserStateSetPluginContext | messages.UserStateSetPluginIdentity |
	messages.UserStateSetRecording

// adminVoiceUserStateFields are UserState fields gated behind MuteDeafen. They are
// checked as a group before any of them is applied, so a partially-authorised message
// changes nothing at all.
const adminVoiceUserStateFields = messages.UserStateSetMute | messages.UserStateSetDeaf |
	messages.UserStateSetSuppress | messages.UserStateSetPrioritySpeaker

// pluginUserStateFields are applied to server-side state but never set murmur's
// bBroadcast flag on their own (Messages.cpp:995-1007): they are stripped before
// any echo and a plugin-only message produces no UserState broadcast.
const pluginUserStateFields = messages.UserStateSetPluginContext | messages.UserStateSetPluginIdentity

// broadcastUserStateFields are the fields that cause a UserState broadcast after
// apply (murmur's bBroadcast). plugin_context/plugin_identity are intentionally
// excluded — see pluginUserStateFields.
const broadcastUserStateFields = (selfOnlyUserStateFields &^ pluginUserStateFields) |
	adminVoiceUserStateFields |
	messages.UserStateSetChannelID | messages.UserStateSetTexture |
	messages.UserStateSetComment

// handledUserStateFields are the UserState bits this server applies. A message with
// none of these set is a no-op. Broadcasting is gated separately on
// broadcastUserStateFields after plugin fields are stripped.
const handledUserStateFields = broadcastUserStateFields | pluginUserStateFields

// handleUserState implements murmur's Server::msgUserState (Messages.cpp:764-1229).
// The broadcast is a delta echo of the (possibly server-mutated) inbound message,
// never a fresh snapshot: presence on this path means "this changed", and building
// the broadcast from userToState was the v0.1.4 regression. See
// docs/architecture/protocol-encoding.md for the full delta-vs-snapshot rule.
func (s *Server) handleUserState(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var us messages.UserState
	if err := us.Unmarshal(payload); err != nil {
		return err
	}

	sender, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	// Target: self (Session absent or same as sender) vs another user (admin ops)
	targetSession := us.Session
	if !us.Has(messages.UserStateSetSession) || targetSession == c.SessionID() {
		targetSession = c.SessionID()
	}
	target, ok := s.users.Snapshot(targetSession)
	if !ok {
		return nil
	}
	isAdminOp := targetSession != c.SessionID()

	deny := func(channelID uint32, denyType messages.DenyType, reason string) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
			ChannelID: channelID, Type: denyType, Reason: reason,
		})
	}

	// Nobody but SuperUser may act on SuperUser, and this outranks every other
	// check in the handler (Messages.cpp:775-780).
	if target.IsSuperUser && !sender.IsSuperUser {
		deny(target.ChannelID, messages.DenySuperUser, "Cannot modify SuperUser")
		return nil
	}

	// Renaming is never done via UserState — names are set at Authenticate time —
	// so has_name() is an unconditional deny (Messages.cpp:784-787), independent of
	// who the message targets.
	if us.Has(messages.UserStateSetName) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyUserName})
		return nil
	}

	// Murmur rate-limits self-targeted UserState (Messages.cpp:790) because every
	// accepted message fans out to every connected client.
	if !isAdminOp && !s.checkUserStateRateLimit(c.SessionID()) {
		return nil
	}

	// Self-registration and channel listening are not implemented by this server, so
	// strip them: applying them silently would misrepresent what happened, and
	// echoing them unstripped (change 2 below) would announce a state we never
	// actually established.
	us.UserID = 0
	us.SetFields &^= messages.UserStateSetUserID
	us.TemporaryAccessTokens = nil
	us.ListeningChannelAdd = nil
	us.ListeningChannelRemove = nil
	us.ListeningVolumeAdjustment = nil

	if us.SetFields == 0 {
		return nil
	}

	// These fields describe a client's own preferences and are meaningless when aimed
	// at somebody else. Drop the message rather than applying the remainder of it.
	if isAdminOp && us.SetFields&selfOnlyUserStateFields != 0 {
		return nil
	}

	// Permissions below are resolved against the target's current channel, before any
	// move requested by this same message is applied.
	//
	// Administrative mute/deaf/suppress/priority_speaker always require MuteDeafen,
	// including when a client targets itself — matching murmur. Clients use
	// self_mute/self_deaf for their own mute state.
	if us.SetFields&adminVoiceUserStateFields != 0 {
		// Suppress is derived from channel speak ACLs by the server, so a client may
		// only ever clear it, never assert it.
		if us.Suppress || !s.aclCheck(acl.SubjectOf(sender), target.ChannelID, mumble.PermissionMuteDeafen) {
			deny(target.ChannelID, messages.DenyPermission, "No mute/deafen permission")
			return nil
		}
		// Muting somebody inside a temporary channel is an escalation risk: anyone
		// can make a temp channel and grant themselves MuteDeafen in it. Murmur
		// therefore re-checks MuteDeafen in the first non-temporary ancestor
		// (Messages.cpp:815-833). priority_speaker is exempt upstream.
		if us.SetFields&(messages.UserStateSetMute|messages.UserStateSetDeaf|messages.UserStateSetSuppress) != 0 {
			if permanent, ok := s.firstPermanentChannel(target.ChannelID); !ok ||
				!s.aclCheck(acl.SubjectOf(sender), permanent, mumble.PermissionMuteDeafen) {
				deny(target.ChannelID, messages.DenyTemporaryChannel, "No mute/deafen permission outside temporary channel")
				return nil
			}
		}
	}

	// Comments and textures belong to their owner. Murmur lets an admin holding
	// ResetUserContent on root clear somebody else's, but never set one
	// (Messages.cpp:890-925), and bounds both against the advertised limits.
	if us.Has(messages.UserStateSetComment) {
		if isAdminOp {
			if !s.aclCheck(acl.SubjectOf(sender), s.chans.RootID(), mumble.PermissionResetUser) {
				deny(s.chans.RootID(), messages.DenyPermission, "No permission to reset user content")
				return nil
			}
			if us.Comment != "" {
				deny(target.ChannelID, messages.DenyTextTooLong, "May only clear another user's comment")
				return nil
			}
		}
		if max := s.maxTextLength(); max > 0 && len(us.Comment) > max {
			deny(target.ChannelID, messages.DenyTextTooLong, "Comment too long")
			return nil
		}
	}
	if us.Has(messages.UserStateSetTexture) {
		if max := s.maxImageLength(); max > 0 && len(us.Texture) > max {
			deny(target.ChannelID, messages.DenyTextTooLong, "Texture too large")
			return nil
		}
		if isAdminOp {
			if !s.aclCheck(acl.SubjectOf(sender), s.chans.RootID(), mumble.PermissionResetUser) {
				deny(s.chans.RootID(), messages.DenyPermission, "No permission to reset user content")
				return nil
			}
			if len(us.Texture) > 0 {
				deny(target.ChannelID, messages.DenyTextTooLong, "May only clear another user's texture")
				return nil
			}
		}
	}

	// A client that starts recording on a server which forbids it is disconnected,
	// not merely ignored (Messages.cpp:1057-1070).
	if us.Has(messages.UserStateSetRecording) && us.Recording && !target.Recording && !s.recordingAllowed() {
		_ = c.WriteMessage(protocol.MessageUserRemove, &messages.UserRemove{
			Session: targetSession,
			Reason:  "Recording is not allowed on this server",
		})
		c.CloseAfterFlush()
		return nil
	}

	// recording only counts as a change (and only ever broadcasts) when the value
	// actually differs from the stored state (Messages.cpp:1048); resending the
	// current value is a no-op even though the field is present on the wire.
	if us.Has(messages.UserStateSetRecording) && us.Recording == target.Recording {
		us.SetFields &^= messages.UserStateSetRecording
	}

	if us.SetFields&handledUserStateFields == 0 {
		return nil
	}

	channelMoved := false
	var maySpeakAtDest bool
	if us.Has(messages.UserStateSetChannelID) && s.chans != nil {
		if _, exists := s.chans.GetChannel(us.ChannelID); !exists {
			return nil
		}
		if us.ChannelID == target.ChannelID {
			// Requesting the channel you're already in drops the entire message, not
			// just the move (Messages.cpp:800-803) — matching murmur exactly.
			return nil
		}
		// Two independent checks, both from Messages.cpp:805-813. Moving somebody
		// else additionally requires Move where they currently are; every move then
		// requires either Move on the destination (an admin pulling someone in) or
		// the target's own right to Enter it.
		if isAdminOp && !s.aclCheck(acl.SubjectOf(sender), target.ChannelID, mumble.PermissionMove) {
			deny(target.ChannelID, messages.DenyPermission, "No move permission")
			return nil
		}
		if !s.aclCheck(acl.SubjectOf(sender), us.ChannelID, mumble.PermissionMove) &&
			!s.aclCheck(acl.SubjectOf(target), us.ChannelID, mumble.PermissionEnter) {
			deny(us.ChannelID, messages.DenyPermission, "Permission denied")
			return nil
		}
		if s.channelIsFull(us.ChannelID, acl.SubjectOf(sender)) {
			deny(us.ChannelID, messages.DenyChannelFull, "Channel is full")
			return nil
		}
		channelMoved = true
		oldChannelID := target.ChannelID
		s.users.SetChannel(targetSession, us.ChannelID)
		target.ChannelID = us.ChannelID
		s.invalidateACLCache()
		s.UpdateChannelCrypto(oldChannelID)
		s.UpdateChannelCrypto(us.ChannelID)
		if targetConn := s.conn(targetSession); targetConn != nil {
			if targetSession == c.SessionID() {
				perms := s.aclPermissions(acl.SubjectOf(target), us.ChannelID)
				_ = targetConn.WriteMessage(protocol.MessagePermissionQuery, &messages.PermissionQuery{
					ChannelID:   us.ChannelID,
					Permissions: uint32(perms),
				})
			}
		}
		// Resolved here, before UpdateUser — see maySpeak.
		maySpeakAtDest = s.maySpeak(acl.SubjectOf(target), us.ChannelID)
	}

	var priorityChanged, suppressChanged bool
	updated, ok := s.users.UpdateUser(targetSession, func(u *mumble.User) {
		applySelfVoiceState(&u.VoiceState, &us)
		applyAdminVoiceState(&u.VoiceState, &us)
		if channelMoved {
			priorityChanged, suppressChanged = applyChannelEnterEffects(u, maySpeakAtDest)
		}
		// Presence, not emptiness, decides here: an empty texture or comment is the
		// admin "reset this user's content" operation and has to actually clear it.
		if us.Has(messages.UserStateSetTexture) {
			u.Texture = append([]byte(nil), us.Texture...)
		}
		if us.Has(messages.UserStateSetComment) {
			u.Comment = us.Comment
		}
		if us.Has(messages.UserStateSetPluginIdentity) {
			u.PluginIdentity = us.PluginIdentity
		}
		if us.Has(messages.UserStateSetPluginContext) {
			u.PluginContext = append([]byte(nil), us.PluginContext...)
		}
	})
	if !ok {
		return nil
	}

	// Suppress/priority-speaker recomputation from a channel move is echoed only
	// when it actually flipped (Server.cpp:2042-2055); if the client also asserted
	// one of these itself in the same message, applyAdminVoiceState above already
	// set the corresponding field, so this only ever adds what changed.
	if priorityChanged {
		us.PrioritySpeaker = false
		us.SetFields |= messages.UserStateSetPrioritySpeaker
	}
	if suppressChanged {
		us.Suppress = updated.Suppress
		us.SetFields |= messages.UserStateSetSuppress
	}

	// plugin_context/plugin_identity are applied to server state above but never
	// transmitted to clients (Mumble.proto), and never set murmur's bBroadcast on
	// their own (Messages.cpp:995-1007). Strip them before the broadcast gate.
	us.PluginContext = nil
	us.PluginIdentity = ""
	us.SetFields &^= pluginUserStateFields

	// A plugin-only (or otherwise non-broadcast) update must not emit a UserState
	// with just session/actor — that would be a presence-only no-op spam.
	if us.SetFields&broadcastUserStateFields == 0 {
		return nil
	}

	us.Session = targetSession
	us.Actor = c.SessionID()
	us.SetFields |= messages.UserStateSetSession | messages.UserStateSetActor

	s.Broadcast(0, protocol.MessageUserState, &us)

	if us.Has(messages.UserStateSetRecording) {
		s.broadcastRecordingAnnouncement(updated.Name, updated.Recording)
	}
	return nil
}

// version1_2_3 is Mumble 1.2.3 packed the same way Version.version_v2 packs
// major/minor/patch (research/mumble/src/Version.h: fromComponents).
const version1_2_3 = uint64(1)<<48 | uint64(2)<<32 | uint64(3)<<16

// broadcastRecordingAnnouncement sends the legacy "started/stopped recording"
// TextMessage only to clients older than 1.2.3 (Messages.cpp:1075); modern clients
// render their own line from the recording field on the UserState broadcast, so
// sending this to everyone would duplicate it.
func (s *Server) broadcastRecordingAnnouncement(name string, recording bool) {
	var text string
	if recording {
		text = fmt.Sprintf("User '%s' started recording", name)
	} else {
		text = fmt.Sprintf("User '%s' stopped recording", name)
	}
	tm := &messages.TextMessage{TreeID: []uint32{0}, Message: text}
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	for _, c := range s.conns {
		if c.ClientVersion() < version1_2_3 {
			_ = c.WriteMessage(protocol.MessageTextMessage, tm)
		}
	}
}

func (s *Server) handleCryptSetup(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.Crypt == nil {
		return nil
	}
	var cs messages.CryptSetup
	if len(payload) > 0 {
		if err := cs.Unmarshal(payload); err != nil {
			return err
		}
	}
	if len(cs.ClientNonce) > 0 {
		_ = c.Crypt.SetDecNonce(cs.ClientNonce)
	}
	if len(cs.ServerNonce) > 0 || (len(cs.Key) == 0 && len(cs.ClientNonce) == 0) {
		encNonce := c.Crypt.EncNonce()
		if len(encNonce) > 0 {
			_ = c.WriteMessage(protocol.MessageCryptSetup, &messages.CryptSetup{ServerNonce: encNonce})
		}
	}
	return nil
}

func (s *Server) handleChannelState(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var cs messages.ChannelState
	if err := cs.Unmarshal(payload); err != nil {
		return err
	}
	u, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	if cs.ChannelID == 0 {
		parent := cs.Parent
		if parent == 0 {
			parent = s.chans.RootID()
		}
		if !s.aclCheck(acl.SubjectOf(u), parent, mumble.PermissionMakeChannel) {
			_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: parent, Type: messages.DenyPermission, Reason: "Cannot create channel"})
			return nil
		}
		ch := s.chans.Create(parent, cs.Name, cs.Description, cs.Position, cs.Temporary, cs.MaxUsers)
		if ch == nil {
			_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyPermission, Reason: "Cannot create channel"})
			return nil
		}
		state := channelToState(ch)
		s.Broadcast(0, protocol.MessageChannelState, state)
		return nil
	}
	opts := channel.UpdateOpts{}
	if cs.Name != "" {
		opts.Name = &cs.Name
	}
	if cs.Description != "" {
		opts.Description = &cs.Description
	}
	if cs.Position != 0 {
		opts.Position = &cs.Position
	}
	if cs.MaxUsers != 0 {
		opts.MaxUsers = &cs.MaxUsers
	}
	opts.Temporary = &cs.Temporary
	if len(cs.Links) > 0 {
		opts.Links = cs.Links
	}
	if len(cs.LinksAdd) > 0 {
		opts.LinksAdd = cs.LinksAdd
	}
	if len(cs.LinksRemove) > 0 {
		opts.LinksRemove = cs.LinksRemove
	}
	if !s.aclCheck(acl.SubjectOf(u), cs.ChannelID, mumble.PermissionWrite) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: cs.ChannelID, Type: messages.DenyPermission})
		return nil
	}
	if s.chans.Update(cs.ChannelID, opts) {
		if ch, ok := s.chans.GetChannel(cs.ChannelID); ok {
			s.Broadcast(0, protocol.MessageChannelState, channelToState(ch))
		}
	}
	return nil
}

func (s *Server) handleChannelRemove(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var cr messages.ChannelRemove
	if err := cr.Unmarshal(payload); err != nil {
		return err
	}
	u, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	if !s.aclCheck(acl.SubjectOf(u), cr.ChannelID, mumble.PermissionWrite) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: cr.ChannelID, Type: messages.DenyPermission})
		return nil
	}
	ch, ok := s.chans.GetChannel(cr.ChannelID)
	if !ok {
		return nil
	}
	parentID := ch.ParentID
	if parentID == 0 {
		parentID = s.chans.RootID()
	}
	for _, mu := range s.users.SnapshotByChannel(cr.ChannelID) {
		sid := mu.SessionID
		// Resolved before UpdateUser — see maySpeak.
		maySpeak := s.maySpeak(acl.SubjectOf(mu), parentID)
		s.users.SetChannel(sid, parentID)
		var priorityChanged, suppressChanged bool
		updated, ok := s.users.UpdateUser(sid, func(u *mumble.User) {
			priorityChanged, suppressChanged = applyChannelEnterEffects(u, maySpeak)
		})
		if !ok {
			continue
		}
		// Minimal {session, channel_id} delta per moved user, plus suppress/priority
		// speaker only when userEnterChannel actually flipped them, matching murmur
		// rather than broadcasting a full snapshot.
		state := &messages.UserState{
			Session:   sid,
			ChannelID: parentID,
			SetFields: messages.UserStateSetSession | messages.UserStateSetChannelID,
		}
		if priorityChanged {
			state.SetFields |= messages.UserStateSetPrioritySpeaker
		}
		if suppressChanged {
			state.Suppress = updated.Suppress
			state.SetFields |= messages.UserStateSetSuppress
		}
		s.Broadcast(0, protocol.MessageUserState, state)
	}
	// The channel these users were in is going away, and they have all moved.
	s.invalidateACLCache()
	if s.chans.Remove(cr.ChannelID) {
		s.Broadcast(0, protocol.MessageChannelRemove, &cr)
	}
	s.UpdateChannelCrypto(parentID)
	return nil
}

func (s *Server) handleTextMessage(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var tm messages.TextMessage
	if err := tm.Unmarshal(payload); err != nil {
		return err
	}
	u, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	if !s.checkTextRateLimit(c.SessionID()) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyTextTooLong, Reason: "Rate limit exceeded"})
		return nil
	}
	// Murmur's isTextAllowed: reject anything over the advertised message_length
	// rather than relaying it (Messages.cpp, Server::isTextAllowed).
	if max := s.maxTextLength(); max > 0 && len(tm.Message) > max {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyTextTooLong})
		return nil
	}
	hasTarget := len(tm.Session) > 0 || len(tm.ChannelID) > 0 || len(tm.TreeID) > 0
	if hasTarget {
		for _, sid := range tm.Session {
			targetUser, ok := s.users.Snapshot(sid)
			if !ok {
				continue
			}
			if !s.aclCheck(acl.SubjectOf(u), targetUser.ChannelID, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
					ChannelID: targetUser.ChannelID, Type: messages.DenyPermission, Reason: "No text permission",
				})
				return nil
			}
		}
		for _, cid := range tm.ChannelID {
			if !s.aclCheck(acl.SubjectOf(u), cid, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: cid, Type: messages.DenyPermission, Reason: "No text permission"})
				return nil
			}
		}
		for _, tid := range tm.TreeID {
			if !s.aclCheck(acl.SubjectOf(u), tid, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: tid, Type: messages.DenyPermission, Reason: "No text permission"})
				return nil
			}
		}
	} else if !s.aclCheck(acl.SubjectOf(u), u.ChannelID, mumble.PermissionTextMessage) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: u.ChannelID, Type: messages.DenyPermission, Reason: "No text permission"})
		return nil
	}
	tm.Actor = c.SessionID()
	var recipients []uint32
	if len(tm.Session) > 0 {
		recipients = append(recipients, tm.Session...)
	}
	if len(tm.ChannelID) > 0 {
		for _, cid := range tm.ChannelID {
			recipients = append(recipients, s.users.SessionIDsInChannel(cid)...)
		}
	}
	if len(tm.TreeID) > 0 {
		for _, tid := range tm.TreeID {
			for _, cid := range s.chans.SubtreeIDs(tid) {
				recipients = append(recipients, s.users.SessionIDsInChannel(cid)...)
			}
		}
	}
	senderSession := c.SessionID()
	seen := make(map[uint32]bool)
	for _, sid := range recipients {
		if sid == senderSession {
			continue
		}
		if !seen[sid] {
			seen[sid] = true
			if conn := s.conn(sid); conn != nil {
				_ = conn.WriteMessage(protocol.MessageTextMessage, &tm)
			}
		}
	}
	return nil
}

func (s *Server) checkTextRateLimit(sessionID uint32) bool {
	return rateLimitAllows(&s.textRateLimiter, sessionID, maxTextMessagesPerSecond)
}

// checkUserStateRateLimit throttles self-targeted UserState (murmur's RATELIMIT at
// Messages.cpp:790). The budget is well above what a human toggling mute can
// produce, so it only bites on a client stuck in an echo loop.
func (s *Server) checkUserStateRateLimit(sessionID uint32) bool {
	return rateLimitAllows(&s.userStateRateLimiter, sessionID, maxUserStatesPerSecond)
}

const (
	maxTextMessagesPerSecond = 30
	maxUserStatesPerSecond   = 10
)

func (s *Server) conn(sessionID uint32) *connection.Conn {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	return s.conns[sessionID]
}

func (s *Server) handleVoiceTarget(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var vt messages.VoiceTarget
	if err := vt.Unmarshal(payload); err != nil {
		return err
	}
	if vt.ID == 0 || vt.ID > 30 {
		return nil
	}
	var recipients []uint32
	seen := make(map[uint32]bool)
	for _, t := range vt.Targets {
		for _, sid := range t.Session {
			if !seen[sid] {
				seen[sid] = true
				recipients = append(recipients, sid)
			}
		}
		addChannel := func(channelID uint32) {
			for _, sid := range s.users.SessionIDsInChannel(channelID) {
				if !seen[sid] {
					seen[sid] = true
					recipients = append(recipients, sid)
				}
			}
		}
		addChannel(t.ChannelID)
		if t.Children {
			cid := t.ChannelID
			if cid == 0 {
				if cur, ok := s.users.ChannelID(c.SessionID()); ok {
					cid = cur
				}
			}
			for _, subID := range s.chans.SubtreeIDs(cid) {
				addChannel(subID)
			}
		}
		if t.Links {
			if ch, ok := s.chans.GetChannel(t.ChannelID); ok {
				for _, lid := range ch.Links {
					addChannel(lid)
				}
			}
		}
	}
	merged := make(map[uint8][]uint32)
	if v, ok := s.voiceTargets.Load(c.SessionID()); ok {
		for k, val := range v.(map[uint8][]uint32) {
			merged[k] = val
		}
	}
	merged[uint8(vt.ID)] = recipients
	s.voiceTargets.Store(c.SessionID(), merged)
	return nil
}

var udpTunnelCount sync.Map // session -> *uint64

func (s *Server) handleUDPTunnel(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	state := c.State()
	sid := c.SessionID()
	voiceDebug := s.voiceDebug.Load()
	if state != connection.StateActive || s.router == nil {
		if voiceDebug {
			slog.Warn("[VOICE-DEBUG] UDPTunnel DROPPED: inactive/no-router",
				"session", sid, "state", state, "router_nil", s.router == nil, "payload_len", len(payload))
		}
		return nil
	}
	if len(payload) < 1 {
		if voiceDebug {
			slog.Warn("[VOICE-DEBUG] UDPTunnel DROPPED: empty payload", "session", sid)
		}
		return nil
	}

	// Rate-limit logging: first 5 packets, then every 50th (only when voice_debug enabled)
	countPtr, _ := udpTunnelCount.LoadOrStore(sid, new(uint64))
	count := countPtr.(*uint64)
	*count++
	shouldLog := voiceDebug && (*count <= 5 || *count%50 == 0)

	codecType := (payload[0] >> 5) & 0x07
	target := uint8(payload[0] & 0x1F)
	if shouldLog {
		slog.Info("[VOICE-DEBUG] UDPTunnel received",
			"session", sid, "pkt_num", *count, "payload_len", len(payload),
			"header_byte", fmt.Sprintf("0x%02x", payload[0]),
			"codec_type", codecType, "target", target,
			"first_bytes", hex.EncodeToString(payload[:min(len(payload), 16)]))

		canSpeak := s.canSenderSpeak(sid)
		u, uOk := s.users.Snapshot(sid)
		if uOk {
			slog.Info("[VOICE-DEBUG] sender info",
				"session", sid, "user_id", u.UserID, "channel_id", u.ChannelID,
				"mute", u.Mute, "self_mute", u.SelfMute, "can_speak", canSpeak)
		} else {
			slog.Warn("[VOICE-DEBUG] sender NOT FOUND in user manager", "session", sid)
		}
	}

	outgoing := rewriteAudioPacket(sid, payload)
	if shouldLog {
		slog.Info("[VOICE-DEBUG] rewritten packet",
			"session", sid, "in_len", len(payload), "out_len", len(outgoing),
			"out_first_bytes", hex.EncodeToString(outgoing[:min(len(outgoing), 20)]))
	}

	err := s.router.Route(sid, target, outgoing)
	if err != nil && voiceDebug {
		slog.Error("[VOICE-DEBUG] Route returned error", "session", sid, "err", err)
	}
	return nil
}

func (s *Server) handleBanList(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var bl messages.BanList
	if len(payload) > 0 {
		_ = bl.Unmarshal(payload)
	}
	if !bl.Query && len(bl.Bans) > 0 {
		u, ok := s.users.Snapshot(c.SessionID())
		if !ok {
			return nil
		}
		if !s.aclCheck(acl.SubjectOf(u), s.chans.RootID(), mumble.PermissionBan) {
			_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyPermission, Reason: "No ban permission"})
			return nil
		}
		if err := s.bans.Replace(bl.Bans); err != nil {
			slog.Debug("ban list replace failed", "err", err)
			return nil
		}
	}
	bl.Bans = s.bans.List()
	bl.Query = false
	_ = c.WriteMessage(protocol.MessageBanList, &bl)
	return nil
}

func (s *Server) handleACL(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var aclMsg messages.ACL
	if len(payload) > 0 {
		_ = aclMsg.Unmarshal(payload)
	}
	if aclMsg.Query {
		aclMsg.Query = false
		aclMsg.InheritACLs = true
		_ = c.WriteMessage(protocol.MessageACL, &aclMsg)
	}
	return nil
}

func (s *Server) handlePermissionQuery(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var pq messages.PermissionQuery
	if len(payload) > 0 {
		_ = pq.Unmarshal(payload)
	}
	u, ok := s.users.Snapshot(c.SessionID())
	if !ok {
		return nil
	}
	// ChannelID 0 = root channel (per Mumble protocol)
	cid := pq.ChannelID
	pq.Permissions = uint32(s.aclPermissions(acl.SubjectOf(u), cid))
	_ = c.WriteMessage(protocol.MessagePermissionQuery, &pq)
	return nil
}

func (s *Server) handleRequestBlob(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var rb messages.RequestBlob
	if len(payload) > 0 {
		_ = rb.Unmarshal(payload)
	}
	s.sendRequestedBlobs(c, &rb)
	return nil
}

func (s *Server) sendRequestedBlobs(c *connection.Conn, rb *messages.RequestBlob) {
	for _, sid := range rb.SessionTexture {
		if u, ok := s.users.Snapshot(sid); ok && len(u.Texture) > 0 {
			_ = c.WriteMessage(protocol.MessageUserState, &messages.UserState{
				Session:   sid,
				Texture:   u.Texture,
				SetFields: messages.UserStateSetSession | messages.UserStateSetTexture,
			})
		}
	}
	for _, sid := range rb.SessionComment {
		if u, ok := s.users.Snapshot(sid); ok && u.Comment != "" {
			_ = c.WriteMessage(protocol.MessageUserState, &messages.UserState{
				Session:   sid,
				Comment:   u.Comment,
				SetFields: messages.UserStateSetSession | messages.UserStateSetComment,
			})
		}
	}
	for _, cid := range rb.ChannelDescription {
		if ch, ok := s.chans.GetChannel(cid); ok && ch.Description != "" {
			_ = c.WriteMessage(protocol.MessageChannelState, &messages.ChannelState{ChannelID: cid, Description: ch.Description})
		}
	}
}

func (s *Server) handleUserStats(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var req messages.UserStats
	if len(payload) > 0 {
		_ = req.Unmarshal(payload)
	}
	if req.Session == 0 {
		req.Session = c.SessionID()
	}
	if s.users.Exists(req.Session) {
		_ = c.WriteMessage(protocol.MessageUserStats, &req)
	}
	return nil
}

func (s *Server) handleQueryUsers(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var qu messages.QueryUsers
	if len(payload) > 0 {
		_ = qu.Unmarshal(payload)
	}
	resp := messages.QueryUsers{}
	for _, sid := range qu.IDs {
		if u, ok := s.users.Snapshot(sid); ok {
			resp.Names = append(resp.Names, u.Name)
		} else {
			resp.Names = append(resp.Names, "")
		}
	}
	for _, name := range qu.Names {
		if u, ok := s.users.SnapshotByName(name); ok {
			resp.IDs = append(resp.IDs, u.SessionID)
		} else {
			resp.IDs = append(resp.IDs, 0)
		}
	}
	_ = c.WriteMessage(protocol.MessageQueryUsers, &resp)
	return nil
}

func (s *Server) handleUserList(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	_ = ctx
	_ = payload
	return nil
}

func (s *Server) handleContextActionModify(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	_ = ctx
	_ = payload
	return nil
}

func (s *Server) handleContextAction(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	_ = ctx
	_ = payload
	return nil
}

func (s *Server) handlePluginDataTransmission(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	_ = ctx
	_ = payload
	return nil
}
