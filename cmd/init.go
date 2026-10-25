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
1. Creating a sample .env file if none exists.
2. Appending .env, .env.*.local, and *.key to .gitignore to prevent secret leaks.
3. Appending *.lock merge=binary to .gitattributes to prevent Git merge conflict corruption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		paths := env.ResolvePaths(cwd, flagEnv, flagFile)

		// 1. Create default .env if missing
		createdEnv, err := env.CreateDefaultEnv(paths.EnvFile)
		if err != nil {
			return fmt.Errorf("failed to create default env file: %w", err)
		}

		// 2. Ensure .gitignore rules
		addedGitignore, err := env.EnsureGitignore(cwd)
		if err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}

		// 3. Ensure .gitattributes rules
		addedGitattributes, err := env.EnsureGitattributes(cwd)
		if err != nil {
			return fmt.Errorf("failed to update .gitattributes: %w", err)
		}

		// Render styled summary
		var lines []string
		if createdEnv {
			lines = append(lines, fmt.Sprintf("+ Created template env file: %s", paths.EnvFile))
		} else {
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
		lines = append(lines, "  1. Generate your identity key:  envlock keygen")
		lines = append(lines, "  2. Encrypt your secrets:        envlock push")

		banner := ui.Banner("EnvLock Initialized", lines)
		fmt.Println(banner)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
