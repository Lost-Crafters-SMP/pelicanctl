// Package completion provides completions for the CLI.
package completion

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.lostcrafters.com/pelicanctl/internal/api"
)

// CompleteServers returns server UUIDs and IDs for client or admin API.
func CompleteServers(apiType string, toComplete string) ([]string, error) {
	cacheKey := getCacheKey(apiType, "servers")
	if cached := getCached(cacheKey); cached != nil {
		return filterCompletions(cached, toComplete), nil
	}

	var servers []map[string]any
	var err error

	if apiType == "client" {
		var client *api.ClientAPI
		client, err = api.NewClientAPI()
		if err != nil {
			fmt.Fprintf(os.Stderr, "completion debug: NewClientAPI failed: %v\n", err)
			return nil, nil
		}
		servers, err = client.ListServers()
	} else {
		var client *api.ApplicationAPI
		client, err = api.NewApplicationAPI()
		if err != nil {
			fmt.Fprintf(os.Stderr, "completion debug: NewApplicationAPI failed: %v\n", err)
			return nil, nil
		}
		servers, err = client.ListServers()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list servers: %v\n", err)
		return nil, nil
	}

	var identifiers []string
	for _, server := range servers {
		if uuid, ok := server["uuid"].(string); ok {
			identifiers = append(identifiers, uuid)
		}
		if id, ok := server["id"]; ok {
			identifiers = append(identifiers, fmt.Sprintf("%v", id))
		}
	}

	setCached(cacheKey, identifiers)
	return filterCompletions(identifiers, toComplete), nil
}

// CompleteNodes returns node IDs for admin API.
func CompleteNodes(toComplete string) ([]string, error) {
	cacheKey := getCacheKey("admin", "nodes")
	if cached := getCached(cacheKey); cached != nil {
		return filterCompletions(cached, toComplete), nil
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return nil, nil
	}

	var nodes []map[string]any
	nodes, err = client.ListNodes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list nodes: %v\n", err)
		return nil, nil
	}

	var identifiers []string
	for _, node := range nodes {
		if id, ok := node["id"]; ok {
			identifiers = append(identifiers, fmt.Sprintf("%v", id))
		}
	}

	setCached(cacheKey, identifiers)
	return filterCompletions(identifiers, toComplete), nil
}

// CompleteUsers returns user IDs for admin API.
func CompleteUsers(toComplete string) ([]string, error) {
	cacheKey := getCacheKey("admin", "users")
	if cached := getCached(cacheKey); cached != nil {
		return filterCompletions(cached, toComplete), nil
	}

	client, err := api.NewApplicationAPI()
	if err != nil {
		return nil, nil
	}

	var users []map[string]any
	users, err = client.ListUsers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list users: %v\n", err)
		return nil, nil
	}

	var identifiers []string
	for _, user := range users {
		if id, ok := user["id"]; ok {
			identifiers = append(identifiers, fmt.Sprintf("%v", id))
		}
	}

	setCached(cacheKey, identifiers)
	return filterCompletions(identifiers, toComplete), nil
}

// CompleteBackups returns backup UUIDs for a server.
func CompleteBackups(serverIdentifier, toComplete string) ([]string, error) {
	cacheKey := getCacheKey("client", "backups:"+serverIdentifier)
	if cached := getCached(cacheKey); cached != nil {
		return filterCompletions(cached, toComplete), nil
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return nil, nil
	}

	var serverUUID string
	serverUUID, err = getServerUUID(client, serverIdentifier)
	if err != nil {
		return nil, nil
	}

	backups, err := client.ListBackups(serverUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list backups: %v\n", err)
		return nil, nil
	}

	var identifiers []string
	for _, backup := range backups {
		if uuid, ok := backup["uuid"].(string); ok {
			identifiers = append(identifiers, uuid)
		} else if name, okName := backup["name"].(string); okName {
			identifiers = append(identifiers, name)
		}
	}

	setCached(cacheKey, identifiers)
	return filterCompletions(identifiers, toComplete), nil
}

// CompleteDatabases returns database names for a server.
func CompleteDatabases(serverIdentifier, toComplete string) ([]string, error) {
	cacheKey := getCacheKey("client", "databases:"+serverIdentifier)
	if cached := getCached(cacheKey); cached != nil {
		return filterCompletions(cached, toComplete), nil
	}

	client, err := api.NewClientAPI()
	if err != nil {
		return nil, nil
	}

	var serverUUID string
	serverUUID, err = getServerUUID(client, serverIdentifier)
	if err != nil {
		return nil, nil
	}

	databases, err := client.ListDatabases(serverUUID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list databases: %v\n", err)
		return nil, nil
	}

	var names []string
	for _, db := range databases {
		if name, ok := db["name"].(string); ok {
			names = append(names, name)
		}
	}

	setCached(cacheKey, names)
	return filterCompletions(names, toComplete), nil
}

// CompleteFiles returns file paths for a server and directory.
func CompleteFiles(serverIdentifier, directory, toComplete string) ([]string, error) {
	// Don't cache file listings as they change frequently
	client, err := api.NewClientAPI()
	if err != nil {
		return nil, nil
	}

	var serverUUID string
	serverUUID, err = getServerUUID(client, serverIdentifier)
	if err != nil {
		return nil, nil
	}

	files, err := client.ListFiles(serverUUID, directory)
	if err != nil {
		fmt.Fprintf(os.Stderr, "completion error: failed to list files: %v\n", err)
		return nil, nil
	}

	var paths []string
	for _, file := range files {
		if name, ok := file["name"].(string); ok {
			// Build full path if directory is provided
			if directory != "" {
				paths = append(paths, strings.TrimSuffix(directory, "/")+"/"+name)
			} else {
				paths = append(paths, name)
			}
		}
	}

	return filterCompletions(paths, toComplete), nil
}

// getServerUUID converts a server identifier (UUID or ID) to UUID using the client API.
// This is a helper that uses the ClientAPI's internal method.
func getServerUUID(client *api.ClientAPI, identifier string) (string, error) {
	// Check if it looks like a UUID (contains hyphens)
	if strings.Contains(identifier, "-") {
		return identifier, nil
	}

	// Try to parse as integer ID - if it fails, assume it's a UUID
	if _, atoiErr := strconv.Atoi(identifier); atoiErr != nil {
		return identifier, nil
	}

	// It's an integer ID, need to look it up from server list
	servers, err := client.ListServers()
	if err != nil {
		return "", err
	}

	// Find server with matching ID
	for _, server := range servers {
		var serverID any
		if id, hasID := server["id"]; hasID {
			serverID = id
		} else if attrs, hasAttrs := server["attributes"].(map[string]any); hasAttrs {
			if idVal, hasIDVal := attrs["id"]; hasIDVal {
				serverID = idVal
			}
		}

		// Compare IDs
		var idInt int
		switch v := serverID.(type) {
		case int:
			idInt = v
		case int64:
			idInt = int(v)
		case float64:
			idInt = int(v)
		case string:
			parsed, parseErr := strconv.Atoi(v)
			if parseErr != nil {
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

// filterCompletions filters completion results based on the prefix to complete.
func filterCompletions(completions []string, toComplete string) []string {
	if toComplete == "" {
		// Limit results if no prefix
		const maxResults = 100
		if len(completions) > maxResults {
			return completions[:maxResults]
		}
		return completions
	}

	var filtered []string
	for _, completion := range completions {
		if strings.HasPrefix(completion, toComplete) {
			filtered = append(filtered, completion)
		}
	}

	// Limit filtered results too
	const maxResults = 100
	if len(filtered) > maxResults {
		return filtered[:maxResults]
	}

	return filtered
}
