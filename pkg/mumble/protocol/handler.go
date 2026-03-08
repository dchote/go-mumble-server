package protocol

// MessageHandler is a function that handles a single message type.
// It receives the message type, raw payload, and a context (e.g., connection).
// Use protocol.ReadMessage to unmarshal into native message types.
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

// NewHandlerTable returns a pre-sized handler table with length MessageCount.
func NewHandlerTable() HandlerTable {
	return make(HandlerTable, MessageCount)
}
