package audio

import (
	"math"
	"testing"
)

func TestVarint_Decode_Encode_RoundTrip(t *testing.T) {
	values := []int64{
		0, 1, 127, 128, 255,
		0x3FFF, 0x1FFFFF, 0xFFFFFFF,
		math.MaxInt32, math.MaxInt64,
		-1, -2, -3, -4, -100,
	}
	buf := make([]byte, MaxVarintLen)
	for _, v := range values {
		n := EncodeVarint(buf, v)
		if n <= 0 {
			t.Errorf("EncodeVarint(%d) = 0", v)
			continue
		}
		got, m := DecodeVarint(buf[:n])
		if m != n {
			t.Errorf("DecodeVarint(%d): read %d bytes, encoded %d", v, m, n)
		}
		if got != v {
			t.Errorf("roundtrip %d: decoded %d", v, got)
		}
	}
}

func TestVarint_Decode_EmptyBuffer(t *testing.T) {
	v, n := DecodeVarint(nil)
	if v != 0 || n != 0 {
		t.Errorf("DecodeVarint(nil) = (%d, %d), want (0, 0)", v, n)
	}
	v, n = DecodeVarint([]byte{})
	if v != 0 || n != 0 {
		t.Errorf("DecodeVarint([]) = (%d, %d), want (0, 0)", v, n)
	}
}

func TestVarint_Decode_Negative(t *testing.T) {
	// 111111xx byte-inverted: 0xFC=-1, 0xFD=-2, 0xFE=-3, 0xFF=-4
	tests := []struct {
		b    []byte
		want int64
		n    int
	}{
		{[]byte{0xFC}, -1, 1},
		{[]byte{0xFD}, -2, 1},
		{[]byte{0xFE}, -3, 1},
		{[]byte{0xFF}, -4, 1},
	}
	for _, tt := range tests {
		got, n := DecodeVarint(tt.b)
		if got != tt.want || n != tt.n {
			t.Errorf("DecodeVarint(%x) = (%d, %d), want (%d, %d)", tt.b, got, n, tt.want, tt.n)
		}
	}
}

func TestVarint_Encode_ShortBuffer(t *testing.T) {
	n := EncodeVarint([]byte{}, 0)
	if n != 0 {
		t.Errorf("EncodeVarint with 0-byte buf: n = %d, want 0", n)
	}
	buf := make([]byte, 1)
	n = EncodeVarint(buf, 0)
	if n != 1 {
		t.Errorf("EncodeVarint(0) with 1-byte buf: n = %d, want 1", n)
	}
}
