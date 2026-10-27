package cmd

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/itskdhere/envlock/pkg/vault"
	"github.com/spf13/cobra"
)

var (
	flagPullForce bool
)

var pullCmd = &cobra.Command{
	Use:     "pull",
	Aliases: []string{"dec", "decrypt"},
	Short:   "Decrypt env.lock into local .env with byte-for-byte fidelity",
	Long: `Decrypts env.lock using your private key and writes the plaintext .env file.
Includes overwrite protection: if local .env has uncommitted changes, you will be prompted
before overwriting unless --force / -y is supplied.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		decrypted, err := decryptVault(paths.LockFile)
		if err != nil {
			return err
		}
		defer crypto.WipeBytes(decrypted)

		if existing, err := env.ReadEnvFile(paths.EnvFile); err == nil {
			defer crypto.WipeBytes(existing)
			if !bytes.Equal(existing, decrypted) {
				if !flagPullForce {
					fmt.Println(ui.Warning("Local %s has changes differing from %s", paths.EnvFile, paths.LockFile))
					fmt.Print("Overwrite local .env? [y/N]: ")

					reader := bufio.NewReader(os.Stdin)
					input, _ := reader.ReadString('\n')
					input = strings.TrimSpace(strings.ToLower(input))

					if input != "y" && input != "yes" {
						return errors.New("pull aborted: local changes preserved")
					}
				}
			}
		}

		if err := env.WriteEnvFile(paths.EnvFile, decrypted); err != nil {
			return fmt.Errorf("failed to write %s: %w", paths.EnvFile, err)
		}

		fmt.Println(ui.Success("Decrypted %s -> %s (byte-for-byte fidelity preserved)", paths.LockFile, paths.EnvFile))
		return nil
	},
}

// decryptVault decrypts lockFile in memory; the caller must wipe the result.
func decryptVault(lockFile string) ([]byte, error) {
	envelope, err := vault.ReadVault(lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault %s: %w", lockFile, err)
	}
	return openVault(envelope, lockFile)
}

func openVault(envelope *vault.VaultEnvelope, lockFile string) ([]byte, error) {
	privKey, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}

	callerPubKeyStr := crypto.FormatPublicKey(privKey.PublicKey())

	var matched *vault.RecipientEntry
	for i := range envelope.Recipients {
		if envelope.Recipients[i].PubKey == callerPubKeyStr {
			matched = &envelope.Recipients[i]
			break
		}
	}

	if matched == nil {
		return nil, fmt.Errorf("unauthorized: your public key (%s) is not authorized in %s", callerPubKeyStr, lockFile)
	}

	ephPub, err := crypto.ParsePublicKey(matched.EphemeralPubKey)
	if err != nil {
		return nil, fmt.Errorf("corrupt ephemeral public key in vault: %w", err)
	}

	dek, err := crypto.UnwrapDEK(matched.WrappedDEK, ephPub, privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
	}
	defer crypto.WipeBytes(dek)

	ivBytes, err := hex.DecodeString(envelope.IV)
	if err != nil {
		return nil, fmt.Errorf("corrupt IV in vault: %w", err)
	}

	ciphertextBytes, err := hex.DecodeString(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("corrupt ciphertext in vault: %w", err)
	}

	decrypted, err := crypto.DecryptPayload(ciphertextBytes, ivBytes, dek, envelope.PayloadAAD())
	if err != nil {
		return nil, fmt.Errorf("%s was modified or corrupted after it was written: %w", lockFile, err)
	}
	return decrypted, nil
}

func init() {
	pullCmd.Flags().BoolVarP(&flagPullForce, "force", "y", false, "Force overwrite local .env without confirmation prompt")
	rootCmd.AddCommand(pullCmd)
}
