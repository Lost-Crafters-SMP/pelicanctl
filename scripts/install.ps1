# Install script for pelicanctl (Windows PowerShell)
# Usage: irm https://raw.githubusercontent.com/Lost-Crafters-SMP/pelicanctl/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$Repo = "Lost-Crafters-SMP/pelicanctl"
$BinaryName = "pelicanctl"
$InstallDir = "$env:LOCALAPPDATA\pelicanctl\bin"

# Determine architecture
$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default {
        # Fallback: check environment variable
        if ($env:PROCESSOR_ARCHITEW6432 -eq "AMD64") {
            "amd64"
        } else {
            Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
            exit 1
        }
    }
}

# Determine version (use latest if not specified)
$Version = if ($env:VERSION) { $env:VERSION } else { "latest" }
if ($Version -eq "latest") {
    $VersionUrl = "https://api.github.com/repos/$Repo/releases/latest"
    $Response = Invoke-RestMethod -Uri $VersionUrl
    $Version = $Response.tag_name
}

# Construct archive filename based on goreleaser naming
$ArchName = if ($Arch -eq "amd64") { "x86_64" } else { "arm64" }
$ArchiveFilename = "${BinaryName}_Windows_${ArchName}.zip"

# Construct download URLs
if ($Version -eq "latest") {
    $DownloadUrl = "https://github.com/$Repo/releases/latest/download/$ArchiveFilename"
    $ChecksumUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"
} else {
    $DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$ArchiveFilename"
    $ChecksumUrl = "https://github.com/$Repo/releases/download/$Version/checksums.txt"
}

Write-Host "Installing $BinaryName $Version for windows/$Arch..." -ForegroundColor Green

# Create install directory if it doesn't exist
if (-not (Test-Path -Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

# Create temporary directory for extraction
$TmpDir = Join-Path $env:TEMP "${BinaryName}_install_$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir -Force | Out-Null

try {
    # Download archive
    $TmpArchive = Join-Path $TmpDir "archive.zip"
    Write-Host "Downloading from $DownloadUrl..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $TmpArchive -UseBasicParsing

    # Download and verify checksum
    $TmpChecksum = Join-Path $TmpDir "checksums.txt"
    Write-Host "Verifying checksum..." -ForegroundColor Yellow
    Invoke-WebRequest -Uri $ChecksumUrl -OutFile $TmpChecksum -UseBasicParsing

    # Verify checksum
    $ExpectedChecksum = (Select-String -Path $TmpChecksum -Pattern $ArchiveFilename).Line.Split()[0]
    $ActualChecksum = (Get-FileHash -Path $TmpArchive -Algorithm SHA256).Hash.ToLower()

    if ($ExpectedChecksum -ne $ActualChecksum) {
        Write-Error "Checksum verification failed!"
        Write-Host "Expected: $ExpectedChecksum" -ForegroundColor Red
        Write-Host "Actual: $ActualChecksum" -ForegroundColor Red
        exit 1
    }

    # Extract archive
    Write-Host "Extracting archive..." -ForegroundColor Yellow
    Expand-Archive -Path $TmpArchive -DestinationPath $TmpDir -Force

    # Find binary in extracted files
    $ExtractedBinary = Get-ChildItem -Path $TmpDir -Filter "${BinaryName}.exe" -Recurse | Select-Object -First 1

    if (-not $ExtractedBinary) {
        Write-Error "Binary not found in archive"
        exit 1
    }

    # Install binary
    $InstallPath = Join-Path $InstallDir "${BinaryName}.exe"
    Move-Item -Path $ExtractedBinary.FullName -Destination $InstallPath -Force
} finally {
    # Cleanup
    Remove-Item -Path $TmpDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host "Successfully installed $BinaryName to $InstallPath" -ForegroundColor Green

# Check if install directory is in PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host ""
    Write-Host "Warning: $InstallDir is not in your PATH" -ForegroundColor Yellow
    Write-Host "Would you like to add it? (Y/N): " -NoNewline -ForegroundColor Yellow
    $Response = Read-Host
    if ($Response -eq "Y" -or $Response -eq "y") {
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Added $InstallDir to PATH. Please restart your terminal." -ForegroundColor Green
    } else {
        Write-Host "To add it manually, run:" -ForegroundColor Yellow
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ';$InstallDir', 'User')" -ForegroundColor Gray
    }
}

Write-Host ""
Write-Host "Run '$BinaryName --version' to verify installation." -ForegroundColor Green
