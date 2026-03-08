// Package ocb2 implements OCB2-AES128 for Mumble legacy mode compatibility.
// OCB2 is insecure (see CRYPTO 2019 attacks); used only for backward compatibility.
//
// Adapted from mumble-voip/grumble (BSD license).
package ocb2

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/subtle"
)

const (
	BlockSize  = 16
	TagSize    = 16
	NonceSize  = 16
	MumbleTag  = 3 // Mumble uses 3-byte truncated tag
)

func zeros(block []byte) {
	for i := range block {
		block[i] = 0
	}
}

func xor(dst, a, b []byte) {
	n := BlockSize
	if len(a) < n {
		n = len(a)
	}
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		dst[i] = a[i] ^ b[i]
	}
}

func times2(block []byte) {
	carry := (block[0] >> 7) & 0x1
	for i := 0; i < BlockSize-1; i++ {
		block[i] = (block[i] << 1) | ((block[i+1] >> 7) & 0x1)
	}
	block[BlockSize-1] = (block[BlockSize-1] << 1) ^ (carry * 135)
}

func times3(block []byte) {
	carry := (block[0] >> 7) & 0x1
	for i := 0; i < BlockSize-1; i++ {
		block[i] ^= (block[i] << 1) | ((block[i+1] >> 7) & 0x1)
	}
	block[BlockSize-1] ^= ((block[BlockSize-1] << 1) ^ (carry * 135))
}

// Encrypt encrypts src into dst, appending tagLen bytes of tag (use MumbleTag for legacy).
func Encrypt(block cipher.Block, dst, src, nonce, tag []byte, tagLen int) {
	if block.BlockSize() != BlockSize || len(nonce) != NonceSize {
		panic("ocb2: invalid block or nonce size")
	}
	var checksum, delta, tmp, pad, calcTag [BlockSize]byte
	block.Encrypt(delta[:], nonce)
	zeros(checksum[:])

	off := 0
	remain := len(src)
	for remain > BlockSize {
		times2(delta[:])
		xor(tmp[:], delta[:], src[off:off+BlockSize])
		block.Encrypt(tmp[:], tmp[:])
		xor(dst[off:off+BlockSize], delta[:], tmp[:])
		xor(checksum[:], checksum[:], src[off:off+BlockSize])
		remain -= BlockSize
		off += BlockSize
	}

	times2(delta[:])
	zeros(tmp[:])
	num := remain * 8
	tmp[BlockSize-2] = byte((uint32(num) >> 8) & 0xff)
	tmp[BlockSize-1] = byte(num & 0xff)
	xor(tmp[:], tmp[:], delta[:])
	block.Encrypt(pad[:], tmp[:])
	copy(tmp[:], src[off:])
	copy(tmp[remain:], pad[remain:])
	xor(checksum[:], checksum[:], tmp[:])
	xor(tmp[:], pad[:], tmp[:])
	copy(dst[off:], tmp[:remain])

	times3(delta[:])
	xor(tmp[:], delta[:], checksum[:])
	block.Encrypt(calcTag[:], tmp[:])
	copy(tag, calcTag[:tagLen])
}

// Decrypt decrypts src into dst. Returns false if tag verification fails.
func Decrypt(block cipher.Block, dst, src, nonce, tag []byte) bool {
	if block.BlockSize() != BlockSize || len(nonce) != NonceSize {
		panic("ocb2: invalid block or nonce size")
	}
	tagLen := len(tag)
	if tagLen > TagSize {
		tagLen = TagSize
	}
	var checksum, delta, tmp, pad, calcTag [BlockSize]byte
	block.Encrypt(delta[:], nonce)
	zeros(checksum[:])

	off := 0
	remain := len(src)
	for remain > BlockSize {
		times2(delta[:])
		xor(tmp[:], delta[:], src[off:off+BlockSize])
		block.Decrypt(tmp[:], tmp[:])
		xor(dst[off:off+BlockSize], delta[:], tmp[:])
		xor(checksum[:], checksum[:], dst[off:off+BlockSize])
		off += BlockSize
		remain -= BlockSize
	}

	times2(delta[:])
	zeros(tmp[:])
	num := remain * 8
	tmp[BlockSize-2] = byte((uint32(num) >> 8) & 0xff)
	tmp[BlockSize-1] = byte(num & 0xff)
	xor(tmp[:], tmp[:], delta[:])
	block.Encrypt(pad[:], tmp[:])
	xor(tmp[:], src[off:off+remain], pad[:])
	xor(checksum[:], checksum[:], tmp[:])
	copy(dst[off:off+remain], tmp[:remain])

	times3(delta[:])
	xor(tmp[:], delta[:], checksum[:])
	block.Encrypt(calcTag[:], tmp[:])
	return subtle.ConstantTimeCompare(calcTag[:tagLen], tag) == 1
}

// NewBlock creates an AES-128 block cipher from a 16-byte key.
func NewBlock(key []byte) (cipher.Block, error) {
	return aes.NewCipher(key)
}
