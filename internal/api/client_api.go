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

	"go.lostcrafters.com/pelicanctl/internal/auth"
	"go.lostcrafters.com/pelicanctl/internal/client"
	"go.lostcrafters.com/pelicanctl/internal/config"
	apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
)

// ClientAPI wraps the Client API endpoints using the generated OpenAPI client.
type ClientAPI struct {
	genClient *client.ClientWithResponses
}

// NewClientAPI creates a new Client API client using the generated OpenAPI client.
func NewClientAPI() (*ClientAPI, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.New("config not loaded")
	}

	token, err := auth.GetToken("client")
	if err != nil {
		return nil, fmt.Errorf("failed to get client token: %w", err)
	}

	baseURL := cfg.API.BaseURL
	if baseURL == "" {
		return nil, fmt.Errorf(
			"API base URL not configured. Set PELICANCTL_API_BASE_URL or run 'pelicanctl auth login %s'",
			"client",
		)
	}

	// Append /api/client to base URL for the generated client.
	apiBaseURL := baseURL
	if len(apiBaseURL) > 0 && apiBaseURL[len(apiBaseURL)-1] == '/' {
		apiBaseURL = apiBaseURL[:len(apiBaseURL)-1]
	}
	apiBaseURL += "/api/client"

	// Create request editor function to add auth header and Accept header.
	withAuth := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		return nil
	}

	genClient, err := client.NewClientWithResponses(
		apiBaseURL,
		client.WithRequestEditorFn(withAuth),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create generated client: %w", err)
	}

	return &ClientAPI{
		genClient: genClient,
	}, nil
}

// handleErrorResponse converts generated client error responses to APIError.
func handleErrorResponse(resp *http.Response, body []byte) error {
	statusCode := resp.StatusCode
	if statusCode < http.StatusBadRequest {
		return nil
	}

	return apierrors.NewAPIError(statusCode, string(body))
}

// makeRawRequest is a helper that executes a raw HTTP request and returns the response body.
// It handles wrapped responses automatically.
func makeRawRequest(httpResp *http.Response, err error) ([]byte, error) {
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, handleErrorResponse(httpResp, body)
	}

	return body, nil
}

// convertInterfaceSliceToMapSlice converts []any to []map[string]any.
func convertInterfaceSliceToMapSlice(ifaceSlice *[]any) ([]map[string]any, error) {
	if ifaceSlice == nil {
		return nil, errors.New("response data is nil")
	}

	result := make([]map[string]any, 0, len(*ifaceSlice))
	for _, item := range *ifaceSlice {
		if m, ok := item.(map[string]any); ok {
			result = append(result, m)
		} else {
			// Try to convert via JSON marshaling/unmarshaling.
			jsonData, err := json.Marshal(item)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response item: %w", err)
			}
			var converted map[string]any
			if err := json.Unmarshal(jsonData, &converted); err != nil {
				return nil, fmt.Errorf("failed to unmarshal response item: %w", err)
			}
			result = append(result, converted)
		}
	}
	return result, nil
}

// convertInterfaceToMap converts any to map[string]any.
func convertInterfaceToMap(iface any) (map[string]any, error) {
	if iface == nil {
		return nil, errors.New("response data is nil")
	}

	if m, ok := iface.(map[string]any); ok {
		return m, nil
	}

	// Try to convert via JSON marshaling/unmarshaling.
	jsonData, err := json.Marshal(iface)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	var converted map[string]any
	if err := json.Unmarshal(jsonData, &converted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return converted, nil
}

// handleWrappedResponse checks if the response body contains a wrapped structure like {"data": [...]}.
func handleWrappedResponse(body []byte) ([]byte, error) {
	var wrapper map[string]any
	if err := json.Unmarshal(body, &wrapper); err != nil {
		// Not wrapped, return original body.
		return body, nil //nolint:nilerr // Intentionally returning body even if unmarshal fails
	}

	// Check for common wrapper keys.
	for _, key := range []string{"data", "servers", "backups", "databases", "files"} {
		if val, ok := wrapper[key]; ok {
			// Extract the wrapped data.
			unwrapped, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal unwrapped data: %w", err)
			}
			return unwrapped, nil
		}
	}

	// No wrapper found, return original body.
	return body, nil
}

