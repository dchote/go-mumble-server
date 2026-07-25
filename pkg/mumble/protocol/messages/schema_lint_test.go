package messages

// Schema conformance lint.
//
// This file guards against wire-protocol drift between our hand-written
// codecs (this package) and the upstream Mumble.proto schema vendored at
// testdata/Mumble.proto. It has three layers:
//
//  1. Every proto message we implement a Go type for must have every one of
//     its Go struct fields explicitly classified in schemaFieldMap, either
//     as "maps to proto field X" or "is a synthetic Go-only field with no
//     wire representation of its own" (empty proto name).
//  2. Every proto field we claim to implement must exist in the vendored
//     schema with the field number we think it has. Every proto field we do
//     NOT implement must be explicitly listed in unimplementedFields (a
//     deliberate, documented gap) or the test fails - this is what catches
//     newly added upstream fields we haven't triaged yet.
//  3. For every top-level message (one that implements the Message
//     interface), a hand-built fixture that populates every implemented
//     field is marshaled and the resulting bytes are scanned tag-by-tag to
//     confirm the field numbers and wire types we actually emit match the
//     schema - this catches a mismatch between Marshal() and the mapping
//     tables above, i.e. real encoding bugs, not just bookkeeping errors.
//
// Every proto message defined upstream must also be accounted for, either
// by a Go type (schemaFieldMap/goTypeToProtoMessage) or by unimplementedMessages.
//
// When upstream Mumble.proto changes, re-vendor testdata/Mumble.proto (see
// its header) and run this package's tests; failures point at exactly what
// needs to be triaged.

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/dchote/go-mumble-server/pkg/mumble/protocol/wire"
)

// ---------------------------------------------------------------------------
// Minimal Mumble.proto parser.
// ---------------------------------------------------------------------------

type protoField struct {
	Name     string
	Number   int
	WireType int
	Packed   bool
}

type protoMessage struct {
	Name   string
	Fields map[string]protoField // by proto field name
}

// scalarWireTypes maps proto2 scalar/bytes type keywords to their wire type.
// Anything else is either a known enum (varint) or an embedded message
// (length-delimited), resolved during parsing.
var scalarWireTypes = map[string]int{
	"double":   wire.WireFixed64,
	"float":    wire.WireFixed32,
	"int32":    wire.WireVarint,
	"int64":    wire.WireVarint,
	"uint32":   wire.WireVarint,
	"uint64":   wire.WireVarint,
	"sint32":   wire.WireVarint,
	"sint64":   wire.WireVarint,
	"fixed32":  wire.WireFixed32,
	"fixed64":  wire.WireFixed64,
	"sfixed32": wire.WireFixed32,
	"sfixed64": wire.WireFixed64,
	"bool":     wire.WireVarint,
	"string":   wire.WireLengthDelimited,
	"bytes":    wire.WireLengthDelimited,
}

var (
	reMessageOpen = regexp.MustCompile(`^\s*message\s+(\w+)\s*\{`)
	reEnumOpen    = regexp.MustCompile(`^\s*enum\s+(\w+)\s*\{`)
	reBraceClose  = regexp.MustCompile(`^\s*\}`)
	reField       = regexp.MustCompile(`^\s*(?:required|optional|repeated)\s+([\w.]+)\s+(\w+)\s*=\s*(\d+)\s*(\[[^\]]*\])?\s*;`)
)

