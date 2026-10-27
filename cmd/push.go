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
	Short:   "Encrypt local .env into tamper-evident env.lock",
	Long: `Encrypts raw .env bytes into env.lock using Curve25519 asymmetric envelope encryption.
Preserves 100% of formatting, comments, blank lines, and custom quotes.
Re-encrypts for the recipients of the existing env.lock, which your key must be able to
decrypt, or auto-discovers your local/global public key on the initial push.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		rawEnv, err := env.ReadEnvFile(paths.EnvFile)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", paths.EnvFile, err)
		}
		defer crypto.WipeBytes(rawEnv)

		var recipientPubKeys []string
		if existingVault, err := vault.ReadVault(paths.LockFile); err == nil {
			// Decrypting authenticates the recipient list, so hand-added recipients are rejected here.
			decrypted, err := openVault(existingVault, paths.LockFile)
			if err != nil {
				return fmt.Errorf("refusing to overwrite %s: %w", paths.LockFile, err)
			}
			crypto.WipeBytes(decrypted)

			seen := make(map[string]bool)
			for _, rec := range existingVault.Recipients {
				if !seen[rec.PubKey] {
					seen[rec.PubKey] = true
					recipientPubKeys = append(recipientPubKeys, rec.PubKey)
				}
			}
		}

		if len(recipientPubKeys) == 0 {
			defaultPub, err := ResolveDefaultPublicKey()
			if err != nil {
				return fmt.Errorf("no recipient public key found for initial push: %w", err)
			}
			recipientPubKeys = append(recipientPubKeys, defaultPub)
		}

		dek, err := crypto.GenerateDEK()
		if err != nil {
			return fmt.Errorf("failed to generate DEK: %w", err)
		}
		defer crypto.WipeBytes(dek)

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

		envelope := &vault.VaultEnvelope{
			Version:    vault.VaultVersion,
			Cipher:     vault.CipherAES256GCM,
			Recipients: recipientEntries,
		}
		iv, ciphertext, err := crypto.EncryptPayload(rawEnv, dek, envelope.PayloadAAD())
		if err != nil {
			return fmt.Errorf("failed to encrypt payload: %w", err)
		}
		envelope.IV = hex.EncodeToString(iv)
		envelope.Ciphertext = hex.EncodeToString(ciphertext)

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
