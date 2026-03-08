package audio

// Codec IDs for Mumble audio. See docs/protocol/voice-data.md.
const (
	CodecCELTAlpha = 0
	CodecSpeex     = 2 // Deprecated
	CodecCELTBeta  = 3
	CodecOpus      = 4 // Preferred
)
