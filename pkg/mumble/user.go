package mumble

// VoiceState holds the flags that gate audio routing for a session. These are
// written from the control (TCP) goroutine and read from the voice (UDP) goroutine,
// so they are grouped to be copied as a unit under the user manager's lock.
type VoiceState struct {
	// Mute, Deaf and Suppress are imposed by the server or an administrator.
	Mute     bool
	Deaf     bool
	Suppress bool
	// SelfMute and SelfDeaf are requested by the client. SelfDeaf implies SelfMute.
	SelfMute bool
	SelfDeaf bool

	PrioritySpeaker bool
	Recording       bool
}

// User represents a connected Mumble user (client session).
type User struct {
	SessionID    uint32
	UserID       uint32
	ChannelID    uint32
	Name         string
	AccessTokens []string // from Authenticate message, for token groups
	VoiceState
	PluginIdentity string
	PluginContext  []byte
	Texture        []byte
	Comment        string
	// Address is the client's remote address (host, no port) for display.
	Address string
	// Ping is the client's reported TCP ping in milliseconds (from last Ping message).
	Ping float32
	// CertHash is the SHA-1 hex fingerprint of the client's TLS certificate (lowercase).
	// Empty if the client did not present a certificate.
	CertHash string
	// CryptoMode is the negotiated UDP crypto tier: "lite", "legacy", or "secure".
	CryptoMode string
}
