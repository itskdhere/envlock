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
2. Appending .env, .env.*, *.key, and *.pub to .gitignore, while exempting .env.example and lock files (!env.lock, !env.*.lock).
3. Appending env.lock and env.*.lock merge=binary rules to .gitattributes to prevent Git merge conflict corruption.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := env.ResolvePaths(".", flagEnv, flagFile)

		_, err := os.Stat(paths.LockFile)
		hasVault := err == nil

		createdEnv := false
		if !hasVault {
			if createdEnv, err = env.CreateDefaultEnv(paths.EnvFile); err != nil {
				return fmt.Errorf("failed to create default env file: %w", err)
			}
		}

		addedGitignore, err := env.EnsureGitignore(".")
		if err != nil {
			return fmt.Errorf("failed to update .gitignore: %w", err)
		}

		addedGitattributes, err := env.EnsureGitattributes(".")
		if err != nil {
			return fmt.Errorf("failed to update .gitattributes: %w", err)
		}

		var lines []string
		switch {
		case hasVault:
			lines = append(lines, fmt.Sprintf("%s Existing vault found, no template env file created: %s", ui.InfoIcon.String(), paths.LockFile))
		case createdEnv:
			lines = append(lines, fmt.Sprintf("%s Created template env file: %s", ui.SuccessIcon.String(), paths.EnvFile))
		default:
			lines = append(lines, fmt.Sprintf("%s Existing env file preserved: %s", ui.InfoIcon.String(), paths.EnvFile))
		}

		if len(addedGitignore) > 0 {
			lines = append(lines, fmt.Sprintf("%s Updated .gitignore with %d safety rules", ui.SuccessIcon.String(), len(addedGitignore)))
		} else {
			lines = append(lines, fmt.Sprintf("%s .gitignore safety rules already present", ui.InfoIcon.String()))
		}

		if len(addedGitattributes) > 0 {
			lines = append(lines, fmt.Sprintf("%s Updated .gitattributes with binary merge rules", ui.SuccessIcon.String()))
		} else {
			lines = append(lines, fmt.Sprintf("%s .gitattributes binary merge rules already present", ui.InfoIcon.String()))
		}

		lines = append(lines, "")
		lines = append(lines, ui.Bold("Next steps:"))
		if hasVault {
			lines = append(lines, fmt.Sprintf("  1. Decrypt your secrets:  %s", ui.Highlight("envlock pull")))
			lines = append(lines, fmt.Sprintf("     No access yet? Run '%s' and ask a teammate to run '%s'", ui.Highlight("envlock keygen"), ui.Highlight("envlock recipient add <key>")))
		} else {
			lines = append(lines, fmt.Sprintf("  1. Generate your identity key:  %s", ui.Highlight("envlock keygen")))
			lines = append(lines, fmt.Sprintf("  2. Encrypt your secrets:        %s", ui.Highlight("envlock push")))
		}

		banner := ui.Banner("EnvLock Initialized", lines)
		fmt.Println(banner)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
