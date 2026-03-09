// Package crypto provides CryptState for UDP voice packet encryption.
// Supports legacy mode (OCB2-AES128) and secure mode (AES-256-GCM).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"

	"github.com/dchote/go-mumble-server/pkg/mumble/crypto/ocb2"
)

// Mode selects the encryption algorithm.
type Mode int

const (
	ModeLite   Mode = iota
	ModeLegacy
	ModeSecure
)

const (
	legacyKeySize    = 16
	legacyNonceSize  = 16
	legacyTagSize    = 3
	legacyOverhead   = 1 + legacyTagSize
	secureKeySize    = 32
	secureNonceSize  = 12
	secureTagSize    = 16
	secureOverhead   = secureNonceSize + secureTagSize
	decryptHistory   = 256
)

var (
	ErrDecryptFailed   = errors.New("crypto: decryption failed")
	ErrInvalidPacket   = errors.New("crypto: invalid packet")
	ErrReplayDetected  = errors.New("crypto: replay detected")
)

// CryptState handles AEAD encryption/decryption of UDP voice packets.
type CryptState struct {
	mode Mode

	// Legacy (OCB2-AES128)
	legacyBlock   cipher.Block
	encNonce      [16]byte
	decNonce      [16]byte
	decHistory    [decryptHistory]byte

	// Secure (AES-256-GCM)
	secureAEAD    cipher.AEAD
	encCounter    uint64
	decMax        uint64
	decBitmap     [8]byte // 64 bits

	// Stats
	Good  uint32
	Late  uint32
	Lost  uint32
	Resync uint32
}

// NewCryptState creates a CryptState for the given mode.
func NewCryptState(mode Mode) *CryptState {
	return &CryptState{mode: mode}
}

// Mode returns the encryption mode.
func (c *CryptState) Mode() Mode {
	return c.mode
}

// EncNonce returns a copy of the encryption nonce (for CryptSetup resync response).
func (c *CryptState) EncNonce() []byte {
	if c.mode == ModeLite {
		return nil
	}
	if c.mode == ModeLegacy {
		out := make([]byte, 16)
		copy(out, c.encNonce[:])
		return out
	}
	return nil
}

// SetDecNonce updates the decryption nonce (when client sends ClientNonce during resync).
func (c *CryptState) SetDecNonce(nonce []byte) error {
	if c.mode == ModeLite {
		return nil
	}
	if c.mode == ModeLegacy {
		if len(nonce) != legacyNonceSize {
			return errors.New("crypto: legacy dec nonce must be 16 bytes")
		}
		copy(c.decNonce[:], nonce)
		return nil
	}
	return nil
}

