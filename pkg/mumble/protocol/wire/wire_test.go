package wire

import (
	"testing"
)

func TestVarintRoundTrip(t *testing.T) {
	values := []uint64{0, 1, 127, 128, 300, 16384, 0x7FFFFFFFFFFFFFFF}
	for _, v := range values {
		var b []byte
		b = AppendVarint(b, v)
		got, n, err := ReadVarint(b)
		if err != nil {
			t.Errorf("ReadVarint(%d): %v", v, err)
			continue
		}
		if n != len(b) {
			t.Errorf("ReadVarint(%d): consumed %d, expected %d", v, n, len(b))
		}
		if got != v {
			t.Errorf("RoundTrip: %d -> %d", v, got)
		}
	}
}
