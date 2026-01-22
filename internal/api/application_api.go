// Package api provides API clients for Pelican panel.
//
//nolint:revive // Package name 'api' is intentionally generic
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"go.lostcrafters.com/pelican-cli/internal/application"
	"go.lostcrafters.com/pelican-cli/internal/auth"
	"go.lostcrafters.com/pelican-cli/internal/config"
	apierrors "go.lostcrafters.com/pelican-cli/internal/errors"
)

// ApplicationAPI wraps the Application API endpoints using the generated OpenAPI client.
type ApplicationAPI struct {
	genClient *application.ClientWithResponses
}

// NewApplicationAPI creates a new Application API client using the generated OpenAPI client.
func NewApplicationAPI() (*ApplicationAPI, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.New("config not loaded")
	}

	token, err := auth.GetToken("admin")
	if err != nil {
		return nil, fmt.Errorf("failed to get admin token: %w", err)
	}

	baseURL := cfg.API.BaseURL
	if baseURL == "" {
		return nil, fmt.Errorf(
			"API base URL not configured. Set PELICAN_API_BASE_URL or run 'pelican auth login %s'",
			"admin",
		)
	}

	// Append /api/application to base URL for the generated client
	apiBaseURL := baseURL
	if len(apiBaseURL) > 0 && apiBaseURL[len(apiBaseURL)-1] == '/' {
		apiBaseURL = apiBaseURL[:len(apiBaseURL)-1]
	}
	apiBaseURL += "/api/application"

	// Create request editor function to add auth header
	withAuth := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	genClient, err := application.NewClientWithResponses(
		apiBaseURL,
		application.WithRequestEditorFn(withAuth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generated client: %w", err)
	}

	return &ApplicationAPI{
		genClient: genClient,
	}, nil
}

// extractServerID extracts the server ID from a server map, checking both root and attributes.
func extractServerID(server map[string]any) any {
	if id, hasID := server["id"]; hasID {
		return id
	}
	if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
		if id, hasID := attrs["id"]; hasID {
			return id
		}
	}
	return nil
}

// getServerIDFromIdentifier converts a server identifier (UUID string or integer ID) to an integer ID.
//
//nolint:gocognit // UUID lookup and ID extraction requires high cognitive complexity
func (a *ApplicationAPI) getServerIDFromIdentifier(_ context.Context, identifier string) (int, error) {
	// Try to parse as integer ID first
	if serverID, err := strconv.Atoi(identifier); err == nil {
		return serverID, nil
	}

	// If not an integer, treat as UUID and look it up from server list
	servers, err := a.ListServers()
	if err != nil {
		return 0, fmt.Errorf("failed to list servers to look up UUID: %w", err)
	}

	// Find server with matching UUID
	for _, server := range servers {
		var serverUUID string

		// Check for uuid field (could be at root or in attributes)
		if uuid, hasUUID := server["uuid"].(string); hasUUID {
			serverUUID = uuid
		} else if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
			if uuidVal, hasUUIDVal := attrs["uuid"].(string); hasUUIDVal {
				serverUUID = uuidVal
			}
		}

		if serverUUID == identifier {
			// Found matching UUID, extract the integer ID
			idVal := extractServerID(server)
			if idVal == nil {
				return 0, errors.New("server ID not found in response")
			}

			// Convert to int
			switch v := idVal.(type) {
			case int:
				return v, nil
			case int64:
				return int(v), nil
			case float64:
				return int(v), nil
			case string:
				id, err := strconv.Atoi(v)
				if err != nil {
					return 0, fmt.Errorf("invalid server ID format: %w", err)
				}
				return id, nil
			default:
				return 0, fmt.Errorf("unexpected server ID type: %T", idVal)
			}
		}
	}

	return 0, fmt.Errorf("server with UUID %s not found", identifier)
}

