package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// Ping (type 3).
type Ping struct {
	Timestamp  uint64
	Good       uint32
	Late       uint32
	Lost       uint32
	Resync     uint32
	UDPPackets uint32
	TCPPackets uint32
	UDPPingAvg float32
	UDPPingVar float32
	TCPPingAvg float32
	TCPPingVar float32
}

func (m *Ping) Marshal() ([]byte, error) {
	var b []byte
	if m.Timestamp != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, m.Timestamp)
	}
	if m.Good != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Good))
	}
	if m.Late != 0 {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Late))
	}
	if m.Lost != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Lost))
	}
	if m.Resync != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Resync))
	}
	if m.UDPPackets != 0 {
		b = wire.AppendTag(b, 6, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.UDPPackets))
	}
	if m.TCPPackets != 0 {
		b = wire.AppendTag(b, 7, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.TCPPackets))
	}
	if m.UDPPingAvg != 0 {
		b = wire.AppendTag(b, 8, wire.WireFixed32)
		b = wire.AppendFixed32(b, wire.Float32bits(m.UDPPingAvg))
	}
	if m.UDPPingVar != 0 {
		b = wire.AppendTag(b, 9, wire.WireFixed32)
		b = wire.AppendFixed32(b, wire.Float32bits(m.UDPPingVar))
	}
	if m.TCPPingAvg != 0 {
		b = wire.AppendTag(b, 10, wire.WireFixed32)
		b = wire.AppendFixed32(b, wire.Float32bits(m.TCPPingAvg))
	}
	if m.TCPPingVar != 0 {
		b = wire.AppendTag(b, 11, wire.WireFixed32)
		b = wire.AppendFixed32(b, wire.Float32bits(m.TCPPingVar))
	}
	return b, nil
}

func (m *Ping) Unmarshal(data []byte) error {
	b := data
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			return err
		}
		b = b[n:]
		switch fn {
		case 1:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Timestamp = v
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Good = uint32(v)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Late = uint32(v)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Lost = uint32(v)
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Resync = uint32(v)
		case 6:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.UDPPackets = uint32(v)
		case 7:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.TCPPackets = uint32(v)
		case 8:
			if len(b) < 4 {
				break
			}
			m.UDPPingAvg = wire.Float32frombits(wire.ReadFixed32(b))
			b = b[4:]
		case 9:
			if len(b) < 4 {
				break
			}
			m.UDPPingVar = wire.Float32frombits(wire.ReadFixed32(b))
			b = b[4:]
		case 10:
			if len(b) < 4 {
				break
			}
			m.TCPPingAvg = wire.Float32frombits(wire.ReadFixed32(b))
			b = b[4:]
		case 11:
			if len(b) < 4 {
				break
			}
			m.TCPPingVar = wire.Float32frombits(wire.ReadFixed32(b))
			b = b[4:]
		default:
			skip, err := wire.SkipField(b, wt)
			if err != nil {
				return err
			}
			b = b[skip:]
		}
	}
	return nil
}
