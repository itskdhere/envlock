package cmd

import (
	"fmt"
	"os"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/exec"
	"github.com/spf13/cobra"
)

var (
	flagRunNoOverride bool
)

var runCmd = &cobra.Command{
	Use:   "run -- <command> [args...]",
	Short: "Run a command with env.lock secrets injected in memory (zero disk)",
	Long: `Decrypts env.lock in memory and runs the command with its variables injected
into the environment. Plaintext secrets never touch disk, making this the
preferred way to consume secrets in CI/CD (set ENVLOCK_KEY from your CI secrets).
Vault variables override host variables unless --no-override is supplied.
Stdin, stdout, stderr, signals, and the exit code pass through to the command.`,
	Args: cobra.MinimumNArgs(1),
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
		vars, err := env.Parse(decrypted)
		crypto.WipeBytes(decrypted)
		if err != nil {
			return fmt.Errorf("failed to parse decrypted %s: %w", paths.LockFile, err)
		}

		return exec.Run(args[0], args[1:], vars, !flagRunNoOverride)
	},
}

func init() {
	runCmd.Flags().BoolVar(&flagRunNoOverride, "no-override", false, "Let host environment variables take precedence over vault variables")
	// Pass flags after the command name through to it: `el run node app.js --port 3000`.
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}
