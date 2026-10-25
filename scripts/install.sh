#!/usr/bin/env bash
set -e

# EnvLock 1-line installer for Linux and macOS
REPO="itskdhere/envlock"
INSTALL_DIR="${ENVLOCK_INSTALL_DIR:-/usr/local/bin}"

echo "Installing EnvLock on $(uname -s)..."

# 1. Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux*)  OS="linux" ;;
  darwin*) OS="darwin" ;;
  *)
    echo "Error: Unsupported operating system: $OS" >&2
    exit 1
    ;;
esac

# 2. Detect Architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    echo "Error: Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

echo "Detected platform: ${OS}/${ARCH}"

# 3. Determine latest version
if [ -z "$VERSION" ]; then
  echo "Fetching latest release from GitHub..."
  # Try release redirect first to avoid GitHub API rate limits
  VERSION=$(curl -sI "https://github.com/${REPO}/releases/latest" 2>/dev/null | grep -i '^location:' | awk -F'/' '{print $NF}' | tr -d '\r\n' || true)
  if [ -z "$VERSION" ]; then
    # Fallback to API if redirect did not resolve
    VERSION=$(curl -sSL -H "User-Agent: EnvLock-Installer" "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)
  fi
  if [ -z "$VERSION" ]; then
    VERSION="v0.1.1"
  fi
fi

TAG="$VERSION"
CLEAN_VER="${VERSION#[vV]}"

FILENAME="envlock_${CLEAN_VER}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${TAG}/${FILENAME}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${TAG}/checksums.txt"

echo "Downloading EnvLock ${TAG} from ${DOWNLOAD_URL}..."

# 4. Download, verify checksum, and extract
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

CURL_PROGRESS="-s"
if [ -t 1 ]; then
  CURL_PROGRESS="--progress-bar"
fi

if ! curl -fL $CURL_PROGRESS -S "$DOWNLOAD_URL" -o "${TMP_DIR}/${FILENAME}"; then
  echo "Error: Failed to download release asset from ${DOWNLOAD_URL}" >&2
  exit 1
fi

# Verify SHA-256 Checksum
echo "Verifying SHA-256 checksum..."
if curl -fsSL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt" 2>/dev/null; then
  EXPECTED_SHA=$(grep -E "[[:space:]]${FILENAME}\$" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
  if [ -n "$EXPECTED_SHA" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
      ACTUAL_SHA=$(sha256sum "${TMP_DIR}/${FILENAME}" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
      ACTUAL_SHA=$(shasum -a 256 "${TMP_DIR}/${FILENAME}" | awk '{print $1}')
    else
      ACTUAL_SHA=""
    fi

    if [ -n "$ACTUAL_SHA" ]; then
      if [ "$EXPECTED_SHA" != "$ACTUAL_SHA" ]; then
        echo "Error: Checksum verification failed for ${FILENAME}!" >&2
        echo "  Expected: ${EXPECTED_SHA}" >&2
        echo "  Actual:   ${ACTUAL_SHA}" >&2
        exit 1
      fi
      echo "Checksum verified successfully (${ACTUAL_SHA:0:16}...)"
    fi
  fi
fi

tar -xzf "${TMP_DIR}/${FILENAME}" -C "$TMP_DIR"

# 5. Determine write permissions and install directory
USE_SUDO=""
if [ -w "$INSTALL_DIR" ] || ([ ! -e "$INSTALL_DIR" ] && mkdir -p "$INSTALL_DIR" 2>/dev/null); then
  USE_SUDO=""
elif command -v sudo >/dev/null 2>&1; then
  USE_SUDO="sudo"
  $USE_SUDO mkdir -p "$INSTALL_DIR"
else
  # Fallback to ~/.local/bin if /usr/local/bin isn't writable and no sudo
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

echo "Installing to ${INSTALL_DIR}..."
if command -v install >/dev/null 2>&1; then
  $USE_SUDO install -m 755 "${TMP_DIR}/envlock" "${INSTALL_DIR}/envlock"
else
  $USE_SUDO cp "${TMP_DIR}/envlock" "${INSTALL_DIR}/envlock"
  $USE_SUDO chmod 755 "${INSTALL_DIR}/envlock"
fi

# Create symlink for el -> envlock (with copy fallback if symlinks unsupported)
LINK_TYPE="symlink to envlock"
if ! $USE_SUDO ln -sf "envlock" "${INSTALL_DIR}/el" 2>/dev/null; then
  if command -v install >/dev/null 2>&1; then
    $USE_SUDO install -m 755 "${TMP_DIR}/el" "${INSTALL_DIR}/el"
  else
    $USE_SUDO cp "${TMP_DIR}/el" "${INSTALL_DIR}/el"
    $USE_SUDO chmod 755 "${INSTALL_DIR}/el"
  fi
  LINK_TYPE="copy"
fi

echo ""
echo "Successfully installed EnvLock into: ${INSTALL_DIR}"
echo "  • envlock -> ${INSTALL_DIR}/envlock"
echo "  • el      -> ${LINK_TYPE}"
echo ""

# 6. Check PATH
case ":$PATH:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    echo "Notice: Add ${INSTALL_DIR} to your PATH to run envlock from anywhere:"
    echo "  export PATH=\"\$PATH:${INSTALL_DIR}\""
    echo ""
    ;;
esac

"${INSTALL_DIR}/envlock" --version || true
