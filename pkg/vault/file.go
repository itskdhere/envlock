package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrUnsupportedVersion = errors.New("unsupported vault version")
	ErrUnsupportedCipher  = errors.New("unsupported cipher")
	ErrCorruptVault       = errors.New("corrupt vault envelope")
	ErrNoRecipients       = errors.New("vault contains no recipients")
)

// ReadVault reads and parses an env.lock file from the specified path.
func ReadVault(filePath string) (*VaultEnvelope, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault file %q: %w", filePath, err)
	}

	var vault VaultEnvelope
	if err := json.Unmarshal(data, &vault); err != nil {
		return nil, fmt.Errorf("%w: invalid JSON: %v", ErrCorruptVault, err)
	}

	if vault.Version != VaultVersion {
		return nil, fmt.Errorf("%w: got %d, expected %d", ErrUnsupportedVersion, vault.Version, VaultVersion)
	}

	if vault.Cipher != CipherAES256GCM {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCipher, vault.Cipher)
	}

	if vault.IV == "" || vault.Ciphertext == "" {
		return nil, fmt.Errorf("%w: missing IV or ciphertext", ErrCorruptVault)
	}

	if len(vault.Recipients) == 0 {
		return nil, ErrNoRecipients
	}

	return &vault, nil
}

// WriteVault atomically writes a VaultEnvelope to disk as formatted JSON.
func WriteVault(filePath string, vault *VaultEnvelope) error {
	if vault == nil {
		return errors.New("cannot write nil vault")
	}

	data, err := json.MarshalIndent(vault, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal vault: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %q: %w", dir, err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(filePath)+".*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Clean up if rename doesn't happen

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write vault to temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic replace
	// On Windows, os.Rename replaces the target if destination exists since Go 1.5,
	// but to be safe across all platforms:
	if err := os.Rename(tmpName, filePath); err != nil {
		// Fallback for Windows if destination exists and cannot be directly replaced
		_ = os.Remove(filePath)
		if err := os.Rename(tmpName, filePath); err != nil {
			return fmt.Errorf("failed to atomically write vault to %q: %w", filePath, err)
		}
	}

	return nil
}
