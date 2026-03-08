package audio

import (
	"bytes"
	"testing"
)

// buildLegacyPacket builds a client-to-server legacy binary packet for testing.
func buildLegacyPacket(codec, target uint8, seq int64, payloadLen int64, terminator bool, payload []byte) []byte {
	var buf bytes.Buffer
	header := (codec & 0x7)<<5 | (target & 0x1F)
	buf.WriteByte(header)
	enc := make([]byte, MaxVarintLen)
	n := EncodeVarint(enc, seq)
	buf.Write(enc[:n])
	rawLen := payloadLen
	if terminator {
		rawLen |= 0x2000
	}
	n = EncodeVarint(enc, rawLen)
	buf.Write(enc[:n])
	buf.Write(payload)
	return buf.Bytes()
}

func TestParsePacket_ValidLegacyPacket(t *testing.T) {
	payload := []byte("audio data")
	data := buildLegacyPacket(CodecOpus, 0, 42, int64(len(payload)), false, payload)
	p, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}
	if p.Codec != CodecOpus {
		t.Errorf("Codec = %d, want %d", p.Codec, CodecOpus)
	}
	if p.Target != 0 {
		t.Errorf("Target = %d, want 0", p.Target)
	}
	if p.Sequence != 42 {
		t.Errorf("Sequence = %d, want 42", p.Sequence)
	}
	if p.PayloadLen != int64(len(payload)) {
		t.Errorf("PayloadLen = %d, want %d", p.PayloadLen, len(payload))
	}
	if p.IsTerminator {
		t.Error("IsTerminator = true, want false")
	}
	if !bytes.Equal(p.Payload, payload) {
		t.Errorf("Payload = %q, want %q", p.Payload, payload)
	}
}

func TestParsePacket_TooShort(t *testing.T) {
	_, err := ParsePacket(nil)
	if err != ErrPacketTooShort {
		t.Errorf("ParsePacket(nil): err = %v, want ErrPacketTooShort", err)
	}
	_, err = ParsePacket([]byte{})
	if err != ErrPacketTooShort {
		t.Errorf("ParsePacket([]): err = %v, want ErrPacketTooShort", err)
	}
	// Header only, no varint
	_, err = ParsePacket([]byte{0x80})
	if err != ErrPacketTooShort {
		t.Errorf("ParsePacket([header]): err = %v, want ErrPacketTooShort", err)
	}
}

func TestParsePacket_ExtractsCodecAndTarget(t *testing.T) {
	// Codec 4 (Opus), target 31 (loopback) -> header = 4<<5 | 31 = 159
	data := buildLegacyPacket(4, 31, 0, 0, false, nil)
	p, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}
	if p.Codec != 4 {
		t.Errorf("Codec = %d, want 4", p.Codec)
	}
	if p.Target != 31 {
		t.Errorf("Target = %d, want 31", p.Target)
	}
}

func TestParsePacket_TerminatorFlag(t *testing.T) {
	payload := []byte("x")
	data := buildLegacyPacket(CodecOpus, 0, 1, int64(len(payload)), true, payload)
	p, err := ParsePacket(data)
	if err != nil {
		t.Fatalf("ParsePacket: %v", err)
	}
	if !p.IsTerminator {
		t.Error("IsTerminator = false, want true")
	}
	if p.PayloadLen != 1 {
		t.Errorf("PayloadLen = %d, want 1", p.PayloadLen)
	}
}
