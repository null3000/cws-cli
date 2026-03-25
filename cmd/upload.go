package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/null3000/cws-cli/internal/api"
	"github.com/null3000/cws-cli/internal/auth"
	"github.com/null3000/cws-cli/internal/config"
	"github.com/null3000/cws-cli/internal/output"
	cwszip "github.com/null3000/cws-cli/internal/zip"
)

var uploadCmd = &cobra.Command{
	Use:   "upload [source]",
	Short: "Upload a package to the Chrome Web Store",
	Long: `Zip (if needed) and upload a package to the Chrome Web Store.

Source can be a .zip file, .crx file, or a directory. If a directory
is given, it will be zipped automatically. Defaults to the current
directory if not specified.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpload,
}

func init() {
	uploadCmd.Flags().Bool("wait", true, "Wait for upload processing to complete")
	uploadCmd.Flags().Int("timeout", 300, "Max seconds to wait for upload processing")
	uploadCmd.Flags().Bool("publish", false, "Automatically publish after successful upload")
	uploadCmd.Flags().Bool("skip-validate", false, "Skip pre-upload validation checks")
	rootCmd.AddCommand(uploadCmd)
}

func runUpload(cmd *cobra.Command, args []string) error {
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

	wait, _ := cmd.Flags().GetBool("wait")
	timeout, _ := cmd.Flags().GetInt("timeout")
	publish, _ := cmd.Flags().GetBool("publish")
	skipValidate, _ := cmd.Flags().GetBool("skip-validate")

	// Resolve source
	var source string
	if len(args) > 0 {
		source = args[0]
	} else {
		source = config.ResolveSource("", cfg)
	}

	// Pre-upload validation
	if !skipValidate {
		output.Info("Validating...")
		results := runValidationChecks(cmd, cfg, source, false)
		failures := 0
		for _, r := range results {
			if !r.Passed {
				output.Info("  x %s", r.Message)
				failures++
			}
		}
		if failures > 0 {
			output.Hint("To upload without validation, use --skip-validate")
			return fmt.Errorf("cws validate failed: %d issue(s) found", failures)
		}
		output.Info("Validation passed!")
		fmt.Println()
	}

	// Prepare zip data
	zipData, err := prepareZip(source)
	if err != nil {
		return err
	}

	// Create API client
	authenticator := auth.NewOAuthAuthenticator(cfg.Auth.ClientID, cfg.Auth.ClientSecret, cfg.Auth.RefreshToken)
	client := api.NewClient(authenticator, cfg.PublisherID)
	ctx := context.Background()

	// Upload
	output.Info("Uploading to extension %s...", extensionID)
	resp, err := client.Upload(ctx, extensionID, zipData)
	if err != nil {
		return err
	}

	output.Info("Upload state: %s", resp.UploadState)

	// Wait for processing
	if wait && resp.UploadState == "UPLOAD_IN_PROGRESS" {
		resp, err = waitForUpload(ctx, client, extensionID, timeout)
		if err != nil {
			return err
		}
	}

	if resp.UploadState == "FAILURE" {
		return api.NewCWSError("upload", 0, resp.ItemError, "upload processing failed")
	}

	if resp.UploadState == "UPLOAD_IN_PROGRESS" {
		output.Info("Upload is still processing. Use 'cws status' to check progress.")
		return nil
	}

	output.Info("Upload successful!")

	// Auto-publish
	if publish && resp.UploadState == "SUCCESS" {
		output.Info("Publishing...")
		pubResp, err := client.Publish(ctx, extensionID, false)
		if err != nil {
			return fmt.Errorf("upload succeeded but publish failed: %w", err)
		}
		if len(pubResp.Status) > 0 {
			output.Info("Publish status: %s", strings.Join(pubResp.Status, ", "))
		}
	}

	return nil
}

func prepareZip(source string) ([]byte, error) {
	absSource, err := filepath.Abs(source)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve source path: %w", err)
	}

	info, err := os.Stat(absSource)
	if err != nil {
		return nil, fmt.Errorf("source not found: %s", source)
	}

	if !info.IsDir() {
		// It's a file — read it directly (.zip or .crx)
		ext := strings.ToLower(filepath.Ext(absSource))
		if ext != ".zip" && ext != ".crx" {
			return nil, fmt.Errorf("source file must be a .zip or .crx file, got: %s", ext)
		}

		data, err := os.ReadFile(absSource)
		if err != nil {
			return nil, fmt.Errorf("failed to read source file: %w", err)
		}

		if ext == ".zip" {
			hasManifest, err := cwszip.ContainsManifestInZip(data)
			if err != nil {
				return nil, err
			}
			if !hasManifest {
				return nil, fmt.Errorf("zip file does not contain a manifest.json")
			}
		}

		return data, nil
	}

	// Directory — validate and zip
	if !cwszip.ContainsManifest(absSource) {
		return nil, fmt.Errorf("directory does not contain a manifest.json: %s", source)
	}

	output.Info("Zipping directory %s...", source)
	data, err := cwszip.ZipDirectory(absSource)
	if err != nil {
		return nil, err
	}
	output.Info("Created zip (%d bytes)", len(data))

	return data, nil
}

func waitForUpload(ctx context.Context, client *api.Client, extensionID string, timeoutSec int) (*api.UploadResponse, error) {
	deadline := time.Now().Add(time.Duration(timeoutSec) * time.Second)
	pollInterval := 5 * time.Second

	output.Progress("Waiting for upload to process")

	for time.Now().Before(deadline) {
		time.Sleep(pollInterval)
		output.Progress(".")

		status, _, err := client.FetchStatus(ctx, extensionID)
		if err != nil {
			return nil, err
		}

		if status.LastAsyncUploadState != "IN_PROGRESS" {
			output.Progress("\n")
			uploadState := status.LastAsyncUploadState
			// Map V2 fetchStatus values to upload response values
			switch uploadState {
			case "SUCCEEDED":
				uploadState = "SUCCESS"
			case "FAILED":
				uploadState = "FAILURE"
			}
			return &api.UploadResponse{
				ID:          status.ItemID,
				UploadState: uploadState,
				ItemError:   status.ItemError,
			}, nil
		}
	}

	output.Progress("\n")
	return nil, fmt.Errorf("timed out waiting for upload processing after %d seconds", timeoutSec)
}
