# Agent Guidelines for pelicanctl

This document provides essential guidelines for agentic coding in the pelicanctl repository.

## Development Commands

### Building and Testing
```bash
just build              # Build the pelicanctl binary
just test               # Run all tests
just test -run TestName # Run a specific test
just fmt                # Format code with go fmt
just lint               # Run go vet
just tidy               # Clean up go.mod dependencies
```

### Code Generation
```bash
just generate           # Generate all API clients from OpenAPI specs
just generate-client    # Generate Client API only (includes post-processing fix)
just generate-application # Generate Application API only (includes post-processing fix)
```

**Note**: Code generation automatically runs a post-processing script (`scripts/fix-generated-params.sh`) to fix duplicate parameter names in generated code (e.g., `server string, server int` -> `server string, serverID int`). This is necessary because `oapi-codegen` doesn't handle path parameters that share names with the base URL parameter.

## Code Style Guidelines

### Import Order
1. Standard library
2. Third-party packages
3. Internal packages (`go.lostcrafters.com/pelicanctl/...`)

```go
import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/spf13/viper"

    "go.lostcrafters.com/pelicanctl/internal/config"
    apierrors "go.lostcrafters.com/pelicanctl/internal/errors"
)
```

### Naming Conventions
- **Packages**: lowercase, single word where possible (`config`, `api`, `bulk`)
- **Exported functions/types**: PascalCase (`NewClientAPI`, `Config`)
- **Unexported functions/types**: camelCase (`makeRequest`, `getConfigDir`)
- **Variables**: camelCase (`client`, `maxConcurrency`)
- **Package aliases**: Use short, clear aliases for imports (e.g., `apierrors`)

### Error Handling
- **Wrap errors with context**: Use `fmt.Errorf("context: %w", err)`
- **User-friendly messages**: Use `apierrors.HandleError(err)` for API responses
- **Never ignore errors**: Handle all errors explicitly
- **API error handling**: Wrap API responses before returning to users

```go
client, err := api.NewClientAPI()
if err != nil {
    return fmt.Errorf("failed to create client: %w", err)
}
servers, err := client.ListServers()
if err != nil {
    return fmt.Errorf(apierrors.HandleError(err))
}
```

### Struct Tags
- Use `mapstructure:"field_name"` for config structs (Viper)
- Use `json:"fieldName"` for API response structs
- Snake_case in tags, PascalCase in field names

```go
type Config struct {
    API    APIConfig    `mapstructure:"api"`
    Client ClientConfig `mapstructure:"client"`
}
```

### Concurrency
- Use `context.Context` for cancellable operations
- Use `sync.WaitGroup` for waiting on goroutines, `sync.Mutex` for shared state
- Semaphores (buffered channels) for limiting concurrency
- Capture loop variables: `uuid := uuid` (see `cmd/client/power.go`)

### CLI Commands (Cobra)
- Package: `cmd/subcommand/` (e.g., `cmd/client/`, `cmd/admin/`)
- Use `RunE` for error-returning command functions
- Extract reusable flag setup into functions (see `addBulkFlags` in `power.go`)
- Use `cobra.ExactArgs(n)` for strict argument validation

### API Layer
- Package: `internal/api/`
- Implement `NewClientAPI()` / `NewApplicationAPI()` constructors that wrap generated OpenAPI clients
- Generated clients: `internal/client/client.gen.go` and `internal/application/application.gen.go` (from `oapi-codegen`)
- Adapter layer (`internal/api/client_api.go` and `internal/api/application_api.go`) provides:
  - Unified interface matching existing command handlers
  - Response conversion from typed responses (`JSON200`, etc.) to `map[string]interface{}`
  - Wrapped response handling (e.g., `{"data": [...]}`)
  - Error conversion to `apierrors.APIError`
- Always `defer resp.Body.Close()` after making requests (handled by generated clients)
- Return `map[string]interface{}` for dynamic API responses (converted from generated types)

### Output Formatting
- Use `output.NewFormatter(format, writer)` for all output
- Support both JSON and table output
- Use colored terminal output for messages (`PrintSuccess`, `PrintError`, `PrintWarning`, `PrintInfo`)
- Format errors with `apierrors.HandleError()` before returning

### Authentication
- API tokens stored in Viper config (`config.Get()`)
- Retrieve tokens via `auth.GetToken("client")` or `auth.GetToken("admin")`
- Set tokens via `auth.SetToken(apiType, token)` for interactive login
- Authorization header format: `Bearer <token>`

### Bulk Operations
- Use `bulk.Executor` for parallel operations on multiple servers
- Semaphore pattern for concurrency limiting (see `internal/bulk/executor.go`)
- Support `--all`, `--from-file`, and positional arguments for server selection
- Flags: `--max-concurrency`, `--continue-on-error`, `--fail-fast`, `--dry-run`, `--yes`
- Return summary via `bulk.GetSummary()`

## Project Structure
```
cmd/              # CLI commands (pelicanctl/main.go, admin/, client/)
internal/         # Core logic
  api/            # Adapter layer wrapping generated clients (client_api.go, application_api.go)
  client/         # Generated Client API client (client.gen.go)
  application/    # Generated Application API client (application.gen.go)
  auth/           # Authentication helpers
  bulk/           # Parallel execution utilities
  config/         # Configuration management
  errors/         # Error types and handlers
  output/         # Formatted output utilities
openapi/          # OpenAPI specs, configs, and overlays
scripts/          # Build and fix scripts (fix-generated-params.sh)
```

## Testing
Currently no tests exist. When adding tests:
- Place test files alongside source: `config.go` → `config_test.go`
- Use table-driven tests for multiple cases; mock HTTP responses for API tests
- Run single test: `go test -run TestFunctionName ./path/to/package`

```go
func TestFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid input", "test", "result", false},
        {"invalid input", "", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Code Quality
- Run `just fmt` before committing
- Run `just lint` to catch issues with `golangci-lint`
- Generated code (`.gen.go` files) is automatically excluded from linting
- Follow Go conventions (Effective Go, CodeReviewComments)
- Never manually edit `.gen.go` files - they are regenerated from OpenAPI specs

## Configuration
- Config loaded in `PersistentPreRunE` via `config.Load()`
- Viper manages env vars: `PELICANCTL_CLIENT_TOKEN`, `PELICANCTL_ADMIN_TOKEN`, `PELICANCTL_API_BASE_URL`
- Config file: `~/.config/pelicanctl/config.yaml` (Linux/macOS), `%APPDATA%\pelicanctl\config.yaml` (Windows)
