package cmd

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/itskdhere/envlock/pkg/vault"
	"github.com/spf13/cobra"
)

var pushCmd = &cobra.Command{
	Use:     "push",
	Aliases: []string{"enc", "encrypt"},
	Short:   "Encrypt local .env into tamper-evident .env.lock",
	Long: `Encrypts raw .env bytes into .env.lock using Curve25519 asymmetric envelope encryption.
Preserves 100% of formatting, comments, blank lines, and custom quotes.
Automatically encrypts for all authorized recipients defined in existing .env.lock,
or auto-discovers your local/global public key on the initial push.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		// 1. Read raw .env bytes
		rawEnv, err := env.ReadEnvFile(paths.EnvFile)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", paths.EnvFile, err)
		}
		defer crypto.WipeBytes(rawEnv)

		// 2. Determine recipient public keys
		var recipientPubKeys []string

		// Check if vault already exists
		if existingVault, err := vault.ReadVault(paths.LockFile); err == nil {
			seen := make(map[string]bool)
			for _, rec := range existingVault.Recipients {
				if !seen[rec.PubKey] {
					seen[rec.PubKey] = true
					recipientPubKeys = append(recipientPubKeys, rec.PubKey)
				}
			}
		}

		// If no recipients found (e.g. initial push), discover local or global public key
		if len(recipientPubKeys) == 0 {
			defaultPub, err := ResolveDefaultPublicKey()
			if err != nil {
				return fmt.Errorf("no recipient public key found for initial push: %w", err)
			}
			recipientPubKeys = append(recipientPubKeys, defaultPub)
		}

		// 3. Generate a fresh 256-bit DEK
		dek, err := crypto.GenerateDEK()
		if err != nil {
			return fmt.Errorf("failed to generate DEK: %w", err)
		}
		defer crypto.WipeBytes(dek)

		// 4. Encrypt raw payload with AES-256-GCM
		iv, ciphertext, err := crypto.EncryptPayload(rawEnv, dek)
		if err != nil {
			return fmt.Errorf("failed to encrypt payload: %w", err)
		}

		// 5. Wrap DEK for every recipient
		recipientEntries := make([]vault.RecipientEntry, 0, len(recipientPubKeys))
		for _, pubKeyStr := range recipientPubKeys {
			pubKey, err := crypto.ParsePublicKey(pubKeyStr)
			if err != nil {
				return fmt.Errorf("invalid recipient public key %q: %w", pubKeyStr, err)
			}

			ephPub, wrappedDEKHex, err := crypto.WrapDEK(dek, pubKey)
			if err != nil {
				return fmt.Errorf("failed to wrap DEK for recipient %s: %w", pubKeyStr, err)
			}

			recipientEntries = append(recipientEntries, vault.RecipientEntry{
				PubKey:          pubKeyStr,
				EphemeralPubKey: crypto.FormatPublicKey(ephPub),
				WrappedDEK:      wrappedDEKHex,
			})
		}

		// 6. Construct and atomically write envelope
		envelope := &vault.VaultEnvelope{
			Version:    vault.VaultVersion,
			Cipher:     vault.CipherAES256GCM,
			IV:         hex.EncodeToString(iv),
			Ciphertext: hex.EncodeToString(ciphertext),
			Recipients: recipientEntries,
		}

		if err := vault.WriteVault(paths.LockFile, envelope); err != nil {
			return fmt.Errorf("failed to write vault file %s: %w", paths.LockFile, err)
		}

		fmt.Println(ui.Success("Encrypted %s -> %s (%d recipient(s))", paths.EnvFile, paths.LockFile, len(recipientEntries)))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
}

