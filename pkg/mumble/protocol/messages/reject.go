package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// Reject (type 4).
type Reject struct {
	Type   RejectType
	Reason string
}

func (m *Reject) Marshal() ([]byte, error) {
	var b []byte
	if m.Type != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Type))
	}
	if m.Reason != "" {
		b = wire.AppendString(b, 2, m.Reason)
	}
	return b, nil
}

func (m *Reject) Unmarshal(data []byte) error {
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
			m.Type = RejectType(v)
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Reason = string(s)
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