// handleApplicationErrorResponse converts generated client error responses to APIError.
func handleApplicationErrorResponse(resp *http.Response, body []byte) error {
	statusCode := resp.StatusCode
	if statusCode < http.StatusBadRequest {
		return nil
	}

	return apierrors.NewAPIError(statusCode, string(body))
}

// ListNodes lists all nodes.
func (a *ApplicationAPI) ListNodes() ([]map[string]any, error) {
	ctx := context.Background()

	httpResp, err := a.genClient.ApplicationNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var nodes []any
	if err := json.Unmarshal(unwrapped, &nodes); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&nodes)
}

// GetNode gets a node by ID.
func (a *ApplicationAPI) GetNode(nodeID string) (map[string]any, error) {
	ctx := context.Background()

	// Try to parse as integer first
	nodeIDInt, err := strconv.Atoi(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %s (must be an integer)", nodeID)
	}

	httpResp, err := a.genClient.ApplicationNodesView(ctx, nodeIDInt)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var node any
	if err := json.Unmarshal(unwrapped, &node); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it
	if arr, ok := node.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(node)
}

// ListServers lists all servers.
func (a *ApplicationAPI) ListServers() ([]map[string]any, error) {
	ctx := context.Background()

	httpResp, err := a.genClient.ApplicationServers(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var servers []any
	if err := json.Unmarshal(unwrapped, &servers); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&servers)
}

// GetServer gets a server by UUID or integer ID.
func (a *ApplicationAPI) GetServer(identifier string) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Use the integer ID endpoint
	httpResp, err := a.genClient.ApplicationServersView(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var server any
	if err := json.Unmarshal(unwrapped, &server); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it
	if arr, ok := server.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(server)
}

// SuspendServer suspends a server by UUID or integer ID.
func (a *ApplicationAPI) SuspendServer(identifier string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	httpResp, err := a.genClient.ApplicationServersSuspend(ctx, serverID)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return handleApplicationErrorResponse(httpResp, bodyBytes)
	}

	return nil
}

// UnsuspendServer unsuspends a server by UUID or integer ID.
func (a *ApplicationAPI) UnsuspendServer(identifier string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	httpResp, err := a.genClient.ApplicationServersUnsuspend(ctx, serverID)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return handleApplicationErrorResponse(httpResp, bodyBytes)
	}

	return nil
}

// ReinstallServer reinstalls a server by UUID or integer ID.
func (a *ApplicationAPI) ReinstallServer(identifier string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	httpResp, err := a.genClient.ApplicationServersReinstall(ctx, serverID)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return handleApplicationErrorResponse(httpResp, bodyBytes)
	}

	return nil
}

// SendPowerCommand sends a power command to a server by UUID or integer ID.
func (a *ApplicationAPI) SendPowerCommand(identifier, command string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	// Map command string to the generated type
	var signal application.SendPowerRequestSignal
	switch command {
	case "start":
		signal = application.Start
	case "stop":
		signal = application.Stop
	case "restart":
		signal = application.Restart
	case "kill":
		signal = application.Kill
	default:
		return fmt.Errorf("invalid power command: %s", command)
	}

	body := application.SendPowerRequest{
		Signal: signal,
	}

	httpResp, err := a.genClient.PowerPowerWithResponse(ctx, serverID, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		return handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	return nil
}

// ListUsers lists all users.
func (a *ApplicationAPI) ListUsers() ([]map[string]any, error) {
	ctx := context.Background()

	httpResp, err := a.genClient.ApplicationUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var users []any
	if err := json.Unmarshal(unwrapped, &users); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&users)
}

// GetUser gets a user by ID.
func (a *ApplicationAPI) GetUser(userID string) (map[string]any, error) {
	ctx := context.Background()

	// Try to parse as integer first
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %s (must be an integer)", userID)
	}

	httpResp, err := a.genClient.ApplicationUsersView(ctx, userIDInt)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var user any
	if err := json.Unmarshal(unwrapped, &user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it
	if arr, ok := user.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(user)
}
