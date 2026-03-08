package protocol

import (
	"encoding/binary"
	"errors"
	"io"
)

// TCPHeaderSize is the size of the Mumble TCP packet header (type + length).
const TCPHeaderSize = 6

// MaxPayloadSize is the maximum allowed payload length (8 MiB, per Murmur).
// ReadPacket rejects larger payloads to prevent memory exhaustion.
const MaxPayloadSize = 8 * 1024 * 1024

// ErrPayloadTooLarge is returned by ReadPacket when payload exceeds MaxPayloadSize.
var ErrPayloadTooLarge = errors.New("protocol: payload exceeds maximum size")

// WritePacket writes a typed packet with 6-byte header (big-endian uint16 type, uint32 length) plus payload.
func WritePacket(w io.Writer, msgType MessageType, payload []byte) error {
	header := make([]byte, TCPHeaderSize)
	binary.BigEndian.PutUint16(header[0:2], uint16(msgType))
	binary.BigEndian.PutUint32(header[2:6], uint32(len(payload)))
	if _, err := w.Write(header); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return err
		}
	}
	return nil
}

// ReadPacket reads the 6-byte header and returns message type, payload, and error.
func ReadPacket(r io.Reader) (MessageType, []byte, error) {
	header := make([]byte, TCPHeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	msgType := MessageType(binary.BigEndian.Uint16(header[0:2]))
	length := binary.BigEndian.Uint32(header[2:6])
	if length > MaxPayloadSize {
		return 0, nil, ErrPayloadTooLarge
	}
	if length == 0 {
		return msgType, nil, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return msgType, payload, nil
}

