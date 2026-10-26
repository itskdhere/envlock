package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	flagEnv     string
	flagFile    string
	flagKeyFile string
	flagVersion bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "envlock",
	Short: "Zero-cloud, asymmetric envelope encryption for environment secrets",
	Long: `EnvLock (el) keeps your .env files local and uncommitted while securely storing 
tamper-evident, encrypted snapshots (env.lock) directly in your Git repository.
Protected by Curve25519 (X25519 ECDH) asymmetric envelope encryption and AES-256-GCM.`,
	// Show usage only for flag/argument mistakes, not for runtime errors.
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
	},
	Run: func(cmd *cobra.Command, args []string) {
		if flagVersion {
			fmt.Println(VersionString())
			return
		}
		_ = cmd.Help()
	},
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	err := rootCmd.Execute()
	var exitErr *exitError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	if err != nil {
		return 1
	}
	return 0
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "", "Target environment name (e.g. 'prod' maps to .env.prod and env.prod.lock)")
	rootCmd.PersistentFlags().StringVarP(&flagFile, "file", "f", "", "Explicit path to .env file")
	rootCmd.PersistentFlags().StringVar(&flagKeyFile, "key-file", "", "Explicit path to private key file (or '-' for stdin)")
	rootCmd.Flags().BoolVarP(&flagVersion, "version", "v", false, "Print version and build information")

	// Set custom help template with styled accents
	rootCmd.SetHelpTemplate(`{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`)
}
