package crypto

import (
	"bytes"
	"testing"
)

func TestKeyFormattingAndParsing(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	pubStr := FormatPublicKey(pub)
	if len(pubStr) != len(PubKeyPrefix)+KeyHexLen {
		t.Fatalf("unexpected pub key string length: %d", len(pubStr))
	}

	parsedPub, err := ParsePublicKey(pubStr)
	if err != nil {
		t.Fatalf("ParsePublicKey failed: %v", err)
	}
	if !bytes.Equal(pub.Bytes(), parsedPub.Bytes()) {
		t.Fatalf("parsed public key bytes do not match original")
	}

	privStr := FormatPrivateKey(priv)
	if len(privStr) != len(SecKeyPrefix)+KeyHexLen {
		t.Fatalf("unexpected priv key string length: %d", len(privStr))
	}

	parsedPriv, err := ParsePrivateKey(privStr)
	if err != nil {
		t.Fatalf("ParsePrivateKey failed: %v", err)
	}
	if !bytes.Equal(priv.Bytes(), parsedPriv.Bytes()) {
		t.Fatalf("parsed private key bytes do not match original")
	}

	// Negative tests
	if _, err := ParsePublicKey("bad_prefix_1234"); err == nil {
		t.Errorf("expected error for bad prefix")
	}
	if _, err := ParsePublicKey(PubKeyPrefix + "1234"); err == nil {
		t.Errorf("expected error for short key")
	}
	if _, err := ParsePrivateKey("bad_prefix_1234"); err == nil {
		t.Errorf("expected error for bad priv prefix")
	}
}

func TestECDHAndHKDF(t *testing.T) {
	privAlice, pubAlice, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Alice's keypair: %v", err)
	}
	privBob, pubBob, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("failed to generate Bob's keypair: %v", err)
	}

	secretAliceBob, err := DeriveSharedSecret(privAlice, pubBob)
	if err != nil {
		t.Fatalf("Alice ECDH failed: %v", err)
	}
	secretBobAlice, err := DeriveSharedSecret(privBob, pubAlice)
	if err != nil {
		t.Fatalf("Bob ECDH failed: %v", err)
	}

	if !bytes.Equal(secretAliceBob, secretBobAlice) {
		t.Fatalf("ECDH shared secrets do not match")
	}

	kekAlice, err := DeriveKEK(secretAliceBob, pubAlice.Bytes(), pubBob.Bytes())
	if err != nil {
		t.Fatalf("Alice KEK derivation failed: %v", err)
	}
	kekBob, err := DeriveKEK(secretBobAlice, pubAlice.Bytes(), pubBob.Bytes())
	if err != nil {
		t.Fatalf("Bob KEK derivation failed: %v", err)
	}

	if !bytes.Equal(kekAlice, kekBob) {
		t.Fatalf("derived KEKs do not match")
	}
}

func TestAESPayloadRoundTrip(t *testing.T) {
	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK failed: %v", err)
	}

	plaintext := []byte("SECRET_KEY=super_secret_value\nDATABASE_URL=postgres://localhost:5432\n# Comment line\n")
	iv, ciphertext, err := EncryptPayload(plaintext, dek, nil)
	if err != nil {
		t.Fatalf("EncryptPayload failed: %v", err)
	}

	if len(iv) != GCMIVLength {
		t.Fatalf("unexpected IV length: %d", len(iv))
	}

	decrypted, err := DecryptPayload(ciphertext, iv, dek, nil)
	if err != nil {
		t.Fatalf("DecryptPayload failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted text does not match plaintext byte-for-byte")
	}

	// Tamper test
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[0] ^= 0xFF

	if _, err := DecryptPayload(tampered, iv, dek, nil); err == nil {
		t.Fatalf("expected error decrypting tampered ciphertext")
	}
}

func TestEnvelopeWrapAndUnwrap(t *testing.T) {
	privRecipient, pubRecipient, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK failed: %v", err)
	}

	ephPub, wrappedHex, err := WrapDEK(dek, pubRecipient)
	if err != nil {
		t.Fatalf("WrapDEK failed: %v", err)
	}

	if len(wrappedHex) != WrappedDEKHexLength {
		t.Fatalf("expected wrapped DEK hex length %d, got %d", WrappedDEKHexLength, len(wrappedHex))
	}

	unwrappedDEK, err := UnwrapDEK(wrappedHex, ephPub, privRecipient)
	if err != nil {
		t.Fatalf("UnwrapDEK failed: %v", err)
	}

	if !bytes.Equal(dek, unwrappedDEK) {
		t.Fatalf("unwrapped DEK does not match original DEK")
	}

	// Unauthorized recipient attempt
	privEve, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair failed: %v", err)
	}

	if _, err := UnwrapDEK(wrappedHex, ephPub, privEve); err == nil {
		t.Fatalf("expected error when unwrapping with unauthorized private key")
	}
}

func TestMultiRecipientWrapping(t *testing.T) {
	privAlice, pubAlice, _ := GenerateKeyPair()
	privBob, pubBob, _ := GenerateKeyPair()
	privCharlie, pubCharlie, _ := GenerateKeyPair()

	dek, err := GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK failed: %v", err)
	}

	// Wrap for each recipient
	ephPubAlice, wrapAlice, err := WrapDEK(dek, pubAlice)
	if err != nil {
		t.Fatalf("wrap for Alice failed: %v", err)
	}

	ephPubBob, wrapBob, err := WrapDEK(dek, pubBob)
	if err != nil {
		t.Fatalf("wrap for Bob failed: %v", err)
	}

	ephPubCharlie, wrapCharlie, err := WrapDEK(dek, pubCharlie)
	if err != nil {
		t.Fatalf("wrap for Charlie failed: %v", err)
	}

	// Each unwraps the exact same DEK
	dekAlice, err := UnwrapDEK(wrapAlice, ephPubAlice, privAlice)
	if err != nil {
		t.Fatalf("Alice unwrap failed: %v", err)
	}
	if !bytes.Equal(dek, dekAlice) {
		t.Fatalf("Alice DEK mismatch")
	}

	dekBob, err := UnwrapDEK(wrapBob, ephPubBob, privBob)
	if err != nil {
		t.Fatalf("Bob unwrap failed: %v", err)
	}
	if !bytes.Equal(dek, dekBob) {
		t.Fatalf("Bob DEK mismatch")
	}

	dekCharlie, err := UnwrapDEK(wrapCharlie, ephPubCharlie, privCharlie)
	if err != nil {
		t.Fatalf("Charlie unwrap failed: %v", err)
	}
	if !bytes.Equal(dek, dekCharlie) {
		t.Fatalf("Charlie DEK mismatch")
	}
}

func TestWipeBytes(t *testing.T) {
	buf := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	WipeBytes(buf)
	for i, b := range buf {
		if b != 0 {
			t.Fatalf("byte at index %d was not wiped: %d", i, b)
		}
	}
}
