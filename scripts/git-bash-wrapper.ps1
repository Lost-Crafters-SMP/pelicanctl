# Git Bash wrapper script
# Finds Git installation directory and calls Git Bash's bash.exe
# This ensures we use Git Bash, not WSL bash

$gitExe = (Get-Command git -ErrorAction SilentlyContinue).Source
if (-not $gitExe) {
    Write-Error "git not found in PATH"
    exit 1
}

# Get Git installation directory (two levels up from git.exe)
$gitDir = Split-Path (Split-Path $gitExe -Parent) -Parent
$bashExe = Join-Path $gitDir "bin\bash.exe"

if (-not (Test-Path $bashExe)) {
    Write-Error "Git Bash not found at: $bashExe"
    exit 1
}

# Execute bash.exe with the provided arguments
# Join all arguments into a single string for bash -c
$command = $args -join ' '
& $bashExe -c $command