// parseMumbleProto parses the vendored schema into a flat, bare-name-keyed
// registry of every message (top-level and nested). None of the message
// names in Mumble.proto collide across nesting levels, so a flat namespace
// is unambiguous and much simpler than tracking qualified paths.
func parseMumbleProto(path string) (map[string]*protoMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	type frame struct {
		kind string // "message" or "enum"
		name string
	}

	messages := map[string]*protoMessage{}
	enumNames := map[string]bool{}
	var stack []frame

	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case reMessageOpen.MatchString(line):
			name := reMessageOpen.FindStringSubmatch(line)[1]
			messages[name] = &protoMessage{Name: name, Fields: map[string]protoField{}}
			stack = append(stack, frame{"message", name})
		case reEnumOpen.MatchString(line):
			name := reEnumOpen.FindStringSubmatch(line)[1]
			enumNames[name] = true
			stack = append(stack, frame{"enum", name})
		case reBraceClose.MatchString(line):
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case reField.MatchString(line):
			if len(stack) == 0 || stack[len(stack)-1].kind != "message" {
				continue
			}
			m := reField.FindStringSubmatch(line)
			typeName, fieldName, numStr, opts := m[1], m[2], m[3], m[4]
			num, err := strconv.Atoi(numStr)
			if err != nil {
				return nil, fmt.Errorf("parsing field number in %q: %w", line, err)
			}
			wt, ok := scalarWireTypes[typeName]
			if !ok {
				if enumNames[typeName] {
					wt = wire.WireVarint
				} else {
					// Embedded message reference (possibly forward/self, e.g.
					// nested types referenced by bare name).
					wt = wire.WireLengthDelimited
				}
			}
			cur := messages[stack[len(stack)-1].name]
			cur.Fields[fieldName] = protoField{
				Name:     fieldName,
				Number:   num,
				WireType: wt,
				Packed:   strings.Contains(opts, "packed") && strings.Contains(opts, "true"),
			}
		}
	}
	return messages, nil
}

func mustParseSchema(t testing.TB) map[string]*protoMessage {
	t.Helper()
	schema, err := parseMumbleProto("testdata/Mumble.proto")
	if err != nil {
		t.Fatalf("parsing vendored Mumble.proto: %v", err)
	}
	if len(schema) < 25 {
		t.Fatalf("parsed suspiciously few messages (%d) from testdata/Mumble.proto; parser likely broken", len(schema))
	}
	return schema
}

// ---------------------------------------------------------------------------
// Go <-> proto mapping tables. This is the single place that declares how
// our structs correspond to Mumble.proto; everything below is validated
// against it.
// ---------------------------------------------------------------------------

// goTypeToProtoMessage overrides the proto message name for Go types whose
// name doesn't match the (possibly nested) proto message name directly.
var goTypeToProtoMessage = map[string]string{
	"VoiceTargetTarget": "Target", // Mumble.proto: VoiceTarget.Target
}

