// Package wire provides hand-written protobuf wire-format encoding for Mumble
// protocol messages. Produces Mumble-compatible binary without using
// google.golang.org/protobuf.
package wire

import (
	"encoding/binary"
	"errors"
	"math"
)

// Wire types per protobuf spec
const (
	WireVarint          = 0
	WireFixed64         = 1
	WireLengthDelimited = 2
	WireFixed32         = 5
)

var (
	ErrBufferTooShort = errors.New("wire: buffer too short")
	ErrInvalidTag     = errors.New("wire: invalid tag")
)

// AppendTag appends (fieldNum<<3)|wireType as a varint.
func AppendTag(b []byte, fieldNum int, wireType int) []byte {
	return AppendVarint(b, uint64(fieldNum)<<3|uint64(wireType))
}

// AppendVarint appends a standard protobuf varint.
func AppendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// AppendFixed32 appends a little-endian uint32 (protobuf fixed32).
func AppendFixed32(b []byte, v uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return append(b, buf[:]...)
}

// AppendFixed64 appends a little-endian uint64 (protobuf fixed64).
func AppendFixed64(b []byte, v uint64) []byte {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	return append(b, buf[:]...)
}

// AppendBytes appends length-delimited bytes (field tag + length + data).
func AppendBytes(b []byte, fieldNum int, data []byte) []byte {
	b = AppendTag(b, fieldNum, WireLengthDelimited)
	b = AppendVarint(b, uint64(len(data)))
	return append(b, data...)
}

// AppendString appends a length-delimited string.
func AppendString(b []byte, fieldNum int, s string) []byte {
	return AppendBytes(b, fieldNum, []byte(s))
}

// ReadVarint reads a varint from b. Returns value and bytes consumed.
func ReadVarint(b []byte) (uint64, int, error) {
	var x uint64
	var s uint
	for i, c := range b {
		if i >= 10 {
			return 0, 0, ErrInvalidTag
		}
		x |= uint64(c&0x7F) << s
		if c < 0x80 {
			return x, i + 1, nil
		}
		s += 7
	}
	return 0, 0, ErrBufferTooShort
}

// ReadTag reads a tag (field num, wire type) and returns bytes consumed.
func ReadTag(b []byte) (fieldNum int, wireType int, n int, err error) {
	v, n, err := ReadVarint(b)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(v >> 3), int(v & 7), n, nil
}

// ReadFixed32 reads a fixed32 from b. Caller must ensure len(b) >= 4.
func ReadFixed32(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b[:4])
}

// ReadFixed64 reads a fixed64 from b. Caller must ensure len(b) >= 8.
func ReadFixed64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b[:8])
}

// SkipField skips a field based on wire type. Returns bytes consumed.
func SkipField(b []byte, wireType int) (int, error) {
	switch wireType {
	case WireVarint:
		_, n, err := ReadVarint(b)
		return n, err
	case WireFixed64:
		if len(b) < 8 {
			return 0, ErrBufferTooShort
		}
		return 8, nil
	case WireLengthDelimited:
		l, n, err := ReadVarint(b)
		if err != nil {
			return 0, err
		}
		if uint64(len(b)-n) < l {
			return 0, ErrBufferTooShort
		}
		return n + int(l), nil
	case WireFixed32:
		if len(b) < 4 {
			return 0, ErrBufferTooShort
		}
		return 4, nil
	default:
		return 0, ErrInvalidTag
	}
}

// ReadLengthDelimited returns the length-delimited bytes and bytes consumed.
func ReadLengthDelimited(b []byte) ([]byte, int, error) {
	l, n, err := ReadVarint(b)
	if err != nil {
		return nil, 0, err
	}
	if uint64(len(b)-n) < l {
		return nil, 0, ErrBufferTooShort
	}
	return append([]byte(nil), b[n:n+int(l)]...), n + int(l), nil
}

// Float32bits converts float32 to uint32 for wire encoding.
func Float32bits(f float32) uint32 {
	return math.Float32bits(f)
}

// Float32frombits converts uint32 from wire to float32.
func Float32frombits(u uint32) float32 {
	return math.Float32frombits(u)
}
