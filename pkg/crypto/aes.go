package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

const (
	GCMIVLength = 12
	DEKLength   = 32
)

var (
	ErrInvalidDEKLength = errors.New("invalid DEK length: expected 32 bytes")
	ErrInvalidIVLength  = errors.New("invalid IV length: expected 12 bytes")
)

// EncryptPayload encrypts plaintext bytes with AES-256-GCM using the provided 32-byte DEK.
// Returns a 12-byte cryptographically secure random IV and the ciphertext (including tag).
func EncryptPayload(plaintext []byte, dek []byte) (iv []byte, ciphertext []byte, err error) {
	if len(dek) != DEKLength {
		return nil, nil, ErrInvalidDEKLength
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}

	iv = make([]byte, GCMIVLength)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random IV: %w", err)
	}

	ciphertext = gcm.Seal(nil, iv, plaintext, nil)
	return iv, ciphertext, nil
}

// DecryptPayload decrypts ciphertext using AES-256-GCM with the provided 12-byte IV and 32-byte DEK.
func DecryptPayload(ciphertext []byte, iv []byte, dek []byte) ([]byte, error) {
	if len(dek) != DEKLength {
		return nil, ErrInvalidDEKLength
	}
	if len(iv) != GCMIVLength {
		return nil, ErrInvalidIVLength
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("payload decryption failed: %w", err)
	}

	return plaintext, nil
}

