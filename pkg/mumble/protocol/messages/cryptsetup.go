package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// CryptSetup (type 15).
type CryptSetup struct {
	Key          []byte
	ClientNonce  []byte
	ServerNonce  []byte
}

func (m *CryptSetup) Marshal() ([]byte, error) {
	var b []byte
	if m.Key != nil {
		b = wire.AppendBytes(b, 1, m.Key)
	}
	if len(m.ClientNonce) > 0 {
		b = wire.AppendBytes(b, 2, m.ClientNonce)
	}
	if len(m.ServerNonce) > 0 {
		b = wire.AppendBytes(b, 3, m.ServerNonce)
	}
	return b, nil
}

func (m *CryptSetup) Unmarshal(data []byte) error {
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
			m.Key = bb
		case 2:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ClientNonce = bb
		case 3:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ServerNonce = bb
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
