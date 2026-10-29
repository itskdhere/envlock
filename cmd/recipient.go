package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/itskdhere/envlock/pkg/vault"
	"github.com/spf13/cobra"
)

var recipientCmd = &cobra.Command{
	Use:   "recipient",
	Short: "List, add, or remove the public keys that can decrypt env.lock",
}

var recipientListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the public keys env.lock is encrypted for",
	Long: `Prints the recipient public keys in env.lock's header, one per line.
No private key is needed. A hand-edited list is only caught when the vault is decrypted.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		lockFile := env.ResolvePaths(cwd, flagEnv, flagFile).LockFile
		envelope, err := vault.ReadVault(lockFile)
		if err != nil {
			return fmt.Errorf("failed to read vault %s: %w", lockFile, err)
		}
		for _, key := range envelope.PubKeys() {
			fmt.Println(key)
		}
		return nil
	},
}

var recipientAddCmd = &cobra.Command{
	Use:   "add <pubkey>",
	Short: "Authorize a public key to decrypt env.lock",
	Long: `Decrypts env.lock with your private key and re-encrypts it under a fresh DEK
for the existing recipients plus <pubkey> (an el_pub_... key from 'envlock keygen').`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := parseRecipient(args[0])
		if err != nil {
			return err
		}
		_, err = editRecipients(func(keys []string) ([]string, error) {
			if slices.Contains(keys, key) {
				return nil, fmt.Errorf("%s is already a recipient", key)
			}
			return append(keys, key), nil
		})
		return err
	},
}

var recipientRemoveCmd = &cobra.Command{
	Use:     "remove <pubkey>",
	Aliases: []string{"rm"},
	Short:   "Revoke a public key's access to env.lock",
	Long: `Decrypts env.lock with your private key and re-encrypts it under a brand-new DEK
for the remaining recipients, so the removed key can't decrypt it.
The removed key still decrypts older env.lock versions in Git history and already
had the plaintext, so rotate the secrets it could read.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key, err := parseRecipient(args[0])
		if err != nil {
			return err
		}
		lockFile, err := editRecipients(func(keys []string) ([]string, error) {
			i := slices.Index(keys, key)
			if i < 0 {
				return nil, fmt.Errorf("%s is not a recipient", key)
			}
			if len(keys) == 1 {
				return nil, errors.New("refusing to remove the last recipient: nobody could decrypt the vault")
			}
			return slices.Delete(keys, i, i+1), nil
		})
		if err != nil {
			return err
		}
		fmt.Println(ui.Warning("%s can still decrypt older %s versions in Git history. Rotate the secrets it could read", key, lockFile))
		return nil
	},
}

func parseRecipient(arg string) (string, error) {
	pub, err := crypto.ParsePublicKey(arg)
	if err != nil {
		return "", fmt.Errorf("invalid public key %q: %w", arg, err)
	}
	return crypto.FormatPublicKey(pub), nil
}

func editRecipients(edit func(keys []string) ([]string, error)) (lockFile string, err error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	lockFile = env.ResolvePaths(cwd, flagEnv, flagFile).LockFile
	envelope, err := vault.ReadVault(lockFile)
	if err != nil {
		return "", fmt.Errorf("failed to read vault %s: %w", lockFile, err)
	}
	plaintext, err := openVault(envelope, lockFile)
	if err != nil {
		return "", err
	}
	defer crypto.WipeBytes(plaintext)

	keys, err := edit(envelope.PubKeys())
	if err != nil {
		return "", err
	}
	if err := sealVault(lockFile, plaintext, keys); err != nil {
		return "", err
	}

	fmt.Println(ui.Success("Re-encrypted %s for %d recipient(s)", lockFile, len(keys)))
	return lockFile, nil
}

func init() {
	recipientCmd.AddCommand(recipientListCmd, recipientAddCmd, recipientRemoveCmd)
	rootCmd.AddCommand(recipientCmd)
}
