package cmd

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/itskdhere/envlock/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	flagEnv     string
	flagFile    string
	flagKeyFile string
	flagVersion bool
)

var rootCmd = &cobra.Command{
	Use:   "envlock",
	Short: "Zero-cloud, asymmetric envelope encryption for environment secrets",
	Long: `EnvLock (el) keeps your .env files local and uncommitted while securely storing 
tamper-evident, encrypted snapshots (env.lock) directly in your Git repository.
Protected by Curve25519 (X25519 ECDH) asymmetric envelope encryption and AES-256-GCM.`,
	// Execute prints errors itself; set here because PersistentPreRun is skipped on parse errors.
	SilenceErrors: true,
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

func Execute() int {
	startUpdateCheck()
	err := rootCmd.Execute()
	printUpdateNotice()
	if err != nil {
		printActionableError(err)
	}
	var exitErr *exitError
	if errors.As(err, &exitErr) {
		return exitErr.code
	}
	if err != nil {
		return 1
	}
	return 0
}

func printActionableError(err error) {
	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "unauthorized: your public key"):
		pubKey := ""
		if start := strings.Index(errStr, "(el_pub_"); start != -1 {
			end := strings.Index(errStr[start:], ")")
			if end != -1 {
				pubKey = errStr[start+1 : start+end]
			}
		}
		msg := "Your public key is not authorized in this vault."
		if pubKey != "" {
			msg += fmt.Sprintf("\nKey: %s", ui.Highlight(pubKey))
		}
		hint := fmt.Sprintf("Ask an authorized teammate to run:\n  %s", ui.Highlight(fmt.Sprintf("el recipient add %s", pubKey)))
		fmt.Fprintln(os.Stderr, ui.ErrorCard("Authorization Failed", msg, hint))

	case strings.Contains(errStr, "no private key found"):
		msg := "EnvLock could not locate a private identity key.\n" +
			ui.Subtle("Searched: --key-file, ENVLOCK_KEY, ENVLOCK_KEY_PATH, ./envlock.key, user config directory")
		hint := fmt.Sprintf("• Generate a new key:       %s\n• Or specify existing key:  %s",
			ui.Highlight("el keygen"), ui.Highlight("el pull --key-file <path>"))
		fmt.Fprintln(os.Stderr, ui.ErrorCard("No Private Key Found", msg, hint))

	case strings.Contains(errStr, "open .env") && (strings.Contains(errStr, "cannot find the file") || strings.Contains(errStr, "no such file")):
		msg := "Local environment file (.env) was not found."
		hint := fmt.Sprintf("• Create template defaults:  %s\n• Or decrypt from vault:     %s",
			ui.Highlight("el init"), ui.Highlight("el pull"))
		fmt.Fprintln(os.Stderr, ui.ErrorCard("File Not Found", msg, hint))

	default:
		fmt.Fprintln(os.Stderr, ui.Error("%s", errStr))
	}
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

func commandTitlePath(cmd *cobra.Command) string {
	if cmd == rootCmd || cmd.Parent() == nil {
		return ""
	}
	var parts []string
	curr := cmd
	for curr != nil && curr != rootCmd {
		name := curr.Name()
		if len(name) > 0 {
			name = strings.ToUpper(name[:1]) + name[1:]
		}
		parts = append([]string{name}, parts...)
		curr = curr.Parent()
	}
	return strings.Join(parts, " ")
}

func formatCommandListing(c *cobra.Command, colWidth int) string {
	name := c.Name()
	if len(c.Aliases) > 0 {
		short := c.Aliases[0]
		for _, a := range c.Aliases {
			if len(a) < len(short) {
				short = a
			}
		}
		name = fmt.Sprintf("%s, %s", c.Name(), short)
	}
	padLen := colWidth - len(name)
	if padLen < 2 {
		padLen = 2
	}
	return fmt.Sprintf("  %s%s%s", ui.Highlight(name), strings.Repeat(" ", padLen), ui.Subtle(c.Short))
}

func maxCommandNameWidth(cmds []*cobra.Command) int {
	max := 16
	for _, c := range cmds {
		name := c.Name()
		if len(c.Aliases) > 0 {
			short := c.Aliases[0]
			for _, a := range c.Aliases {
				if len(a) < len(short) {
					short = a
				}
			}
			name = fmt.Sprintf("%s, %s", c.Name(), short)
		}
		if len(name) > max {
			max = len(name)
		}
	}
	return max + 2
}

