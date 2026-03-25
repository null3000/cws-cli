package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the CLI.
type Config struct {
	PublisherID string           `mapstructure:"publisher_id" toml:"publisher_id"`
	Auth        AuthConfig       `mapstructure:"auth" toml:"auth"`
	Extensions  ExtensionsConfig `mapstructure:"extensions" toml:"extensions,omitempty"`
}

// AuthConfig holds OAuth2 credentials.
type AuthConfig struct {
	ClientID     string `mapstructure:"client_id" toml:"client_id"`
	ClientSecret string `mapstructure:"client_secret" toml:"client_secret"`
	RefreshToken string `mapstructure:"refresh_token" toml:"refresh_token"`
}

// ExtensionsConfig holds extension configurations (designed for multi-extension M2 support).
type ExtensionsConfig struct {
	Default ExtensionConfig `mapstructure:"default" toml:"default,omitempty"`
}

// ExtensionConfig holds configuration for a single extension.
type ExtensionConfig struct {
	ID     string `mapstructure:"id" toml:"id,omitempty"`
	Source string `mapstructure:"source" toml:"source,omitempty"`
}

// GlobalConfigDir returns the path to the global config directory.
func GlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "cws")
}

// GlobalConfigPath returns the path to the global config file.
// Returns an error if the home directory cannot be determined.
func GlobalConfigPath() (string, error) {
	dir := GlobalConfigDir()
	if dir == "" {
		return "", fmt.Errorf("could not determine home directory for global config")
	}
	return filepath.Join(dir, "cws.toml"), nil
}

// Load reads configuration from config files and environment variables.
// Priority: env vars > local cws.toml > global cws.toml
func Load() (*Config, error) {
	v := viper.New()
	v.SetConfigName("cws")
	v.SetConfigType("toml")

	// Environment variable bindings
	v.SetEnvPrefix("CWS")
	v.AutomaticEnv()
	_ = v.BindEnv("auth.client_id", "CWS_CLIENT_ID")
	_ = v.BindEnv("auth.client_secret", "CWS_CLIENT_SECRET")
	_ = v.BindEnv("auth.refresh_token", "CWS_REFRESH_TOKEN")
	_ = v.BindEnv("publisher_id", "CWS_PUBLISHER_ID")
	_ = v.BindEnv("extensions.default.id", "CWS_EXTENSION_ID")

	// Read global config first (lowest priority file)
	globalDir := GlobalConfigDir()
	if globalDir != "" {
		v.AddConfigPath(globalDir)
	}

	// Try to read global config
	_ = v.ReadInConfig()

	// Read local config (overrides global)
	localV := viper.New()
	localV.SetConfigName("cws")
	localV.SetConfigType("toml")
	localV.AddConfigPath(".")

	if err := localV.ReadInConfig(); err == nil {
		for _, key := range localV.AllKeys() {
			v.Set(key, localV.Get(key))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return &cfg, nil
}

// ResolveExtensionID returns the extension ID from the flag override or config.
func ResolveExtensionID(flagValue string, cfg *Config) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if id := os.Getenv("CWS_EXTENSION_ID"); id != "" {
		return id, nil
	}
	if cfg != nil && cfg.Extensions.Default.ID != "" {
		return cfg.Extensions.Default.ID, nil
	}
	return "", fmt.Errorf("no extension ID specified. Use --extension-id flag, set CWS_EXTENSION_ID, or add [extensions.default] to cws.toml")
}

// ResolveSource returns the source path from the flag override or config.
func ResolveSource(argValue string, cfg *Config) string {
	if argValue != "" {
		return argValue
	}
	if cfg != nil && cfg.Extensions.Default.Source != "" {
		return cfg.Extensions.Default.Source
	}
	return "."
}

// ValidateAuth checks that all required auth fields are present.
func ValidateAuth(cfg *Config) error {
	if cfg.Auth.ClientID == "" {
		return fmt.Errorf("no configuration found. Run 'cws init' to set up credentials, or set CWS_* environment variables")
	}
	if cfg.Auth.ClientSecret == "" {
		return fmt.Errorf("client secret not configured. Run 'cws init' to set up credentials, or set CWS_CLIENT_SECRET")
	}
	if cfg.Auth.RefreshToken == "" {
		return fmt.Errorf("refresh token not configured. Run 'cws init' to set up credentials, or set CWS_REFRESH_TOKEN")
	}
	if cfg.PublisherID == "" {
		return fmt.Errorf("publisher ID not configured. Run 'cws init' to set up credentials, or set CWS_PUBLISHER_ID")
	}
	return nil
}

// WriteConfig writes a config file to the specified path.
func WriteConfig(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	var content string

	// Check if this is a global config (has auth) or project config
	if cfg.Auth.ClientID != "" {
		content = fmt.Sprintf(`publisher_id = %q

[auth]
client_id = %q
client_secret = %q
refresh_token = %q
`,
			cfg.PublisherID,
			cfg.Auth.ClientID,
			cfg.Auth.ClientSecret,
			cfg.Auth.RefreshToken,
		)

		if cfg.Extensions.Default.ID != "" {
			content += fmt.Sprintf(`
[extensions.default]
id = %q
`, cfg.Extensions.Default.ID)
		}
	} else {
		content = fmt.Sprintf(`[extensions.default]
id = %q
`, cfg.Extensions.Default.ID)
		if cfg.Extensions.Default.Source != "" {
			content += fmt.Sprintf(`source = %q
`, cfg.Extensions.Default.Source)
		}
	}

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	return nil
}
