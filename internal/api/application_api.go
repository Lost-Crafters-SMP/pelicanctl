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
	"strings"
	"time"

	"go.lostcrafters.com/pelicanctl/internal/application"
	"go.lostcrafters.com/pelicanctl/internal/auth"
	"go.lostcrafters.com/pelicanctl/internal/config"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
)

const (
	// errorContextBefore is the number of characters to include before an error in context extraction.
	errorContextBefore = 50
	// errorContextAfter is the number of characters to include after an error in context extraction.
	errorContextAfter = 200
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
			"API base URL not configured. Set PELICANCTL_API_BASE_URL or run 'pelicanctl auth login %s'",
			"admin",
		)
	}

	// Append /api/application to base URL for the generated client.
	apiBaseURL := baseURL
	if len(apiBaseURL) > 0 && apiBaseURL[len(apiBaseURL)-1] == '/' {
		apiBaseURL = apiBaseURL[:len(apiBaseURL)-1]
	}
	apiBaseURL += "/api/application"

	// Create request editor function to add auth header and Accept header.
	withAuth := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
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
	// Try to parse as integer ID first.
	if serverID, err := strconv.Atoi(identifier); err == nil {
		return serverID, nil
	}

	// If not an integer, treat as UUID and look it up from server list.
	servers, err := a.ListServers()
	if err != nil {
		return 0, fmt.Errorf("failed to list servers to look up UUID: %w", err)
	}

	// Find server with matching UUID.
	for _, server := range servers {
		var serverUUID string

		// Check for uuid field (could be at root or in attributes).
		if uuid, hasUUID := server["uuid"].(string); hasUUID {
			serverUUID = uuid
		} else if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
			if uuidVal, hasUUIDVal := attrs["uuid"].(string); hasUUIDVal {
				serverUUID = uuidVal
			}
		}

		if serverUUID == identifier {
			// Found matching UUID, extract the integer ID.
			idVal := extractServerID(server)
			if idVal == nil {
				return 0, errors.New("server ID not found in response")
			}

			// Convert to int.
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

// extractErrorMessages extracts error messages from a structured error response.
func extractErrorMessages(errorResponse struct {
	Errors []struct {
		Code   string `json:"code"`
		Status string `json:"status"`
		Detail string `json:"detail"`
	} `json:"errors"`
	Message string `json:"message"`
	Error   string `json:"error"`
}) []string {
	var messages []string
	if len(errorResponse.Errors) > 0 {
		for _, e := range errorResponse.Errors {
			if e.Detail != "" {
				messages = append(messages, e.Detail)
			} else if e.Code != "" {
				messages = append(messages, e.Code)
			}
		}
	}
	if errorResponse.Message != "" {
		messages = append(messages, errorResponse.Message)
	}
	if errorResponse.Error != "" {
		messages = append(messages, errorResponse.Error)
	}
	return messages
}

// handleApplicationErrorResponse converts generated client error responses to APIError.
func handleApplicationErrorResponse(resp *http.Response, body []byte) error {
	statusCode := resp.StatusCode
	if statusCode < http.StatusBadRequest {
		return nil
	}

	// Try to parse structured error response.
	var errorResponse struct {
		Errors []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"errors"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}

	if err := json.Unmarshal(body, &errorResponse); err == nil {
		messages := extractErrorMessages(errorResponse)
		if len(messages) > 0 {
			return apierrors.NewAPIError(statusCode, messages[0])
		}
	}

	// Fall back to raw body as string, or status text if body is empty.
	errorMsg := string(body)
	if errorMsg == "" {
		errorMsg = fmt.Sprintf("HTTP %d %s", statusCode, http.StatusText(statusCode))
	}
	return apierrors.NewAPIError(statusCode, errorMsg)
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

	// Handle wrapped response.
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

	// Try to parse as integer first.
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

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var node any
	if err := json.Unmarshal(unwrapped, &node); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
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

	// Handle wrapped response.
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

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Use the integer ID endpoint.
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

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var server any
	if err := json.Unmarshal(unwrapped, &server); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := server.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(server)
}

// SuspendServer suspends a server by UUID or integer ID.
func (a *ApplicationAPI) SuspendServer(identifier string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
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

	// Convert identifier (UUID or integer ID) to integer ID.
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

	// Convert identifier (UUID or integer ID) to integer ID.
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

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	// Map command string to the generated type.
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

	httpResp, err := a.genClient.PowerIndexWithResponse(ctx, serverID, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		return handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	return nil
}

// SendCommand sends a console command to a server by UUID or integer ID.
func (a *ApplicationAPI) SendCommand(identifier, command string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	body := application.SendCommandRequest{
		Command: command,
	}

	httpResp, err := a.genClient.CommandIndexWithResponse(ctx, serverID, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		return handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	return nil
}

// GetServerHealth gets the health status of a server by UUID or integer ID.
func (a *ApplicationAPI) GetServerHealth(identifier string, since *time.Time, window *int) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Build parameters.
	params := &application.PowerHealthParams{
		Since:  since,
		Window: window,
	}

	httpResp, err := a.genClient.PowerHealthWithResponse(ctx, serverID, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// The response is already parsed into JSON200, convert it to map[string]any.
	if httpResp.JSON200 == nil {
		return nil, errors.New("health response data is nil")
	}

	// Convert the typed response to map[string]any via JSON marshaling/unmarshaling.
	jsonData, err := json.Marshal(httpResp.JSON200)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal health response: %w", err)
	}

	var healthData map[string]any
	if err := json.Unmarshal(jsonData, &healthData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal health response: %w", err)
	}

	return healthData, nil
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

	// Handle wrapped response.
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

	// Try to parse as integer first.
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

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var user any
	if err := json.Unmarshal(unwrapped, &user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := user.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(user)
}

// CreateNode creates a new node.
func (a *ApplicationAPI) CreateNode(nodeData map[string]any) (map[string]any, error) {
	ctx := context.Background()

	// Convert map to StoreNodeRequest.
	jsonData, err := json.Marshal(nodeData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node data: %w", err)
	}

	var nodeReq application.StoreNodeRequest
	if err := json.Unmarshal(jsonData, &nodeReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node request: %w", err)
	}

	httpResp, err := a.genClient.NodeStoreWithResponse(ctx, nodeReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK && httpResp.HTTPResponse.StatusCode != http.StatusCreated {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(httpResp.Body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var node any
	if err := json.Unmarshal(unwrapped, &node); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := node.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(node)
}

// UpdateNode updates an existing node.
// Note: The generated client's NodeUpdate method doesn't accept a request body based on the OpenAPI spec.
// This method may need to be updated if the API spec is updated to include a request body.
func (a *ApplicationAPI) UpdateNode(nodeID string) (map[string]any, error) {
	ctx := context.Background()

	// Try to parse as integer first.
	nodeIDInt, err := strconv.Atoi(nodeID)
	if err != nil {
		return nil, fmt.Errorf("invalid node ID: %s (must be an integer)", nodeID)
	}

	httpResp, err := a.genClient.NodeUpdateWithResponse(ctx, nodeIDInt)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(httpResp.Body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var node any
	if err := json.Unmarshal(unwrapped, &node); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := node.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(node)
}

// DeleteNode deletes a node by ID.
func (a *ApplicationAPI) DeleteNode(nodeID string) error {
	ctx := context.Background()

	// Try to parse as integer first.
	nodeIDInt, err := strconv.Atoi(nodeID)
	if err != nil {
		return fmt.Errorf("invalid node ID: %s (must be an integer)", nodeID)
	}

	httpResp, err := a.genClient.NodeDeleteWithResponse(ctx, nodeIDInt)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.HTTPResponse.Body)
		return handleApplicationErrorResponse(httpResp.HTTPResponse, bodyBytes)
	}

	return nil
}

// CreateServer creates a new server.
func (a *ApplicationAPI) CreateServer(serverData map[string]any) (map[string]any, error) {
	ctx := context.Background()

	// Convert map to StoreServerRequest.
	jsonData, err := json.Marshal(serverData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal server data: %w", err)
	}

	var serverReq application.StoreServerRequest
	if err := json.Unmarshal(jsonData, &serverReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal server request: %w", err)
	}

	httpResp, err := a.genClient.ServerStoreWithResponse(ctx, serverReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK && httpResp.HTTPResponse.StatusCode != http.StatusCreated {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(httpResp.Body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var server any
	if err := json.Unmarshal(unwrapped, &server); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertInterfaceToMap(server)
}

// DeleteServer deletes a server by UUID or integer ID.
func (a *ApplicationAPI) DeleteServer(identifier string, force bool) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	var httpResp *http.Response
	if force {
		httpResp, err = a.genClient.ApplicationServersServerDelete0(ctx, serverID, "force")
	} else {
		httpResp, err = a.genClient.ApplicationServersServerDelete1(ctx, serverID)
	}
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

// CreateUser creates a new user.
func (a *ApplicationAPI) CreateUser(userData map[string]any) (map[string]any, error) {
	ctx := context.Background()

	// Convert map to StoreUserRequest.
	jsonData, err := json.Marshal(userData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	var userReq application.StoreUserRequest
	if err := json.Unmarshal(jsonData, &userReq); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user request: %w", err)
	}

	httpResp, err := a.genClient.UserStoreWithResponse(ctx, userReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK && httpResp.HTTPResponse.StatusCode != http.StatusCreated {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(httpResp.Body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var user any
	if err := json.Unmarshal(unwrapped, &user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := user.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(user)
}

// UpdateUser updates an existing user.
// Note: Similar to NodeUpdate, the generated client's UserUpdate method may not accept a request body.
func (a *ApplicationAPI) UpdateUser(userID string) (map[string]any, error) {
	ctx := context.Background()

	// Try to parse as integer first.
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %s (must be an integer)", userID)
	}

	httpResp, err := a.genClient.UserUpdateWithResponse(ctx, userIDInt)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode != http.StatusOK {
		return nil, handleApplicationErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(httpResp.Body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var user any
	if err := json.Unmarshal(unwrapped, &user); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := user.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(user)
}

// DeleteUser deletes a user by ID.
func (a *ApplicationAPI) DeleteUser(userID string) error {
	ctx := context.Background()

	// Try to parse as integer first.
	userIDInt, err := strconv.Atoi(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %s (must be an integer)", userID)
	}

	httpResp, err := a.genClient.UserDeleteWithResponse(ctx, userIDInt)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.HTTPResponse.Body)
		return handleApplicationErrorResponse(httpResp.HTTPResponse, bodyBytes)
	}

	return nil
}

// ListBackups lists all backups for a server by UUID or integer ID.
func (a *ApplicationAPI) ListBackups(identifier string) ([]map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Use non-WithResponse version to handle parsing manually (API may return object instead of array).
	httpResp, err := a.genClient.BackupIndex(ctx, serverID)
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

	// Handle wrapped response - try to extract array from response.
	var backups []any

	// First, try to unmarshal directly as array.
	if err := json.Unmarshal(body, &backups); err == nil {
		return convertInterfaceSliceToMapSlice(&backups)
	}

	// If not an array, try as object (wrapped response).
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Check for common wrapper keys in the object.
	for _, key := range []string{"data", "backups"} {
		if val, hasKey := obj[key]; hasKey {
			if arr, isArray := val.([]any); isArray {
				backups = arr
				break
			}
		}
	}

	// If still no backups found, check if the object itself represents a single backup.
	if len(backups) == 0 {
		// Check if object has backup-like fields (uuid, name, etc.).
		if _, hasUUID := obj["uuid"]; hasUUID {
			backups = []any{obj}
		} else {
			return nil, errors.New("unexpected response format: could not extract backup array")
		}
	}

	return convertInterfaceSliceToMapSlice(&backups)
}

// CreateBackup creates a backup for a server by UUID or integer ID.
func (a *ApplicationAPI) CreateBackup(identifier string, backupData map[string]any) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Convert map to StoreBackupRequest.
	var backupReq application.StoreBackupRequest
	if backupData != nil {
		jsonData, err := json.Marshal(backupData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal backup data: %w", err)
		}
		if err := json.Unmarshal(jsonData, &backupReq); err != nil {
			return nil, fmt.Errorf("failed to unmarshal backup request: %w", err)
		}
	}

	// Use non-WithResponse version to handle parsing manually (API may return object instead of array).
	httpResp, err := a.genClient.BackupStore(ctx, serverID, backupReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		return nil, handleApplicationErrorResponse(httpResp, body)
	}

	// Handle wrapped response - try to extract backup from response.
	backup, err := extractBackupFromResponse(body)
	if err != nil {
		return nil, err
	}
	return convertInterfaceToMap(backup)
}

// GetBackup gets a backup by server UUID/ID and backup UUID.
func (a *ApplicationAPI) GetBackup(serverIdentifier, backupUUID string) (map[string]any, error) {
	ctx := context.Background()

	// Convert server identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get server ID: %w", err)
	}

	// Use non-WithResponse version to handle parsing manually (API may return object instead of array).
	httpResp, err := a.genClient.BackupView(ctx, serverID, backupUUID)
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

	// Handle wrapped response - try to extract backup from response.
	backup, err := extractBackupFromResponse(body)
	if err != nil {
		return nil, err
	}
	return convertInterfaceToMap(backup)
}

// extractBackupFromResponse extracts backup data from various response formats.
func extractBackupFromResponse(body []byte) (any, error) {
	// First, try to unmarshal directly as object.
	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		return extractBackupFromObject(obj), nil
	}

	// Try as array.
	var arr []any
	if err := json.Unmarshal(body, &arr); err == nil && len(arr) > 0 {
		return arr[0], nil
	}

	return nil, fmt.Errorf("failed to decode response: %w", json.Unmarshal(body, &obj))
}

// extractBackupFromObject extracts backup from a JSON object, handling wrapped responses.
func extractBackupFromObject(obj map[string]any) any {
	if data, hasData := obj["data"]; hasData {
		return data
	}
	if arr, hasBackups := obj["backups"].([]any); hasBackups && len(arr) > 0 {
		return arr[0]
	}
	// Object itself is the backup.
	return obj
}

// DeleteBackup deletes a backup by server UUID/ID and backup UUID.
func (a *ApplicationAPI) DeleteBackup(serverIdentifier, backupUUID string) error {
	ctx := context.Background()

	// Convert server identifier (UUID or integer ID) to integer ID.
	serverID, err := a.getServerIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return fmt.Errorf("failed to get server ID: %w", err)
	}

	// Use non-WithResponse version to properly handle all error status codes (including 400).
	httpResp, err := a.genClient.BackupDelete(ctx, serverID, backupUUID)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read body first (needed for both success and error cases).
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	statusCode := httpResp.StatusCode

	// Check status code first - if it's an error code, handle it immediately.
	// Only 204 No Content is considered success for DELETE operations.
	if statusCode == http.StatusNoContent {
		return checkNoContentWithErrorBody(body)
	}

	// Check for error status codes (400, 403, 404, 422, 500, etc.).
	if statusCode >= http.StatusBadRequest {
		return handleApplicationErrorResponse(httpResp, body)
	}

	// For non-error status codes (like 200), check body for error structure.
	return checkNonErrorStatusBody(httpResp, body, statusCode)
}

// checkNoContentWithErrorBody checks if a 204 response has an error body (edge case).
func checkNoContentWithErrorBody(body []byte) error {
	if len(body) == 0 {
		return nil
	}

	var errorCheck struct {
		Errors []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"errors"`
	}

	// If unmarshaling fails, there's no error structure - return nil (no error to report).
	// We intentionally ignore the unmarshal error here because it means the body doesn't
	// contain a valid error structure, which is acceptable for a 204 response.
	if _ = json.Unmarshal(body, &errorCheck); len(errorCheck.Errors) == 0 {
		return nil
	}

	errorMsg := errorCheck.Errors[0].Detail
	if errorMsg == "" {
		errorMsg = errorCheck.Errors[0].Code
	}
	if errorMsg == "" {
		errorMsg = "Backup deletion failed"
	}
	return apierrors.NewAPIError(http.StatusBadRequest, errorMsg)
}

// checkNonErrorStatusBody checks body for errors when status code is not an error code.
func checkNonErrorStatusBody(httpResp *http.Response, body []byte, statusCode int) error {
	bodyStr := string(body)
	if len(bodyStr) == 0 {
		return nil
	}

	// Check if response is HTML (indicates error page instead of JSON response).
	if htmlErr := checkHTMLErrorResponse(httpResp, bodyStr); htmlErr != nil {
		return htmlErr
	}

	// Check if body contains error-like JSON structure.
	if jsonErr := checkJSONErrorResponse(body, bodyStr, statusCode); jsonErr != nil {
		return jsonErr
	}

	// For any other status code (like 200), if we got here and body has content,
	// it might be an error we didn't detect - be conservative and check.
	if statusCode != http.StatusOK && statusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected response status %d: %s", statusCode, bodyStr)
	}

	return nil
}

// checkHTMLErrorResponse checks if the response is an HTML error page.
func checkHTMLErrorResponse(httpResp *http.Response, bodyStr string) error {
	bodyLower := strings.ToLower(bodyStr)
	if !strings.Contains(bodyLower, "<!doctype html") && !strings.Contains(bodyLower, "<html") {
		return nil
	}

	// Check if we were redirected to login page (authentication failure).
	if httpResp.Request != nil && httpResp.Request.URL != nil {
		urlPath := httpResp.Request.URL.Path
		if strings.Contains(strings.ToLower(urlPath), "login") {
			return apierrors.NewAPIError(
				http.StatusUnauthorized,
				"Authentication failed: request was redirected to login page. Please check your API token.",
			)
		}
	}

	// Generic HTML error page.
	errorMsg := "Backup deletion failed: API returned HTML error page instead of JSON response"
	if idx := strings.Index(bodyLower, "error"); idx != -1 {
		start := max(0, idx-errorContextBefore)
		end := min(len(bodyStr), idx+errorContextAfter)
		context := bodyStr[start:end]
		if len(context) > 0 {
			errorMsg = fmt.Sprintf("Backup deletion failed: %s", strings.TrimSpace(context))
		}
	}
	return apierrors.NewAPIError(http.StatusBadRequest, errorMsg)
}

// checkJSONErrorResponse checks if the body contains JSON error structures.
func checkJSONErrorResponse(body []byte, bodyStr string, statusCode int) error {
	if !strings.Contains(bodyStr, `"errors"`) && !strings.Contains(bodyStr, `"error"`) {
		return nil
	}

	var errorCheck struct {
		Errors []struct {
			Code   string `json:"code"`
			Status string `json:"status"`
			Detail string `json:"detail"`
		} `json:"errors"`
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	// If unmarshaling fails, there's no valid error structure - return nil (no error to report).
	// We intentionally ignore the unmarshal error here because it means the body doesn't
	// contain a valid error structure, which is acceptable.
	_ = json.Unmarshal(body, &errorCheck)

	// Found error structure - check if it has actual errors.
	if len(errorCheck.Errors) > 0 {
		return handleErrorsArray(errorCheck.Errors, statusCode)
	}

	// Check for other error fields.
	if errorCheck.Error != "" {
		return apierrors.NewAPIError(max(statusCode, http.StatusBadRequest), errorCheck.Error)
	}

	if errorCheck.Message != "" &&
		(strings.Contains(strings.ToLower(errorCheck.Message), "error") ||
			strings.Contains(strings.ToLower(errorCheck.Message), "fail")) {
		return apierrors.NewAPIError(max(statusCode, http.StatusBadRequest), errorCheck.Message)
	}

	return nil
}

// handleErrorsArray processes an array of errors from the response.
func handleErrorsArray(errors []struct {
	Code   string `json:"code"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}, statusCode int) error {
	errorMsg := errors[0].Detail
	if errorMsg == "" {
		errorMsg = errors[0].Code
	}
	if errorMsg == "" {
		errorMsg = "Backup deletion failed"
	}

	errStatusCode := max(statusCode, http.StatusBadRequest)
	if errors[0].Status != "" {
		if code, parseErr := strconv.Atoi(errors[0].Status); parseErr == nil {
			errStatusCode = max(code, http.StatusBadRequest)
		}
	}

	return apierrors.NewAPIError(errStatusCode, errorMsg)
}
