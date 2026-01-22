#!/bin/bash
set -e

# Install script for pelicanctl
# Usage: curl -fsSL https://raw.githubusercontent.com/Lost-Crafters-SMP/pelicanctl/main/scripts/install.sh | sh

REPO="Lost-Crafters-SMP/pelicanctl"
BINARY_NAME="pelicanctl"
INSTALL_DIR="${HOME}/.local/bin"

# Determine OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

# Normalize architecture names
case "${ARCH}" in
  x86_64|amd64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

# Map OS names
case "${OS}" in
  linux)
    OS_NAME="Linux"
    BINARY_EXT=""
    ;;
  darwin)
    OS_NAME="Darwin"
    BINARY_EXT=""
    ;;
  *)
    echo "Unsupported OS: ${OS}"
    exit 1
    ;;
esac

# Determine version (use latest if not specified)
VERSION="${VERSION:-latest}"
if [ "${VERSION}" = "latest" ]; then
  VERSION_URL="https://api.github.com/repos/${REPO}/releases/latest"
  VERSION=$(curl -s "${VERSION_URL}" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
fi

# Construct archive filename based on goreleaser naming
ARCH_NAME="x86_64"
if [ "${ARCH}" = "arm64" ]; then
  ARCH_NAME="arm64"
fi

ARCHIVE_FILENAME="${BINARY_NAME}_${OS_NAME}_${ARCH_NAME}.tar.gz"

# Construct download URLs
if [ "${VERSION}" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/${ARCHIVE_FILENAME}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/latest/download/checksums.txt"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_FILENAME}"
  CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"
fi

echo "Installing ${BINARY_NAME} ${VERSION} for ${OS,,}/${ARCH}..."

# Create install directory if it doesn't exist
mkdir -p "${INSTALL_DIR}"

# Create temporary directory for extraction
TMP_DIR=$(mktemp -d)
trap "rm -rf ${TMP_DIR}" EXIT

# Download archive
TMP_ARCHIVE="${TMP_DIR}/archive.tar.gz"
echo "Downloading from ${DOWNLOAD_URL}..."
curl -fsSL -o "${TMP_ARCHIVE}" "${DOWNLOAD_URL}"

# Download and verify checksum
TMP_CHECKSUM="${TMP_DIR}/checksums.txt"
echo "Verifying checksum..."
curl -fsSL -o "${TMP_CHECKSUM}" "${CHECKSUM_URL}"

# Verify checksum
EXPECTED_CHECKSUM=$(grep "${ARCHIVE_FILENAME}" "${TMP_CHECKSUM}" | awk '{print $1}')
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL_CHECKSUM=$(sha256sum "${TMP_ARCHIVE}" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL_CHECKSUM=$(shasum -a 256 "${TMP_ARCHIVE}" | awk '{print $1}')
else
  echo "Warning: Could not verify checksum (sha256sum or shasum not found)"
  ACTUAL_CHECKSUM=""
fi

if [ -n "${ACTUAL_CHECKSUM}" ] && [ "${EXPECTED_CHECKSUM}" != "${ACTUAL_CHECKSUM}" ]; then
  echo "Error: Checksum verification failed!"
  echo "Expected: ${EXPECTED_CHECKSUM}"
  echo "Actual: ${ACTUAL_CHECKSUM}"
  exit 1
fi

# Extract archive
echo "Extracting archive..."
tar -xzf "${TMP_ARCHIVE}" -C "${TMP_DIR}"

# Find binary in extracted files
EXTRACTED_BINARY=$(find "${TMP_DIR}" -name "${BINARY_NAME}" -type f | head -n 1)

if [ -z "${EXTRACTED_BINARY}" ]; then
  echo "Error: Binary not found in archive"
  exit 1
fi

# Make binary executable
chmod +x "${EXTRACTED_BINARY}"

# Install binary
INSTALL_PATH="${INSTALL_DIR}/${BINARY_NAME}"
mv "${EXTRACTED_BINARY}" "${INSTALL_PATH}"

echo "Successfully installed ${BINARY_NAME} to ${INSTALL_PATH}"

# Check if install directory is in PATH
if ! echo "${PATH}" | grep -q "${INSTALL_DIR}"; then
  echo ""
  echo "⚠️  Warning: ${INSTALL_DIR} is not in your PATH"
  echo "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
  echo "  export PATH=\"\${HOME}/.local/bin:\${PATH}\""
fi

echo ""
echo "Run '${BINARY_NAME} --version' to verify installation."
