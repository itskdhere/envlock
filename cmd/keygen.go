package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/itskdhere/envlock/pkg/crypto"
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
The private key (el_sec_...) is saved with restricted 0600 permissions.
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

		// Write private key with 0600
		if err := os.WriteFile(privPath, []byte(privFormatted+"\n"), 0600); err != nil {
			return fmt.Errorf("failed to write private key to %q: %w", privPath, err)
		}
		_ = os.Chmod(privPath, 0600)

		// Write public key with 0644
		if err := os.WriteFile(pubPath, []byte(pubFormatted+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write public key to %q: %w", pubPath, err)
		}

		lines := []string{
			fmt.Sprintf("• Private Key: %s  (mode 0600, NEVER share or commit)", privPath),
			fmt.Sprintf("• Public Key:  %s", pubPath),
			"",
			"Public Identity Key:",
			fmt.Sprintf("  %s", pubFormatted),
			"",
			"Share this public key with your team to authorize vault access.",
		}

		fmt.Println(ui.Banner("Keypair Generated Successfully", lines))
		return nil
	},
}

func init() {
	keygenCmd.Flags().BoolVarP(&flagKeygenGlobal, "global", "g", false, "Save keys to global user config directory")
	keygenCmd.Flags().StringVar(&flagKeygenOut, "out", "", "Directory to write key files to")
	keygenCmd.Flags().BoolVar(&flagKeygenStdout, "stdout", false, "Print keys directly to stdout without writing files")
	keygenCmd.Flags().BoolVarP(&flagKeygenForce, "force", "y", false, "Overwrite existing key files if present")

	rootCmd.AddCommand(keygenCmd)
}
