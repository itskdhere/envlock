package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"

	"github.com/itskdhere/envlock/pkg/crypto"
	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/exec"
	"github.com/itskdhere/envlock/pkg/ui"
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
Stdin, stdout, stderr, signals, and the exit code pass through to the command.

Exit codes: the command's own, or 125 if envlock fails, 126 if the command
cannot be executed, 127 if it is not found.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return &exitError{125, fmt.Errorf("failed to get current working directory: %w", err)}
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		decrypted, err := decryptVault(paths.LockFile)
		if err != nil {
			return &exitError{125, err}
		}
		// Unset before Parse, or a ${ENVLOCK_KEY} vault value would expand to the key.
		os.Unsetenv("ENVLOCK_KEY")
		os.Unsetenv("ENVLOCK_KEY_PATH")

		local, err := env.ReadEnvFile(paths.EnvFile)
		if err == nil && !bytes.Equal(local, decrypted) {
			fmt.Fprintln(os.Stderr, ui.Warning("Local %s differs from %s; running with the vault's values (push or pull to sync)", paths.EnvFile, paths.LockFile))
		}
		crypto.WipeBytes(local)
		vars, err := env.Parse(decrypted)
		crypto.WipeBytes(decrypted)
		if err != nil {
			return &exitError{125, fmt.Errorf("failed to parse decrypted %s: %w", paths.LockFile, err)}
		}

		switch err := exec.Run(args[0], args[1:], vars, !flagRunNoOverride); {
		case errors.Is(err, osexec.ErrNotFound):
			return &exitError{127, err}
		case err != nil:
			return &exitError{126, err}
		}
		return nil
	},
}

func init() {
	runCmd.Flags().BoolVar(&flagRunNoOverride, "no-override", false, "Let host environment variables take precedence over vault variables")
	// Pass flags after the command name through to it: `el run node app.js --port 3000`.
	runCmd.Flags().SetInterspersed(false)
	rootCmd.AddCommand(runCmd)
}
