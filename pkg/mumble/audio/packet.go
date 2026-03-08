package audio

// ParsePacket parses a UDP audio packet (varint codec header, payload).
// See docs/protocol/voice-data.md.
func ParsePacket(data []byte) (codec uint64, payload []byte, err error) {
	// TODO: implement varint decode and header parsing
	return 0, nil, nil
}
