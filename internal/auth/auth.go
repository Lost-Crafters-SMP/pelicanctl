// Package auth provides authentication and token management for pelicanctl.
package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"

	"go.lostcrafters.com/pelicanctl/internal/config"
)

const (
	keyringService = "pelicanctl"
	apiTypeClient  = "client"
	apiTypeAdmin   = "admin"
)

var (
	// warnedAPITypes tracks which API types have already shown config file warnings.
	//
	//nolint:gochecknoglobals // Global state needed for session-wide warning tracking
	warnedAPITypes = make(map[string]bool)

	//nolint:gochecknoglobals // Global mutex needed to protect warnedAPITypes map
	warnedMutex sync.Mutex
)

// getKeyringKey returns the keyring user/account key for the given API type.
func getKeyringKey(apiType string) string {
	return fmt.Sprintf("%s-token", apiType)
}

// warnIfTokenInConfig warns the user if a token is found in the config file.
// Only warns once per API type per session.
func warnIfTokenInConfig(apiType string) {
	warnedMutex.Lock()
	defer warnedMutex.Unlock()

	if warnedAPITypes[apiType] {
		return
	}

	warnedAPITypes[apiType] = true
	_, _ = fmt.Fprintf(os.Stderr,
		"⚠ Warning: Token found in config file. Consider migrating to system keyring for better security.\n"+
			"  Run 'pelicanctl auth login %s' to migrate.\n",
		apiType)
}

// GetToken retrieves the token for the specified API type.
func GetToken(apiType string) (string, error) {
	cfg := config.Get()
	if cfg == nil {
		return "", errors.New("config not loaded")
	}

	// 1. Check environment variable first (highest priority)
	envVar := fmt.Sprintf("PELICANCTL_%s_TOKEN", strings.ToUpper(apiType))
	if envToken := os.Getenv(envVar); envToken != "" {
		return envToken, nil
	}

	// 2. Try keyring (for developer machines)
	keyringToken, err := keyring.Get(keyringService, getKeyringKey(apiType))
	if err == nil && keyringToken != "" {
		return keyringToken, nil
	}
	// Silently continue if keyring unavailable or token not found

	// 3. Check config file (fallback with warning)
	var token string
	switch apiType {
	case apiTypeClient:
		token = cfg.Client.Token
	case apiTypeAdmin:
		token = cfg.Admin.Token
	default:
		return "", fmt.Errorf("invalid API type: %s", apiType)
	}

	if token != "" {
		warnIfTokenInConfig(apiType)
	}

	// Return token from config (may be empty)
	return token, nil
}

// SetToken sets the token for the specified API type and saves it to keyring.
func SetToken(apiType, token string) error {
	cfg := config.Get()
	if cfg == nil {
		return errors.New("config not loaded")
	}

	// Validate API type
	switch apiType {
	case apiTypeClient, apiTypeAdmin:
		// Valid API type
	default:
		return fmt.Errorf("invalid API type: %s", apiType)
	}

	// Save to keyring
	if err := keyring.Set(keyringService, getKeyringKey(apiType), token); err != nil {
		// Log warning but don't fail - fallback to config if keyring unavailable
		_, _ = fmt.Fprintf(os.Stderr, "Warning: Failed to save to keyring: %v\n", err)
	}

	// Clear token from config file
	switch apiType {
	case apiTypeClient:
		cfg.Client.Token = ""
	case apiTypeAdmin:
		cfg.Admin.Token = ""
	}

	return config.Save()
}

// DeleteToken removes the token for the specified API type from keyring and config.
func DeleteToken(apiType string) error {
	cfg := config.Get()
	if cfg == nil {
		return errors.New("config not loaded")
	}

	// Validate API type
	switch apiType {
	case apiTypeClient, apiTypeAdmin:
		// Valid API type
	default:
		return fmt.Errorf("invalid API type: %s", apiType)
	}

	// Delete from keyring (ignore errors - keyring may not have token)
	_ = keyring.Delete(keyringService, getKeyringKey(apiType))

	// Clear from config
	switch apiType {
	case apiTypeClient:
		cfg.Client.Token = ""
	case apiTypeAdmin:
		cfg.Admin.Token = ""
	}

	return config.Save()
}

// PromptAPIURL prompts the user for an API base URL with a default value.
func PromptAPIURL(defaultURL string) (string, error) {
	prompt := "Enter API base URL"
	if defaultURL != "" {
		_, _ = fmt.Fprintf(os.Stderr, "%s [%s]: ", prompt, defaultURL)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "%s: ", prompt)
	}

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)

	// If empty input and we have a default, use the default
	if input == "" && defaultURL != "" {
		return defaultURL, nil
	}

	// If empty input and no default, return error - URL is required
	if input == "" {
		return "", errors.New("API base URL is required")
	}

	return input, nil
}

// PromptToken prompts the user for a token interactively.
// Supports pasting on all modern terminals.
func PromptToken(apiType string) (string, error) {
	_, _ = fmt.Fprintf(os.Stderr, "Enter %s API token: ", apiType)

	// Read from stdin with password masking - supports pasting
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}
	_, _ = fmt.Fprintln(os.Stderr) // New line after password input

	token := strings.TrimSpace(string(tokenBytes))
	if token == "" {
		return "", errors.New("token cannot be empty")
	}

	return token, nil
}

// SetAPIURL sets the API base URL in the configuration.
func SetAPIURL(baseURL string) error {
	cfg := config.Get()
	if cfg == nil {
		return errors.New("config not loaded")
	}

	cfg.API.BaseURL = baseURL
	return config.Save()
}

// Login handles interactive login for the specified API type.
func Login(apiType string) error {
	token, err := PromptToken(apiType)
	if err != nil {
		return err
	}

	if err := SetToken(apiType, token); err != nil {
		return fmt.Errorf("failed to save token: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stderr, "✓ %s token saved successfully\n", apiType)
	return nil
}
