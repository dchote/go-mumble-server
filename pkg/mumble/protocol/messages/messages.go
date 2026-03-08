// Package messages provides native Go structs for Mumble protocol messages.
// Field layout and wire encoding match the Mumble spec for client compatibility.
// See docs/protocol/control-messages.md.
package messages

// Message is the interface for protocol messages that can be marshaled/unmarshaled.
type Message interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
}

// UDPTunnel (type 1) carries raw audio; payload is not a structured message.
// Use raw bytes for the payload.

// Enums
type RejectType uint32

const (
	RejectNone              RejectType = 0
	RejectWrongVersion      RejectType = 1
	RejectInvalidUsername   RejectType = 2
	RejectWrongUserPW       RejectType = 3
	RejectWrongServerPW     RejectType = 4
	RejectUsernameInUse     RejectType = 5
	RejectServerFull        RejectType = 6
	RejectNoCertificate     RejectType = 7
	RejectAuthenticatorFail RejectType = 8
	RejectNoNewConnections  RejectType = 9
)

type DenyType uint32

const (
	DenyText                DenyType = 0
	DenyPermission          DenyType = 1
	DenySuperUser           DenyType = 2
	DenyChannelName         DenyType = 3
	DenyTextTooLong         DenyType = 4
	DenyH9K                  DenyType = 5
	DenyTemporaryChannel    DenyType = 6
	DenyMissingCertificate  DenyType = 7
	DenyUserName            DenyType = 8
	DenyChannelFull         DenyType = 9
	DenyNestingLimit        DenyType = 10
	DenyChannelCountLimit   DenyType = 11
	DenyChannelListenerLimit DenyType = 12
	DenyUserListenerLimit   DenyType = 13
)
