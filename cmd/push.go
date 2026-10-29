package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
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
decrypt, or encrypts for your own key on the initial push.`,
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
		existingVault, err := vault.ReadVault(paths.LockFile)
		switch {
		case err == nil:
			// Decrypting authenticates the recipient list, so hand-added recipients are rejected here.
			decrypted, err := openVault(existingVault, paths.LockFile)
			if err != nil {
				return fmt.Errorf("refusing to overwrite %s: %w", paths.LockFile, err)
			}
			crypto.WipeBytes(decrypted)

			recipientPubKeys, err = uniqueRecipients(existingVault)
			if err != nil {
				return err
			}
		case errors.Is(err, fs.ErrNotExist):
			privKey, err := loadPrivateKey()
			if err != nil {
				return err
			}
			recipientPubKeys = []string{crypto.FormatPublicKey(privKey.PublicKey())}
		default:
			return fmt.Errorf("refusing to overwrite %s: %w", paths.LockFile, err)
		}

		if err := sealVault(paths.LockFile, rawEnv, recipientPubKeys); err != nil {
			return err
		}

		fmt.Println(ui.Success("Encrypted %s -> %s (%d recipient(s))", paths.EnvFile, paths.LockFile, len(recipientPubKeys)))
		return nil
	},
}

func sealVault(lockFile string, plaintext []byte, pubKeys []string) error {
	dek, err := crypto.GenerateDEK()
	if err != nil {
		return fmt.Errorf("failed to generate DEK: %w", err)
	}
	defer crypto.WipeBytes(dek)

	recipientEntries := make([]vault.RecipientEntry, 0, len(pubKeys))
	for _, pubKeyStr := range pubKeys {
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
	iv, ciphertext, err := crypto.EncryptPayload(plaintext, dek, envelope.PayloadAAD())
	if err != nil {
		return fmt.Errorf("failed to encrypt payload: %w", err)
	}
	envelope.IV = hex.EncodeToString(iv)
	envelope.Ciphertext = hex.EncodeToString(ciphertext)

	if err := vault.WriteVault(lockFile, envelope); err != nil {
		return fmt.Errorf("failed to write vault file %s: %w", lockFile, err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(pushCmd)
}