// schemaFieldMap declares, per Go type, how each exported struct field maps
// to a Mumble.proto field name. An empty proto name marks a synthetic
// Go-only field (no wire representation of its own): either bookkeeping
// (e.g. SetFields, HasParent) or a deliberate, documented protocol extension
// that upstream Mumble does not define (e.g. Version.CryptoModes).
var schemaFieldMap = map[string]map[string]string{
	"Version": {
		"VersionV1":   "version_v1",
		"VersionV2":   "version_v2",
		"Release":     "release",
		"OS":          "os",
		"OSVersion":   "os_version",
		"CryptoModes": "", // custom extension (field 6); see docs/features/0002-negotiated-security-modes.md
	},
	"Authenticate": {
		"Username":     "username",
		"Password":     "password",
		"Tokens":       "tokens",
		"CeltVersions": "celt_versions",
		"Opus":         "opus",
		"ClientType":   "client_type",
	},
	"Ping": {
		"Timestamp":  "timestamp",
		"Good":       "good",
		"Late":       "late",
		"Lost":       "lost",
		"Resync":     "resync",
		"UDPPackets": "udp_packets",
		"TCPPackets": "tcp_packets",
		"UDPPingAvg": "udp_ping_avg",
		"UDPPingVar": "udp_ping_var",
		"TCPPingAvg": "tcp_ping_avg",
		"TCPPingVar": "tcp_ping_var",
	},
	"Reject": {
		"Type":   "type",
		"Reason": "reason",
	},
	"ServerSync": {
		"Session":      "session",
		"MaxBandwidth": "max_bandwidth",
		"WelcomeText":  "welcome_text",
		"Permissions":  "permissions",
	},
	"ChannelRemove": {
		"ChannelID": "channel_id",
	},
	"ChannelState": {
		"ChannelID":         "channel_id",
		"Parent":            "parent",
		"HasParent":         "", // synthetic: presence flag for optional parent (root omits it)
		"Name":              "name",
		"Links":             "links",
		"Description":       "description",
		"LinksAdd":          "links_add",
		"LinksRemove":       "links_remove",
		"Temporary":         "temporary",
		"Position":          "position",
		"DescriptionHash":   "description_hash",
		"MaxUsers":          "max_users",
		"IsEnterRestricted": "is_enter_restricted",
		"CanEnter":          "can_enter",
	},
	"UserRemove": {
		"Session": "session",
		"Actor":   "actor",
		"Reason":  "reason",
		"Ban":     "ban",
	},
	"UserState": {
		"Session":                   "session",
		"Actor":                     "actor",
		"Name":                      "name",
		"UserID":                    "user_id",
		"ChannelID":                 "channel_id",
		"Mute":                      "mute",
		"Deaf":                      "deaf",
		"Suppress":                  "suppress",
		"SelfMute":                  "self_mute",
		"SelfDeaf":                  "self_deaf",
		"Texture":                   "texture",
		"PluginContext":             "plugin_context",
		"PluginIdentity":            "plugin_identity",
		"Comment":                   "comment",
		"Hash":                      "hash",
		"CommentHash":               "comment_hash",
		"TextureHash":               "texture_hash",
		"PrioritySpeaker":           "priority_speaker",
		"Recording":                 "recording",
		"TemporaryAccessTokens":     "temporary_access_tokens",
		"ListeningChannelAdd":       "listening_channel_add",
		"ListeningChannelRemove":    "listening_channel_remove",
		"ListeningVolumeAdjustment": "listening_volume_adjustment",
		"SetFields":                 "", // synthetic: proto2 field-presence bitmask
	},
	"VolumeAdjustment": {
		"ListeningChannel": "listening_channel",
		"VolumeAdjustment": "volume_adjustment",
	},
	"BanEntry": {
		"Address":  "address",
		"Mask":     "mask",
		"Name":     "name",
		"Hash":     "hash",
		"Reason":   "reason",
		"Start":    "start",
		"Duration": "duration",
	},
	"BanList": {
		"Bans":  "bans",
		"Query": "query",
	},
	"TextMessage": {
		"Actor":     "actor",
		"Session":   "session",
		"ChannelID": "channel_id",
		"TreeID":    "tree_id",
		"Message":   "message",
	},
	"PermissionDenied": {
		"Permission": "permission",
		"ChannelID":  "channel_id",
		"Session":    "session",
		"Reason":     "reason",
		"Type":       "type",
		"Name":       "name",
	},
	"ACL": {
		"ChannelID":   "channel_id",
		"InheritACLs": "inherit_acls",
		"Query":       "query",
	},
	"QueryUsers": {
		"IDs":   "ids",
		"Names": "names",
	},
	"CryptSetup": {
		"Key":         "key",
		"ClientNonce": "client_nonce",
		"ServerNonce": "server_nonce",
	},
	"ContextActionModify": {
		"Action":    "action",
		"Text":      "text",
		"Context":   "context",
		"Operation": "operation",
	},
	"ContextAction": {
		"Session":   "session",
		"ChannelID": "channel_id",
		"Action":    "action",
	},
	"UserList": {},
	"VoiceTarget": {
		"ID":      "id",
		"Targets": "targets",
	},
	"VoiceTargetTarget": {
		"Session":   "session",
		"ChannelID": "channel_id",
		"Group":     "group",
		"Links":     "links",
		"Children":  "children",
	},
	"PermissionQuery": {
		"ChannelID":   "channel_id",
		"Permissions": "permissions",
		"Flush":       "flush",
	},
	"CodecVersion": {
		"Alpha":       "alpha",
		"Beta":        "beta",
		"PreferAlpha": "prefer_alpha",
		"Opus":        "opus",
	},
	"UserStats": {
		"Session": "session",
	},
	"RequestBlob": {
		"SessionTexture":     "session_texture",
		"SessionComment":     "session_comment",
		"ChannelDescription": "channel_description",
	},
	"ServerConfig": {
		"MaxBandwidth":       "max_bandwidth",
		"WelcomeText":        "welcome_text",
		"AllowHTML":          "allow_html",
		"MessageLength":      "message_length",
		"ImageMessageLength": "image_message_length",
		"MaxUsers":           "max_users",
		"RecordingAllowed":   "recording_allowed",
	},
	"SuggestConfig": {
		"VersionV1":  "version_v1",
		"VersionV2":  "version_v2",
		"Positional": "positional",
		"PushToTalk": "push_to_talk",
	},
	"PluginDataTransmission": {
		"SenderSession":    "senderSession",
		"ReceiverSessions": "receiverSessions",
		"Data":             "data",
		"DataID":           "dataID",
	},
}

