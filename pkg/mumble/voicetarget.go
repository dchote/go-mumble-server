package mumble

// VoiceTarget defines a whisper target (users, channels, or groups).
// Voice target ID 0 = normal talk (current + linked channels).
// Voice target ID 31 = server loopback.
type VoiceTarget struct {
	ID         uint32
	Targets    []VoiceTargetEntry
}

// VoiceTargetEntry is a single target within a VoiceTarget.
type VoiceTargetEntry struct {
	Type    uint32 // 0=normal, 1=channel, 2=group, 3=session
	Session uint32
	ChannelID uint32
	Group   string
	Links   bool
	Children bool
}
