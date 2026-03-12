package mumble

import (
	"crypto/rand"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"sync"
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
	textRateLimiter sync.Map // session -> *textRateState
	channelCryptoMu sync.RWMutex
	channelCrypto   map[uint32]string // channelID -> "legacy"|"lite"|"secure"|"mixed"|""
	router          *audio.Router
	udpConn         net.PacketConn
}

type textRateState struct {
	mu       sync.Mutex
	count    int
	windowAt time.Time
}

// HandleUDP processes an incoming UDP voice or ping packet.
func (s *Server) HandleUDP(addr net.Addr, data []byte) {
	if s.udpConn == nil {
		return
	}
	plain := make([]byte, len(data)+256)
	var senderSession uint32
	var senderCrypt *crypto.CryptState

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
			} else {
				// Cached address no longer valid (NAT rebinding); clear and fall through.
				s.sessionByAddr.Delete(addrKey)
			}
		}
	}

	// Fallback: trial decrypt for unmapped addresses (first packet from a new client).
	if senderSession == 0 {
		s.connMu.RLock()
		for _, mode := range []crypto.Mode{crypto.ModeSecure, crypto.ModeLegacy, crypto.ModeLite} {
			for sid, c := range s.conns {
				if c.Crypt == nil || c.State() != connection.StateActive || c.Crypt.Mode() != mode {
					continue
				}
				if err := c.Crypt.Decrypt(plain, data); err == nil {
					senderSession = sid
					senderCrypt = c.Crypt
					plain = plain[:len(data)-c.Crypt.Overhead()]
					goto found
				}
			}
		}
	found:
		s.connMu.RUnlock()
	}

	if senderSession == 0 {
		return
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
		u, ok := s.users.GetUser(senderSession)
		if ok && s.channelHasMixedCrypto(u.ChannelID) {
			return
		}
		enc := make([]byte, len(plain)+senderCrypt.Overhead())
		if err := senderCrypt.Encrypt(enc, plain); err == nil {
			s.udpConn.WriteTo(enc, addr)
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
		return nil
	}

	// Try UDP first if recipient has a known UDP address,
	// but force TCP tunnel when the channel has mixed crypto modes.
	if recipientAddr != nil && c.Crypt != nil && s.udpConn != nil {
		u, uOk := s.users.GetUser(sessionID)
		if !uOk || !s.channelHasMixedCrypto(u.ChannelID) {
			addr := recipientAddr.(net.Addr)
			overhead := c.Crypt.Overhead()
			enc := make([]byte, len(packet)+overhead)
			if err := c.Crypt.Encrypt(enc, packet); err == nil {
				_, err := s.udpConn.WriteTo(enc, addr)
				return err
			}
		}
	}

	// TCP fallback: wrap as UDPTunnel message
	return c.WriteRaw(protocol.MessageUDPTunnel, packet)
}

func (s *Server) canSenderSpeak(sessionID uint32) bool {
	u, ok := s.users.GetUser(sessionID)
	if !ok {
		return false
	}
	return !u.Mute && s.acl.Check(u.UserID, u.ChannelID, mumble.PermissionSpeak)
}

