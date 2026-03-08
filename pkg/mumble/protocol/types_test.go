package protocol

import (
	"testing"
)

func TestMessageTypeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  MessageType
		want uint16
	}{
		{"Version", MessageVersion, 0},
		{"UDPTunnel", MessageUDPTunnel, 1},
		{"Authenticate", MessageAuthenticate, 2},
		{"Ping", MessagePing, 3},
		{"Reject", MessageReject, 4},
		{"ServerSync", MessageServerSync, 5},
		{"ChannelRemove", MessageChannelRemove, 6},
		{"ChannelState", MessageChannelState, 7},
		{"UserRemove", MessageUserRemove, 8},
		{"UserState", MessageUserState, 9},
		{"BanList", MessageBanList, 10},
		{"TextMessage", MessageTextMessage, 11},
		{"PermissionDenied", MessagePermissionDenied, 12},
		{"ACL", MessageACL, 13},
		{"QueryUsers", MessageQueryUsers, 14},
		{"CryptSetup", MessageCryptSetup, 15},
		{"ContextActionModify", MessageContextActionModify, 16},
		{"ContextAction", MessageContextAction, 17},
		{"UserList", MessageUserList, 18},
		{"VoiceTarget", MessageVoiceTarget, 19},
		{"PermissionQuery", MessagePermissionQuery, 20},
		{"CodecVersion", MessageCodecVersion, 21},
		{"UserStats", MessageUserStats, 22},
		{"RequestBlob", MessageRequestBlob, 23},
		{"ServerConfig", MessageServerConfig, 24},
		{"SuggestConfig", MessageSuggestConfig, 25},
		{"PluginDataTransmission", MessagePluginDataTransmission, 26},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if uint16(tt.got) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
			}
		})
	}
}

func TestMessageCount(t *testing.T) {
	if MessageCount != 27 {
		t.Errorf("MessageCount = %d, want 27", MessageCount)
	}
}
