set unstable := true

# Pelican CLI Justfile
# Command runner for development tasks
# Infer Git home directory from git executable location
# git.exe is typically at <git_home>/cmd/git.exe or <git_home>/bin/git.exe
# Going up two levels from git.exe gives us the Git installation root
# Note: Computed at parse time, so git must be in PATH when justfile is loaded

git_home := parent_directory(parent_directory(require("git.exe")))

# Set shell for Windows (other platforms use default shell)
# Uses a PowerShell wrapper script to find and call Git Bash's bash.exe
# This ensures we use Git Bash (not WSL bash) by computing the path dynamically
# Note: Path is relative to the justfile directory

set windows-shell := ["powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", "scripts/git-bash-wrapper.ps1"]

# Go parameters

binary_name := "pelican"

# Add .exe extension when targeting Windows (check GOOS first for cross-compilation, fallback to current OS)

target_os := env_var_or_default('GOOS', if os() == 'windows' { 'windows' } else { '' })
exe_suffix := if target_os == 'windows' { '.exe' } else { '' }

# Directories

internal_dir := "internal"
client_dir := internal_dir / "client"
application_dir := internal_dir / "application"

# OpenAPI specs

client_spec := "openapi/client.json"
application_spec := "openapi/application.json"

# Default recipe - show available recipes
default:
    @just --list

# Generate all code
generate: generate-client generate-application

# Generate Client API client

# Note: Tool is managed via `go get -tool` which adds it to go.mod, then called via `go tool`
generate-client:
    @echo "Generating Client API client..."
    @go tool oapi-codegen -config openapi/client-config.yaml {{ client_spec }}
    @echo "Fixing duplicate parameter names..."
    @bash scripts/fix-generated-params.sh {{ client_dir }}/client.gen.go

# Generate Application API client

# Note: Tool added with `go get -tool` can be executed via `go tool`
generate-application:
    @echo "Generating Application API client..."
    @go tool oapi-codegen -config openapi/application-config.yaml {{ application_spec }}
    @echo "Fixing duplicate parameter names..."
    @bash scripts/fix-generated-params.sh {{ application_dir }}/application.gen.go

# Build the binary (without code generation - using manual API clients)

# Supports cross-compilation via GOOS/GOARCH environment variables
build:
    @echo "Building {{ binary_name }}..."
    @go build -o bin/{{ binary_name }}{{ exe_suffix }} ./cmd/pelican

# Install the binary
install:
    @echo "Installing {{ binary_name }}..."
    @go install ./cmd/pelican

# Clean generated files
clean:
    @echo "Cleaning..."
    @rm -rf bin/
    @rm -f {{ client_dir }}/*.gen.go
    @rm -f {{ application_dir }}/*.gen.go
    @rm -f openapi/*-3.0.json

# Downgrade OpenAPI 3.1 specs to 3.0 (temporary files for comparison)
downgrade-client:
    @echo "Downgrading client.json from 3.1 to 3.0..."
    @npx --yes @apiture/openapi-down-convert@latest -i {{ client_spec }} -o openapi/client-3.0.json

downgrade-application:
    @echo "Downgrading application.json from 3.1 to 3.0..."
    @npx --yes @apiture/openapi-down-convert@latest -i {{ application_spec }} -o openapi/application-3.0.json

# Generate overlays by comparing 3.1 vs 3.0 specs
# Note: Uses Go tool (installed via go get -tool) for overlay generation
# spec1 is the original (3.1), spec2 is the downgraded (3.0) - overlay transforms spec1 to spec2

# Note: Temporary 3.0 files are cleaned up by the `clean` recipe
generate-client-overlay: downgrade-client
    @echo "Generating client overlay from differences..."
    @go tool openapi overlay compare {{ client_spec }} openapi/client-3.0.json > openapi/client-overlay.yaml

generate-application-overlay: downgrade-application
    @echo "Generating application overlay from differences..."
    @go tool openapi overlay compare {{ application_spec }} openapi/application-3.0.json > openapi/application-overlay.yaml

# Generate all overlays automatically from spec differences
generate-overlays: generate-client-overlay generate-application-overlay
    @echo "All overlays generated successfully!"

# Run go mod tidy
tidy:
    @go mod tidy

# Format code
fmt:
    @go fmt ./...

# Run tests
test:
    @go test ./...

# Run tests with coverage
test-coverage:
    @go test -coverprofile=coverage.out ./...
    @go tool cover -html=coverage.out -o coverage.html

# Lint code with golangci-lint
# Note: Tool is managed via `go get -tool` which adds it to go.mod, then called via `go tool`
lint:
    @go tool golangci-lint run

# Lint code with auto-fix
lint-fix:
    @go tool golangci-lint run --fix

# Run go vet (simpler, faster check)
vet:
    @go vet ./...

# Show help
help:
    @just --list
