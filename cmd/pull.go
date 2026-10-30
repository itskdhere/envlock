package cmd

import (
	"bytes"
	"crypto/ecdh"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/charmbracelet/lipgloss"
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
		paths := env.ResolvePaths(".", flagEnv, flagFile)

		decrypted, err := decryptVault(paths.LockFile)
		if err != nil {
			return err
		}
		defer crypto.WipeBytes(decrypted)

		if existing, err := env.ReadEnvFile(paths.EnvFile); err == nil {
			defer crypto.WipeBytes(existing)
			if !bytes.Equal(existing, decrypted) && !flagPullForce {
				fmt.Println(ui.Warning("Local %s has changes differing from %s", paths.EnvFile, paths.LockFile))
				printEnvDiffSummary(existing, decrypted)
				if !confirm("Overwrite local .env?") {
					return errors.New("pull aborted: local changes preserved")
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

func printEnvDiffSummary(local, remote []byte) {
	localVars, errLocal := env.Parse(local)
	remoteVars, errRemote := env.Parse(remote)
	if errLocal != nil || errRemote != nil {
		return
	}

	var added, removed, modified []string
	for k, vLocal := range localVars {
		if vRemote, ok := remoteVars[k]; !ok {
			added = append(added, k)
		} else if vLocal != vRemote {
			modified = append(modified, k)
		}
	}
	for k := range remoteVars {
		if _, ok := localVars[k]; !ok {
			removed = append(removed, k)
		}
	}

	slices.Sort(added)
	slices.Sort(removed)
	slices.Sort(modified)

	if len(added) == 0 && len(removed) == 0 && len(modified) == 0 {
		fmt.Println(ui.Subtle("  (Differences are in comments, whitespace, or formatting)"))
		return
	}

	for _, k := range added {
		fmt.Printf("  %s %s %s\n", lipgloss.NewStyle().Foreground(ui.SuccessColor).Render("+"), ui.Bold(k), ui.Subtle("(added locally)"))
	}
	for _, k := range modified {
		fmt.Printf("  %s %s %s\n", lipgloss.NewStyle().Foreground(ui.WarningColor).Render("~"), ui.Bold(k), ui.Subtle("(modified locally)"))
	}
	for _, k := range removed {
		fmt.Printf("  %s %s %s\n", lipgloss.NewStyle().Foreground(ui.ErrorColor).Render("-"), ui.Bold(k), ui.Subtle("(missing locally / in vault)"))
	}
}

// decryptVault decrypts lockFile in memory; the caller must wipe the result.
func decryptVault(lockFile string) ([]byte, error) {
	envelope, err := vault.ReadVault(lockFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read vault %s: %w", lockFile, err)
	}
	privKey, err := loadPrivateKey()
	if err != nil {
		return nil, err
	}
	plaintext, err := openVault(envelope, lockFile, privKey)
	if err != nil {
		return nil, err
	}
	if keys, err := uniqueRecipients(envelope); err == nil {
		if unknown, _ := unknownRecipients(lockFile, keys); len(unknown) > 0 {
			fmt.Fprintln(os.Stderr, unknownRecipientsWarning(lockFile, unknown))
		}
	}
	return plaintext, nil
}

func openVault(envelope *vault.VaultEnvelope, lockFile string, privKey *ecdh.PrivateKey) ([]byte, error) {
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
