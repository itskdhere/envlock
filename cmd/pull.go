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
	Short:   "Decrypt .env.lock into local .env with byte-for-byte fidelity",
	Long: `Decrypts .env.lock using your private key and writes the plaintext .env file.
Includes overwrite protection: if local .env has uncommitted changes, you will be prompted
before overwriting unless --force / -y is supplied.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		// 1. Read .env.lock
		envelope, err := vault.ReadVault(paths.LockFile)
		if err != nil {
			return fmt.Errorf("failed to read vault %s: %w", paths.LockFile, err)
		}

		// 2. Resolve caller's private key
		privKeyStr, err := ResolvePrivateKey(flagKeyFile)
		if err != nil {
			return err
		}

		privKey, err := crypto.ParsePrivateKey(privKeyStr)
		if err != nil {
			return fmt.Errorf("failed to parse private key: %w", err)
		}

		callerPubKeyStr := crypto.FormatPublicKey(privKey.PublicKey())

		// 3. Find matching recipient entry
		var matched *vault.RecipientEntry
		for i := range envelope.Recipients {
			if envelope.Recipients[i].PubKey == callerPubKeyStr {
				matched = &envelope.Recipients[i]
				break
			}
		}

		if matched == nil {
			return fmt.Errorf("unauthorized: your public key (%s) is not authorized in %s", callerPubKeyStr, paths.LockFile)
		}

		// 4. Parse ephemeral public key
		ephPub, err := crypto.ParsePublicKey(matched.EphemeralPubKey)
		if err != nil {
			return fmt.Errorf("corrupt ephemeral public key in vault: %w", err)
		}

		// 5. Unwrap DEK
		dek, err := crypto.UnwrapDEK(matched.WrappedDEK, ephPub, privKey)
		if err != nil {
			return fmt.Errorf("failed to unwrap DEK: %w", err)
		}
		defer crypto.WipeBytes(dek)

		// 6. Decrypt ciphertext payload
		ivBytes, err := hex.DecodeString(envelope.IV)
		if err != nil {
			return fmt.Errorf("corrupt IV in vault: %w", err)
		}

		ciphertextBytes, err := hex.DecodeString(envelope.Ciphertext)
		if err != nil {
			return fmt.Errorf("corrupt ciphertext in vault: %w", err)
		}

		decrypted, err := crypto.DecryptPayload(ciphertextBytes, ivBytes, dek)
		if err != nil {
			return fmt.Errorf("failed to decrypt payload: %w", err)
		}
		defer crypto.WipeBytes(decrypted)

		// 7. Safety check: Compare with local .env if it exists
		if existing, err := env.ReadEnvFile(paths.EnvFile); err == nil {
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

		// 8. Atomically write decrypted bytes with 0600 permissions
		if err := env.WriteEnvFile(paths.EnvFile, decrypted); err != nil {
			return fmt.Errorf("failed to write %s: %w", paths.EnvFile, err)
		}

		fmt.Println(ui.Success("Decrypted %s -> %s (byte-for-byte fidelity preserved)", paths.LockFile, paths.EnvFile))
		return nil
	},
}

func init() {
	pullCmd.Flags().BoolVarP(&flagPullForce, "force", "y", false, "Force overwrite local .env without confirmation prompt")
	rootCmd.AddCommand(pullCmd)
}

