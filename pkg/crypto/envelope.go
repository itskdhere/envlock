package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const (
	WrappedDEKByteLength = GCMIVLength + DEKLength + 16 // 12 + 32 + 16 = 60 bytes
	WrappedDEKHexLength  = WrappedDEKByteLength * 2     // 120 hex characters
)

var (
	ErrInvalidWrappedDEK = errors.New("invalid wrapped DEK format or length")
)

// GenerateDEK generates a cryptographically secure random 256-bit (32-byte) Data Encryption Key.
func GenerateDEK() ([]byte, error) {
	dek := make([]byte, DEKLength)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, fmt.Errorf("failed to generate random DEK: %w", err)
	}
	return dek, nil
}

// WrapDEK encrypts a DEK for a recipient using Curve25519 ECDH + HKDF-SHA256 + AES-256-GCM.
// Returns the ephemeral public key and the wrapped DEK as a 120-character hex string:
// hex(iv_12bytes || ciphertext_32bytes || tag_16bytes).
func WrapDEK(dek []byte, recipientPub *ecdh.PublicKey) (*ecdh.PublicKey, string, error) {
	if len(dek) != DEKLength {
		return nil, "", ErrInvalidDEKLength
	}

	ephPriv, ephPub, err := GenerateKeyPair()
	if err != nil {
		return nil, "", err
	}

	sharedSecret, err := DeriveSharedSecret(ephPriv, recipientPub)
	if err != nil {
		return nil, "", err
	}
	defer WipeBytes(sharedSecret)

	kek, err := DeriveKEK(sharedSecret, recipientPub.Bytes())
	if err != nil {
		return nil, "", err
	}
	defer WipeBytes(kek)

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create KEK cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create KEK GCM: %w", err)
	}

	iv := make([]byte, GCMIVLength)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, "", fmt.Errorf("failed to generate IV for wrapped DEK: %w", err)
	}

	sealed := gcm.Seal(nil, iv, dek, nil)
	wrappedBytes := make([]byte, 0, GCMIVLength+len(sealed))
	wrappedBytes = append(wrappedBytes, iv...)
	wrappedBytes = append(wrappedBytes, sealed...)

	if len(wrappedBytes) != WrappedDEKByteLength {
		return nil, "", fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidWrappedDEK, WrappedDEKByteLength, len(wrappedBytes))
	}

	return ephPub, hex.EncodeToString(wrappedBytes), nil
}

// UnwrapDEK decrypts a wrapped DEK using the recipient's private key and the ephemeral public key.
func UnwrapDEK(wrappedDEKHex string, ephemeralPub *ecdh.PublicKey, recipientPriv *ecdh.PrivateKey) ([]byte, error) {
	if len(wrappedDEKHex) != WrappedDEKHexLength {
		return nil, fmt.Errorf("%w: expected %d hex chars, got %d", ErrInvalidWrappedDEK, WrappedDEKHexLength, len(wrappedDEKHex))
	}

	wrappedBytes, err := hex.DecodeString(wrappedDEKHex)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid hex encoding: %v", ErrInvalidWrappedDEK, err)
	}
	if len(wrappedBytes) != WrappedDEKByteLength {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrInvalidWrappedDEK, WrappedDEKByteLength, len(wrappedBytes))
	}

	sharedSecret, err := DeriveSharedSecret(recipientPriv, ephemeralPub)
	if err != nil {
		return nil, err
	}
	defer WipeBytes(sharedSecret)

	kek, err := DeriveKEK(sharedSecret, recipientPriv.PublicKey().Bytes())
	if err != nil {
		return nil, err
	}
	defer WipeBytes(kek)

	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, fmt.Errorf("failed to create KEK cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create KEK GCM: %w", err)
	}

	iv := wrappedBytes[:GCMIVLength]
	sealed := wrappedBytes[GCMIVLength:]

	dek, err := gcm.Open(nil, iv, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK (unauthorized or tampered): %w", err)
	}

	if len(dek) != DEKLength {
		WipeBytes(dek)
		return nil, ErrInvalidDEKLength
	}

	return dek, nil
}
