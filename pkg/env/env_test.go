package env

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWriteEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	original := []byte("# Header comment\r\n\nVAR1=value1\nVAR2=\"multi\nline\nvalue\"\n  # Indented comment\nEMPTY=\nTRAILING_SPACE=abc   \n")

	if err := WriteEnvFile(envPath, original); err != nil {
		t.Fatalf("WriteEnvFile failed: %v", err)
	}

	readBack, err := ReadEnvFile(envPath)
	if err != nil {
		t.Fatalf("ReadEnvFile failed: %v", err)
	}

	if !bytes.Equal(original, readBack) {
		t.Fatalf("readBack does not match original byte-for-byte:\nGot:\n%s\nExpected:\n%s", string(readBack), string(original))
	}
}

func TestEnsureGitignore(t *testing.T) {
	tempDir := t.TempDir()

	// Initial run
	added, err := EnsureGitignore(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitignore failed: %v", err)
	}
	if len(added) != 6 {
		t.Fatalf("expected 6 entries added, got %d", len(added))
	}

	// Second run should be no-op
	added2, err := EnsureGitignore(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitignore second run failed: %v", err)
	}
	if len(added2) != 0 {
		t.Fatalf("expected 0 entries added on second run, got %d", len(added2))
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(tempDir, ".gitignore"))
	str := string(content)
	for _, expected := range []string{".env", ".env.*.local", "*.key", "*.pub", "!env.lock", "!env.*.lock"} {
		if !strings.Contains(str, expected) {
			t.Errorf("gitignore missing expected entry: %s", expected)
		}
	}
}

func TestEnsureGitattributes(t *testing.T) {
	tempDir := t.TempDir()

	// Initial run
	added, err := EnsureGitattributes(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitattributes failed: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("expected 2 entries added, got %d", len(added))
	}

	// Second run should be no-op
	added2, err := EnsureGitattributes(tempDir)
	if err != nil {
		t.Fatalf("EnsureGitattributes second run failed: %v", err)
	}
	if len(added2) != 0 {
		t.Fatalf("expected 0 entries added on second run, got %d", len(added2))
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(tempDir, ".gitattributes"))
	str := string(content)
	if !strings.Contains(str, "env.lock merge=binary") || !strings.Contains(str, "env.*.lock merge=binary") {
		t.Errorf("gitattributes missing binary merge rules: %s", str)
	}
}

func TestResolvePaths(t *testing.T) {
	base := filepath.Clean("/my/project")

	// Default
	p1 := ResolvePaths(base, "", "")
	if p1.EnvFile != filepath.Join(base, ".env") || p1.LockFile != filepath.Join(base, "env.lock") {
		t.Errorf("default paths mismatch: %+v", p1)
	}

	// Named env
	p2 := ResolvePaths(base, "production", "")
	if p2.EnvFile != filepath.Join(base, ".env.production") || p2.LockFile != filepath.Join(base, "env.production.lock") {
		t.Errorf("named env mismatch: %+v", p2)
	}

	// Explicit file
	p3 := ResolvePaths(base, "", "custom.env")
	if p3.EnvFile != filepath.Join(base, "custom.env") || p3.LockFile != filepath.Join(base, "custom.env.lock") {
		t.Errorf("explicit file mismatch: %+v", p3)
	}

	// Explicit .env file
	p4 := ResolvePaths(base, "", ".env")
	if p4.EnvFile != filepath.Join(base, ".env") || p4.LockFile != filepath.Join(base, "env.lock") {
		t.Errorf("explicit .env mismatch: %+v", p4)
	}
}

func TestCreateDefaultEnv(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	created, err := CreateDefaultEnv(envPath)
	if err != nil {
		t.Fatalf("CreateDefaultEnv failed: %v", err)
	}
	if !created {
		t.Errorf("expected created to be true")
	}

	// Second call should not overwrite
	created2, err := CreateDefaultEnv(envPath)
	if err != nil {
		t.Fatalf("CreateDefaultEnv second run failed: %v", err)
	}
	if created2 {
		t.Errorf("expected created2 to be false")
	}
}
