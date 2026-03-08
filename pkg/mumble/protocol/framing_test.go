package protocol

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/messages"
)

func TestWritePacket(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte("hello")
	err := WritePacket(&buf, MessageVersion, payload)
	if err != nil {
		t.Fatalf("WritePacket: %v", err)
	}
	data := buf.Bytes()
	if len(data) != TCPHeaderSize+len(payload) {
		t.Errorf("expected %d bytes, got %d", TCPHeaderSize+len(payload), len(data))
	}
	// Header: type 0 (Version), length 5
	if data[0] != 0 || data[1] != 0 {
		t.Errorf("type: expected 0x0000, got %02x %02x", data[0], data[1])
	}
	if data[2] != 0 || data[3] != 0 || data[4] != 0 || data[5] != 5 {
		t.Errorf("length: expected 5, got %d", uint32(data[2])<<24|uint32(data[3])<<16|uint32(data[4])<<8|uint32(data[5]))
	}
	if !bytes.Equal(data[6:], payload) {
		t.Errorf("payload: expected %q, got %q", payload, data[6:])
	}
}

func TestReadPacket(t *testing.T) {
	payload := []byte("test payload")
	var buf bytes.Buffer
	_ = WritePacket(&buf, MessagePing, payload)
	r := bytes.NewReader(buf.Bytes())
	msgType, got, err := ReadPacket(r)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if msgType != MessagePing {
		t.Errorf("type: expected %d, got %d", MessagePing, msgType)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload: expected %q, got %q", payload, got)
	}
}

func TestReadPacket_EmptyPayload(t *testing.T) {
	var buf bytes.Buffer
	_ = WritePacket(&buf, MessageVersion, nil)
	r := bytes.NewReader(buf.Bytes())
	msgType, payload, err := ReadPacket(r)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if msgType != MessageVersion {
		t.Errorf("type: expected Version, got %d", msgType)
	}
	if payload != nil {
		t.Errorf("expected nil payload, got %q", payload)
	}
}

func TestReadPacket_MaxPayload(t *testing.T) {
	payload := make([]byte, MaxPayloadSize)
	for i := range payload {
		payload[i] = byte(i)
	}
	var buf bytes.Buffer
	_ = WritePacket(&buf, MessageAuthenticate, payload)
	r := bytes.NewReader(buf.Bytes())
	msgType, got, err := ReadPacket(r)
	if err != nil {
		t.Fatalf("ReadPacket: %v", err)
	}
	if msgType != MessageAuthenticate {
		t.Errorf("type: expected Authenticate, got %d", msgType)
	}
	if len(got) != MaxPayloadSize {
		t.Errorf("payload length: expected %d, got %d", MaxPayloadSize, len(got))
	}
	if !bytes.Equal(got, payload) {
		t.Error("payload mismatch")
	}
}

func TestReadPacket_ExceedsMaxPayload(t *testing.T) {
	// Craft a packet with length = MaxPayloadSize+1
	header := make([]byte, TCPHeaderSize)
	header[0] = 0
	header[1] = 0
	length := uint32(MaxPayloadSize + 1)
	header[2] = byte(length >> 24)
	header[3] = byte(length >> 16)
	header[4] = byte(length >> 8)
	header[5] = byte(length)
	r := bytes.NewReader(header)
	_, _, err := ReadPacket(r)
	if err == nil {
		t.Fatal("expected ErrPayloadTooLarge, got nil")
	}
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Errorf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestReadPacket_ShortRead(t *testing.T) {
	r := bytes.NewReader([]byte{0, 0}) // Only 2 bytes, need 6
	_, _, err := ReadPacket(r)
	if err != io.ErrUnexpectedEOF && err != io.EOF {
		t.Errorf("expected EOF error, got %v", err)
	}
}

func TestWriteMessage(t *testing.T) {
	msg := &messages.Version{Release: "test 1.0", OS: "linux"}
	payload, err := msg.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var buf bytes.Buffer
	err = WriteMessage(&buf, MessageVersion, msg)
	if err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
	data := buf.Bytes()
	if len(data) < TCPHeaderSize {
		t.Fatalf("expected at least %d bytes, got %d", TCPHeaderSize, len(data))
	}
	r := bytes.NewReader(data)
	msgType, got, err := ReadPacket(r)
	if err != nil {
		t.Fatalf("ReadPacket after WriteMessage: %v", err)
	}
	if msgType != MessageVersion {
		t.Errorf("type: expected Version, got %d", msgType)
	}
	if !bytes.Equal(got, payload) {
		t.Error("WriteMessage payload does not match Marshal output")
	}
}
