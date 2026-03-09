package mumble

import (
	"net"
	"path/filepath"
	"testing"

	"github.com/dchote/go-mumble-server/internal/config"
	"github.com/dchote/go-mumble-server/internal/connection"
	"github.com/dchote/go-mumble-server/internal/database"
	"github.com/dchote/go-mumble-server/pkg/mumble/crypto"
)

func TestNegotiateCryptoMode(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "negotiate.sqlite")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	cfg := &config.Config{MaxUsers: 100}
	srv := NewServer(cfg, db, 1, nil)

	_, raw := net.Pipe()
	conn := connection.New(raw, crypto.NewCryptState(crypto.ModeLegacy), func(*connection.Conn) {})

	t.Run("legacy for standard client (no capabilities)", func(t *testing.T) {
		conn.SetClientCryptoModes(0)
		mode := srv.negotiateCryptoMode(conn)
		if mode != crypto.ModeLegacy {
			t.Errorf("negotiateCryptoMode(0) = %v, want ModeLegacy", mode)
		}
	})

	t.Run("legacy for non-TLS conn with all capabilities", func(t *testing.T) {
		conn.SetClientCryptoModes(0x07)
		mode := srv.negotiateCryptoMode(conn)
		if mode != crypto.ModeLegacy {
			t.Errorf("negotiateCryptoMode(0x07) with non-TLS = %v, want ModeLegacy (no TLS 1.3 + cert)", mode)
		}
	})

	t.Run("legacy for client with legacy only", func(t *testing.T) {
		conn.SetClientCryptoModes(0x02)
		mode := srv.negotiateCryptoMode(conn)
		if mode != crypto.ModeLegacy {
			t.Errorf("negotiateCryptoMode(0x02) = %v, want ModeLegacy", mode)
		}
	})
}
