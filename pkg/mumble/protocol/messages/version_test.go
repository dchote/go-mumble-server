package messages

import (
	"testing"
)

func TestVersion_MarshalUnmarshal(t *testing.T) {
	v := &Version{
		VersionV1: 0x010203,
		VersionV2: 0x05060708,
		Release:   "test 1.0",
		OS:        "linux",
		OSVersion: "5.0",
	}
	data, err := v.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var v2 Version
	if err := v2.Unmarshal(data); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v2.VersionV1 != v.VersionV1 || v2.VersionV2 != v.VersionV2 ||
		v2.Release != v.Release || v2.OS != v.OS || v2.OSVersion != v.OSVersion {
		t.Errorf("round-trip mismatch: got %+v", v2)
	}
}
