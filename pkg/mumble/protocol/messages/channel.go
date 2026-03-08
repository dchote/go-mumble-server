package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// ChannelRemove (type 6).
type ChannelRemove struct {
	ChannelID uint32
}

func (m *ChannelRemove) Marshal() ([]byte, error) {
	var b []byte
	b = wire.AppendTag(b, 1, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(m.ChannelID))
	return b, nil
}

func (m *ChannelRemove) Unmarshal(data []byte) error {
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
			m.ChannelID = uint32(v)
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

// ChannelState (type 7).
type ChannelState struct {
	ChannelID         uint32
	Parent            uint32
	Name              string
	Links             []uint32
	Description       string
	LinksAdd          []uint32
	LinksRemove       []uint32
	Temporary         bool
	Position          int32
	DescriptionHash   []byte
	MaxUsers          uint32
	IsEnterRestricted bool
	CanEnter          bool
}

func (m *ChannelState) Marshal() ([]byte, error) {
	var b []byte
	if m.ChannelID != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ChannelID))
	}
	if m.Parent != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Parent))
	}
	if m.Name != "" {
		b = wire.AppendString(b, 3, m.Name)
	}
	for _, v := range m.Links {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	if m.Description != "" {
		b = wire.AppendString(b, 5, m.Description)
	}
	for _, v := range m.LinksAdd {
		b = wire.AppendTag(b, 6, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.LinksRemove {
		b = wire.AppendTag(b, 7, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	if m.Temporary {
		b = wire.AppendTag(b, 8, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.Position != 0 {
		b = wire.AppendTag(b, 9, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(int32(m.Position)))
	}
	if len(m.DescriptionHash) > 0 {
		b = wire.AppendBytes(b, 10, m.DescriptionHash)
	}
	if m.MaxUsers != 0 {
		b = wire.AppendTag(b, 11, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.MaxUsers))
	}
	if m.IsEnterRestricted {
		b = wire.AppendTag(b, 12, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.CanEnter {
		b = wire.AppendTag(b, 13, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *ChannelState) Unmarshal(data []byte) error {
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
			m.ChannelID = uint32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Parent = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Name = string(s)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Links = append(m.Links, uint32(v))
		case 5:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Description = string(s)
		case 6:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.LinksAdd = append(m.LinksAdd, uint32(v))
		case 7:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.LinksRemove = append(m.LinksRemove, uint32(v))
		case 8:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Temporary = v != 0
		case 9:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Position = int32(v)
		case 10:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.DescriptionHash = bb
		case 11:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.MaxUsers = uint32(v)
		case 12:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.IsEnterRestricted = v != 0
		case 13:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.CanEnter = v != 0
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
