package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/null3000/cws-cli/internal/auth"
	"github.com/null3000/cws-cli/internal/config"
	"github.com/null3000/cws-cli/internal/output"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive credential setup wizard",
	Long: `Set up Chrome Web Store API credentials interactively.

This wizard will guide you through configuring:
  - OAuth2 Client ID and Secret (from Google Cloud Console)
  - Refresh Token (from OAuth Playground)
  - Publisher ID (from Chrome Web Store Developer Dashboard)`,
	RunE: runInit,
}

func init() {
	initCmd.Flags().Bool("global", false, "Write config to ~/.config/cws/cws.toml instead of current directory")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	reader := bufio.NewReader(os.Stdin)
	global, _ := cmd.Flags().GetBool("global")

	fmt.Println("Chrome Web Store CLI — Credential Setup")
	fmt.Println("========================================")
	fmt.Println()

	// Client ID
	fmt.Println("Step 1: Client ID")
	fmt.Println("  Create OAuth2 credentials in the Google Cloud Console:")
	fmt.Println("  https://console.cloud.google.com/apis/credentials")
	fmt.Println("  Make sure the Chrome Web Store API is enabled for your project.")
	fmt.Println()
	clientID, err := prompt(reader, "Client ID: ")
	if err != nil {
		return err
	}

	// Client Secret
	fmt.Println()
	fmt.Println("Step 2: Client Secret")
	clientSecret, err := prompt(reader, "Client Secret: ")
	if err != nil {
		return err
	}

	// Refresh Token
	fmt.Println()
	fmt.Println("Step 3: Refresh Token")
	fmt.Println("  Obtain a refresh token using the OAuth Playground:")
	fmt.Println("  https://developers.google.com/oauthplayground")
	fmt.Println("  Use scope: https://www.googleapis.com/auth/chromewebstore")
	fmt.Println()
	refreshToken, err := prompt(reader, "Refresh Token: ")
	if err != nil {
		return err
	}

	// Publisher ID
	fmt.Println()
	fmt.Println("Step 4: Publisher ID")
	fmt.Println("  Find your Publisher ID in the Chrome Web Store Developer Dashboard:")
	fmt.Println("  Developer Dashboard → Account section")
	fmt.Println()
	publisherID, err := prompt(reader, "Publisher ID: ")
	if err != nil {
		return err
	}

	// Optional Extension ID
	fmt.Println()
	fmt.Println("Step 5: Default Extension ID (optional, press Enter to skip)")
	extensionID, err := prompt(reader, "Extension ID: ")
	if err != nil {
		return err
	}

	// Validate credentials
	fmt.Println()
	output.Progress("Validating credentials...")
	if err := auth.ValidateCredentials(clientID, clientSecret, refreshToken); err != nil {
		fmt.Println()
		return fmt.Errorf("credential validation failed: %w", err)
	}
	fmt.Println(" valid!")

	// Write config
	cfg := &config.Config{
		PublisherID: publisherID,
		Auth: config.AuthConfig{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RefreshToken: refreshToken,
		},
	}
	if extensionID != "" {
		cfg.Extensions.Default.ID = extensionID
	}

	var configPath string
	if global {
		var err error
		configPath, err = config.GlobalConfigPath()
		if err != nil {
			return err
		}
	} else {
		configPath = "cws.toml"
	}

	if err := config.WriteConfig(configPath, cfg); err != nil {
		return err
	}

	fmt.Println()
	output.Info("Configuration saved to %s", configPath)
	if !global {
		output.Info("Tip: Add cws.toml to .gitignore — it contains secrets.")
	}

	return nil
}

func prompt(reader *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(input), nil
}
