//go:build !windows

package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteEnvFileIsOwnerOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := WriteEnvFile(path, []byte("SECRET=1\n")); err != nil {
		t.Fatalf("WriteEnvFile failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Mode().Perm() != FilePermOwnerOnly {
		t.Errorf("mode = %v, want %v", info.Mode().Perm(), os.FileMode(FilePermOwnerOnly))
	}
}
