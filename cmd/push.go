package cmd

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/itskdhere/envlock/pkg/vault"
	"github.com/spf13/cobra"
)

var flagPushForce bool

var pushCmd = &cobra.Command{
	Use:     "push",
	Aliases: []string{"enc", "encrypt"},
	Short:   "Encrypt local .env into tamper-evident env.lock",
	Long: `Encrypts raw .env bytes into env.lock using Curve25519 asymmetric envelope encryption.
Preserves 100% of formatting, comments, blank lines, and custom quotes.
Re-encrypts for the recipients of the existing env.lock, which your key must be able to
decrypt, or encrypts for your own key on the initial push.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := env.ResolvePaths(".", flagEnv, flagFile)

		rawEnv, err := env.ReadEnvFile(paths.EnvFile)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", paths.EnvFile, err)
		}
		defer crypto.WipeBytes(rawEnv)

		privKey, err := loadPrivateKey()
		if err != nil {
			return err
		}

		var recipientPubKeys []string
		existingVault, err := vault.ReadVault(paths.LockFile)
		switch {
		case err == nil:
			// Decrypting rejects a hand-edited recipient list, not a whole vault forged for extra keys.
			decrypted, err := openVault(existingVault, paths.LockFile, privKey)
			if err != nil {
				return fmt.Errorf("refusing to overwrite %s: %w", paths.LockFile, err)
			}
			crypto.WipeBytes(decrypted)

			recipientPubKeys, err = uniqueRecipients(existingVault)
			if err != nil {
				return err
			}
			if err := confirmNewRecipients(paths.LockFile, recipientPubKeys, flagPushForce); err != nil {
				return err
			}
		case errors.Is(err, fs.ErrNotExist):
			recipientPubKeys = []string{crypto.FormatPublicKey(privKey.PublicKey())}
		default:
			return fmt.Errorf("refusing to overwrite %s: %w", paths.LockFile, err)
		}

		if err := sealVault(paths.LockFile, rawEnv, recipientPubKeys); err != nil {
			return err
		}

		parsedVars, _ := env.Parse(rawEnv)
		if len(parsedVars) > 0 {
			fmt.Println(ui.Success("Encrypted %d variable(s) from %s -> %s (%d recipient(s))", len(parsedVars), paths.EnvFile, paths.LockFile, len(recipientPubKeys)))
		} else {
			fmt.Println(ui.Success("Encrypted %s -> %s (%d recipient(s))", paths.EnvFile, paths.LockFile, len(recipientPubKeys)))
		}
		fmt.Println(ui.Subtle("  Commit to Git: git add " + paths.LockFile))
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
	if err := rememberRecipients(lockFile, pubKeys); err != nil {
		fmt.Println(ui.Warning("Failed to record the recipients of %s, so keys added elsewhere won't be flagged: %v", lockFile, err))
	}
	return nil
}

func init() {
	pushCmd.Flags().BoolVarP(&flagPushForce, "force", "y", false, "Encrypt for recipients you haven't encrypted for before without asking")
	rootCmd.AddCommand(pushCmd)
}
