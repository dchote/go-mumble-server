package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// TextMessage (type 11).
type TextMessage struct {
	Actor     uint32
	Session   []uint32
	ChannelID []uint32
	TreeID    []uint32
	Message   string
}

func (m *TextMessage) Marshal() ([]byte, error) {
	var b []byte
	if m.Actor != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Actor))
	}
	for _, v := range m.Session {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.ChannelID {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.TreeID {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	b = wire.AppendString(b, 5, m.Message)
	return b, nil
}

func (m *TextMessage) Unmarshal(data []byte) error {
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
			m.Actor = uint32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Session = append(m.Session, uint32(v))
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ChannelID = append(m.ChannelID, uint32(v))
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.TreeID = append(m.TreeID, uint32(v))
		case 5:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Message = string(s)
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

// PermissionDenied (type 12).
type PermissionDenied struct {
	Permission uint32
	ChannelID  uint32
	Session    uint32
	Reason     string
	Type       DenyType
	Name       string
}

func (m *PermissionDenied) Marshal() ([]byte, error) {
	var b []byte
	if m.Permission != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Permission))
	}
	if m.ChannelID != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ChannelID))
	}
	if m.Session != 0 {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	if m.Reason != "" {
		b = wire.AppendString(b, 4, m.Reason)
	}
	if m.Type != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Type))
	}
	if m.Name != "" {
		b = wire.AppendString(b, 6, m.Name)
	}
	return b, nil
}

func (m *PermissionDenied) Unmarshal(data []byte) error {
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
			m.Permission = uint32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ChannelID = uint32(v)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Session = uint32(v)
		case 4:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Reason = string(s)
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Type = DenyType(v)
		case 6:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Name = string(s)
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
