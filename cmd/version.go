package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time via ldflags.
var Version = "dev"

// versionCmd is the version subcommand.
var versionCmd = &cobra.Command{
	Use:     "version",
	Aliases: []string{"v"},
	Short:   "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("cws %s\n", Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("cws {{.Version}}\n")
}