func renderStyledHelp(cmd *cobra.Command) {
	title := "EnvLock (el)"
	if titlePath := commandTitlePath(cmd); titlePath != "" {
		title += " " + titlePath
	}

	if ui.IsColorDisabled() {
		fmt.Println(title + " – " + cmd.Short)
		fmt.Println()
		_ = cmd.Usage()
		return
	}

	fmt.Println(ui.Primary(title) + " – " + cmd.Short)
	fmt.Println()

	fmt.Println(ui.Bold("USAGE:"))
	if cmd.Runnable() {
		useLine := cmd.UseLine()
		if !strings.Contains(useLine, "[flags]") && (cmd.Flags().HasAvailableFlags() || cmd.PersistentFlags().HasAvailableFlags()) {
			useLine += " [flags]"
		}
		fmt.Printf("  %s\n", ui.Highlight(useLine))
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Printf("  %s\n", ui.Highlight(cmd.CommandPath()+" [command] [flags]"))
	}
	fmt.Println()

	if len(cmd.Aliases) > 0 {
		fmt.Println(ui.Bold("ALIASES:"))
		fmt.Printf("  %s\n\n", ui.Highlight(cmd.Name()+", "+strings.Join(cmd.Aliases, ", ")))
	}

	colWidth := maxCommandNameWidth(cmd.Commands())

	if len(cmd.Groups()) > 0 {
		for _, group := range cmd.Groups() {
			var groupCmds []*cobra.Command
			for _, c := range cmd.Commands() {
				if c.GroupID == group.ID && c.IsAvailableCommand() {
					groupCmds = append(groupCmds, c)
				}
			}
			if len(groupCmds) > 0 {
				fmt.Println(ui.Bold(group.Title))
				for _, c := range groupCmds {
					fmt.Println(formatCommandListing(c, colWidth))
				}
				fmt.Println()
			}
		}

		var otherCmds []*cobra.Command
		for _, c := range cmd.Commands() {
			if c.GroupID == "" && c.IsAvailableCommand() && c.Name() != "help" {
				otherCmds = append(otherCmds, c)
			}
		}
		if len(otherCmds) > 0 {
			fmt.Println(ui.Bold("Additional Commands:"))
			for _, c := range otherCmds {
				fmt.Println(formatCommandListing(c, colWidth))
			}
			fmt.Println()
		}
	} else if cmd.HasAvailableSubCommands() {
		fmt.Println(ui.Bold("AVAILABLE COMMANDS:"))
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() && c.Name() != "help" {
				fmt.Println(formatCommandListing(c, colWidth))
			}
		}
		fmt.Println()
	}

	flags := cmd.LocalFlags()
	if flags.HasAvailableFlags() {
		fmt.Println(ui.Bold("FLAGS:"))
		fmt.Print(flags.FlagUsages())
		fmt.Println()
	}

	pflags := cmd.PersistentFlags()
	if pflags.HasAvailableFlags() {
		fmt.Println(ui.Bold("GLOBAL FLAGS:"))
		fmt.Print(pflags.FlagUsages())
		fmt.Println()
	}

	if cmd == rootCmd {
		fmt.Println(ui.Bold("EXAMPLES:"))
		fmt.Println("  " + ui.Subtle("# Initialize safety defaults in a new project"))
		fmt.Println("  " + ui.Highlight("el init"))
		fmt.Println("  " + ui.Subtle("# Generate your personal keypair"))
		fmt.Println("  " + ui.Highlight("el keygen"))
		fmt.Println("  " + ui.Subtle("# Encrypt .env into env.lock"))
		fmt.Println("  " + ui.Highlight("el push"))
		fmt.Println("  " + ui.Subtle("# Decrypt env.lock back into .env"))
		fmt.Println("  " + ui.Highlight("el pull"))
		fmt.Println("  " + ui.Subtle("# Run a command with injected secrets (zero disk)"))
		fmt.Println("  " + ui.Highlight("el run -- npm start"))
		fmt.Println("  " + ui.Subtle("# Multi-environment support"))
		fmt.Println("  " + ui.Highlight("el push -e staging") + "  /  " + ui.Highlight("el pull -e prod"))
		fmt.Println()
	}

	fmt.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&flagEnv, "env", "e", "", "Target environment name (e.g. 'prod' maps to .env.prod and env.prod.lock)")
	rootCmd.PersistentFlags().StringVarP(&flagFile, "file", "f", "", "Explicit path to .env file")
	rootCmd.PersistentFlags().StringVar(&flagKeyFile, "key-file", "", "Explicit path to private key file (or '-' for stdin)")
	rootCmd.Flags().BoolVarP(&flagVersion, "version", "v", false, "Print version and build information")

	_ = rootCmd.RegisterFlagCompletionFunc("env", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		entries, err := os.ReadDir(".")
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		envMap := make(map[string]bool)
		for _, e := range entries {
			name := e.Name()
			if strings.HasPrefix(name, ".env.") && !strings.HasSuffix(name, ".example") {
				envName := strings.TrimPrefix(name, ".env.")
				if envName != "" {
					envMap[envName] = true
				}
			}
			if strings.HasPrefix(name, "env.") && strings.HasSuffix(name, ".lock") {
				envName := strings.TrimSuffix(strings.TrimPrefix(name, "env."), ".lock")
				if envName != "" {
					envMap[envName] = true
				}
			}
		}
		var envs []string
		for envName := range envMap {
			if strings.HasPrefix(envName, toComplete) {
				envs = append(envs, envName)
			}
		}
		slices.Sort(envs)
		return envs, cobra.ShellCompDirectiveNoFileComp
	})

	workflowGroup := &cobra.Group{
		ID:    "workflow",
		Title: "Core Workflow:",
	}
	teamGroup := &cobra.Group{
		ID:    "team",
		Title: "Team & Key Management:",
	}
	rootCmd.AddGroup(workflowGroup, teamGroup)

	pushCmd.GroupID = "workflow"
	pullCmd.GroupID = "workflow"
	runCmd.GroupID = "workflow"

	initCmd.GroupID = "team"
	keygenCmd.GroupID = "team"
	recipientCmd.GroupID = "team"

	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		renderStyledHelp(cmd)
	})
}