// SetKey configures the key and nonces (from CryptSetup message).
func (c *CryptState) SetKey(key, encNonce, decNonce []byte) error {
	if c.mode == ModeLite {
		return nil // lite mode uses no key
	}
	if c.mode == ModeLegacy {
		if len(key) != legacyKeySize || len(encNonce) != legacyNonceSize || len(decNonce) != legacyNonceSize {
			return errors.New("crypto: legacy key/nonce size mismatch")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return err
		}
		c.legacyBlock = block
		copy(c.encNonce[:], encNonce)
		copy(c.decNonce[:], decNonce)
		return nil
	}
	if len(key) != secureKeySize {
		return errors.New("crypto: secure key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	c.secureAEAD = aead
	return nil
}

// GenerateKey creates random key and nonces for the configured mode.
func (c *CryptState) GenerateKey() error {
	if c.mode == ModeLite {
		return nil
	}
	if c.mode == ModeLegacy {
		key := make([]byte, legacyKeySize)
		encN := make([]byte, legacyNonceSize)
		decN := make([]byte, legacyNonceSize)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return err
		}
		if _, err := io.ReadFull(rand.Reader, encN); err != nil {
			return err
		}
		if _, err := io.ReadFull(rand.Reader, decN); err != nil {
			return err
		}
		return c.SetKey(key, encN, decN)
	}
	key := make([]byte, secureKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return err
	}
	return c.SetKey(key, nil, nil)
}

// Overhead returns the per-packet encryption overhead in bytes.
func (c *CryptState) Overhead() int {
	if c.mode == ModeLite {
		return 0
	}
	if c.mode == ModeLegacy {
		return legacyOverhead
	}
	return secureOverhead
}

// Encrypt encrypts src into dst. Dst must have length len(src)+Overhead().
func (c *CryptState) Encrypt(dst, src []byte) error {
	if c.mode == ModeLite {
		copy(dst, src)
		return nil
	}
	if c.mode == ModeLegacy {
		return c.encryptLegacy(dst, src)
	}
	return c.encryptSecure(dst, src)
}

// Decrypt decrypts src into dst. Returns error on failure.
func (c *CryptState) Decrypt(dst, src []byte) error {
	if c.mode == ModeLite {
		copy(dst, src)
		return nil
	}
	if c.mode == ModeLegacy {
		return c.decryptLegacy(dst, src)
	}
	return c.decryptSecure(dst, src)
}

func (c *CryptState) encryptLegacy(dst, src []byte) error {
	if c.legacyBlock == nil {
		return errors.New("crypto: key not set")
	}
	if len(dst) < len(src)+legacyOverhead {
		return errors.New("crypto: dst too small")
	}
	// Increment nonce (MSB first, per grumble)
	for i := 0; i < 16; i++ {
		c.encNonce[i]++
		if c.encNonce[i] != 0 {
			break
		}
	}
	dst[0] = c.encNonce[0] // transmit first byte
	ocb2.Encrypt(c.legacyBlock, dst[4:], src, c.encNonce[:], dst[1:4], legacyTagSize)
	return nil
}

func (c *CryptState) decryptLegacy(dst, src []byte) error {
	if c.legacyBlock == nil {
		return errors.New("crypto: key not set")
	}
	if len(src) < legacyOverhead {
		return ErrInvalidPacket
	}
	plainLen := len(src) - legacyOverhead
	if len(dst) < plainLen {
		return errors.New("crypto: dst too small")
	}

	ivByte := src[0]
	saveNonce := c.decNonce
	restore := false

	// Update dec nonce from ivByte (first byte, per Mumble/grumble)
	decFirst := c.decNonce[0]
	diff := int(ivByte) - int(decFirst)
	if diff < -128 {
		diff += 256
	} else if diff > 128 {
		diff -= 256
	}

	if diff == 1 {
		c.decNonce[0] = ivByte
	} else if diff > 1 {
		c.Lost += uint32(diff - 1)
		c.decNonce[0] = ivByte
	} else if diff > -30 && diff < 0 {
		c.Late++
		c.decNonce[0] = ivByte
		restore = true
	} else {
		return ErrReplayDetected
	}

	// Replay check: have we seen this (ivByte, second byte) before?
	if c.decHistory[ivByte] == c.decNonce[1] {
		c.decNonce = saveNonce
		return ErrReplayDetected
	}

	ok := ocb2.Decrypt(c.legacyBlock, dst, src[4:], c.decNonce[:], src[1:4])
	if !ok {
		c.decNonce = saveNonce
		return ErrDecryptFailed
	}
	c.decHistory[ivByte] = c.decNonce[1]
	if restore {
		c.decNonce = saveNonce
	}
	c.Good++
	return nil
}

func (c *CryptState) encryptSecure(dst, src []byte) error {
	if c.secureAEAD == nil {
		return errors.New("crypto: key not set")
	}
	if len(dst) < len(src)+secureOverhead {
		return errors.New("crypto: dst too small")
	}
	c.encCounter++
	nonce := make([]byte, secureNonceSize)
	binary.BigEndian.PutUint64(nonce[4:], c.encCounter)
	ciphertext := c.secureAEAD.Seal(nil, nonce, src, nil)
	copy(dst, nonce)
	copy(dst[secureNonceSize:], ciphertext)
	return nil
}

func (c *CryptState) decryptSecure(dst, src []byte) error {
	if c.secureAEAD == nil {
		return errors.New("crypto: key not set")
	}
	if len(src) < secureOverhead {
		return ErrInvalidPacket
	}
	nonce := src[:secureNonceSize]
	ct := src[secureNonceSize:]
	counter := binary.BigEndian.Uint64(nonce[4:])

	if counter <= c.decMax {
		if c.decMax-counter >= 64 {
			return ErrReplayDetected
		}
		idx := 63 - (c.decMax - counter)
		byteIdx := idx / 8
		bitIdx := idx % 8
		if c.decBitmap[byteIdx]&(1<<bitIdx) != 0 {
			return ErrReplayDetected
		}
		c.decBitmap[byteIdx] |= 1 << bitIdx
		c.Late++
	} else {
		shift := counter - c.decMax
		if shift > 64 {
			c.decBitmap = [8]byte{}
			shift = 64
		} else {
			for i := 0; i < 8; i++ {
				c.decBitmap[i] = c.decBitmap[i] << shift
				if i+1 < 8 {
					c.decBitmap[i] |= c.decBitmap[i+1] >> (8 - shift)
				}
			}
		}
		c.decBitmap[0] |= 1 << 7
		c.decMax = counter
		if shift > 1 {
			c.Lost += uint32(shift - 1)
		}
		c.Good++
	}

	plain, err := c.secureAEAD.Open(nil, nonce, ct, nil)
	if err != nil {
		return ErrDecryptFailed
	}
	if len(dst) < len(plain) {
		return errors.New("crypto: dst too small")
	}
	copy(dst, plain)
	return nil
}
