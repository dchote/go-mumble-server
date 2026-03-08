package audio

import (
	"errors"
)

// ErrPacketTooShort is returned when the packet buffer is too short to parse.
var ErrPacketTooShort = errors.New("audio: packet too short")

// ParsedPacket is the result of parsing a client-to-server legacy binary audio packet.
type ParsedPacket struct {
	Codec        uint8  // Audio codec ID (bits 7-5 of header)
	Target       uint8  // Voice target (bits 4-0 of header)
	Sequence     int64  // Sequence number
	PayloadLen   int64  // Payload length in bytes (bit 13 masked out)
	IsTerminator bool   // True if bit 13 of payload length varint was set
	Payload      []byte // Audio frame data
}

// ParsePacket parses a client-to-server legacy binary UDP audio packet.
// The server receives these from clients; no session field is present.
// See docs/protocol/voice-data.md.
func ParsePacket(data []byte) (*ParsedPacket, error) {
	if len(data) < 1 {
		return nil, ErrPacketTooShort
	}
	p := &ParsedPacket{}
	p.Codec = uint8((data[0] >> 5) & 0x7)
	p.Target = uint8(data[0] & 0x1F)
	buf := data[1:]

	// Sequence varint
	seq, n := DecodeVarint(buf)
	if n <= 0 {
		return nil, ErrPacketTooShort
	}
	p.Sequence = seq
	buf = buf[n:]

	// Payload length varint (bit 13 = terminator)
	if len(buf) < 1 {
		return nil, ErrPacketTooShort
	}
	rawLen, n := DecodeVarint(buf)
	if n <= 0 {
		return nil, ErrPacketTooShort
	}
	p.PayloadLen = rawLen & 0x1FFF
	p.IsTerminator = (rawLen & 0x2000) != 0
	buf = buf[n:]

	// Remaining bytes = payload
	if int64(len(buf)) < p.PayloadLen {
		return nil, ErrPacketTooShort
	}
	p.Payload = buf[:p.PayloadLen]
	return p, nil
}
