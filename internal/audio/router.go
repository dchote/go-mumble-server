package audio

// Router forwards voice packets to appropriate recipients.
type Router struct{}

// Route determines recipients and forwards the packet.
func (r *Router) Route(sessionID uint32, voiceTarget uint8, payload []byte) error {
	// TODO: implement; see docs/patterns/audio-pipeline-pattern.md
	return nil
}