// unimplementedFields lists proto fields that exist upstream but have no
// corresponding Go struct field yet. Each entry here is a deliberate,
// tracked gap, not a bug. If a field is removed from this list it must
// gain a real implementation (and a schemaFieldMap entry); if a new
// upstream field appears, the lint fails until it is triaged into either
// schemaFieldMap or this list.
var unimplementedFields = map[string][]string{
	"UserRemove": {"ban_certificate", "ban_ip"},
	"ACL":        {"groups", "acls"}, // ACL group/entry editing via this message is not implemented
	"UserList":   {"users"},          // registered-user listing via this message is not implemented
	"UserStats": {
		"stats_only", "certificates", "from_client", "from_server",
		"udp_packets", "tcp_packets", "udp_ping_avg", "udp_ping_var",
		"tcp_ping_avg", "tcp_ping_var", "version", "celt_versions",
		"address", "bandwidth", "onlinesecs", "idlesecs",
		"strong_certificate", "opus", "rolling_stats",
	},
}

// unimplementedMessages lists proto messages with no corresponding Go type
// at all (whole sub-features not implemented), plus UDPTunnel, which is
// deliberately unstructured (see messages.go).
var unimplementedMessages = map[string]bool{
	"UDPTunnel":    true,
	"ChanGroup":    true, // nested in ACL
	"ChanACL":      true, // nested in ACL
	"User":         true, // nested in UserList
	"Stats":        true, // nested in UserStats
	"RollingStats": true, // nested in UserStats
}

// packedWireTolerant lists (message, field) pairs declared `[packed = true]`
// upstream where our encoder emits the field unpacked (repeated
// varint-wire-type tags) instead of a single packed length-delimited run.
// Both forms decode identically per the protobuf spec (parsers must accept
// unpacked encodings of packed fields for backward compatibility), so this
// is not a bug, just a wire-type exception for the marshal probe below.
var packedWireTolerant = map[string]bool{
	"PluginDataTransmission.receiverSessions": true,
}

// ---------------------------------------------------------------------------
// Layer 1 + 2: static mapping validation.
// ---------------------------------------------------------------------------

func protoMessageFor(goTypeName string) string {
	if alias, ok := goTypeToProtoMessage[goTypeName]; ok {
		return alias
	}
	return goTypeName
}

// goTypesUnderTest lists every Go type schema_lint validates, mapped to its
// reflect.Type. Nested-only types (no exported Marshal) are included for
// static field-mapping validation but excluded from the wire-probe test.
var goTypesUnderTest = map[string]reflect.Type{
	"Version":                reflect.TypeOf(Version{}),
	"Authenticate":           reflect.TypeOf(Authenticate{}),
	"Ping":                   reflect.TypeOf(Ping{}),
	"Reject":                 reflect.TypeOf(Reject{}),
	"ServerSync":             reflect.TypeOf(ServerSync{}),
	"ChannelRemove":          reflect.TypeOf(ChannelRemove{}),
	"ChannelState":           reflect.TypeOf(ChannelState{}),
	"UserRemove":             reflect.TypeOf(UserRemove{}),
	"UserState":              reflect.TypeOf(UserState{}),
	"VolumeAdjustment":       reflect.TypeOf(VolumeAdjustment{}),
	"BanEntry":               reflect.TypeOf(BanEntry{}),
	"BanList":                reflect.TypeOf(BanList{}),
	"TextMessage":            reflect.TypeOf(TextMessage{}),
	"PermissionDenied":       reflect.TypeOf(PermissionDenied{}),
	"ACL":                    reflect.TypeOf(ACL{}),
	"QueryUsers":             reflect.TypeOf(QueryUsers{}),
	"CryptSetup":             reflect.TypeOf(CryptSetup{}),
	"ContextActionModify":    reflect.TypeOf(ContextActionModify{}),
	"ContextAction":          reflect.TypeOf(ContextAction{}),
	"UserList":               reflect.TypeOf(UserList{}),
	"VoiceTarget":            reflect.TypeOf(VoiceTarget{}),
	"VoiceTargetTarget":      reflect.TypeOf(VoiceTargetTarget{}),
	"PermissionQuery":        reflect.TypeOf(PermissionQuery{}),
	"CodecVersion":           reflect.TypeOf(CodecVersion{}),
	"UserStats":              reflect.TypeOf(UserStats{}),
	"RequestBlob":            reflect.TypeOf(RequestBlob{}),
	"ServerConfig":           reflect.TypeOf(ServerConfig{}),
	"SuggestConfig":          reflect.TypeOf(SuggestConfig{}),
	"PluginDataTransmission": reflect.TypeOf(PluginDataTransmission{}),
}

