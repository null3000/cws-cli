package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
	"github.com/null3000/cws-cli/internal/api"
	"github.com/null3000/cws-cli/internal/output"
)

var rootCmd = &cobra.Command{
	Use:   "cws",
	Short: "Chrome Web Store CLI",
	Long:  "A command-line tool for managing Chrome Web Store extensions using the V2 API.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Error("%s", err)
		var cwsErr *api.CWSError
		if errors.As(err, &cwsErr) && cwsErr.Hint != "" && len(cwsErr.Details) <= 1 {
			output.Hint("%s", cwsErr.Hint)
		}
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringP("extension-id", "e", "", "Extension ID (overrides config)")
}
