# pelicanctlctl

A command-line tool for managing servers on the Pelican panel (Pterodactyl replacement).

## Features

- **Client API** - Manage your servers as an end-user
- **Application API** - Admin operations for managing the panel
- **Bulk Operations** - Execute operations on multiple servers in parallel
- **Secure Authentication** - System keyring support (macOS Keychain, Linux Secret Service, Windows Credential Manager) with config file fallback
- **Flexible Authentication** - Environment variables (CI/CD), keyring (developer), and config file fallback
- **Cross-Platform** - Works on Linux, macOS, and Windows
- **JSON & Table Output** - Choose your preferred output format

## Installation

### From Source

```bash
git clone https://github.com/yourusername/pelicanctlctl.git
cd pelicanctlctl

# Using just (recommended)
just build

# Or manually
go build -o pelicanctlctl ./cmd/pelicanctlctl
```

### Using Go Install

```bash
go install go.lostcrafters.com/pelicanctlctl/cmd/pelicanctlctl@latest
```

## Configuration

The CLI supports multiple methods for configuration:

1. **Environment Variables** (highest priority) - For CI/CD environments
2. **System Keyring** - Secure token storage for developer machines (macOS Keychain, Linux Secret Service, Windows Credential Manager)
3. **Config File** (lowest priority) - Fallback storage (default: `~/.config/pelicanctlctl/config.yaml` on Linux/macOS, `%APPDATA%\pelicanctlctl\config.yaml` on Windows)
4. **Interactive Login** - Prompt for tokens and save to keyring

### Config File Format

```yaml
api:
  base_url: https://your-panel-url.com
client:
  token: your-client-api-token
admin:
  token: your-admin-api-token
```

### Environment Variables

- `PELICANCTL_CLIENT_TOKEN` - Client API token
- `PELICANCTL_ADMIN_TOKEN` - Admin API token
- `PELICANCTL_API_BASE_URL` - API base URL

## Authentication

The CLI uses a priority system for token retrieval:

1. **Environment Variables** - `PELICANCTL_CLIENT_TOKEN` and `PELICANCTL_ADMIN_TOKEN` (for CI/CD)
2. **System Keyring** - Secure storage in macOS Keychain, Linux Secret Service, or Windows Credential Manager
3. **Config File** - Fallback storage (with security warnings)

### Interactive Login

```bash
# Login for Client API
pelicanctlctl auth login client

# Login for Admin API
pelicanctlctl auth login admin
```

This will prompt you for your API token and save it to the system keyring (or config file if keyring is unavailable).

### Logout

```bash
# Clear Client API token
pelicanctlctl auth logout client

# Clear Admin API token
pelicanctlctl auth logout admin
```

This removes the token from both the keyring and config file.

### Migrating from Config File to Keyring

If you have existing tokens in your config file, you'll see a warning when using the CLI:

```
⚠ Warning: Token found in config file. Consider migrating to system keyring for better security.
  Run 'pelicanctlctl auth login <client|admin>' to migrate.
```

To migrate, simply run `pelicanctlctl auth login <client|admin>` again. This will save the new token to the keyring and clear it from the config file.

### CI/CD Usage

For CI/CD environments, use environment variables:

```bash
export PELICANCTL_CLIENT_TOKEN="your-token"
pelicanctlctl client server list
```

The keyring is not required for CI/CD environments.

## Usage

### Client API Commands

#### List Servers

```bash
pelicanctl client server list
pelicanctl client server list --output json
```

#### View Server Details

```bash
pelicanctl client server view <uuid>
pelicanctl client server resources <uuid>
```

#### Power Controls

```bash
# Single server
pelicanctl client power start <uuid>
pelicanctl client power stop <uuid>
pelicanctl client power restart <uuid>
pelicanctl client power kill <uuid>

# Multiple servers
pelicanctl client power restart <uuid1> <uuid2> <uuid3>

# All servers
pelicanctl client power restart --all

# From file (one UUID per line)
pelicanctl client power restart --from-file servers.txt

# Bulk options
pelicanctl client power restart --all --max-concurrency 5
pelicanctl client power restart --all --continue-on-error
pelicanctl client power restart --all --fail-fast
pelicanctl client power restart --all --dry-run
pelicanctl client power restart --all --yes  # Skip confirmation
```

#### File Management

```bash
# List files
pelicanctl client file list <server-uuid> [directory]

# Download file
pelicanctl client file download <server-uuid> <remote-path> [local-path]
```

#### Backups

```bash
# List backups
pelicanctl client backup list <server-uuid>

# Create backup
pelicanctl client backup create <server-uuid>
```

#### Databases

```bash
# List databases
pelicanctl client database list <server-uuid>
```

### Admin API Commands

#### Nodes

```bash
pelicanctl admin node list
pelicanctl admin node view <node-id>
```

#### Servers

```bash
# List all servers
pelicanctl admin server list

# View server
pelicanctl admin server view <uuid>

# Suspend/Unsuspend
pelicanctl admin server suspend <uuid>
pelicanctl admin server unsuspend <uuid>

# Reinstall
pelicanctl admin server reinstall <uuid>

# Bulk operations
pelicanctl admin server suspend --all
pelicanctl admin server reinstall <uuid1> <uuid2> --yes
```

#### Users

```bash
pelicanctl admin user list
pelicanctl admin user view <user-id>
```

## Global Flags

- `--config <path>` - Override config file path
- `--output json|table` - Output format (default: table)
- `--verbose` - Enable debug logging
- `--quiet` - Minimal output (errors only)

## Examples

```bash
# List all your servers in JSON format
pelicanctl client server list --output json

# Restart all servers without confirmation
pelicanctl client power restart --all --yes

# View server resources
pelicanctl client server resources abc-123-def

# Create backups for multiple servers
pelicanctl client backup create --from-file servers.txt --max-concurrency 3

# Admin: List all nodes
pelicanctl admin node list

# Admin: Suspend servers matching criteria (requires --from-file)
pelicanctl admin server suspend --from-file server-uuids.txt --dry-run
```

## Error Handling

The CLI provides user-friendly error messages:

- **401/403** - Suggests running `pelicanctl auth login <client|admin>` to re-authenticate
- **404** - Clear "not found" messages
- **500+** - Indicates server issues
- **Bulk Operations** - Shows success/failure counts and details

### Expired Tokens

When tokens expire, you'll see an authentication error. Simply run:

```bash
pelicanctl auth login <client|admin>
```

This will prompt for a new token and replace the expired one. You can also clear expired tokens with:

```bash
pelicanctl auth logout <client|admin>
```

## Output Formats

### Table (Default)

Human-readable tables with colors and formatting.

### JSON

Machine-readable JSON output for scripting and automation.

```bash
pelicanctl client server list --output json | jq '.[0].name'
```

## Development

### Prerequisites

- Go 1.21 or later
- `just` command runner ([install here](https://just.systems/man/en/chapter_1.html))

### Available Commands

```bash
just --list              # Show all available recipes
just build               # Build the binary
just generate            # Generate code from OpenAPI specs
just test                # Run tests
just fmt                 # Format code
just lint                # Lint code
just tidy                # Run go mod tidy
just clean               # Clean generated files
```

### Code Generation

The project uses `go run` to execute generators instead of requiring pre-installed binaries:

```bash
just generate-client      # Generate Client API types
just generate-application # Generate Application API types
just generate             # Generate all code
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

[Your License Here]