func (s *Server) audioFilterRecipient(senderSessionID, recipientSessionID uint32) bool {
	u, ok := s.users.GetUser(recipientSessionID)
	if !ok {
		return false
	}
	return !u.Deaf && !u.SelfDeaf
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
	s.router = audio.NewRouterWithConfig(audio.RouterConfig{
		Sender: s,
		GetChan: func(sid uint32) uint32 {
			u, ok := s.users.GetUser(sid)
			if !ok {
				return 0
			}
			return u.ChannelID
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

// BanManager returns the ban manager (for cache invalidation when bans change via REST).
func (s *Server) BanManager() *ban.Manager {
	return s.bans
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
	s.chans.CleanEmptyTempChannels(func(cid uint32) bool {
		return len(s.users.ListByChannel(cid)) > 0
	})
}

// KickSession kicks a user (server-initiated, e.g. from REST API). Actor 0 = server.
func (s *Server) KickSession(sessionID uint32, reason string) bool {
	u, ok := s.users.GetUser(sessionID)
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
		targetConn.Close()
	}
	slog.Info("User kicked via REST", "session", sessionID, "name", u.Name, "reason", reason)
	return true
}

// MuteSession sets server mute state for a user.
func (s *Server) MuteSession(sessionID uint32, mute bool) bool {
	u, ok := s.users.GetUser(sessionID)
	if !ok {
		return false
	}
	u.Mute = mute
	state := userToState(u)
	state.Actor = 0
	s.Broadcast(0, protocol.MessageUserState, state)
	slog.Info("User mute changed via REST", "session", sessionID, "name", u.Name, "mute", mute)
	return true
}

// BanAndKickSession bans the user by IP and kicks them.
func (s *Server) BanAndKickSession(sessionID uint32, reason string) bool {
	u, ok := s.users.GetUser(sessionID)
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
		newBan := messages.BanEntry{
			Hash:   strings.ToLower(u.CertHash),
			Name:   u.Name,
			Reason: reason,
			Start:  time.Now().Format(time.RFC3339),
		}
		existing = append(existing, newBan)
		_ = s.bans.Replace(existing)
	} else if ip != nil {
		mask := uint32(32)
		if ip4 := ip.To4(); ip4 != nil {
			ip = ip4
		} else {
			mask = 128
		}
		newBan := messages.BanEntry{
			Address: ip,
			Mask:    mask,
			Name:    u.Name,
			Reason:  reason,
			Start:   time.Now().Format(time.RFC3339),
		}
		existing = append(existing, newBan)
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
		targetConn.Close()
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
	}
	return nil
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
	if _, ok := s.users.GetByName(authMsg.Username); ok {
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
	defaultChan := uint32(s.chans.RootID())
	if s.cfg.DefaultChannel > 0 {
		defaultChan = uint32(s.cfg.DefaultChannel)
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
	u := &mumble.User{
		SessionID:    0,
		UserID:       userID,
		ChannelID:    defaultChan,
		Name:         authMsg.Username,
		AccessTokens: authMsg.Tokens,
	}
	if addr != nil {
		if host, _, err := net.SplitHostPort(addr.String()); err == nil {
			u.Address = host
		} else {
			u.Address = addr.String()
		}
	}
	u.CertHash = certHash
	if !s.users.Add(u) {
		return s.sendReject(c, messages.RejectServerFull, "Server full")
	}
	c.SetSessionID(u.SessionID)
	c.SetUser(u.Name, u.UserID, u.ChannelID)
	c.SetActive()
	s.sendSync(c, u)
	s.RegisterConn(u.SessionID, c)
	s.UpdateChannelCrypto(u.ChannelID)
	s.Broadcast(u.SessionID, protocol.MessageUserState, userToState(u))
	slog.Info("Mumble client authenticated", "user", u.Name, "session", u.SessionID, "channel", u.ChannelID)
	return nil
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

func (s *Server) sendSync(c *connection.Conn, u *mumble.User) {
	mode := s.negotiateCryptoMode(c)
	u.CryptoMode = cryptoModeString(mode)
	c.Crypt = crypto.NewCryptState(mode)
	key, encNonce, decNonce := s.generateCryptSetup(mode)
	c.Crypt.SetKey(key, encNonce, decNonce)
	_ = c.WriteMessage(protocol.MessageCryptSetup, &messages.CryptSetup{
		Key:         key,
		ClientNonce: decNonce, // server's decrypt nonce = client's encrypt nonce
		ServerNonce: encNonce, // server's encrypt nonce = client's decrypt nonce
	})
	_ = c.WriteMessage(protocol.MessageCodecVersion, &messages.CodecVersion{Opus: true})
	for _, ch := range s.chans.GetTree() {
		cs := channelToState(ch)
		_ = c.WriteMessage(protocol.MessageChannelState, cs)
	}
	_ = c.WriteMessage(protocol.MessageUserState, userToState(u))
	for _, ou := range s.users.ListAll() {
		if ou.SessionID != u.SessionID {
			_ = c.WriteMessage(protocol.MessageUserState, userToState(ou))
		}
	}
	perms := uint64(s.acl.EffectivePermissions(u.UserID, s.chans.RootID()))
	_ = c.WriteMessage(protocol.MessageServerSync, &messages.ServerSync{
		Session:      u.SessionID,
		MaxBandwidth: uint32(s.cfg.MaxBandwidth),
		WelcomeText:  s.cfg.WelcomeText,
		Permissions:  perms,
	})
	_ = c.WriteMessage(protocol.MessageServerConfig, &messages.ServerConfig{
		MaxBandwidth:  uint32(s.cfg.MaxBandwidth),
		WelcomeText:   s.cfg.WelcomeText,
		AllowHTML:     true,
		MessageLength: 5000,
		MaxUsers:      uint32(s.cfg.MaxUsers),
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

func userToState(u *mumble.User) *messages.UserState {
	return &messages.UserState{
		Session:   u.SessionID,
		UserID:    u.UserID,
		Name:      u.Name,
		ChannelID: u.ChannelID,
		Mute:      u.Mute,
		Deaf:      u.Deaf,
		SelfMute:  u.SelfMute,
		SelfDeaf:  u.SelfDeaf,
		Texture:   u.Texture,
		Comment:   u.Comment,
		// PluginIdentity and PluginContext are not transmitted to clients per Mumble proto.
	}
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
	sender, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	target, ok := s.users.GetUser(ur.Session)
	if !ok {
		return nil
	}
	perm := mumble.PermissionKick
	if ur.Ban {
		perm = mumble.PermissionBan
	}
	if !s.acl.Check(sender.UserID, s.chans.RootID(), perm) {
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
			mask := uint32(32)
			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
			} else {
				mask = 128
			}
			existing := s.bans.List()
			newBan := messages.BanEntry{
				Address: ip,
				Mask:    mask,
				Name:    target.Name,
				Reason:  ur.Reason,
				Start:   time.Now().Format(time.RFC3339),
			}
			existing = append(existing, newBan)
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
		targetConn.Close()
	}
	return nil
}

func (s *Server) handleUserState(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive {
		return nil
	}
	var us messages.UserState
	if err := us.Unmarshal(payload); err != nil {
		return err
	}
	sender, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	// Target: self (Session absent or same as sender) vs another user (admin ops)
	targetSession := us.Session
	if us.SetFields&messages.UserStateSetSession == 0 || targetSession == c.SessionID() {
		targetSession = c.SessionID()
	}
	u, ok := s.users.GetUser(targetSession)
	if !ok {
		return nil
	}
	isAdminOp := targetSession != c.SessionID()

	if us.SetFields&messages.UserStateSetChannelID != 0 {
		if _, exists := s.chans.GetChannel(us.ChannelID); !exists {
			return nil
		}
		if isAdminOp {
			if !s.acl.Check(sender.UserID, u.ChannelID, mumble.PermissionMove) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
					ChannelID: u.ChannelID, Type: messages.DenyPermission, Reason: "No move permission",
				})
				return nil
			}
		} else if !s.acl.Check(u.UserID, us.ChannelID, mumble.PermissionEnter) {
			_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
				ChannelID: us.ChannelID, Type: messages.DenyPermission, Reason: "Permission denied",
			})
			return nil
		}
		oldChannelID := u.ChannelID
		u.ChannelID = us.ChannelID
		s.users.SetChannel(targetSession, us.ChannelID)
		s.acl.InvalidateCache() // @in/@out depend on channel
		s.UpdateChannelCrypto(oldChannelID)
		s.UpdateChannelCrypto(us.ChannelID)
		if targetConn := s.conn(targetSession); targetConn != nil {
			targetConn.SetUser(u.Name, u.UserID, u.ChannelID)
			if targetSession == c.SessionID() {
				perms := s.acl.EffectivePermissions(u.UserID, us.ChannelID)
				_ = targetConn.WriteMessage(protocol.MessagePermissionQuery, &messages.PermissionQuery{
					ChannelID:   us.ChannelID,
					Permissions: uint32(perms),
				})
			}
		}
	}
	if us.SetFields&messages.UserStateSetSelfMute != 0 {
		u.SelfMute = us.SelfMute
	}
	if us.SetFields&messages.UserStateSetSelfDeaf != 0 {
		u.SelfDeaf = us.SelfDeaf
	}
	if us.SetFields&messages.UserStateSetMute != 0 {
		if isAdminOp && !s.acl.Check(sender.UserID, u.ChannelID, mumble.PermissionMuteDeafen) {
			return nil
		}
		u.Mute = us.Mute
	}
	if us.SetFields&messages.UserStateSetDeaf != 0 {
		if isAdminOp && !s.acl.Check(sender.UserID, u.ChannelID, mumble.PermissionMuteDeafen) {
			return nil
		}
		u.Deaf = us.Deaf
	}
	if us.SetFields&messages.UserStateSetTexture != 0 && len(us.Texture) > 0 {
		u.Texture = us.Texture
	}
	if us.SetFields&messages.UserStateSetComment != 0 {
		u.Comment = us.Comment
	}
	if us.SetFields&messages.UserStateSetPluginIdentity != 0 {
		u.PluginIdentity = us.PluginIdentity
	}
	if us.SetFields&messages.UserStateSetPluginContext != 0 && len(us.PluginContext) > 0 {
		u.PluginContext = us.PluginContext
	}
	state := userToState(u)
	state.Actor = c.SessionID()
	s.Broadcast(0, protocol.MessageUserState, state)
	return nil
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
	u, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	if cs.ChannelID == 0 {
		parent := cs.Parent
		if parent == 0 {
			parent = s.chans.RootID()
		}
		if !s.acl.Check(u.UserID, parent, mumble.PermissionMakeChannel) {
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
	if !s.acl.Check(u.UserID, cs.ChannelID, mumble.PermissionWrite) {
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
	u, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	if !s.acl.Check(u.UserID, cr.ChannelID, mumble.PermissionWrite) {
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
	for _, mu := range s.users.ListByChannel(cr.ChannelID) {
		s.users.SetChannel(mu.SessionID, parentID)
		mu.ChannelID = parentID
		if conn := s.conn(mu.SessionID); conn != nil {
			conn.SetUser(mu.Name, mu.UserID, parentID)
		}
		s.Broadcast(0, protocol.MessageUserState, userToState(mu))
	}
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
	u, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	if !s.checkTextRateLimit(c.SessionID()) {
		_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{Type: messages.DenyTextTooLong, Reason: "Rate limit exceeded"})
		return nil
	}
	hasTarget := len(tm.Session) > 0 || len(tm.ChannelID) > 0 || len(tm.TreeID) > 0
	if hasTarget {
		for _, sid := range tm.Session {
			targetUser, ok := s.users.GetUser(sid)
			if !ok {
				continue
			}
			if !s.acl.Check(u.UserID, targetUser.ChannelID, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{
					ChannelID: targetUser.ChannelID, Type: messages.DenyPermission, Reason: "No text permission",
				})
				return nil
			}
		}
		for _, cid := range tm.ChannelID {
			if !s.acl.Check(u.UserID, cid, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: cid, Type: messages.DenyPermission, Reason: "No text permission"})
				return nil
			}
		}
		for _, tid := range tm.TreeID {
			if !s.acl.Check(u.UserID, tid, mumble.PermissionTextMessage) {
				_ = c.WriteMessage(protocol.MessagePermissionDenied, &messages.PermissionDenied{ChannelID: tid, Type: messages.DenyPermission, Reason: "No text permission"})
				return nil
			}
		}
	} else if !s.acl.Check(u.UserID, u.ChannelID, mumble.PermissionTextMessage) {
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
			for _, mu := range s.users.ListByChannel(cid) {
				recipients = append(recipients, mu.SessionID)
			}
		}
	}
	if len(tm.TreeID) > 0 {
		for _, tid := range tm.TreeID {
			for _, cid := range s.chans.SubtreeIDs(tid) {
				for _, mu := range s.users.ListByChannel(cid) {
					recipients = append(recipients, mu.SessionID)
				}
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
	v, _ := s.textRateLimiter.LoadOrStore(sessionID, &textRateState{})
	rs := v.(*textRateState)
	rs.mu.Lock()
	defer rs.mu.Unlock()
	now := time.Now()
	if now.Sub(rs.windowAt) > time.Second {
		rs.windowAt = now
		rs.count = 0
	}
	rs.count++
	return rs.count <= 30
}

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
		for _, u := range s.users.ListByChannel(t.ChannelID) {
			if !seen[u.SessionID] {
				seen[u.SessionID] = true
				recipients = append(recipients, u.SessionID)
			}
		}
		if t.Children {
			cid := t.ChannelID
			if cid == 0 {
				if u, ok := s.users.GetUser(c.SessionID()); ok {
					cid = u.ChannelID
				}
			}
			for _, subID := range s.chans.SubtreeIDs(cid) {
				for _, u := range s.users.ListByChannel(subID) {
					if !seen[u.SessionID] {
						seen[u.SessionID] = true
						recipients = append(recipients, u.SessionID)
					}
				}
			}
		}
		if t.Links {
			if ch, ok := s.chans.GetChannel(t.ChannelID); ok {
				for _, lid := range ch.Links {
					for _, u := range s.users.ListByChannel(lid) {
						if !seen[u.SessionID] {
							seen[u.SessionID] = true
							recipients = append(recipients, u.SessionID)
						}
					}
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

func (s *Server) handleUDPTunnel(msgType protocol.MessageType, payload []byte, ctx interface{}) error {
	c := ctx.(*connection.Conn)
	if c.State() != connection.StateActive || s.router == nil {
		return nil
	}
	if len(payload) < 1 {
		return nil
	}
	target := uint8(payload[0] & 0x1F)
	sid := c.SessionID()
	outgoing := rewriteAudioPacket(sid, payload)
	_ = s.router.Route(sid, target, outgoing)
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
		u, ok := s.users.GetUser(c.SessionID())
		if !ok {
			return nil
		}
		if !s.acl.Check(u.UserID, s.chans.RootID(), mumble.PermissionBan) {
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
	u, ok := s.users.GetUser(c.SessionID())
	if !ok {
		return nil
	}
	// ChannelID 0 = root channel (per Mumble protocol)
	cid := pq.ChannelID
	pq.Permissions = uint32(s.acl.EffectivePermissions(u.UserID, cid))
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
		if u, ok := s.users.GetUser(sid); ok && len(u.Texture) > 0 {
			_ = c.WriteMessage(protocol.MessageUserState, &messages.UserState{Session: sid, Texture: u.Texture})
		}
	}
	for _, sid := range rb.SessionComment {
		if u, ok := s.users.GetUser(sid); ok && u.Comment != "" {
			_ = c.WriteMessage(protocol.MessageUserState, &messages.UserState{Session: sid, Comment: u.Comment})
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
	if _, ok := s.users.GetUser(req.Session); ok {
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
		if u, ok := s.users.GetUser(sid); ok {
			resp.Names = append(resp.Names, u.Name)
		} else {
			resp.Names = append(resp.Names, "")
		}
	}
	for _, name := range qu.Names {
		if u, ok := s.users.GetByName(name); ok {
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
