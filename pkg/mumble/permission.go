package mumble

// Permission represents the Mumble permission bitmask.
// See docs/protocol/permissions.md for full definitions.
type Permission uint32

const (
	PermissionNone           Permission = 0
	PermissionWrite          Permission = 0x1
	PermissionTraverse       Permission = 0x2
	PermissionEnter          Permission = 0x4
	PermissionSpeak          Permission = 0x8
	PermissionMuteDeafen     Permission = 0x10
	PermissionMove           Permission = 0x20
	PermissionMakeChannel    Permission = 0x40
	PermissionLinkChannel    Permission = 0x80
	PermissionWhisper        Permission = 0x100
	PermissionTextMessage    Permission = 0x200
	PermissionMakeTempChannel Permission = 0x400
	PermissionListen         Permission = 0x800
	// Root-only permissions (handled specially)
	PermissionKick           Permission = 0x10000
	PermissionBan            Permission = 0x20000
	PermissionRegister       Permission = 0x40000
	PermissionSelfRegister   Permission = 0x80000
	PermissionResetUser      Permission = 0x100000
	PermissionCached         Permission = 0x8000000
	PermissionAll            Permission = 0xF07FF
)
