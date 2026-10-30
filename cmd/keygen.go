package cmd

import (
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	flagKeygenGlobal bool
	flagKeygenOut    string
	flagKeygenStdout bool
	flagKeygenForce  bool
)

var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a Curve25519 (X25519) identity keypair",
	Long: `Generates a cryptographic X25519 keypair for envelope encryption.
The private key (el_sec_...) is saved readable only by you (0600 on Unix, an owner-only ACL on Windows).
The public key (el_pub_...) can be safely shared with teammates.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		priv, pub, err := crypto.GenerateKeyPair()
		if err != nil {
			return fmt.Errorf("failed to generate keypair: %w", err)
		}

		privFormatted := crypto.FormatPrivateKey(priv)
		pubFormatted := crypto.FormatPublicKey(pub)

		if flagKeygenStdout {
			fmt.Printf("PRIVATE_KEY=%s\n", privFormatted)
			fmt.Printf("PUBLIC_KEY=%s\n", pubFormatted)
			return nil
		}

		var privPath, pubPath string
		if flagKeygenGlobal {
			var err error
			privPath, pubPath, err = GlobalKeyPaths()
			if err != nil {
				return err
			}
		} else if flagKeygenOut != "" {
			privPath = filepath.Join(flagKeygenOut, "envlock.key")
			pubPath = filepath.Join(flagKeygenOut, "envlock.pub")
		} else {
			privPath = "envlock.key"
			pubPath = "envlock.pub"
		}

		// Overwrite guard
		if !flagKeygenForce {
			if _, err := os.Stat(privPath); err == nil {
				return fmt.Errorf("key file %q already exists. Use --force / -y to overwrite", privPath)
			}
			if _, err := os.Stat(pubPath); err == nil {
				return fmt.Errorf("public key file %q already exists. Use --force / -y to overwrite", pubPath)
			}
		}

		// Ensure target directory exists
		if dir := filepath.Dir(privPath); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("failed to create directory %q: %w", dir, err)
			}
		}

		if err := env.WriteEnvFile(privPath, []byte(privFormatted+"\n")); err != nil {
			return fmt.Errorf("failed to write private key to %q: %w", privPath, err)
		}

		// Write public key with 0644
		if err := os.WriteFile(pubPath, []byte(pubFormatted+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write public key to %q: %w", pubPath, err)
		}

		copied := ui.CopyToClipboardOSC52(pubFormatted)

		lines := []string{
			fmt.Sprintf("🔒 %s %s  %s", ui.Bold("Private Key:"), privPath, ui.Subtle("(owner-only ACL, NEVER share or commit)")),
			fmt.Sprintf("🌐 %s  %s", ui.Bold("Public Key:"), pubPath),
			"",
			ui.Bold("Public Identity Key:"),
			fmt.Sprintf("  %s", ui.Highlight(pubFormatted)),
			"",
		}

		if copied {
			lines = append(lines, fmt.Sprintf("%s Public identity key copied to clipboard! Share it with your team.", ui.SuccessIcon.String()))
		} else {
			lines = append(lines, "Share this public key with your team to authorize vault access.")
		}

		fmt.Println(ui.Banner("Keypair Generated Successfully", lines))
		if !flagKeygenGlobal && notGitIgnored(privPath) {
			fmt.Println(ui.Warning("%s is not gitignored. Run 'envlock init' before committing, or use --global", privPath))
		}
		return nil
	},
}

// notGitIgnored reports whether path is inside a git work tree and not ignored by it.
func notGitIgnored(path string) bool {
	cmd := osexec.Command("git", "check-ignore", "-q", filepath.Base(path))
	cmd.Dir = filepath.Dir(path)
	err := cmd.Run()
	var exitErr *osexec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func init() {
	keygenCmd.Flags().BoolVarP(&flagKeygenGlobal, "global", "g", false, "Save keys to global user config directory")
	keygenCmd.Flags().StringVar(&flagKeygenOut, "out", "", "Directory to write key files to")
	keygenCmd.Flags().BoolVar(&flagKeygenStdout, "stdout", false, "Print keys directly to stdout without writing files")
	keygenCmd.Flags().BoolVarP(&flagKeygenForce, "force", "y", false, "Overwrite existing key files if present")

	rootCmd.AddCommand(keygenCmd)
}
