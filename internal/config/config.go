// Package config provides configuration management for the Pelican CLI.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration.
type Config struct {
	API    APIConfig    `mapstructure:"api"`
	Client ClientConfig `mapstructure:"client"`
	Admin  AdminConfig  `mapstructure:"admin"`
}

// APIConfig holds API-related configuration.
type APIConfig struct {
	BaseURL string `mapstructure:"base_url"`
}

// ClientConfig holds client API token configuration.
type ClientConfig struct {
	Token string `mapstructure:"token"`
}

// AdminConfig holds admin API token configuration.
type AdminConfig struct {
	Token string `mapstructure:"token"`
}

var (
	globalConfig *Config
	globalViper  *viper.Viper
)

// Load loads configuration from file, environment variables, and flags.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("api.base_url", "")
	v.SetDefault("client.token", "")
	v.SetDefault("admin.token", "")

	// Set config type
	v.SetConfigType("yaml")

	// If config path is provided, use it
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Otherwise, use default config directory
		configDir, err := getConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get config directory: %w", err)
		}

		v.SetConfigName("config")
		v.AddConfigPath(configDir)
	}

	// Environment variables
	v.SetEnvPrefix("PELICAN")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(stringReplacer())
	if err := v.BindEnv("client.token", "PELICAN_CLIENT_TOKEN"); err != nil {
		return nil, fmt.Errorf("failed to bind env var: %w", err)
	}
	if err := v.BindEnv("admin.token", "PELICAN_ADMIN_TOKEN"); err != nil {
		return nil, fmt.Errorf("failed to bind env var: %w", err)
	}
	if err := v.BindEnv("api.base_url", "PELICAN_API_BASE_URL"); err != nil {
		return nil, fmt.Errorf("failed to bind env var: %w", err)
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// If config file doesn't exist, that's okay - we'll use defaults and env vars
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	globalConfig = &config
	globalViper = v

	return &config, nil
}

// Get returns the global configuration.
func Get() *Config {
	return globalConfig
}

// Save saves the current configuration to the config file.
func Save() error {
	if globalViper == nil {
		return errors.New("config not loaded")
	}

	// Ensure config directory exists
	configDir := globalViper.ConfigFileUsed()
	if configDir == "" {
		var err error
		configDir, err = getConfigDir()
		if err != nil {
			return fmt.Errorf("failed to get config directory: %w", err)
		}
		configFile := filepath.Join(configDir, "config.yaml")
		globalViper.SetConfigFile(configFile)
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(globalViper.ConfigFileUsed())
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Update viper values from config
	if globalConfig != nil {
		globalViper.Set("api.base_url", globalConfig.API.BaseURL)
		globalViper.Set("client.token", globalConfig.Client.Token)
		globalViper.Set("admin.token", globalConfig.Admin.Token)
	}

	return globalViper.WriteConfig()
}

// GetConfigDir returns the platform-specific config directory.
func getConfigDir() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "pelican"), nil
}

// GetConfigPath returns the full path to the config file.
func GetConfigPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// stringReplacer creates a replacer for environment variable keys.
func stringReplacer() *strings.Replacer {
	return strings.NewReplacer(".", "_")
}
