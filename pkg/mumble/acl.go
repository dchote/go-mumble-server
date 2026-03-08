package mumble

// ACL represents an access control list entry for a channel.
type ACL struct {
	ChannelID  uint32
	ApplyHere  bool
	ApplySubs  bool
	UserID     int32
	Group      string
	Grant      Permission
	Deny       Permission
}

// Group represents a channel group (named set of users).
type Group struct {
	Name       string
	Inherit    bool
	Inheritable bool
	Add        []uint32
	Remove     []uint32
	InheritedMembers []uint32
}
