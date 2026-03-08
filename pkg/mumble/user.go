package mumble

// User represents a connected Mumble user (client session).
type User struct {
	SessionID      uint32
	UserID         uint32
	ChannelID      uint32
	Name           string
	Mute           bool
	Deaf           bool
	Suppress       bool
	SelfMute       bool
	SelfDeaf       bool
	PluginIdentity string
	PluginContext  []byte
	Texture        []byte
	Comment        string
}
