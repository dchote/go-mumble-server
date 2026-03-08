package protocol

import (
	"errors"
	"io"

	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

// Message is an alias for the messages package interface.
type Message = messages.Message

var (
	ErrUnknownMessageType = errors.New("protocol: unknown message type")
	ErrUDPTunnelPayload   = errors.New("protocol: UDPTunnel has raw payload, use ReadPacket")
)

// WriteMessage marshals the message and writes it as a typed packet.
func WriteMessage(w io.Writer, msgType MessageType, msg Message) error {
	if msg == nil {
		return WritePacket(w, msgType, nil)
	}
	payload, err := msg.Marshal()
	if err != nil {
		return err
	}
	return WritePacket(w, msgType, payload)
}

// ReadMessage unmarshals the payload into the appropriate message type.
func ReadMessage(msgType MessageType, payload []byte) (Message, error) {
	if len(payload) == 0 && msgType != MessageUDPTunnel {
		return nil, nil
	}
	var msg Message
	switch msgType {
	case MessageVersion:
		msg = &messages.Version{}
	case MessageUDPTunnel:
		return nil, ErrUDPTunnelPayload
	case MessageAuthenticate:
		msg = &messages.Authenticate{}
	case MessagePing:
		msg = &messages.Ping{}
	case MessageReject:
		msg = &messages.Reject{}
	case MessageServerSync:
		msg = &messages.ServerSync{}
	case MessageChannelRemove:
		msg = &messages.ChannelRemove{}
	case MessageChannelState:
		msg = &messages.ChannelState{}
	case MessageUserRemove:
		msg = &messages.UserRemove{}
	case MessageUserState:
		msg = &messages.UserState{}
	case MessageBanList:
		msg = &messages.BanList{}
	case MessageTextMessage:
		msg = &messages.TextMessage{}
	case MessagePermissionDenied:
		msg = &messages.PermissionDenied{}
	case MessageACL:
		msg = &messages.ACL{}
	case MessageQueryUsers:
		msg = &messages.QueryUsers{}
	case MessageCryptSetup:
		msg = &messages.CryptSetup{}
	case MessageContextActionModify:
		msg = &messages.ContextActionModify{}
	case MessageContextAction:
		msg = &messages.ContextAction{}
	case MessageUserList:
		msg = &messages.UserList{}
	case MessageVoiceTarget:
		msg = &messages.VoiceTarget{}
	case MessagePermissionQuery:
		msg = &messages.PermissionQuery{}
	case MessageCodecVersion:
		msg = &messages.CodecVersion{}
	case MessageUserStats:
		msg = &messages.UserStats{}
	case MessageRequestBlob:
		msg = &messages.RequestBlob{}
	case MessageServerConfig:
		msg = &messages.ServerConfig{}
	case MessageSuggestConfig:
		msg = &messages.SuggestConfig{}
	case MessagePluginDataTransmission:
		msg = &messages.PluginDataTransmission{}
	default:
		return nil, ErrUnknownMessageType
	}
	if msg == nil || len(payload) == 0 {
		return msg, nil
	}
	if err := msg.Unmarshal(payload); err != nil {
		return nil, err
	}
	return msg, nil
}
