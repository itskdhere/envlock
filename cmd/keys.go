package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/itskdhere/envlock/pkg/crypto"
)

// DefaultKeyDir returns the OS user config directory for envlock keys.
// Windows: %APPDATA%\envlock\keys\
// Linux/macOS: ~/.config/envlock/keys/
func DefaultKeyDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("unable to determine user config directory: %w", err)
	}
	return filepath.Join(configDir, "envlock", "keys"), nil
}

// GlobalKeyPaths returns the path for the global private and public keys.
func GlobalKeyPaths() (privPath string, pubPath string, err error) {
	dir, err := DefaultKeyDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(dir, "default.key"), filepath.Join(dir, "default.pub"), nil
}

// ResolvePrivateKey returns the private key from the first source that has one:
// --key-file (path or "-" for stdin), ENVLOCK_KEY, ENVLOCK_KEY_PATH, ./envlock.key, global default.key.
func ResolvePrivateKey(keyFileFlag string) (string, error) {
	if keyFileFlag != "" {
		if keyFileFlag == "-" {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return "", fmt.Errorf("failed to read private key from stdin: %w", err)
			}
			return strings.TrimSpace(string(data)), nil
		}
		data, err := os.ReadFile(keyFileFlag)
		if err != nil {
			return "", fmt.Errorf("failed to read private key file %q: %w", keyFileFlag, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	if keyEnv := strings.TrimSpace(os.Getenv("ENVLOCK_KEY")); keyEnv != "" {
		return keyEnv, nil
	}

	if keyPathEnv := strings.TrimSpace(os.Getenv("ENVLOCK_KEY_PATH")); keyPathEnv != "" {
		data, err := os.ReadFile(keyPathEnv)
		if err != nil {
			return "", fmt.Errorf("failed to read private key from ENVLOCK_KEY_PATH %q: %w", keyPathEnv, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	localKey := "envlock.key"
	if data, err := os.ReadFile(localKey); err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	if privGlobal, _, err := GlobalKeyPaths(); err == nil {
		if data, err := os.ReadFile(privGlobal); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}

	return "", errors.New("no private key found. Set ENVLOCK_KEY, use --key-file, or run 'envlock keygen'")
}

// ResolveDefaultPublicKey locates an available public key for initial vault push:
// 1. Local ./envlock.pub
// 2. Global user config default.pub
func ResolveDefaultPublicKey() (string, error) {
	// 1. Local ./envlock.pub
	localPub := "envlock.pub"
	if data, err := os.ReadFile(localPub); err == nil {
		keyStr := strings.TrimSpace(string(data))
		if _, err := crypto.ParsePublicKey(keyStr); err == nil {
			return keyStr, nil
		}
	}

	// 2. Global user config
	if _, pubGlobal, err := GlobalKeyPaths(); err == nil {
		if data, err := os.ReadFile(pubGlobal); err == nil {
			keyStr := strings.TrimSpace(string(data))
			if _, err := crypto.ParsePublicKey(keyStr); err == nil {
				return keyStr, nil
			}
		}
	}

	return "", errors.New("no public key found. Run 'envlock keygen' to generate your keypair")
}
