package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/vault"
)

func TestEndToEndFidelity(t *testing.T) {
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	defer os.Chdir(origDir)

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// 1. Complex raw .env data preserving whitespace, comments, multiline, unicode, and quoting
	originalEnv := []byte(`# Core Configuration 🚀
APP_NAME="EnvLock Vault / Test"
PORT=8080

# Multi-line Certificate
PUBLIC_CERT="-----BEGIN CERTIFICATE-----
MIIDXTCCAkWgAwIBAgIJAL...
-----END CERTIFICATE-----"

# Quotes & Special Characters
DB_URI='postgresql://admin:p@$$w0rd!#%^&*()@localhost:5432/db?sslmode=require'
EMPTY_VAL=
TRAILING_WHITESPACE=value_with_spaces   
JSON_PAYLOAD={"key": "value", "active": true}

# Final trailing comment
`)

	envPath := filepath.Join(tempDir, ".env")
	lockPath := filepath.Join(tempDir, ".env.lock")

	if err := env.WriteEnvFile(envPath, originalEnv); err != nil {
		t.Fatalf("failed to write initial .env: %v", err)
	}

	// 2. Run keygen
	flagKeygenGlobal = false
	flagKeygenOut = ""
	flagKeygenStdout = false
	flagKeygenForce = false
	if err := keygenCmd.RunE(keygenCmd, nil); err != nil {
		t.Fatalf("keygen failed: %v", err)
	}

	// Verify key files generated
	if _, err := os.Stat("envlock.key"); err != nil {
		t.Fatalf("envlock.key not found: %v", err)
	}
	if _, err := os.Stat("envlock.pub"); err != nil {
		t.Fatalf("envlock.pub not found: %v", err)
	}

	// 3. Run push
	flagEnv = ""
	flagFile = ""
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// Verify .env.lock exists and validates against schema
	envelope, err := vault.ReadVault(lockPath)
	if err != nil {
		t.Fatalf("failed to read generated vault: %v", err)
	}
	if envelope.Version != 1 || envelope.Cipher != "AES-256-GCM" || len(envelope.Recipients) != 1 {
		t.Fatalf("unexpected envelope content: %+v", envelope)
	}

	// 4. Delete local .env
	if err := os.Remove(envPath); err != nil {
		t.Fatalf("failed to remove .env: %v", err)
	}

	// 5. Run pull
	flagPullForce = false
	flagKeyFile = ""
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull failed: %v", err)
	}

	// 6. Verify byte-for-byte exact fidelity
	decryptedBytes, err := env.ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("failed to read restored .env: %v", err)
	}

	if !bytes.Equal(originalEnv, decryptedBytes) {
		t.Fatalf("byte-for-byte fidelity failed!\nExpected:\n%s\nGot:\n%s", string(originalEnv), string(decryptedBytes))
	}
}

func TestMultiEnvironmentWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	_ = os.Chdir(tempDir)

	// Generate key
	flagKeygenForce = true
	_ = keygenCmd.RunE(keygenCmd, nil)

	// Create .env.staging
	stagingEnv := []byte("ENVIRONMENT=staging\nDEBUG=true\n")
	stagingFile := filepath.Join(tempDir, ".env.staging")
	stagingLock := filepath.Join(tempDir, ".env.staging.lock")
	_ = env.WriteEnvFile(stagingFile, stagingEnv)

	// Push staging
	flagEnv = "staging"
	flagFile = ""
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push staging failed: %v", err)
	}

	if _, err := os.Stat(stagingLock); err != nil {
		t.Fatalf("staging lockfile not created: %v", err)
	}

	// Remove staging .env and pull
	_ = os.Remove(stagingFile)
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull staging failed: %v", err)
	}

	restored, err := env.ReadEnvFile(stagingFile)
	if err != nil {
		t.Fatalf("failed to read restored staging env: %v", err)
	}

	if !bytes.Equal(stagingEnv, restored) {
		t.Fatalf("staging env mismatch")
	}
}

func TestPullOverwriteProtection(t *testing.T) {
	tempDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	_ = os.Chdir(tempDir)

	_ = keygenCmd.RunE(keygenCmd, nil)

	initialContent := []byte("FOO=bar\n")
	envPath := filepath.Join(tempDir, ".env")
	_ = env.WriteEnvFile(envPath, initialContent)

	flagEnv = ""
	flagFile = ""
	if err := pushCmd.RunE(pushCmd, nil); err != nil {
		t.Fatalf("push failed: %v", err)
	}

	// Modify local file
	modifiedContent := []byte("FOO=bar_modified\n")
	_ = env.WriteEnvFile(envPath, modifiedContent)

	// Pull with force = true should overwrite modified local file
	flagPullForce = true
	if err := pullCmd.RunE(pullCmd, nil); err != nil {
		t.Fatalf("pull with force failed: %v", err)
	}

	readBack, _ := env.ReadEnvFile(envPath)
	if !bytes.Equal(readBack, initialContent) {
		t.Fatalf("expected pull --force to overwrite with vault content")
	}
}

func TestKeyResolutionWithEnvVar(t *testing.T) {
	priv, _, _ := crypto.GenerateKeyPair()
	privStr := crypto.FormatPrivateKey(priv)

	t.Setenv("ENVLOCK_KEY", privStr)

	resolved, err := ResolvePrivateKey("")
	if err != nil {
		t.Fatalf("ResolvePrivateKey with ENVLOCK_KEY failed: %v", err)
	}

	if resolved != privStr {
		t.Fatalf("resolved key mismatch: got %s, expected %s", resolved, privStr)
	}
}

