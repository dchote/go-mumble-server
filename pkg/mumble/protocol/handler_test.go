package protocol

import (
	"errors"
	"testing"
)

func TestHandlerTable_Dispatch_NilHandler(t *testing.T) {
	ht := NewHandlerTable()
	err := ht.Dispatch(MessageVersion, []byte("x"), nil)
	if err != nil {
		t.Errorf("expected nil for unregistered handler, got %v", err)
	}
}

func TestHandlerTable_Dispatch_RegisteredHandler(t *testing.T) {
	ht := NewHandlerTable()
	var receivedType MessageType
	var receivedPayload []byte
	ht[MessageVersion] = func(msgType MessageType, payload []byte, _ interface{}) error {
		receivedType = msgType
		receivedPayload = payload
		return nil
	}
	payload := []byte("test")
	err := ht.Dispatch(MessageVersion, payload, nil)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if receivedType != MessageVersion {
		t.Errorf("received type %d, want %d", receivedType, MessageVersion)
	}
	if string(receivedPayload) != "test" {
		t.Errorf("received payload %q, want %q", receivedPayload, payload)
	}
}

func TestHandlerTable_Dispatch_ReturnsError(t *testing.T) {
	wantErr := errors.New("handler error")
	ht := NewHandlerTable()
	ht[MessagePing] = func(MessageType, []byte, interface{}) error {
		return wantErr
	}
	err := ht.Dispatch(MessagePing, nil, nil)
	if err != wantErr {
		t.Errorf("expected %v, got %v", wantErr, err)
	}
}

func TestHandlerTable_Dispatch_OutOfBounds(t *testing.T) {
	ht := NewHandlerTable()
	ht[MessageVersion] = func(MessageType, []byte, interface{}) error {
		t.Fatal("handler should not be called for out-of-bounds type")
		return nil
	}
	err := ht.Dispatch(MessageType(999), nil, nil)
	if err != nil {
		t.Errorf("expected nil for out-of-bounds, got %v", err)
	}
}

func TestNewHandlerTable(t *testing.T) {
	ht := NewHandlerTable()
	if len(ht) != MessageCount {
		t.Errorf("length = %d, want %d", len(ht), MessageCount)
	}
}
