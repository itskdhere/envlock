package crypto

import (
	"crypto/ecdh"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const (
	PubKeyPrefix = "el_pub_"
	SecKeyPrefix = "el_sec_"
	KeyByteLen   = 32
	KeyHexLen    = 64
)

var (
	ErrInvalidKeyFormat = errors.New("invalid key format")
	ErrInvalidKeyLength = errors.New("invalid key length")
)

// FormatPublicKey serializes an ECDH X25519 public key to the el_pub_<hex> format.
func FormatPublicKey(pub *ecdh.PublicKey) string {
	raw := pub.Bytes()
	return PubKeyPrefix + hex.EncodeToString(raw)
}

// FormatPrivateKey serializes an ECDH X25519 private key to the el_sec_<hex> format.
func FormatPrivateKey(priv *ecdh.PrivateKey) string {
	raw := priv.Bytes()
	defer WipeBytes(raw)
	return SecKeyPrefix + hex.EncodeToString(raw)
}

// ParsePublicKey parses an el_pub_<hex> string into an ECDH X25519 public key.
func ParsePublicKey(s string) (*ecdh.PublicKey, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, PubKeyPrefix) {
		return nil, fmt.Errorf("%w: missing %q prefix", ErrInvalidKeyFormat, PubKeyPrefix)
	}

	hexPart := strings.TrimPrefix(s, PubKeyPrefix)
	if len(hexPart) != KeyHexLen {
		return nil, fmt.Errorf("%w: expected %d hex chars, got %d", ErrInvalidKeyLength, KeyHexLen, len(hexPart))
	}

	raw, err := hex.DecodeString(hexPart)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeyFormat, err)
	}

	return ecdh.X25519().NewPublicKey(raw)
}

// ParsePrivateKey parses an el_sec_<hex> string into an ECDH X25519 private key.
func ParsePrivateKey(s string) (*ecdh.PrivateKey, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, SecKeyPrefix) {
		return nil, fmt.Errorf("%w: missing %q prefix", ErrInvalidKeyFormat, SecKeyPrefix)
	}

	hexPart := strings.TrimPrefix(s, SecKeyPrefix)
	if len(hexPart) != KeyHexLen {
		return nil, fmt.Errorf("%w: expected %d hex chars, got %d", ErrInvalidKeyLength, KeyHexLen, len(hexPart))
	}

	raw, err := hex.DecodeString(hexPart)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidKeyFormat, err)
	}
	defer WipeBytes(raw)

	return ecdh.X25519().NewPrivateKey(raw)
}
