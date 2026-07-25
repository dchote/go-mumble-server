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
	ListeningChannel uint32
	VolumeAdjustment float32
}

// UserState field presence bits ("has bits"). Mumble.proto is proto2, where every
// UserState field is optional with explicit presence: an explicitly assigned false
// or zero is still written to the wire, and absent is distinct from default. These
// bits carry that presence in both directions - Unmarshal records which fields the
// peer sent, and Marshal uses them to emit explicitly-set defaults.
const (
	UserStateSetSession         uint32 = 1 << 0
	UserStateSetActor           uint32 = 1 << 1
	UserStateSetName            uint32 = 1 << 2
	UserStateSetUserID          uint32 = 1 << 3
	UserStateSetChannelID       uint32 = 1 << 4
	UserStateSetMute            uint32 = 1 << 5
	UserStateSetDeaf            uint32 = 1 << 6
	UserStateSetSuppress        uint32 = 1 << 7
	UserStateSetSelfMute        uint32 = 1 << 8
	UserStateSetSelfDeaf        uint32 = 1 << 9
	UserStateSetTexture         uint32 = 1 << 10
	UserStateSetPluginContext   uint32 = 1 << 11
	UserStateSetPluginIdentity  uint32 = 1 << 12
	UserStateSetComment         uint32 = 1 << 13
	UserStateSetHash            uint32 = 1 << 14
	UserStateSetCommentHash     uint32 = 1 << 15
	UserStateSetTextureHash     uint32 = 1 << 16
	UserStateSetPrioritySpeaker uint32 = 1 << 17
	UserStateSetRecording       uint32 = 1 << 18
)

// UserState (type 9).
type UserState struct {
	Session                   uint32
	Actor                     uint32
	Name                      string
	UserID                    uint32
	ChannelID                 uint32
	Mute                      bool
	Deaf                      bool
	Suppress                  bool
	SelfMute                  bool
	SelfDeaf                  bool
	Texture                   []byte
	PluginContext             []byte
	PluginIdentity            string
	Comment                   string
	Hash                      string
	CommentHash               []byte
	TextureHash               []byte
	PrioritySpeaker           bool
	Recording                 bool
	TemporaryAccessTokens     []string
	ListeningChannelAdd       []uint32
	ListeningChannelRemove    []uint32
	ListeningVolumeAdjustment []VolumeAdjustment

	// SetFields carries proto2 field presence. See the UserStateSet* bits.
	SetFields uint32
}

// Has reports whether the given presence bit is set.
func (m *UserState) Has(bit uint32) bool { return m.SetFields&bit != 0 }

// appendBool writes a proto2 optional bool. The field is emitted when its presence
// bit is set (so an explicit false reaches the peer) or when the value is true.
func (m *UserState) appendBool(b []byte, field int, bit uint32, v bool) []byte {
	if !m.Has(bit) && !v {
		return b
	}
	b = wire.AppendTag(b, field, wire.WireVarint)
	if v {
		return wire.AppendVarint(b, 1)
	}
	return wire.AppendVarint(b, 0)
}

func (m *UserState) Marshal() ([]byte, error) {
	var b []byte
	// Session uses presence like other proto2 fields. Server snapshots set the
	// Session has-bit (see userToState). Client-originated mute toggles often omit
	// session entirely to mean "self"; always writing session=0 would mis-target them.
	if m.Has(UserStateSetSession) || m.Session != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	if m.Has(UserStateSetActor) || m.Actor != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Actor))
	}
	if m.Has(UserStateSetName) || m.Name != "" {
		b = wire.AppendString(b, 3, m.Name)
	}
	if m.Has(UserStateSetUserID) || m.UserID != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.UserID))
	}
	// channel_id 0 is a real value (root). Server snapshots set the ChannelID
	// has-bit; client mute toggles omit channel_id entirely.
	if m.Has(UserStateSetChannelID) || m.ChannelID != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ChannelID))
	}
	b = m.appendBool(b, 6, UserStateSetMute, m.Mute)
	b = m.appendBool(b, 7, UserStateSetDeaf, m.Deaf)
	b = m.appendBool(b, 8, UserStateSetSuppress, m.Suppress)
	b = m.appendBool(b, 9, UserStateSetSelfMute, m.SelfMute)
	b = m.appendBool(b, 10, UserStateSetSelfDeaf, m.SelfDeaf)
	if m.Has(UserStateSetTexture) || len(m.Texture) > 0 {
		b = wire.AppendBytes(b, 11, m.Texture)
	}
	if m.Has(UserStateSetPluginContext) || len(m.PluginContext) > 0 {
		b = wire.AppendBytes(b, 12, m.PluginContext)
	}
	if m.Has(UserStateSetPluginIdentity) || m.PluginIdentity != "" {
		b = wire.AppendString(b, 13, m.PluginIdentity)
	}
	if m.Has(UserStateSetComment) || m.Comment != "" {
		b = wire.AppendString(b, 14, m.Comment)
	}
	if m.Has(UserStateSetHash) || m.Hash != "" {
		b = wire.AppendString(b, 15, m.Hash)
	}
	if m.Has(UserStateSetCommentHash) || len(m.CommentHash) > 0 {
		b = wire.AppendBytes(b, 16, m.CommentHash)
	}
	if m.Has(UserStateSetTextureHash) || len(m.TextureHash) > 0 {
		b = wire.AppendBytes(b, 17, m.TextureHash)
	}
	b = m.appendBool(b, 18, UserStateSetPrioritySpeaker, m.PrioritySpeaker)
	b = m.appendBool(b, 19, UserStateSetRecording, m.Recording)
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
		// Always write listening_channel (root is 0)
		vaBuf = wire.AppendTag(vaBuf, 1, wire.WireVarint)
		vaBuf = wire.AppendVarint(vaBuf, uint64(va.ListeningChannel))
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
			m.SetFields |= UserStateSetHash
		case 16:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.CommentHash = bb
			m.SetFields |= UserStateSetCommentHash
		case 17:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.TextureHash = bb
			m.SetFields |= UserStateSetTextureHash
		case 18:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PrioritySpeaker = v != 0
			m.SetFields |= UserStateSetPrioritySpeaker
		case 19:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Recording = v != 0
			m.SetFields |= UserStateSetRecording
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
