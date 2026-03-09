package mumble

// User represents a connected Mumble user (client session).
type User struct {
	SessionID      uint32
	UserID         uint32
	ChannelID      uint32
	Name           string
	AccessTokens   []string // from Authenticate message, for token groups
	Mute           bool
	Deaf           bool
	Suppress       bool
	SelfMute       bool
	SelfDeaf       bool
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
}
