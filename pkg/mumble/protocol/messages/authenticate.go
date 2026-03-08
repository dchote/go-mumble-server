package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// Authenticate (type 2).
type Authenticate struct {
	Username     string
	Password     string
	Tokens       []string
	CeltVersions []int32
	Opus         bool
	ClientType   int32
}

func (m *Authenticate) Marshal() ([]byte, error) {
	var b []byte
	if m.Username != "" {
		b = wire.AppendString(b, 1, m.Username)
	}
	if m.Password != "" {
		b = wire.AppendString(b, 2, m.Password)
	}
	for _, t := range m.Tokens {
		b = wire.AppendString(b, 3, t)
	}
	for _, v := range m.CeltVersions {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(int32(v)))
	}
	if m.Opus {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.ClientType != 0 {
		b = wire.AppendTag(b, 6, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(int32(m.ClientType)))
	}
	return b, nil
}

func (m *Authenticate) Unmarshal(data []byte) error {
	b := data
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			return err
		}
		b = b[n:]
		switch fn {
		case 1:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Username = string(s)
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Password = string(s)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Tokens = append(m.Tokens, string(s))
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.CeltVersions = append(m.CeltVersions, int32(v))
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Opus = v != 0
		case 6:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ClientType = int32(v)
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
