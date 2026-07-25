package audio

import (
	"log/slog"
	"sync"
	"sync/atomic"
)

// RecipientSender sends an audio packet to a recipient.
type RecipientSender interface {
	SendAudio(sessionID uint32, packet []byte) error
}

// RouterConfig configures the audio router.
type RouterConfig struct {
	Sender          RecipientSender
	GetChan         func(sessionID uint32) uint32
	GetUsersInChan  func(channelID uint32) []uint32
	GetVoiceTarget  func(sessionID uint32, targetID uint8) []uint32
	GetLinkedChans  func(channelID uint32) []uint32
	FilterRecipient func(senderSessionID, recipientSessionID uint32) bool
	CanSenderSpeak  func(senderSessionID uint32) bool
	VoiceDebug      bool
}

// Router forwards voice packets to appropriate recipients.
type Router struct {
	mu     sync.RWMutex
	config RouterConfig
}

// NewRouterWithConfig creates an audio router with full configuration.
func NewRouterWithConfig(cfg RouterConfig) *Router {
	return &Router{config: cfg}
}

// SetVoiceDebug updates the voice debug flag for runtime toggling.
func (r *Router) SetVoiceDebug(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config.VoiceDebug = enabled
}

var routeLogCount atomic.Uint64

// Route determines recipients and forwards the decrypted packet.
func (r *Router) Route(senderSessionID uint32, voiceTarget uint8, decryptedPacket []byte) error {
	r.mu.RLock()
	cfg := r.config
	voiceDebug := cfg.VoiceDebug
	r.mu.RUnlock()
	if cfg.Sender == nil {
		if voiceDebug {
			slog.Warn("[VOICE-DEBUG] Route: no Sender configured")
		}
		return nil
	}

	n := routeLogCount.Add(1)
	shouldLog := voiceDebug && (n <= 5 || n%50 == 0)

	var recipients []uint32
	switch voiceTarget {
	case 31:
		recipients = []uint32{senderSessionID}
	case 0:
		if cfg.GetChan != nil && cfg.GetUsersInChan != nil {
			if cfg.CanSenderSpeak != nil && !cfg.CanSenderSpeak(senderSessionID) {
				if voiceDebug {
					slog.Warn("[VOICE-DEBUG] Route: CanSenderSpeak=false, dropping",
						"sender", senderSessionID)
				}
				return nil
			}
			ch := cfg.GetChan(senderSessionID)
			channelIDs := []uint32{ch}
			if cfg.GetLinkedChans != nil {
				channelIDs = append(channelIDs, cfg.GetLinkedChans(ch)...)
			}
			seen := make(map[uint32]bool)
			for _, cid := range channelIDs {
				usersInChan := cfg.GetUsersInChan(cid)
				if shouldLog {
					slog.Info("[VOICE-DEBUG] Route: channel scan",
						"sender", senderSessionID, "channel", cid, "users_in_channel", usersInChan)
				}
				for _, sid := range usersInChan {
					if sid != senderSessionID && !seen[sid] {
						seen[sid] = true
						filtered := false
						if cfg.FilterRecipient != nil && !cfg.FilterRecipient(senderSessionID, sid) {
							filtered = true
						}
						if !filtered {
							recipients = append(recipients, sid)
						} else if shouldLog {
							slog.Info("[VOICE-DEBUG] Route: recipient filtered out",
								"sender", senderSessionID, "recipient", sid)
						}
					}
				}
			}
		} else if voiceDebug {
			slog.Warn("[VOICE-DEBUG] Route: GetChan or GetUsersInChan is nil")
		}
	default:
		if voiceTarget >= 1 && voiceTarget <= 30 && cfg.GetVoiceTarget != nil {
			recipients = cfg.GetVoiceTarget(senderSessionID, voiceTarget)
		}
	}
	if shouldLog || (voiceDebug && len(recipients) == 0) {
		slog.Info("[VOICE-DEBUG] Route: forwarding",
			"sender", senderSessionID, "target", voiceTarget,
			"recipient_count", len(recipients), "recipients", recipients,
			"pkt_len", len(decryptedPacket))
	}
	for _, sid := range recipients {
		_ = cfg.Sender.SendAudio(sid, decryptedPacket)
	}
	return nil
}
