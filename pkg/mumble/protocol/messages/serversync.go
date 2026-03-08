package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// ServerSync (type 5).
type ServerSync struct {
	Session      uint32
	MaxBandwidth uint32
	WelcomeText  string
	Permissions  uint64
}

func (m *ServerSync) Marshal() ([]byte, error) {
	var b []byte
	if m.Session != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	if m.MaxBandwidth != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.MaxBandwidth))
	}
	if m.WelcomeText != "" {
		b = wire.AppendString(b, 3, m.WelcomeText)
	}
	if m.Permissions != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, m.Permissions)
	}
	return b, nil
}

func (m *ServerSync) Unmarshal(data []byte) error {
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
			m.Session = uint32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.MaxBandwidth = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.WelcomeText = string(s)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Permissions = v
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
