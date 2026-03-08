package audio

import (
	"sync"
)

// RecipientSender sends an audio packet to a recipient.
type RecipientSender interface {
	SendAudio(sessionID uint32, packet []byte) error
}

// RouterConfig configures the audio router.
type RouterConfig struct {
	Sender           RecipientSender
	GetChan          func(sessionID uint32) uint32
	GetUsersInChan   func(channelID uint32) []uint32
	GetVoiceTarget   func(sessionID uint32, targetID uint8) []uint32
	GetLinkedChans   func(channelID uint32) []uint32
	FilterRecipient  func(senderSessionID, recipientSessionID uint32) bool
	CanSenderSpeak   func(senderSessionID uint32) bool
}

// Router forwards voice packets to appropriate recipients.
type Router struct {
	mu     sync.RWMutex
	config RouterConfig
}

// NewRouter creates an audio router.
func NewRouter(sender RecipientSender, getChan func(uint32) uint32, getUsersInChan func(uint32) []uint32, getVoiceTarget func(uint32, uint8) []uint32) *Router {
	return &Router{
		config: RouterConfig{
			Sender:         sender,
			GetChan:        getChan,
			GetUsersInChan: getUsersInChan,
			GetVoiceTarget: getVoiceTarget,
		},
	}
}

// NewRouterWithConfig creates an audio router with full configuration.
func NewRouterWithConfig(cfg RouterConfig) *Router {
	return &Router{config: cfg}
}

// Route determines recipients and forwards the decrypted packet.
func (r *Router) Route(senderSessionID uint32, voiceTarget uint8, decryptedPacket []byte) error {
	cfg := r.config
	if cfg.Sender == nil {
		return nil
	}
	var recipients []uint32
	switch voiceTarget {
	case 31:
		recipients = []uint32{senderSessionID}
	case 0:
		if cfg.GetChan != nil && cfg.GetUsersInChan != nil {
			if cfg.CanSenderSpeak != nil && !cfg.CanSenderSpeak(senderSessionID) {
				return nil
			}
			ch := cfg.GetChan(senderSessionID)
			channelIDs := []uint32{ch}
			if cfg.GetLinkedChans != nil {
				channelIDs = append(channelIDs, cfg.GetLinkedChans(ch)...)
			}
			seen := make(map[uint32]bool)
			for _, cid := range channelIDs {
				for _, sid := range cfg.GetUsersInChan(cid) {
					if sid != senderSessionID && !seen[sid] {
						seen[sid] = true
						if cfg.FilterRecipient == nil || cfg.FilterRecipient(senderSessionID, sid) {
							recipients = append(recipients, sid)
						}
					}
				}
			}
		}
	default:
		if voiceTarget >= 1 && voiceTarget <= 30 && cfg.GetVoiceTarget != nil {
			recipients = cfg.GetVoiceTarget(senderSessionID, voiceTarget)
		}
	}
	for _, sid := range recipients {
		_ = cfg.Sender.SendAudio(sid, decryptedPacket)
	}
	return nil
}
