package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/null3000/cws-cli/internal/api"
	"github.com/null3000/cws-cli/internal/auth"
	"github.com/null3000/cws-cli/internal/config"
	"github.com/null3000/cws-cli/internal/output"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the current status of an extension",
	RunE:  runStatus,
}

func init() {
	statusCmd.Flags().Bool("json", false, "Output raw JSON response")
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := config.ValidateAuth(cfg); err != nil {
		return err
	}

	extensionIDFlag, _ := cmd.Flags().GetString("extension-id")
	extensionID, err := config.ResolveExtensionID(extensionIDFlag, cfg)
	if err != nil {
		return err
	}

	jsonOutput, _ := cmd.Flags().GetBool("json")

	authenticator := auth.NewOAuthAuthenticator(cfg.Auth.ClientID, cfg.Auth.ClientSecret, cfg.Auth.RefreshToken)
	client := api.NewClient(authenticator, cfg.PublisherID)
	ctx := context.Background()

	resp, rawJSON, err := client.FetchStatus(ctx, extensionID)
	if err != nil {
		return err
	}

	if jsonOutput {
		fmt.Println(string(rawJSON))
		return nil
	}

	// Formatted output
	output.Info("Extension: %s", extensionID)
	if resp.PublishedItemRevisionStatus != nil {
		output.Info("")
		output.Info("Published:")
		output.Info("  State:   %s", FormatState(resp.PublishedItemRevisionStatus.State))
		if resp.PublishedItemRevisionStatus.CrxVersion != "" {
			output.Info("  Version: %s", resp.PublishedItemRevisionStatus.CrxVersion)
		}
		for _, ch := range resp.PublishedItemRevisionStatus.DistributionChannels {
			if ch.CrxVersion != "" {
				output.Info("  Version: %s", ch.CrxVersion)
			}
			output.Info("  Deploy:  %d%%", ch.DeployPercentage)
		}
	}
	if resp.SubmittedItemRevisionStatus != nil {
		output.Info("")
		output.Info("Submitted:")
		output.Info("  State:   %s", FormatState(resp.SubmittedItemRevisionStatus.State))
		for _, ch := range resp.SubmittedItemRevisionStatus.DistributionChannels {
			if ch.CrxVersion != "" {
				output.Info("  Version: %s", ch.CrxVersion)
			}
			output.Info("  Deploy:  %d%%", ch.DeployPercentage)
		}
	}
	if resp.LastAsyncUploadState != "" {
		output.Info("")
		output.Info("Upload:    %s", resp.LastAsyncUploadState)
	}
	if len(resp.ItemError) > 0 {
		output.Info("Errors:")
		for _, e := range resp.ItemError {
			output.Info("  - [%s] %s", e.ErrorCode, e.ErrorDetail)
			if hint := api.HintForItemError(e.ErrorCode); hint != "" {
				output.Hint("%s", hint)
			}
		}
	}

	return nil
}

// FormatState converts an API state string to a human-readable label.
func FormatState(state string) string {
	switch state {
	case "PUBLISHED":
		return "Published"
	case "PENDING_REVIEW":
		return "Pending Review"
	case "DRAFT":
		return "Draft"
	case "DEFERRED":
		return "Staged (Deferred)"
	case "STATE_UNSPECIFIED":
		return "Unknown"
	default:
		return state
	}
}
