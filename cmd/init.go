package cmd

import (
	"fmt"
	"os"

	"github.com/itskdhere/envlock/pkg/env"
	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize project safety defaults (.gitignore, .gitattributes, sample .env)",
	Long: `Prepares the current workspace for EnvLock by:
1. Creating a sample .env file if neither it nor env.lock exists.
2. Appending .env, .env.*.local, *.key, and *.pub to .gitignore, while exempting lock files (!env.lock, !env.*.lock).
3. Appending *.lock merge=binary to .gitattributes to prevent Git merge conflict corruption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		_, err = os.Stat(paths.LockFile)
		hasVault := err == nil

		createdEnv := false
		if !hasVault {
			if createdEnv, err = env.CreateDefaultEnv(paths.EnvFile); err != nil {
				return fmt.Errorf("failed to create default env file: %w", err)
			}
		}

		addedGitignore, err := env.EnsureGitignore(cwd)
		if err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}

		addedGitattributes, err := env.EnsureGitattributes(cwd)
		if err != nil {
			return fmt.Errorf("failed to update .gitattributes: %w", err)
		}

		var lines []string
		switch {
		case hasVault:
			lines = append(lines, fmt.Sprintf("• Existing vault found, no template env file created: %s", paths.LockFile))
		case createdEnv:
			lines = append(lines, fmt.Sprintf("+ Created template env file: %s", paths.EnvFile))
		default:
			lines = append(lines, fmt.Sprintf("• Existing env file preserved: %s", paths.EnvFile))
		}

		if len(addedGitignore) > 0 {
			lines = append(lines, fmt.Sprintf("+ Updated .gitignore with %d safety rules", len(addedGitignore)))
		} else {
			lines = append(lines, "• .gitignore safety rules already present")
		}

		if len(addedGitattributes) > 0 {
			lines = append(lines, "+ Updated .gitattributes with binary merge rules")
		} else {
			lines = append(lines, "• .gitattributes binary merge rules already present")
		}

		lines = append(lines, "")
		lines = append(lines, "Next steps:")
		if hasVault {
			lines = append(lines, "  1. Decrypt your secrets:  envlock pull")
			lines = append(lines, "     No access yet? Run 'envlock keygen' and ask a teammate to add your envlock.pub")
		} else {
			lines = append(lines, "  1. Generate your identity key:  envlock keygen")
			lines = append(lines, "  2. Encrypt your secrets:        envlock push")
		}

		banner := ui.Banner("EnvLock Initialized", lines)
		fmt.Println(banner)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
