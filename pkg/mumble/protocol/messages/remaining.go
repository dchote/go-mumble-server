package messages

import "github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"

// QueryUsers (type 14).
type QueryUsers struct {
	IDs   []uint32
	Names []string
}

func (m *QueryUsers) Marshal() ([]byte, error) {
	var b []byte
	for _, v := range m.IDs {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, s := range m.Names {
		b = wire.AppendString(b, 2, s)
	}
	return b, nil
}

func (m *QueryUsers) Unmarshal(data []byte) error {
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
			m.IDs = append(m.IDs, uint32(v))
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Names = append(m.Names, string(s))
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

// CodecVersion (type 21).
type CodecVersion struct {
	Alpha      int32
	Beta       int32
	PreferAlpha bool
	Opus       bool
}

func (m *CodecVersion) Marshal() ([]byte, error) {
	var b []byte
	b = wire.AppendTag(b, 1, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(int32(m.Alpha)))
	b = wire.AppendTag(b, 2, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(int32(m.Beta)))
	b = wire.AppendTag(b, 3, wire.WireVarint)
	if m.PreferAlpha {
		b = wire.AppendVarint(b, 1)
	} else {
		b = wire.AppendVarint(b, 0)
	}
	if m.Opus {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *CodecVersion) Unmarshal(data []byte) error {
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
			m.Alpha = int32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Beta = int32(v)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PreferAlpha = v != 0
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Opus = v != 0
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

// ServerConfig (type 24).
type ServerConfig struct {
	MaxBandwidth         uint32
	WelcomeText          string
	AllowHTML            bool
	MessageLength        uint32
	ImageMessageLength   uint32
	MaxUsers             uint32
	RecordingAllowed     bool
}

func (m *ServerConfig) Marshal() ([]byte, error) {
	var b []byte
	if m.MaxBandwidth != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.MaxBandwidth))
	}
	if m.WelcomeText != "" {
		b = wire.AppendString(b, 2, m.WelcomeText)
	}
	if m.AllowHTML {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.MessageLength != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.MessageLength))
	}
	if m.ImageMessageLength != 0 {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ImageMessageLength))
	}
	if m.MaxUsers != 0 {
		b = wire.AppendTag(b, 6, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.MaxUsers))
	}
	if m.RecordingAllowed {
		b = wire.AppendTag(b, 7, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *ServerConfig) Unmarshal(data []byte) error {
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
			m.MaxBandwidth = uint32(v)
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.WelcomeText = string(s)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.AllowHTML = v != 0
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.MessageLength = uint32(v)
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ImageMessageLength = uint32(v)
		case 6:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.MaxUsers = uint32(v)
		case 7:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.RecordingAllowed = v != 0
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

// ACL, VoiceTarget, ContextAction, etc. - stubs that satisfy Message interface
// Full implementation can be added as needed. For now we provide minimal stubs
// so the protocol layer compiles.

// ACL (type 13) - minimal for handlers to receive.
type ACL struct {
	ChannelID   uint32
	InheritACLs bool
	Query       bool
}

func (m *ACL) Marshal() ([]byte, error) {
	var b []byte
	b = wire.AppendTag(b, 1, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(m.ChannelID))
	if !m.InheritACLs {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, 0)
	}
	if m.Query {
		b = wire.AppendTag(b, 5, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *ACL) Unmarshal(data []byte) error {
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
			m.InheritACLs = v != 0
		case 5:
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

// VoiceTarget (type 19).
type VoiceTarget struct {
	ID      uint32
	Targets []VoiceTargetTarget
}

type VoiceTargetTarget struct {
	Session   []uint32
	ChannelID uint32
	Group     string
	Links     bool
	Children  bool
}

func (m *VoiceTarget) Marshal() ([]byte, error) {
	var b []byte
	if m.ID != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.ID))
	}
	for _, t := range m.Targets {
		var tb []byte
		for _, s := range t.Session {
			tb = wire.AppendTag(tb, 1, wire.WireVarint)
			tb = wire.AppendVarint(tb, uint64(s))
		}
		// Always write channel_id (root is 0)
		tb = wire.AppendTag(tb, 2, wire.WireVarint)
		tb = wire.AppendVarint(tb, uint64(t.ChannelID))
		if t.Group != "" {
			tb = wire.AppendString(tb, 3, t.Group)
		}
		if t.Links {
			tb = wire.AppendTag(tb, 4, wire.WireVarint)
			tb = wire.AppendVarint(tb, 1)
		}
		if t.Children {
			tb = wire.AppendTag(tb, 5, wire.WireVarint)
			tb = wire.AppendVarint(tb, 1)
		}
		if len(tb) > 0 {
			b = wire.AppendBytes(b, 2, tb)
		}
	}
	return b, nil
}

func (m *VoiceTarget) Unmarshal(data []byte) error {
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
			m.ID = uint32(v)
		case 2:
			tb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			var t VoiceTargetTarget
			t.unmarshal(tb)
			m.Targets = append(m.Targets, t)
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

func (t *VoiceTargetTarget) unmarshal(data []byte) error {
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
			t.Session = append(t.Session, uint32(v))
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			t.ChannelID = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			t.Group = string(s)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			t.Links = v != 0
		case 5:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			t.Children = v != 0
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

// PermissionQuery (type 20).
type PermissionQuery struct {
	ChannelID   uint32
	Permissions uint32
	Flush       bool
}

func (m *PermissionQuery) Marshal() ([]byte, error) {
	var b []byte
	// Always write channel_id (root is 0)
	b = wire.AppendTag(b, 1, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(m.ChannelID))
	if m.Permissions != 0 {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Permissions))
	}
	if m.Flush {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	return b, nil
}

func (m *PermissionQuery) Unmarshal(data []byte) error {
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
			m.Permissions = uint32(v)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Flush = v != 0
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

// ContextAction (type 17).
type ContextAction struct {
	Session   uint32
	ChannelID uint32
	Action    string
}

func (m *ContextAction) Marshal() ([]byte, error) {
	var b []byte
	if m.Session != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	// Always write channel_id (root is 0)
	b = wire.AppendTag(b, 2, wire.WireVarint)
	b = wire.AppendVarint(b, uint64(m.ChannelID))
	b = wire.AppendString(b, 3, m.Action)
	return b, nil
}

func (m *ContextAction) Unmarshal(data []byte) error {
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
			m.ChannelID = uint32(v)
		case 3:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Action = string(s)
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

// RequestBlob (type 23).
type RequestBlob struct {
	SessionTexture      []uint32
	SessionComment      []uint32
	ChannelDescription  []uint32
}

func (m *RequestBlob) Marshal() ([]byte, error) {
	var b []byte
	for _, v := range m.SessionTexture {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.SessionComment {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	for _, v := range m.ChannelDescription {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	return b, nil
}

func (m *RequestBlob) Unmarshal(data []byte) error {
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
			m.SessionTexture = append(m.SessionTexture, uint32(v))
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.SessionComment = append(m.SessionComment, uint32(v))
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ChannelDescription = append(m.ChannelDescription, uint32(v))
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

// SuggestConfig (type 25).
type SuggestConfig struct {
	VersionV1  uint32
	VersionV2  uint64
	Positional bool
	PushToTalk bool
}

func (m *SuggestConfig) Marshal() ([]byte, error) {
	var b []byte
	if m.VersionV1 != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.VersionV1))
	}
	if m.Positional {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.PushToTalk {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, 1)
	}
	if m.VersionV2 != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, m.VersionV2)
	}
	return b, nil
}

func (m *SuggestConfig) Unmarshal(data []byte) error {
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
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Positional = v != 0
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.PushToTalk = v != 0
		case 4:
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

// PluginDataTransmission (type 26).
type PluginDataTransmission struct {
	SenderSession     uint32
	ReceiverSessions  []uint32
	Data              []byte
	DataID            string
}

func (m *PluginDataTransmission) Marshal() ([]byte, error) {
	var b []byte
	if m.SenderSession != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.SenderSession))
	}
	for _, v := range m.ReceiverSessions {
		b = wire.AppendTag(b, 2, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(v))
	}
	if len(m.Data) > 0 {
		b = wire.AppendBytes(b, 3, m.Data)
	}
	if m.DataID != "" {
		b = wire.AppendString(b, 4, m.DataID)
	}
	return b, nil
}

func (m *PluginDataTransmission) Unmarshal(data []byte) error {
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
			m.SenderSession = uint32(v)
		case 2:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.ReceiverSessions = append(m.ReceiverSessions, uint32(v))
		case 3:
			bb, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Data = bb
		case 4:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.DataID = string(s)
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

// UserList (type 18) - minimal stub.
type UserList struct{}

func (m *UserList) Marshal() ([]byte, error) { return nil, nil }
func (m *UserList) Unmarshal([]byte) error   { return nil }

// UserStats (type 22) - minimal stub.
type UserStats struct {
	Session uint32
}

func (m *UserStats) Marshal() ([]byte, error) {
	var b []byte
	if m.Session != 0 {
		b = wire.AppendTag(b, 1, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Session))
	}
	return b, nil
}

func (m *UserStats) Unmarshal(data []byte) error {
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

// ContextActionModify (type 16) - minimal stub for server-sent.
type ContextActionModify struct {
	Action    string
	Text      string
	Context   uint32
	Operation uint32
}

func (m *ContextActionModify) Marshal() ([]byte, error) {
	var b []byte
	b = wire.AppendString(b, 1, m.Action)
	if m.Text != "" {
		b = wire.AppendString(b, 2, m.Text)
	}
	if m.Context != 0 {
		b = wire.AppendTag(b, 3, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Context))
	}
	if m.Operation != 0 {
		b = wire.AppendTag(b, 4, wire.WireVarint)
		b = wire.AppendVarint(b, uint64(m.Operation))
	}
	return b, nil
}

func (m *ContextActionModify) Unmarshal(data []byte) error {
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
			m.Action = string(s)
		case 2:
			s, n, err := wire.ReadLengthDelimited(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Text = string(s)
		case 3:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Context = uint32(v)
		case 4:
			v, n, err := wire.ReadVarint(b)
			if err != nil {
				return err
			}
			b = b[n:]
			m.Operation = uint32(v)
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