func TestSchemaLint_AllProtoMessagesCovered(t *testing.T) {
	schema := mustParseSchema(t)

	covered := map[string]bool{}
	for goName := range goTypesUnderTest {
		covered[protoMessageFor(goName)] = true
	}

	var missing []string
	for name := range schema {
		if covered[name] || unimplementedMessages[name] {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("Mumble.proto defines message(s) %v with no corresponding Go type and no entry in unimplementedMessages; "+
			"add a Go type + schemaFieldMap entry, or add to unimplementedMessages if the gap is deliberate", missing)
	}
}

func TestSchemaLint_StaticFieldMapping(t *testing.T) {
	schema := mustParseSchema(t)

	for goName, goType := range goTypesUnderTest {
		goName, goType := goName, goType
		t.Run(goName, func(t *testing.T) {
			protoName := protoMessageFor(goName)
			pm, ok := schema[protoName]
			if !ok {
				t.Fatalf("no Mumble.proto message %q for Go type %s (check goTypeToProtoMessage)", protoName, goName)
			}
			mapping, ok := schemaFieldMap[goName]
			if !ok {
				t.Fatalf("no schemaFieldMap entry for Go type %s", goName)
			}

			// Every exported Go struct field must be classified.
			implementedProtoNames := map[string]bool{}
			for i := 0; i < goType.NumField(); i++ {
				f := goType.Field(i)
				if !f.IsExported() {
					continue
				}
				protoFieldName, known := mapping[f.Name]
				if !known {
					t.Errorf("%s.%s has no schemaFieldMap entry (map to a proto field name, or \"\" if synthetic)", goName, f.Name)
					continue
				}
				if protoFieldName == "" {
					continue // synthetic/documented extension
				}
				pf, ok := pm.Fields[protoFieldName]
				if !ok {
					t.Errorf("%s.%s maps to proto field %q which does not exist on message %s", goName, f.Name, protoFieldName, protoName)
					continue
				}
				implementedProtoNames[protoFieldName] = true
				_ = pf // number is cross-checked by the wire probe test for top-level types
			}

			// Every declared mapping entry must correspond to a real Go field
			// (catches stale mapping entries after a rename).
			for goField := range mapping {
				if _, ok := goType.FieldByName(goField); !ok {
					t.Errorf("schemaFieldMap[%s] references non-existent Go field %q", goName, goField)
				}
			}

			// Every proto field must be either implemented or an acknowledged gap.
			allowlisted := map[string]bool{}
			for _, f := range unimplementedFields[goName] {
				allowlisted[f] = true
			}
			var unaccounted []string
			for name := range pm.Fields {
				if implementedProtoNames[name] || allowlisted[name] {
					continue
				}
				unaccounted = append(unaccounted, name)
			}
			sort.Strings(unaccounted)
			if len(unaccounted) > 0 {
				t.Errorf("proto message %s has field(s) %v not implemented and not listed in unimplementedFields[%q]", protoName, unaccounted, goName)
			}

			// Allowlist entries that no longer exist upstream are stale; keep
			// the allowlist honest so it doesn't silently mask new gaps.
			for _, f := range unimplementedFields[goName] {
				if _, ok := pm.Fields[f]; !ok {
					t.Errorf("unimplementedFields[%q] references %q which no longer exists on proto message %s", goName, f, protoName)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Layer 3: wire-level marshal probe for top-level messages.
// ---------------------------------------------------------------------------

// fixtures builds one fully-populated instance per top-level message type,
// exercising every implemented (non-synthetic) field so its wire tag shows
// up in the marshaled output. Values are chosen only to be "truthy" for
// whatever conditional each Marshal() uses to decide whether to emit a
// field; they carry no other significance.
var fixtures = map[string]Message{
	"Version": &Version{
		VersionV1: 1, VersionV2: 2, Release: "r", OS: "os", OSVersion: "1.0",
	},
	"Authenticate": &Authenticate{
		Username: "u", Password: "p", Tokens: []string{"t"}, CeltVersions: []int32{1}, Opus: true, ClientType: 1,
	},
	"Ping": &Ping{
		Timestamp: 1, Good: 2, Late: 3, Lost: 4, Resync: 5, UDPPackets: 6, TCPPackets: 7,
		UDPPingAvg: 1.5, UDPPingVar: 1.5, TCPPingAvg: 1.5, TCPPingVar: 1.5,
	},
	"Reject": &Reject{Type: RejectServerFull, Reason: "r"},
	"ServerSync": &ServerSync{
		Session: 1, MaxBandwidth: 2, WelcomeText: "w", Permissions: 3,
	},
	"ChannelRemove": &ChannelRemove{ChannelID: 1},
	"ChannelState": &ChannelState{
		ChannelID: 1, Parent: 2, HasParent: true, Name: "n", Links: []uint32{1},
		Description: "d", LinksAdd: []uint32{2}, LinksRemove: []uint32{3},
		Temporary: true, Position: 5, DescriptionHash: []byte{0xAB}, MaxUsers: 10,
		IsEnterRestricted: true, CanEnter: true,
	},
	"UserRemove": &UserRemove{Session: 1, Actor: 2, Reason: "r", Ban: true},
	"UserState": &UserState{
		Session: 1, Actor: 2, Name: "n", UserID: 3, ChannelID: 4,
		Mute: true, Deaf: true, Suppress: true, SelfMute: true, SelfDeaf: true,
		Texture: []byte{1}, PluginContext: []byte{2}, PluginIdentity: "pi",
		Comment: "c", Hash: "h", CommentHash: []byte{3}, TextureHash: []byte{4},
		PrioritySpeaker: true, Recording: true,
		TemporaryAccessTokens: []string{"t"}, ListeningChannelAdd: []uint32{5},
		ListeningChannelRemove:    []uint32{6},
		ListeningVolumeAdjustment: []VolumeAdjustment{{ListeningChannel: 7, VolumeAdjustment: 1.5}},
	},
	"BanList": &BanList{
		Bans:  []BanEntry{{Address: []byte{1}, Mask: 8, Name: "n", Hash: "h", Reason: "r", Start: "s", Duration: 60}},
		Query: true,
	},
	"TextMessage": &TextMessage{
		Actor: 1, Session: []uint32{2}, ChannelID: []uint32{3}, TreeID: []uint32{4}, Message: "m",
	},
	"PermissionDenied": &PermissionDenied{
		Permission: 1, ChannelID: 2, Session: 3, Reason: "r", Type: DenyPermission, Name: "n",
	},
	"ACL": &ACL{ChannelID: 1, InheritACLs: false, Query: true},
	"QueryUsers": &QueryUsers{
		IDs: []uint32{1}, Names: []string{"n"},
	},
	"CryptSetup": &CryptSetup{Key: []byte{1}, ClientNonce: []byte{2}, ServerNonce: []byte{3}},
	"ContextActionModify": &ContextActionModify{
		Action: "a", Text: "t", Context: 1, Operation: 1,
	},
	"ContextAction": &ContextAction{Session: 1, ChannelID: 2, Action: "a"},
	"UserList":      &UserList{},
	"VoiceTarget": &VoiceTarget{
		ID: 1, Targets: []VoiceTargetTarget{{Session: []uint32{5}, ChannelID: 2, Group: "g", Links: true, Children: true}},
	},
	"PermissionQuery": &PermissionQuery{ChannelID: 1, Permissions: 2, Flush: true},
	"CodecVersion":    &CodecVersion{Alpha: 1, Beta: 2, PreferAlpha: true, Opus: true},
	"UserStats":       &UserStats{Session: 1},
	"RequestBlob": &RequestBlob{
		SessionTexture: []uint32{1}, SessionComment: []uint32{2}, ChannelDescription: []uint32{3},
	},
	"ServerConfig": &ServerConfig{
		MaxBandwidth: 1, WelcomeText: "w", AllowHTML: true, MessageLength: 2,
		ImageMessageLength: 3, MaxUsers: 4, RecordingAllowed: true,
	},
	"SuggestConfig": &SuggestConfig{VersionV1: 1, VersionV2: 2, Positional: true, PushToTalk: true},
	"PluginDataTransmission": &PluginDataTransmission{
		SenderSession: 1, ReceiverSessions: []uint32{2}, Data: []byte{3}, DataID: "d",
	},
}

// scanTags decodes every top-level (fieldNumber, wireType) tag present in a
// marshaled message, without recursing into length-delimited payloads.
func scanTags(t testing.TB, data []byte) map[int][]int {
	t.Helper()
	seen := map[int][]int{}
	b := data
	for len(b) > 0 {
		fn, wt, n, err := wire.ReadTag(b)
		if err != nil {
			t.Fatalf("ReadTag: %v", err)
		}
		b = b[n:]
		seen[fn] = append(seen[fn], wt)
		skip, err := wire.SkipField(b, wt)
		if err != nil {
			t.Fatalf("SkipField: %v", err)
		}
		b = b[skip:]
	}
	return seen
}

func TestSchemaLint_WireProbe(t *testing.T) {
	schema := mustParseSchema(t)

	// Every top-level type must have a fixture; keeps the probe exhaustive
	// as new message types are added.
	for goName, goType := range goTypesUnderTest {
		if _, isNestedOnly := map[string]bool{"VolumeAdjustment": true, "BanEntry": true, "VoiceTargetTarget": true}[goName]; isNestedOnly {
			continue
		}
		if _, ok := fixtures[goName]; !ok {
			t.Errorf("no wire-probe fixture for top-level Go type %s", goName)
		}
		_ = goType
	}

	for goName, msg := range fixtures {
		goName, msg := goName, msg
		t.Run(goName, func(t *testing.T) {
			protoName := protoMessageFor(goName)
			pm := schema[protoName]
			mapping := schemaFieldMap[goName]

			data, err := msg.Marshal()
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			seen := scanTags(t, data)

			goType := goTypesUnderTest[goName]
			for i := 0; i < goType.NumField(); i++ {
				f := goType.Field(i)
				if !f.IsExported() {
					continue
				}
				protoFieldName := mapping[f.Name]
				if protoFieldName == "" {
					continue
				}
				pf := pm.Fields[protoFieldName]
				wireTypes, ok := seen[pf.Number]
				if !ok {
					t.Errorf("%s.%s (proto field %q = %d) never appears on the wire for a fully-populated fixture", goName, f.Name, protoFieldName, pf.Number)
					continue
				}
				exceptionKey := protoName + "." + protoFieldName
				ok = false
				for _, wt := range wireTypes {
					if wt == pf.WireType {
						ok = true
						break
					}
				}
				if !ok && pf.Packed && packedWireTolerant[exceptionKey] {
					// Accept either packed (length-delimited) or unpacked
					// (repeated varint) encoding; see packedWireTolerant.
					for _, wt := range wireTypes {
						if wt == wire.WireVarint || wt == wire.WireLengthDelimited {
							ok = true
							break
						}
					}
				}
				if !ok {
					t.Errorf("%s.%s (proto field %q = %d) emitted with wire type(s) %v, want %d", goName, f.Name, protoFieldName, pf.Number, wireTypes, pf.WireType)
				}
			}
		})
	}
}
