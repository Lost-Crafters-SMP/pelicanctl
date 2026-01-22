// Package main provides the pelicanctl application entry point.
package main

import (
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"

	"go.lostcrafters.com/pelicanctl/cmd/admin"
	"go.lostcrafters.com/pelicanctl/cmd/client"
	"go.lostcrafters.com/pelicanctl/internal/auth"
	"go.lostcrafters.com/pelicanctl/internal/config"
	"go.lostcrafters.com/pelicanctl/internal/output"
)

// Version is set during build via ldflags
var Version = "dev"

type appConfig struct {
	configPath string
	json       bool
	verbose    bool
	quiet      bool
}

func setupRootCmd(cfg *appConfig) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "pelicanctl",
		Short: "pelicanctl - Manage your Pelican panel servers",
		Long: `pelicanctl is a command-line tool for managing servers on the Pelican panel.

It provides both client and admin interfaces for server management, file operations,
backups, databases, and more.`,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Skip PersistentPreRunE entirely for _carapace command to avoid interfering with completion
			// The _carapace command is a hidden subcommand added by carapace.Gen() and needs direct access
			if cmd.Name() == "_carapace" {
				// Still load config for API clients in completions, but don't initialize logger
				_, _ = config.Load(cfg.configPath)
				return nil
			}

			// Load configuration
			_, err := config.Load(cfg.configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Initialize logger for normal commands
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
		"config file (default is $XDG_CONFIG_HOME/pelicanctl/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&cfg.json, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&cfg.verbose, "verbose", false, "enable verbose logging")
	rootCmd.PersistentFlags().BoolVar(&cfg.quiet, "quiet", false, "minimal output (errors only)")

	// Disable Cobra's default completion command to avoid conflicts with carapace
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Initialize carapace - this sets up the completion infrastructure
	// Carapace automatically adds a hidden _carapace subcommand that carapace-bin uses
	// We call this before adding subcommands to initialize the infrastructure
	carapace.Gen(rootCmd)

	// Add subcommands - PositionalCompletion setups will be discovered by carapace
	rootCmd.AddCommand(client.NewClientCmd())
	rootCmd.AddCommand(admin.NewAdminCmd())
	rootCmd.AddCommand(newAuthCmd(cfg))
	rootCmd.AddCommand(newVersionCmd())

	// Call carapace.Gen again after all subcommands are added to ensure discovery
	// This matches the pattern in reference examples where Gen is called multiple times
	carapace.Gen(rootCmd)

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
	loginCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"client", "admin"}, cobra.ShellCompDirectiveNoFileComp
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
	logoutCmd.ValidArgsFunction = func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{"client", "admin"}, cobra.ShellCompDirectiveNoFileComp
	}

	// Add subcommands FIRST (matching carapace example pattern)
	cmd.AddCommand(loginCmd)
	cmd.AddCommand(logoutCmd)

	// Set up carapace completion AFTER adding to parent (matching carapace example pattern)
	// Using direct ActionValues (no ActionCallback) to test basic functionality
	carapace.Gen(loginCmd).PositionalCompletion(
		carapace.ActionValues("client", "admin"),
	)
	carapace.Gen(logoutCmd).PositionalCompletion(
		carapace.ActionValues("client", "admin"),
	)

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
		if envURL := os.Getenv("PELICANCTL_API_BASE_URL"); envURL != "" {
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

// newVersionCmd creates the version command.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the version number of pelicanctl",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("pelicanctl version %s\n", Version)
		},
	}
}