// ListServers lists all servers available to the client.
func (c *ClientAPI) ListServers() ([]map[string]any, error) {
	ctx := context.Background()

	// Use the raw request method to avoid generated client parsing failures with wrapped responses.
	body, err := makeRawRequest(c.genClient.ApiClientIndex(ctx, nil))
	if err != nil {
		return nil, err
	}

	// Handle wrapped response (e.g., {"data": [...]}).
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

// getServerUUIDFromIdentifier converts a server identifier (UUID string or integer ID) to a UUID.
// Client API only accepts UUIDs, so if an integer ID is provided, we look it up.
func (c *ClientAPI) getServerUUIDFromIdentifier(_ context.Context, identifier string) (string, error) {
	// Check if it looks like a UUID (contains hyphens).
	if strings.Contains(identifier, "-") {
		return identifier, nil
	}

	// Try to parse as integer ID.
	if _, err := strconv.Atoi(identifier); err != nil {
		// Not an integer, assume it's a UUID and pass through.
		return identifier, nil //nolint:nilerr // Intentionally returning identifier even if parse fails
	}

	// It's an integer ID, need to look it up.
	servers, err := c.ListServers()
	if err != nil {
		return "", fmt.Errorf("failed to list servers to look up UUID: %w", err)
	}

	// Find server with matching ID.
	for _, server := range servers {
		var serverID any

		// Check for id field (could be at root or in attributes).
		if id, hasID := server["id"]; hasID {
			serverID = id
		} else if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
			if idVal, hasIDVal := attrs["id"]; hasIDVal {
				serverID = idVal
			}
		}

		// Compare IDs (handle float64 from JSON).
		var idInt int
		switch v := serverID.(type) {
		case int:
			idInt = v
		case int64:
			idInt = int(v)
		case float64:
			idInt = int(v)
		case string:
			parsed, err := strconv.Atoi(v)
			if err != nil {
				continue
			}
			idInt = parsed
		default:
			continue
		}

		targetID, _ := strconv.Atoi(identifier)
		if idInt == targetID {
			if uuid, ok := server["uuid"].(string); ok {
				return uuid, nil
			}
		}
	}

	return "", fmt.Errorf("server with ID %s not found", identifier)
}

// GetServer gets a server by UUID or integer ID.
func (c *ClientAPI) GetServer(identifier string) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID..
	uuid, err := c.getServerUUIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, err
	}

	body, err := makeRawRequest(c.genClient.ApiClientServerView(ctx, uuid))
	if err != nil {
		return nil, err
	}

	// Handle wrapped response or single object.
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

// GetServerResources gets server resource usage by UUID or integer ID.
func (c *ClientAPI) GetServerResources(identifier string) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID..
	uuid, err := c.getServerUUIDFromIdentifier(ctx, identifier)
	if err != nil {
		return nil, err
	}

	body, err := makeRawRequest(c.genClient.ApiClientServerResources(ctx, uuid))
	if err != nil {
		return nil, err
	}

	// Try to parse the response body directly.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var resources any
	if err := json.Unmarshal(unwrapped, &resources); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return convertInterfaceToMap(resources)
}

// ListFiles lists files in a directory by server UUID or integer ID.
func (c *ClientAPI) ListFiles(serverIdentifier, directory string) ([]map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, err
	}

	params := &client.FileDirectoryParams{}
	if directory != "" {
		params.Directory = &directory
	}

	body, err := makeRawRequest(c.genClient.FileDirectory(ctx, serverUUID, params))
	if err != nil {
		return nil, err
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var files []any
	if err := json.Unmarshal(unwrapped, &files); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&files)
}

