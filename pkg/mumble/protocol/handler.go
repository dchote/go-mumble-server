package protocol

// MessageHandler is a function that handles a single message type.
// It receives the message type, raw protobuf payload, and a context (e.g., connection).
type MessageHandler func(msgType MessageType, payload []byte, ctx interface{}) error

// HandlerTable is an array-indexed dispatch table for message types.
// Index by uint16(msgType). Nil entries are no-ops.
type HandlerTable []MessageHandler

// Dispatch calls the handler for the given message type if one is registered.
func (t HandlerTable) Dispatch(msgType MessageType, payload []byte, ctx interface{}) error {
	if int(msgType) >= len(t) || t[msgType] == nil {
		return nil
	}
	return t[msgType](msgType, payload, ctx)
}
