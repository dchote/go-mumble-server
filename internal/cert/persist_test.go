package cert

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/internal/database/models"
)

func TestGetOrCreateCertForVirtualServer_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	vs := models.VirtualServer{
		ID:       1,
		Name:     "Test",
		Host:     "0.0.0.0",
		Port:     64738,
		MaxUsers: 100,
	}
	if err := db.Create(&vs).Error; err != nil {
		t.Fatalf("create virtual server: %v", err)
	}

	cert1, key1, err := GetOrCreateCertForVirtualServer(db, 1)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(cert1) == 0 || len(key1) == 0 {
		t.Error("expected non-empty cert and key")
	}

	cert2, key2, err := GetOrCreateCertForVirtualServer(db, 1)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if !bytes.Equal(cert1, cert2) || !bytes.Equal(key1, key2) {
		t.Error("expected same cert and key on second call (persistence)")
	}
}

func TestGetOrCreateCertForVirtualServer_ReturnsErrorWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	_, _, err = GetOrCreateCertForVirtualServer(db, 999)
	if err == nil {
		t.Error("expected error when virtual server does not exist")
	}
}
