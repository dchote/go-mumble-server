package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// BanEntry (embedded in BanList).
type BanEntry struct {
	Address  []byte
	Mask     uint32
	Name     string
	Hash     string
	Reason   string
	Start    string
	Duration uint32
}

// BanList (type 10).
type BanList struct {
	Bans  []BanEntry
	Query bool
}

func (m *BanList) Marshal() ([]byte, error) {
	var b []byte
	for _, be := range m.Bans {
		var eb []byte
		eb = wire.AppendBytes(eb, 1, be.Address)
		eb = wire.AppendTag(eb, 2, wire.WireVarint)
		eb = wire.AppendVarint(eb, uint64(be.Mask))
		if be.Name != "" {
			eb = wire.AppendString(eb, 3, be.Name)
		}
		if be.Hash != "" {
			eb = wire.AppendString(eb, 4, be.Hash)
		}
		if be.Reason != "" {
			eb = wire.AppendString(eb, 5, be.Reason)
		}
		if be.Start != "" {
			eb = wire.AppendString(eb, 6, be.Start)
		}
		if be.Duration != 0 {
			eb = wire.AppendTag(eb, 7, wire.WireVarint)
			eb = wire.AppendVarint(eb, uint64(be.Duration))
		}
		b = wire.AppendBytes(b, 1, eb)
	}
	if m.Query {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *BanList) Unmarshal(data []byte) error {
	b := data
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			return err
		}
		b = b[n:]
		switch fn {
		case 1:
			eb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			var be BanEntry
			be.unmarshal(eb)
			m.Bans = append(m.Bans, be)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Query = v != 0
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

func (be *BanEntry) unmarshal(data []byte) error {
	b := data
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			return err
		}
		b = b[n:]
		switch fn {
		case 1:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Address = bb
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Mask = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Name = string(s)
		case 4:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Hash = string(s)
		case 5:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Reason = string(s)
		case 6:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Start = string(s)
		case 7:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			be.Duration = uint32(v)
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
