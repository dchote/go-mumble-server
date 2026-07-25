package audio

import (
	"encoding/binary"
	"math"
)

// MaxVarintLen is the maximum number of bytes required to encode a Mumble varint.
const MaxVarintLen = 10

// Decode reads the first varint-encoded number from the given buffer.
// Returns the value and the number of bytes read. Returns 0 bytes read on error.
func DecodeVarint(b []byte) (int64, int) {
	if len(b) == 0 {
		return 0, 0
	}
	// 0xxxxxxx 7-bit positive number
	if (b[0] & 0x80) == 0 {
		return int64(b[0]), 1
	}
	// 10xxxxxx + 1 byte 14-bit positive number
	if (b[0]&0xC0) == 0x80 && len(b) >= 2 {
		return int64(b[0]&0x3F)<<8 | int64(b[1]), 2
	}
	// 110xxxxx + 2 bytes 21-bit positive number
	if (b[0]&0xE0) == 0xC0 && len(b) >= 3 {
		return int64(b[0]&0x1F)<<16 | int64(b[1])<<8 | int64(b[2]), 3
	}
	// 1110xxxx + 3 bytes 28-bit positive number
	if (b[0]&0xF0) == 0xE0 && len(b) >= 4 {
		return int64(b[0]&0xF)<<24 | int64(b[1])<<16 | int64(b[2])<<8 | int64(b[3]), 4
	}
	// 111100__ + int (32-bit) 32-bit positive number
	if (b[0]&0xFC) == 0xF0 && len(b) >= 5 {
		return int64(binary.BigEndian.Uint32(b[1:])), 5
	}
	// 111101__ + long (64-bit) 64-bit number
	if (b[0]&0xFC) == 0xF4 && len(b) >= 9 {
		return int64(binary.BigEndian.Uint64(b[1:])), 9
	}
	// 111110__ + varint negative recursive
	if b[0]&0xFC == 0xF8 {
		if v, n := DecodeVarint(b[1:]); n > 0 {
			return -v, n + 1
		}
		return 0, 0
	}
	// 111111xx byte-inverted negative two bit number (~xx)
	if b[0]&0xFC == 0xFC {
		return ^int64(b[0] & 0x03), 1
	}
	return 0, 0
}

// EncodeVarint encodes the given value in Mumble varint format.
// Returns the number of bytes written. Caller must ensure b has sufficient space (MaxVarintLen for safety).
func EncodeVarint(b []byte, value int64) int {
	if len(b) < 1 {
		return 0
	}
	// 111111xx byte-inverted negative two bit number (~xx)
	if value >= -4 && value <= -1 {
		b[0] = 0xFC | byte(^value&0xFF)
		return 1
	}
	// 111110__ + varint negative recursive
	if value < 0 {
		b[0] = 0xF8
		return 1 + EncodeVarint(b[1:], -value)
	}
	// 0xxxxxxx 7-bit positive number
	if value <= 0x7F {
		b[0] = byte(value)
		return 1
	}
	// 10xxxxxx + 1 byte 14-bit positive number
	if value <= 0x3FFF {
		b[0] = byte(((value >> 8) & 0x3F) | 0x80)
		b[1] = byte(value & 0xFF)
		return 2
	}
	// 110xxxxx + 2 bytes 21-bit positive number
	if value <= 0x1FFFFF {
		b[0] = byte((value>>16)&0x1F | 0xC0)
		b[1] = byte((value >> 8) & 0xFF)
		b[2] = byte(value & 0xFF)
		return 3
	}
	// 1110xxxx + 3 bytes 28-bit positive number
	if value <= 0xFFFFFFF {
		b[0] = byte((value>>24)&0xF | 0xE0)
		b[1] = byte((value >> 16) & 0xFF)
		b[2] = byte((value >> 8) & 0xFF)
		b[3] = byte(value & 0xFF)
		return 4
	}
	// 111100__ + int (32-bit) 32-bit positive number
	if value <= math.MaxInt32 {
		b[0] = 0xF0
		binary.BigEndian.PutUint32(b[1:], uint32(value))
		return 5
	}
	// 111101__ + long (64-bit) 64-bit number
	b[0] = 0xF4
	binary.BigEndian.PutUint64(b[1:], uint64(value))
	return 9
}
