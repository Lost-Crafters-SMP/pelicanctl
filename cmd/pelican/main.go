// Package main provides the Pelican CLI application entry point.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelican-cli/cmd/admin"
	"go.lostcrafters.com/pelican-cli/cmd/client"
	"go.lostcrafters.com/pelican-cli/internal/auth"
	"go.lostcrafters.com/pelican-cli/internal/config"
	"go.lostcrafters.com/pelican-cli/internal/output"
)

type appConfig struct {
	configPath string
	json       bool
	verbose    bool
	quiet      bool
}

func setupRootCmd(cfg *appConfig) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "pelican",
		Short: "Pelican CLI - Manage your Pelican panel servers",
		Long: `Pelican CLI is a command-line tool for managing servers on the Pelican panel.

It provides both client and admin interfaces for server management, file operations,
backups, databases, and more.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			// Load configuration
			_, err := config.Load(cfg.configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Initialize logger
			var format output.OutputFormat
			if cfg.json {
				format = output.OutputFormatJSON
			} else {
				format = output.OutputFormatTable
			}
			output.InitLogger(cfg.verbose, cfg.quiet, format, os.Stderr)

			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(
		&cfg.configPath, "config", "",
		"config file (default is $XDG_CONFIG_HOME/pelican/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&cfg.json, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&cfg.quiet, "quiet", false, "minimal output (errors only)")

	// Add subcommands
	rootCmd.AddCommand(client.NewClientCmd())
	rootCmd.AddCommand(admin.NewAdminCmd())
	rootCmd.AddCommand(newAuthCmd(cfg))

	return rootCmd
}

func main() {
	cfg := &appConfig{}
	rootCmd := setupRootCmd(cfg)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// newAuthCmd creates the auth command.
func newAuthCmd(cfg *appConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  "Manage API tokens for client and admin APIs",
	}

	loginCmd := &cobra.Command{
		Use:   "login [client|admin]",
		Short: "Login interactively and save token",
		Long:  "Prompts for an API token and saves it to the system keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			apiType := args[0]
			if apiType != "client" && apiType != "admin" {
				return fmt.Errorf("invalid API type: %s (must be 'client' or 'admin')", apiType)
			}

			return authLogin(apiType, cfg)
		},
	}

	logoutCmd := &cobra.Command{
		Use:   "logout [client|admin]",
		Short: "Logout and clear saved token",
		Long:  "Removes the API token from keyring and config file",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			apiType := args[0]
			if apiType != "client" && apiType != "admin" {
				return fmt.Errorf("invalid API type: %s (must be 'client' or 'admin')", apiType)
			}

			return authLogout(apiType, cfg)
		},
	}

	cmd.AddCommand(loginCmd)
	cmd.AddCommand(logoutCmd)
	return cmd
}

func authLogin(apiType string, cfg *appConfig) error {
	appCfg := config.Get()
	if appCfg == nil {
		_, err := config.Load(cfg.configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		appCfg = config.Get()
	}

	var format output.OutputFormat
	if cfg.json {
		format = output.OutputFormatJSON
	} else {
		format = output.OutputFormatTable
	}
	formatter := output.NewFormatter(format, os.Stdout)

	// Only prompt for API URL if it's not already configured
	// Check both config and environment variable
	currentURL := appCfg.API.BaseURL
	if currentURL == "" {
		// Check environment variable
		if envURL := os.Getenv("PELICAN_API_BASE_URL"); envURL != "" {
			currentURL = envURL
		}
	}

	// If still empty, prompt for it
	if currentURL == "" {
		apiURL, err := auth.PromptAPIURL("")
		if err != nil {
			return fmt.Errorf("failed to get API URL: %w", err)
		}

		// Save API URL to config
		if setErr := auth.SetAPIURL(apiURL); setErr != nil {
			formatter.PrintError("Failed to save API URL: %v", setErr)
			return setErr
		}
	}

	// Prompt for token
	token, err := auth.PromptToken(apiType)
	if err != nil {
		return err
	}

	if setErr := auth.SetToken(apiType, token); setErr != nil {
		formatter.PrintError("Failed to save token: %v", setErr)
		return setErr
	}

	formatter.PrintSuccess("%s token saved successfully", apiType)
	return nil
}

func authLogout(apiType string, cfg *appConfig) error {
	appCfg := config.Get()
	if appCfg == nil {
		_, err := config.Load(cfg.configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		// Config is now loaded globally via config.Load
	}

	var format output.OutputFormat
	if cfg.json {
		format = output.OutputFormatJSON
	} else {
		format = output.OutputFormatTable
	}
	formatter := output.NewFormatter(format, os.Stdout)

	if err := auth.DeleteToken(apiType); err != nil {
		formatter.PrintError("Failed to logout: %v", err)
		return err
	}

	formatter.PrintSuccess("%s token cleared successfully", apiType)
	return nil
}
