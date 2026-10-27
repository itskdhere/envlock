package vault

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVaultReadWrite(t *testing.T) {
	tempDir := t.TempDir()
	vaultPath := filepath.Join(tempDir, "env.lock")

	envelope := &VaultEnvelope{
		Version:    VaultVersion,
		Cipher:     CipherAES256GCM,
		IV:         "0102030405060708090a0b0c",
		Ciphertext: "deadbeefcafebabe",
		Recipients: []RecipientEntry{
			{
				PubKey:          "el_pub_1111",
				EphemeralPubKey: "el_pub_2222",
				WrappedDEK:      "3333",
			},
		},
	}

	if err := WriteVault(vaultPath, envelope); err != nil {
		t.Fatalf("WriteVault failed: %v", err)
	}
	if info, err := os.Stat(vaultPath); err == nil && runtime.GOOS != "windows" && info.Mode().Perm() != 0644 {
		t.Errorf("env.lock mode = %v, want 0644", info.Mode().Perm())
	}

	loaded, err := ReadVault(vaultPath)
	if err != nil {
		t.Fatalf("ReadVault failed: %v", err)
	}

	if loaded.Version != envelope.Version {
		t.Errorf("version mismatch: expected %d, got %d", envelope.Version, loaded.Version)
	}
	if loaded.Cipher != envelope.Cipher {
		t.Errorf("cipher mismatch: expected %s, got %s", envelope.Cipher, loaded.Cipher)
	}
	if loaded.IV != envelope.IV {
		t.Errorf("IV mismatch: expected %s, got %s", envelope.IV, loaded.IV)
	}
	if loaded.Ciphertext != envelope.Ciphertext {
		t.Errorf("Ciphertext mismatch: expected %s, got %s", envelope.Ciphertext, loaded.Ciphertext)
	}
	if len(loaded.Recipients) != 1 || loaded.Recipients[0].PubKey != "el_pub_1111" {
		t.Errorf("Recipients mismatch")
	}

	// Overwrite test
	envelope.Ciphertext = "cafebabe"
	if err := WriteVault(vaultPath, envelope); err != nil {
		t.Fatalf("WriteVault overwrite failed: %v", err)
	}
	loaded2, err := ReadVault(vaultPath)
	if err != nil {
		t.Fatalf("ReadVault after overwrite failed: %v", err)
	}
	if loaded2.Ciphertext != "cafebabe" {
		t.Errorf("overwritten ciphertext mismatch: %s", loaded2.Ciphertext)
	}

	// Corrupt JSON test
	badPath := filepath.Join(tempDir, "bad.lock")
	if err := os.WriteFile(badPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}
	if _, err := ReadVault(badPath); err == nil {
		t.Errorf("expected error reading corrupt vault")
	}
}
