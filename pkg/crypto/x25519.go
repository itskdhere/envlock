package crypto

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

var (
	HKDFSalt = []byte("envlock_v1")
)

// GenerateKeyPair generates a new random X25519 keypair.
func GenerateKeyPair() (*ecdh.PrivateKey, *ecdh.PublicKey, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate X25519 keypair: %w", err)
	}
	return priv, priv.PublicKey(), nil
}

// DeriveSharedSecret performs Curve25519 ECDH between a private key and a public key.
func DeriveSharedSecret(priv *ecdh.PrivateKey, pub *ecdh.PublicKey) ([]byte, error) {
	sharedSecret, err := priv.ECDH(pub)
	if err != nil {
		return nil, fmt.Errorf("ecdh derivation failed: %w", err)
	}
	return sharedSecret, nil
}

// DeriveKEK derives a 256-bit Key Encryption Key (KEK) using HKDF-SHA256
// with salt "envlock_v1" and the recipient's raw public key as info.
func DeriveKEK(sharedSecret []byte, recipientPubBytes []byte) ([]byte, error) {
	kek, err := hkdf.Key(sha256.New, sharedSecret, HKDFSalt, string(recipientPubBytes), 32)
	if err != nil {
		return nil, fmt.Errorf("hkdf key derivation failed: %w", err)
	}
	return kek, nil
}

