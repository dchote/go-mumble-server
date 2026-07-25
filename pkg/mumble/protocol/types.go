package protocol

// MessageType is the 16-bit Mumble control message type ID.
// See docs/protocol/control-messages.md for the full catalog.
type MessageType uint16

const (
	MessageVersion                MessageType = 0
	MessageUDPTunnel              MessageType = 1
	MessageAuthenticate           MessageType = 2
	MessagePing                   MessageType = 3
	MessageReject                 MessageType = 4
	MessageServerSync             MessageType = 5
	MessageChannelRemove          MessageType = 6
	MessageChannelState           MessageType = 7
	MessageUserRemove             MessageType = 8
	MessageUserState              MessageType = 9
	MessageBanList                MessageType = 10
	MessageTextMessage            MessageType = 11
	MessagePermissionDenied       MessageType = 12
	MessageACL                    MessageType = 13
	MessageQueryUsers             MessageType = 14
	MessageCryptSetup             MessageType = 15
	MessageContextActionModify    MessageType = 16
	MessageContextAction          MessageType = 17
	MessageUserList               MessageType = 18
	MessageVoiceTarget            MessageType = 19
	MessagePermissionQuery        MessageType = 20
	MessageCodecVersion           MessageType = 21
	MessageUserStats              MessageType = 22
	MessageRequestBlob            MessageType = 23
	MessageServerConfig           MessageType = 24
	MessageSuggestConfig          MessageType = 25
	MessagePluginDataTransmission MessageType = 26

	// MessageCount is the number of control message types (0-26).
	// Use for pre-sized handler tables.
	MessageCount = 27
)
