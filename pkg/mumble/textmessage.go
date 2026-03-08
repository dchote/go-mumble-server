package mumble

// TextMessage represents a Mumble text message (channel, private, or tree).
type TextMessage struct {
	Actor       uint32
	Session     []uint32
	ChannelID   []uint32
	TreeID      []uint32
	Message     string
}
