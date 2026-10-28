package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// Version info set by compiler -ldflags
	Version = "0.5.0"
	Commit  = "dev"
)

// VersionString returns formatted version details.
func VersionString() string {
	return fmt.Sprintf("envlock version %s (%s) %s/%s", Version, Commit, runtime.GOOS, runtime.GOARCH)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number and build details",
	Long:  "Display semantic version, Git commit hash, and target OS/architecture.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(VersionString())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