// SendPowerCommand sends a power command to a server by UUID or integer ID.
func (c *ClientAPI) SendPowerCommand(serverIdentifier, command string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return err
	}

	// Map command string to the generated type.
	var signal client.SendPowerRequestSignal
	switch command {
	case "start":
		signal = client.Start
	case "stop":
		signal = client.Stop
	case "restart":
		signal = client.Restart
	case "kill":
		signal = client.Kill
	default:
		return fmt.Errorf("invalid power command: %s", command)
	}

	body := client.PowerIndexJSONRequestBody{
		Signal: signal,
	}

	httpResp, err := c.genClient.PowerIndex(ctx, serverUUID, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode >= http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return handleErrorResponse(httpResp, bodyBytes)
	}

	return nil
}

// SendCommand sends a console command to a server by UUID or integer ID.
func (c *ClientAPI) SendCommand(serverIdentifier, command string) error {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return err
	}

	body := client.SendCommandRequest{
		Command: command,
	}

	httpResp, err := c.genClient.CommandIndexWithResponse(ctx, serverUUID, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.HTTPResponse.Body.Close()

	if httpResp.HTTPResponse.StatusCode >= http.StatusBadRequest {
		return handleErrorResponse(httpResp.HTTPResponse, httpResp.Body)
	}

	return nil
}

// ListBackups lists backups for a server by UUID or integer ID.
func (c *ClientAPI) ListBackups(serverIdentifier string) ([]map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, err
	}

	body, err := makeRawRequest(c.genClient.BackupIndex(ctx, serverUUID))
	if err != nil {
		return nil, err
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var backups []any
	if err := json.Unmarshal(unwrapped, &backups); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&backups)
}

// CreateBackup creates a backup for a server by UUID or integer ID.
func (c *ClientAPI) CreateBackup(serverIdentifier string) (map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, err
	}

	httpResp, err := c.genClient.BackupStore(ctx, serverUUID, client.BackupStoreJSONRequestBody{})
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusCreated {
		return nil, handleErrorResponse(httpResp, body)
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var backup any
	if err := json.Unmarshal(unwrapped, &backup); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// If it's a slice with one item, extract it.
	if arr, ok := backup.([]any); ok && len(arr) > 0 {
		return convertInterfaceToMap(arr[0])
	}

	return convertInterfaceToMap(backup)
}

// ListDatabases lists databases for a server by UUID or integer ID.
func (c *ClientAPI) ListDatabases(serverIdentifier string) ([]map[string]any, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, err
	}

	body, err := makeRawRequest(c.genClient.DatabaseIndex(ctx, serverUUID))
	if err != nil {
		return nil, err
	}

	// Handle wrapped response.
	unwrapped, unwrapErr := handleWrappedResponse(body)
	if unwrapErr != nil {
		return nil, fmt.Errorf("failed to decode response: %w", unwrapErr)
	}

	var databases []any
	if err := json.Unmarshal(unwrapped, &databases); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return convertInterfaceSliceToMapSlice(&databases)
}

// DownloadFile downloads a file from the server by UUID or integer ID.
func (c *ClientAPI) DownloadFile(serverIdentifier, filePath string) (io.ReadCloser, error) {
	ctx := context.Background()

	// Convert identifier (UUID or integer ID) to UUID.
	serverUUID, err := c.getServerUUIDFromIdentifier(ctx, serverIdentifier)
	if err != nil {
		return nil, err
	}

	params := &client.FileDownloadParams{
		File: filePath,
	}

	// Use the low-level method to get the raw response body.
	// ClientWithResponses embeds ClientInterface which has FileDownload.
	httpResp, err := c.genClient.ClientInterface.FileDownload(ctx, serverUUID, params)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		return nil, handleErrorResponse(httpResp, bodyBytes)
	}

	// Return the response body - caller is responsible for closing.
	return httpResp.Body, nil
}

// UploadFile uploads a file to the server.
func (c *ClientAPI) UploadFile(_, _, _ string) error {
	// This is a simplified version - actual implementation would need multipart form data.
	return errors.New("file upload not yet implemented")
}
