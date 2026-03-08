package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// Version (type 0). Field nums from Mumble.proto.
type Version struct {
	VersionV1 uint32
	VersionV2 uint64
	Release   string
	OS        string
	OSVersion string
}

func (m *Version) Marshal() ([]byte, error) {
	var b []byte
	if m.VersionV1 != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.VersionV1))
	}
	if m.Release != "" {
		b = wire.AppendString(b, 2, m.Release)
	}
	if m.OS != "" {
		b = wire.AppendString(b, 3, m.OS)
	}
	if m.OSVersion != "" {
		b = wire.AppendString(b, 4, m.OSVersion)
	}
	if m.VersionV2 != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, m.VersionV2)
	}
	return b, nil
}

func (m *Version) Unmarshal(data []byte) error {
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
			m.VersionV1 = uint32(v)
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Release = string(s)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.OS = string(s)
		case 4:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.OSVersion = string(s)
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.VersionV2 = v
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
