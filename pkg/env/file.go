package env

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	FilePermOwnerOnly = 0600
	FilePermStandard  = 0644
)

// ReadEnvFile reads raw bytes from the specified path.
func ReadEnvFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read env file %q: %w", filePath, err)
	}
	return data, nil
}

// WriteEnvFile atomically writes raw bytes to filePath, readable only by the current user
// (0600 on Unix, an owner-only ACL on Windows).
func WriteEnvFile(filePath string, data []byte) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(filePath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if err := restrictToOwner(tmpName); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to restrict permissions on %q: %w", tmpName, err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write data: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpName, filePath); err != nil {
		_ = os.Remove(filePath)
		if err := os.Rename(tmpName, filePath); err != nil {
			return fmt.Errorf("failed to replace %q: %w", filePath, err)
		}
	}

	return nil
}

// EnsureGitignore ensures that sensitive files (.env, .env.*, *.key, *.pub) are ignored
// while .env.example and env lock files (!env.lock, !env.*.lock) are never ignored.
// Returns the list of patterns that were newly appended.
func EnsureGitignore(baseDir string) ([]string, error) {
	gitignorePath := filepath.Join(baseDir, ".gitignore")
	required := []string{
		".env",
		".env.*",
		"*.key",
		"*.pub",
		"!.env.example",
		"!env.lock",
		"!env.*.lock",
	}

	var existingLines []string
	if data, err := os.ReadFile(gitignorePath); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			existingLines = append(existingLines, strings.TrimSpace(scanner.Text()))
		}
	}

	existingSet := make(map[string]bool)
	for _, line := range existingLines {
		existingSet[line] = true
	}

	var added []string
	for _, req := range required {
		if !existingSet[req] {
			added = append(added, req)
		}
	}

	if len(added) > 0 {
		var buf bytes.Buffer
		if len(existingLines) > 0 {
			buf.WriteString("\n# EnvLock secrets & keys\n")
		} else {
			buf.WriteString("# EnvLock secrets & keys\n")
		}
		for _, entry := range added {
			buf.WriteString(entry)
			buf.WriteByte('\n')
		}

		f, err := os.OpenFile(gitignorePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, FilePermStandard)
		if err != nil {
			return nil, fmt.Errorf("failed to update .gitignore: %w", err)
		}
		defer f.Close()

		if _, err := f.Write(buf.Bytes()); err != nil {
			return nil, fmt.Errorf("failed to write to .gitignore: %w", err)
		}
	}

	return added, nil
}

// EnsureGitattributes ensures that env.lock and env.*.lock are marked with merge=binary.
// Returns the list of patterns that were newly appended.
func EnsureGitattributes(baseDir string) ([]string, error) {
	gitattributesPath := filepath.Join(baseDir, ".gitattributes")
	required := []string{"env.lock merge=binary", "env.*.lock merge=binary"}

	var existingLines []string
	if data, err := os.ReadFile(gitattributesPath); err == nil {
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			existingLines = append(existingLines, strings.TrimSpace(scanner.Text()))
		}
	}

	existingSet := make(map[string]bool)
	for _, line := range existingLines {
		existingSet[line] = true
	}

	var added []string
	for _, req := range required {
		if !existingSet[req] {
			added = append(added, req)
		}
	}

	if len(added) > 0 {
		var buf bytes.Buffer
		if len(existingLines) > 0 {
			buf.WriteString("\n# EnvLock binary merge protection\n")
		} else {
			buf.WriteString("# EnvLock binary merge protection\n")
		}
		for _, entry := range added {
			buf.WriteString(entry)
			buf.WriteByte('\n')
		}

		f, err := os.OpenFile(gitattributesPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, FilePermStandard)
		if err != nil {
			return nil, fmt.Errorf("failed to update .gitattributes: %w", err)
		}
		defer f.Close()

		if _, err := f.Write(buf.Bytes()); err != nil {
			return nil, fmt.Errorf("failed to write to .gitattributes: %w", err)
		}
	}

	return added, nil
}

// SampleTemplate is the template content created if .env does not exist on init.
const SampleTemplate = `# EnvLock Configuration File
# This file contains sensitive secrets. Never commit it to Git!
# Run 'envlock push' to securely encrypt this into env.lock

APP_ENV=development
PORT=8080
DATABASE_URL=postgres://user:password@localhost:5432/mydb?sslmode=disable
`

// CreateDefaultEnv creates a starter .env file if it does not already exist.
func CreateDefaultEnv(filePath string) (bool, error) {
	if _, err := os.Stat(filePath); err == nil {
		return false, nil // File already exists
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := WriteEnvFile(filePath, []byte(SampleTemplate)); err != nil {
		return false, err
	}
	return true, nil
}
