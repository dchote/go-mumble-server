package mumble

import (
	"testing"

	"github.com/dchote/go-mumble-server/internal/auth"
)

func TestRegisteredUserCredentialsValid(t *testing.T) {
	t.Run("rejects username-only access", func(t *testing.T) {
		if registeredUserCredentialsValid("", "", "", "") {
			t.Fatalf("expected credentials validation to fail without password or cert")
		}
	})

	t.Run("accepts matching argon2id password", func(t *testing.T) {
		hash, err := auth.HashArgon2id("correct-horse-battery-staple")
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		if !registeredUserCredentialsValid(hash, "", "correct-horse-battery-staple", "") {
			t.Fatalf("expected argon2id password to validate")
		}
	})

	t.Run("accepts matching cert hash", func(t *testing.T) {
		if !registeredUserCredentialsValid("", "abcdef", "", "ABCDEF") {
			t.Fatalf("expected case-insensitive cert hash match to validate")
		}
	})
}
