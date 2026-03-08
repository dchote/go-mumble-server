package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// UserRemove (type 8).
type UserRemove struct {
	Session uint32
	Actor   uint32
	Reason  string
	Ban     bool
}

func (m *UserRemove) Marshal() ([]byte, error) {
	var b []byte
	b = wire.AppendTag(b, 1, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(m.Session))
	if m.Actor != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Actor))
	}
	if m.Reason != "" {
		b = wire.AppendString(b, 3, m.Reason)
	}
	if m.Ban {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *UserRemove) Unmarshal(data []byte) error {
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
			m.Actor = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Reason = string(s)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Ban = v != 0
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

// UserState.VolumeAdjustment
type VolumeAdjustment struct {
	ListeningChannel   uint32
	VolumeAdjustment   float32
}

// UserState field presence bits (which fields were present in the wire data).
const (
	UserStateSetSession   uint32 = 1 << 0
	UserStateSetActor     uint32 = 1 << 1
	UserStateSetName      uint32 = 1 << 2
	UserStateSetUserID    uint32 = 1 << 3
	UserStateSetChannelID uint32 = 1 << 4
	UserStateSetMute      uint32 = 1 << 5
	UserStateSetDeaf      uint32 = 1 << 6
	UserStateSetSuppress  uint32 = 1 << 7
	UserStateSetSelfMute  uint32 = 1 << 8
	UserStateSetSelfDeaf  uint32 = 1 << 9
	UserStateSetTexture   uint32 = 1 << 10
	UserStateSetPluginContext  uint32 = 1 << 11
	UserStateSetPluginIdentity uint32 = 1 << 12
	UserStateSetComment   uint32 = 1 << 13
)

// UserState (type 9).
type UserState struct {
	Session                  uint32
	Actor                    uint32
	Name                     string
	UserID                   uint32
	ChannelID                uint32
	Mute                     bool
	Deaf                     bool
	Suppress                 bool
	SelfMute                 bool
	SelfDeaf                 bool
	Texture                  []byte
	PluginContext            []byte
	PluginIdentity           string
	Comment                  string
	Hash                     string
	CommentHash              []byte
	TextureHash              []byte
	PrioritySpeaker          bool
	Recording                bool
	TemporaryAccessTokens    []string
	ListeningChannelAdd      []uint32
	ListeningChannelRemove   []uint32
	ListeningVolumeAdjustment []VolumeAdjustment

	// SetFields indicates which fields were present in the wire data during Unmarshal.
	// Used by handlers to avoid overwriting with default zero values.
	SetFields uint32
}

func (m *UserState) Marshal() ([]byte, error) {
	var b []byte
	if m.Session != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	if m.Actor != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Actor))
	}
	if m.Name != "" {
		b = wire.AppendString(b, 3, m.Name)
	}
	if m.UserID != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.UserID))
	}
	if m.ChannelID != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ChannelID))
	}
	if m.Mute {
		b = wire.AppendTag(b, 6, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.Deaf {
		b = wire.AppendTag(b, 7, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.Suppress {
		b = wire.AppendTag(b, 8, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.SelfMute {
		b = wire.AppendTag(b, 9, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.SelfDeaf {
		b = wire.AppendTag(b, 10, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if len(m.Texture) > 0 {
		b = wire.AppendBytes(b, 11, m.Texture)
	}
	if len(m.PluginContext) > 0 {
		b = wire.AppendBytes(b, 12, m.PluginContext)
	}
	if m.PluginIdentity != "" {
		b = wire.AppendString(b, 13, m.PluginIdentity)
	}
	if m.Comment != "" {
		b = wire.AppendString(b, 14, m.Comment)
	}
	if m.Hash != "" {
		b = wire.AppendString(b, 15, m.Hash)
	}
	if len(m.CommentHash) > 0 {
		b = wire.AppendBytes(b, 16, m.CommentHash)
	}
	if len(m.TextureHash) > 0 {
		b = wire.AppendBytes(b, 17, m.TextureHash)
	}
	if m.PrioritySpeaker {
		b = wire.AppendTag(b, 18, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.Recording {
		b = wire.AppendTag(b, 19, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	for _, t := range m.TemporaryAccessTokens {
		b = wire.AppendString(b, 20, t)
	}
	for _, v := range m.ListeningChannelAdd {
		b = wire.AppendTag(b, 21, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.ListeningChannelRemove {
		b = wire.AppendTag(b, 22, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, va := range m.ListeningVolumeAdjustment {
		var vaBuf []byte
		if va.ListeningChannel != 0 {
			vaBuf = wire.AppendTag(vaBuf, 1, wire.WireVarint)
			vaBuf = wire.AppendVarint(vaBuf, uint64(va.ListeningChannel))
		}
		if va.VolumeAdjustment != 0 {
			vaBuf = wire.AppendTag(vaBuf, 2, wire.WireFixed32)
			vaBuf = wire.AppendFixed32(vaBuf, wire.Float32bits(va.VolumeAdjustment))
		}
		if len(vaBuf) > 0 {
			b = wire.AppendBytes(b, 23, vaBuf)
		}
	}
	return b, nil
}

func (m *UserState) Unmarshal(data []byte) error {
	b := data
	m.SetFields = 0
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
			m.SetFields |= UserStateSetSession
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Actor = uint32(v)
			m.SetFields |= UserStateSetActor
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Name = string(s)
			m.SetFields |= UserStateSetName
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.UserID = uint32(v)
			m.SetFields |= UserStateSetUserID
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ChannelID = uint32(v)
			m.SetFields |= UserStateSetChannelID
		case 6:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Mute = v != 0
			m.SetFields |= UserStateSetMute
		case 7:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Deaf = v != 0
			m.SetFields |= UserStateSetDeaf
		case 8:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Suppress = v != 0
			m.SetFields |= UserStateSetSuppress
		case 9:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.SelfMute = v != 0
			m.SetFields |= UserStateSetSelfMute
		case 10:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.SelfDeaf = v != 0
			m.SetFields |= UserStateSetSelfDeaf
		case 11:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Texture = bb
			m.SetFields |= UserStateSetTexture
		case 12:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PluginContext = bb
			m.SetFields |= UserStateSetPluginContext
		case 13:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PluginIdentity = string(s)
			m.SetFields |= UserStateSetPluginIdentity
		case 14:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Comment = string(s)
			m.SetFields |= UserStateSetComment
		case 15:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Hash = string(s)
		case 16:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.CommentHash = bb
		case 17:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.TextureHash = bb
		case 18:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PrioritySpeaker = v != 0
		case 19:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Recording = v != 0
		case 20:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.TemporaryAccessTokens = append(m.TemporaryAccessTokens, string(s))
		case 21:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ListeningChannelAdd = append(m.ListeningChannelAdd, uint32(v))
		case 22:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ListeningChannelRemove = append(m.ListeningChannelRemove, uint32(v))
		case 23:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			var va VolumeAdjustment
			_ = va.unmarshal(bb)
			m.ListeningVolumeAdjustment = append(m.ListeningVolumeAdjustment, va)
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

func (va *VolumeAdjustment) unmarshal(b []byte) error {
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
			va.ListeningChannel = uint32(v)
		case 2:
			if len(b) >= 4 {
				va.VolumeAdjustment = wire.Float32frombits(wire.ReadFixed32(b))
				b = b[4:]
			}
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
