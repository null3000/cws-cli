package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/null3000/cws-cli/internal/api"
	"github.com/null3000/cws-cli/internal/auth"
	"github.com/null3000/cws-cli/internal/config"
	"github.com/null3000/cws-cli/internal/manifest"
	"github.com/null3000/cws-cli/internal/output"
	cwszip "github.com/null3000/cws-cli/internal/zip"
)

const maxPackageSize = 512 * 1024 * 1024 // 512 MB

var validateCmd = &cobra.Command{
	Use:   "validate [source]",
	Short: "Run pre-flight checks before uploading",
	Long: `Validate an extension package before uploading to the Chrome Web Store.

Checks manifest.json structure, version format, package size, and optionally
verifies the version is higher than the currently published version.

Use --local to skip remote checks (no credentials needed).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().Bool("local", false, "Only run local checks (skip API calls)")
	rootCmd.AddCommand(validateCmd)
}

// ValidationResult tracks the outcome of a single check.
type ValidationResult struct {
	Passed  bool
	Message string
}

func runValidate(cmd *cobra.Command, args []string) error {
	localOnly, _ := cmd.Flags().GetBool("local")

	cfg, err := config.Load()
	if err != nil && !localOnly {
		return err
	}

	// Resolve source
	var source string
	if len(args) > 0 {
		source = args[0]
	} else {
		source = config.ResolveSource("", cfg)
	}

	results := runValidationChecks(cmd, cfg, source, localOnly)
	return printResults(results)
}

func runValidationChecks(cmd *cobra.Command, cfg *config.Config, source string, localOnly bool) []ValidationResult {
	var results []ValidationResult

	// Check 1: Source exists
	absSource, err := filepath.Abs(source)
	if err != nil {
		results = append(results, fail("Source path: %s", err))
		return results
	}

	info, err := os.Stat(absSource)
	if err != nil {
		results = append(results, fail("Source not found: %s", source))
		return results
	}

	// Check 2: Parse manifest.json
	var m *manifest.Manifest
	var zipData []byte

	if info.IsDir() {
		manifestPath := filepath.Join(absSource, "manifest.json")
		if _, err := os.Stat(manifestPath); err != nil {
			results = append(results, fail("manifest.json not found in %s", source))
			return results
		}
		results = append(results, pass("manifest.json found"))

		m, err = manifest.Parse(manifestPath)
		if err != nil {
			results = append(results, fail("manifest.json: %s", err))
			return results
		}
		results = append(results, pass("manifest.json is valid JSON"))

		// Zip for size check
		zipData, err = cwszip.ZipDirectory(absSource)
		if err != nil {
			results = append(results, fail("Failed to zip directory: %s", err))
			return results
		}
	} else {
		ext := strings.ToLower(filepath.Ext(absSource))
		if ext != ".zip" && ext != ".crx" {
			results = append(results, fail("Source must be a directory, .zip, or .crx file"))
			return results
		}

		zipData, err = os.ReadFile(absSource)
		if err != nil {
			results = append(results, fail("Failed to read %s: %s", source, err))
			return results
		}

		if ext == ".zip" {
			m, err = manifest.ParseFromZip(zipData)
			if err != nil {
				results = append(results, fail("manifest.json: %s", err))
				return results
			}
			results = append(results, pass("manifest.json found"))
			results = append(results, pass("manifest.json is valid JSON"))
		} else {
			// .crx files can't be easily parsed for manifest
			results = append(results, pass("Source file exists (%s)", ext))
		}
	}

	// Check 3: Required fields
	if m != nil {
		missing := manifest.ValidateRequired(m)
		if len(missing) > 0 {
			results = append(results, fail("Missing required fields: %s", strings.Join(missing, ", ")))
		} else {
			results = append(results, pass("Required fields present (name, version, manifest_version)"))
		}

		// Check 4: Version format
		if m.Version != "" {
			if err := manifest.ValidateVersion(m.Version); err != nil {
				results = append(results, fail("Invalid version %q: %s", m.Version, err))
			} else {
				results = append(results, pass("Version format valid: %s", m.Version))
			}
		}
	}

	// Check 5: Package size
	if zipData != nil {
		sizeMB := float64(len(zipData)) / (1024 * 1024)
		if len(zipData) > maxPackageSize {
			results = append(results, fail("Package too large: %.1f MB (max 512 MB)", sizeMB))
		} else if sizeMB >= 1.0 {
			results = append(results, pass("Package size OK (%.1f MB)", sizeMB))
		} else {
			sizeKB := float64(len(zipData)) / 1024
			results = append(results, pass("Package size OK (%.0f KB)", sizeKB))
		}
	}

	// Remote checks
	if localOnly || m == nil {
		return results
	}

	if cfg == nil {
		results = append(results, fail("No configuration found (skipping remote checks)"))
		return results
	}
	if err := config.ValidateAuth(cfg); err != nil {
		results = append(results, fail("Auth not configured: %s (use --local to skip remote checks)", err))
		return results
	}

	extensionIDFlag, _ := cmd.Flags().GetString("extension-id")
	extensionID, err := config.ResolveExtensionID(extensionIDFlag, cfg)
	if err != nil {
		results = append(results, fail("Extension ID: %s", err))
		return results
	}

	authenticator := auth.NewOAuthAuthenticator(cfg.Auth.ClientID, cfg.Auth.ClientSecret, cfg.Auth.RefreshToken)
	client := api.NewClient(authenticator, cfg.PublisherID)
	ctx := context.Background()

	resp, _, err := client.FetchStatus(ctx, extensionID)
	if err != nil {
		results = append(results, fail("Failed to fetch status: %s", err))
		return results
	}

	// Check 6: Version higher than published
	if resp.PublishedItemRevisionStatus != nil && m.Version != "" {
		publishedVersion := resp.PublishedItemRevisionStatus.CrxVersion
		if publishedVersion == "" && len(resp.PublishedItemRevisionStatus.DistributionChannels) > 0 {
			publishedVersion = resp.PublishedItemRevisionStatus.DistributionChannels[0].CrxVersion
		}
		if publishedVersion != "" {
			higher, err := manifest.CompareVersions(m.Version, publishedVersion)
			if err != nil {
				results = append(results, fail("Version comparison: %s", err))
			} else if !higher {
				results = append(results, fail("Version %s is not higher than published %s", m.Version, publishedVersion))
			} else {
				results = append(results, pass("Version %s > published %s", m.Version, publishedVersion))
			}
		} else {
			results = append(results, pass("No published version found (first upload)"))
		}
	} else if resp.PublishedItemRevisionStatus == nil {
		results = append(results, pass("No published version found (first upload)"))
	}

	// Check 7: No pending submission
	if resp.SubmittedItemRevisionStatus != nil {
		state := FormatState(resp.SubmittedItemRevisionStatus.State)
		results = append(results, fail("Pending submission exists (%s). Use 'cws cancel' first, or wait for review", state))
	} else {
		results = append(results, pass("No pending submission"))
	}

	return results
}

func pass(format string, args ...any) ValidationResult {
	return ValidationResult{Passed: true, Message: fmt.Sprintf(format, args...)}
}

func fail(format string, args ...any) ValidationResult {
	return ValidationResult{Passed: false, Message: fmt.Sprintf(format, args...)}
}

func printResults(results []ValidationResult) error {
	failures := 0
	for _, r := range results {
		if r.Passed {
			output.Info("  + %s", r.Message)
		} else {
			output.Info("  x %s", r.Message)
			failures++
		}
	}

	fmt.Println()
	if failures > 0 {
		return fmt.Errorf("validation failed: %d issue(s) found", failures)
	}
	output.Info("Validation passed!")
	return nil
}
