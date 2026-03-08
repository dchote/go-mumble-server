package protocol

// MessageType is the 16-bit Mumble control message type ID.
// See docs/protocol/control-messages.md for the full catalog.
type MessageType uint16

const (
	MessageVersion       MessageType = 0
	MessageUDPTunnel     MessageType = 1
	MessageAuthenticate  MessageType = 2
	MessagePing          MessageType = 3
	MessageReject        MessageType = 4
	MessageServerSync    MessageType = 5
	MessageChannelRemove MessageType = 6
	MessageChannelState  MessageType = 7
	MessageUserRemove    MessageType = 8
	MessageUserState     MessageType = 9
	MessageBanList       MessageType = 10
	MessageTextMessage   MessageType = 11
	MessagePermissionDenied MessageType = 12
	MessageACL           MessageType = 13
	MessageQueryUsers    MessageType = 14
	MessageCryptSetup    MessageType = 15
	MessageContextAction MessageType = 16
	MessageUserList      MessageType = 17
	MessageRequestBlob   MessageType = 18
	MessageServerConfig  MessageType = 19
	MessageSuggestConfig MessageType = 20
	// Types 21-26 reserved/other
)
