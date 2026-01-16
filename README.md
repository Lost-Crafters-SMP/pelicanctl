# Pelican CLI

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
git clone https://github.com/yourusername/pelican-cli.git
cd pelican-cli

# Using just (recommended)
just build

# Or manually
go build -o pelican ./cmd/pelican
```

### Using Go Install

```bash
go install go.lostcrafters.com/pelican-cli/cmd/pelican@latest
```

## Configuration

The CLI supports multiple methods for configuration:

1. **Environment Variables** (highest priority) - For CI/CD environments
2. **System Keyring** - Secure token storage for developer machines (macOS Keychain, Linux Secret Service, Windows Credential Manager)
3. **Config File** (lowest priority) - Fallback storage (default: `~/.config/pelican/config.yaml` on Linux/macOS, `%APPDATA%\pelican\config.yaml` on Windows)
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

- `PELICAN_CLIENT_TOKEN` - Client API token
- `PELICAN_ADMIN_TOKEN` - Admin API token
- `PELICAN_API_BASE_URL` - API base URL

## Authentication

The CLI uses a priority system for token retrieval:

1. **Environment Variables** - `PELICAN_CLIENT_TOKEN` and `PELICAN_ADMIN_TOKEN` (for CI/CD)
2. **System Keyring** - Secure storage in macOS Keychain, Linux Secret Service, or Windows Credential Manager
3. **Config File** - Fallback storage (with security warnings)

### Interactive Login

```bash
# Login for Client API
pelican auth login client

# Login for Admin API
pelican auth login admin
```

This will prompt you for your API token and save it to the system keyring (or config file if keyring is unavailable).

### Logout

```bash
# Clear Client API token
pelican auth logout client

# Clear Admin API token
pelican auth logout admin
```

This removes the token from both the keyring and config file.

### Migrating from Config File to Keyring

If you have existing tokens in your config file, you'll see a warning when using the CLI:

```
⚠ Warning: Token found in config file. Consider migrating to system keyring for better security.
  Run 'pelican auth login <client|admin>' to migrate.
```

To migrate, simply run `pelican auth login <client|admin>` again. This will save the new token to the keyring and clear it from the config file.

### CI/CD Usage

For CI/CD environments, use environment variables:

```bash
export PELICAN_CLIENT_TOKEN="your-token"
pelican client server list
```

The keyring is not required for CI/CD environments.

## Usage

### Client API Commands

#### List Servers

```bash
pelican client server list
pelican client server list --output json
```

#### View Server Details

```bash
pelican client server view <uuid>
pelican client server resources <uuid>
```

#### Power Controls

```bash
# Single server
pelican client power start <uuid>
pelican client power stop <uuid>
pelican client power restart <uuid>
pelican client power kill <uuid>

# Multiple servers
pelican client power restart <uuid1> <uuid2> <uuid3>

# All servers
pelican client power restart --all

# From file (one UUID per line)
pelican client power restart --from-file servers.txt

# Bulk options
pelican client power restart --all --max-concurrency 5
pelican client power restart --all --continue-on-error
pelican client power restart --all --fail-fast
pelican client power restart --all --dry-run
pelican client power restart --all --yes  # Skip confirmation
```

#### File Management

```bash
# List files
pelican client file list <server-uuid> [directory]

# Download file
pelican client file download <server-uuid> <remote-path> [local-path]
```

#### Backups

```bash
# List backups
pelican client backup list <server-uuid>

# Create backup
pelican client backup create <server-uuid>
```

#### Databases

```bash
# List databases
pelican client database list <server-uuid>
```

### Admin API Commands

#### Nodes

```bash
pelican admin node list
pelican admin node view <node-id>
```

#### Servers

```bash
# List all servers
pelican admin server list

# View server
pelican admin server view <uuid>

# Suspend/Unsuspend
pelican admin server suspend <uuid>
pelican admin server unsuspend <uuid>

# Reinstall
pelican admin server reinstall <uuid>

# Bulk operations
pelican admin server suspend --all
pelican admin server reinstall <uuid1> <uuid2> --yes
```

#### Users

```bash
pelican admin user list
pelican admin user view <user-id>
```

## Global Flags

- `--config <path>` - Override config file path
- `--output json|table` - Output format (default: table)
- `--verbose` - Enable debug logging
- `--quiet` - Minimal output (errors only)

## Examples

```bash
# List all your servers in JSON format
pelican client server list --output json

# Restart all servers without confirmation
pelican client power restart --all --yes

# View server resources
pelican client server resources abc-123-def

# Create backups for multiple servers
pelican client backup create --from-file servers.txt --max-concurrency 3

# Admin: List all nodes
pelican admin node list

# Admin: Suspend servers matching criteria (requires --from-file)
pelican admin server suspend --from-file server-uuids.txt --dry-run
```

## Error Handling

The CLI provides user-friendly error messages:

- **401/403** - Suggests running `pelican auth login <client|admin>` to re-authenticate
- **404** - Clear "not found" messages
- **500+** - Indicates server issues
- **Bulk Operations** - Shows success/failure counts and details

### Expired Tokens

When tokens expire, you'll see an authentication error. Simply run:

```bash
pelican auth login <client|admin>
```

This will prompt for a new token and replace the expired one. You can also clear expired tokens with:

```bash
pelican auth logout <client|admin>
```

## Output Formats

### Table (Default)

Human-readable tables with colors and formatting.

### JSON

Machine-readable JSON output for scripting and automation.

```bash
pelican client server list --output json | jq '.[0].name'
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
