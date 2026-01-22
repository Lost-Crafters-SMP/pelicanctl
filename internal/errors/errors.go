// Package errors provides error handling for pelicanctl.
//
//nolint:revive // Package name conflicts with stdlib but is intentional for domain-specific errors
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError represents an error from the API.
type APIError struct {
	StatusCode int
	Message    string
	Details    map[string]any
}

// NewAPIError creates a new API error.
func NewAPIError(statusCode int, message string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
	}
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("API error: %d %s", e.StatusCode, http.StatusText(e.StatusCode))
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsUnauthorized returns true if the error is a 401 or 403.
func (e *APIError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}

// HandleError provides user-friendly error messages.
func HandleError(err error) string {
	if err == nil {
		return ""
	}

	// Handle API errors
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsNotFound():
			return fmt.Sprintf("Resource not found: %s", apiErr.Message)
		case apiErr.IsUnauthorized():
			return fmt.Sprintf(
				"Authentication failed: %s\n  Tip: Run 'pelicanctl auth login' to configure your API token",
				apiErr.Message)
		case apiErr.StatusCode >= http.StatusInternalServerError:
			return fmt.Sprintf("Server error: %s\n  The Pelican panel may be experiencing issues", apiErr.Message)
		case apiErr.StatusCode >= http.StatusBadRequest:
			return fmt.Sprintf("Request error: %s", apiErr.Message)
		default:
			return apiErr.Error()
		}
	}

	// Generic error
	return fmt.Sprintf("Error: %s", err.Error())
}

// WrapError wraps an error with context.
func WrapError(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}
