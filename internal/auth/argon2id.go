package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (RFC 9106, PHC format).
const (
	argon2idMemory      = 64 * 1024
	argon2idIterations  = 3
	argon2idParallelism = 4
	argon2idSaltLen     = 16
	argon2idKeyLen      = 32
)

var (
	ErrInvalidArgon2Hash         = errors.New("argon2 encoded hash is not in the correct format")
	ErrIncompatibleArgon2Version = errors.New("incompatible argon2 version")
)

// HashArgon2id hashes a password using Argon2id and returns a PHC-encoded string.
func HashArgon2id(password string) (string, error) {
	salt := make([]byte, argon2idSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, argon2idIterations, argon2idMemory, argon2idParallelism, argon2idKeyLen)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argon2idMemory, argon2idIterations, argon2idParallelism, b64Salt, b64Hash), nil
}

// CompareArgon2id returns true if password matches the Argon2id-encoded hash.
func CompareArgon2id(hash, password string) bool {
	match, err := compareArgon2idHash(password, hash)
	return err == nil && match
}

func compareArgon2idHash(password, encodedHash string) (bool, error) {
	p, salt, hash, err := decodeArgon2Hash(encodedHash)
	if err != nil {
		return false, err
	}
	otherHash := argon2.IDKey([]byte(password), salt, p.iterations, p.memory, p.parallelism, uint32(len(hash)))
	return subtle.ConstantTimeCompare(hash, otherHash) == 1, nil
}

type argon2Params struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func decodeArgon2Hash(encodedHash string) (*argon2Params, []byte, []byte, error) {
	if !strings.HasPrefix(encodedHash, "$argon2id$") {
		return nil, nil, nil, ErrInvalidArgon2Hash
	}
	vals := strings.Split(encodedHash, "$")
	if len(vals) != 6 {
		return nil, nil, nil, ErrInvalidArgon2Hash
	}
	var version int
	if _, err := fmt.Sscanf(vals[2], "v=%d", &version); err != nil {
		return nil, nil, nil, err
	}
	if version != argon2.Version {
		return nil, nil, nil, ErrIncompatibleArgon2Version
	}
	p := &argon2Params{}
	if _, err := fmt.Sscanf(vals[3], "m=%d,t=%d,p=%d", &p.memory, &p.iterations, &p.parallelism); err != nil {
		return nil, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(vals[4])
	if err != nil {
		return nil, nil, nil, err
	}
	hash, err := base64.RawStdEncoding.Strict().DecodeString(vals[5])
	if err != nil {
		return nil, nil, nil, err
	}
	return p, salt, hash, nil
}
