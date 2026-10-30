package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/itskdhere/envlock/pkg/vault"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var flagRecipientForce bool

var recipientCmd = &cobra.Command{
	Use:     "recipient",
	Aliases: []string{"rec"},
	Short:   "List, add, or remove the public keys that can decrypt env.lock",
	// Without RunE, cobra prints help and exits 0 on a mistyped subcommand.
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var recipientListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List the public keys env.lock is encrypted for",
	Long: `Prints the recipient public keys in env.lock's header, one per line.
No private key is needed. A hand-edited list is only caught when the vault is decrypted.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		lockFile := env.ResolvePaths(".", flagEnv, flagFile).LockFile
		envelope, err := vault.ReadVault(lockFile)
		if err != nil {
			return fmt.Errorf("failed to read vault %s: %w", lockFile, err)
		}
		pubKeys := envelope.PubKeys()

		// If stdout is redirected or non-terminal, output raw keys for machine-friendliness
		if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
			for _, key := range pubKeys {
				fmt.Println(key)
			}
			return nil
		}

		callerPubStr := ""
		if privKey, err := loadPrivateKey(); err == nil {
			callerPubStr = crypto.FormatPublicKey(privKey.PublicKey())
		}

		fmt.Printf("Authorized recipients for %s (%d total):\n", ui.Bold(lockFile), len(pubKeys))
		for _, key := range pubKeys {
			if key == callerPubStr {
				fmt.Printf("  • %s %s\n", key, ui.Highlight("(you)"))
			} else {
				fmt.Printf("  • %s\n", key)
			}
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
		lockFile, err := editRecipients(func(keys []string, _ string) ([]string, error) {
			if slices.Contains(keys, key) {
				return nil, fmt.Errorf("%s is already a recipient", key)
			}
			return append(keys, key), nil
		})
		if err == nil {
			fmt.Println(ui.Subtle("  Commit to Git: git add " + lockFile))
		}
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
		lockFile, err := editRecipients(func(keys []string, self string) ([]string, error) {
			i := slices.Index(keys, key)
			if i < 0 {
				return nil, fmt.Errorf("%s is not a recipient", key)
			}
			if len(keys) == 1 {
				return nil, errors.New("refusing to remove the last recipient: nobody could decrypt the vault")
			}
			if key == self && !flagRecipientForce && !confirm("Remove your own key? You won't be able to decrypt the vault anymore") {
				return nil, errors.New("aborted: your key was not removed")
			}
			return slices.Delete(keys, i, i+1), nil
		})
		if err != nil {
			return err
		}
		fmt.Println(ui.Warning("%s can still decrypt older %s versions in Git history. Rotate the secrets it could read", key, lockFile))
		fmt.Println(ui.Subtle("  Commit to Git: git add " + lockFile))
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

func uniqueRecipients(envelope *vault.VaultEnvelope) ([]string, error) {
	var keys []string
	for _, k := range envelope.PubKeys() {
		key, err := parseRecipient(k)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

func unknownRecipients(lockFile string, keys []string) ([]string, error) {
	file := knownRecipientsFile(lockFile)
	if file == "" {
		return nil, nil
	}
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read the recipients you last encrypted %s for: %w", lockFile, err)
	}

	known := strings.Fields(string(data))
	var unknown []string
	for _, k := range keys {
		if !slices.Contains(known, k) {
			unknown = append(unknown, k)
		}
	}
	return unknown, nil
}

func unknownRecipientsWarning(lockFile string, unknown []string) string {
	return ui.Warning("%s is also encrypted for key(s) you haven't encrypted it for before:\n  %s", lockFile, strings.Join(unknown, "\n  "))
}

func confirmNewRecipients(lockFile string, keys []string, force bool) error {
	unknown, err := unknownRecipients(lockFile, keys)
	if err != nil || len(unknown) == 0 {
		return err
	}
	fmt.Println(unknownRecipientsWarning(lockFile, unknown))
	if !force && !confirm("Encrypt for them too?") {
		return errors.New("aborted: check these keys with your team, then rerun with --force / -y")
	}
	return nil
}

func rememberRecipients(lockFile string, keys []string) error {
	file := knownRecipientsFile(lockFile)
	if file == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(strings.Join(keys, "\n")+"\n"), 0600)
}

func knownRecipientsFile(lockFile string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	lockFile, err = filepath.Abs(lockFile)
	if err != nil {
		return ""
	}
	// Terminals disagree on drive letter case.
	if runtime.GOOS == "windows" {
		lockFile = strings.ToLower(lockFile)
	}
	sum := sha256.Sum256([]byte(lockFile))
	return filepath.Join(dir, "envlock", "known", hex.EncodeToString(sum[:]))
}

func editRecipients(edit func(keys []string, self string) ([]string, error)) (lockFile string, err error) {
	lockFile = env.ResolvePaths(".", flagEnv, flagFile).LockFile
	envelope, err := vault.ReadVault(lockFile)
	if err != nil {
		return "", fmt.Errorf("failed to read vault %s: %w", lockFile, err)
	}
	privKey, err := loadPrivateKey()
	if err != nil {
		return "", err
	}
	plaintext, err := openVault(envelope, lockFile, privKey)
	if err != nil {
		return "", err
	}
	defer crypto.WipeBytes(plaintext)

	keys, err := uniqueRecipients(envelope)
	if err != nil {
		return "", err
	}
	if err := confirmNewRecipients(lockFile, keys, flagRecipientForce); err != nil {
		return "", err
	}
	keys, err = edit(keys, crypto.FormatPublicKey(privKey.PublicKey()))
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
	for _, c := range []*cobra.Command{recipientAddCmd, recipientRemoveCmd} {
		c.Flags().BoolVarP(&flagRecipientForce, "force", "y", false, "Skip confirmations for unfamiliar recipients and removing your own key")
	}
	recipientCmd.AddCommand(recipientListCmd, recipientAddCmd, recipientRemoveCmd)
	rootCmd.AddCommand(recipientCmd)
}
