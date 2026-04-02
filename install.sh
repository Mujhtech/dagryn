#!/bin/sh
set -e

# Dagryn installer script
# Usage: curl -fsSL https://raw.githubusercontent.com/Mujhtech/dagryn/main/install.sh | sh

REPO="Mujhtech/dagryn"
BINARY="dagryn"
INSTALL_DIR="/usr/local/bin"

# Detect OS and architecture (matches goreleaser naming)
detect_platform() {
    RAW_OS=$(uname -s)
    ARCH=$(uname -m)

    case "$RAW_OS" in
        Linux)  OS="Linux" ;;
        Darwin) OS="Darwin" ;;
        *)      echo "Error: Unsupported OS: $RAW_OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="x86_64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        i386|i686)     ARCH="i386" ;;
        *)             echo "Error: Unsupported architecture: $ARCH"; exit 1 ;;
    esac
}

# Get the latest release version from GitHub
get_latest_version() {
    if [ -n "$DAGRYN_VERSION" ]; then
        VERSION="$DAGRYN_VERSION"
        return
    fi
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Error: Could not determine latest version."
        echo "Tip: Set DAGRYN_VERSION=v0.x.x to install a specific version."
        exit 1
    fi
}

# Verify checksum if available
verify_checksum() {
    CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}_checksums.txt"
    CHECKSUMS_FILE="${TMPDIR}/checksums.txt"

    if curl -fsSL -o "$CHECKSUMS_FILE" "$CHECKSUMS_URL" 2>/dev/null; then
        echo "Verifying checksum..."
        EXPECTED=$(grep "$ARCHIVE" "$CHECKSUMS_FILE" | awk '{print $1}')
        if [ -n "$EXPECTED" ]; then
            if command -v sha256sum >/dev/null 2>&1; then
                ACTUAL=$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                ACTUAL=$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')
            else
                echo "Warning: No sha256sum or shasum found, skipping checksum verification."
                return
            fi
            if [ "$EXPECTED" != "$ACTUAL" ]; then
                echo "Error: Checksum mismatch!"
                echo "  Expected: $EXPECTED"
                echo "  Actual:   $ACTUAL"
                exit 1
            fi
            echo "Checksum verified."
        fi
    fi
}

# Download and install the binary
install() {
    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    ARCHIVE="${BINARY}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    echo "Downloading ${BINARY} ${VERSION} for ${OS}/${ARCH}..."
    if ! curl -fsSL -o "${TMPDIR}/${ARCHIVE}" "$URL"; then
        echo "Error: Failed to download ${URL}"
        echo "Check that the release exists: https://github.com/${REPO}/releases/tag/${VERSION}"
        exit 1
    fi

    verify_checksum

    echo "Extracting..."
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

    echo "Installing to ${INSTALL_DIR}/${BINARY}..."
    if [ -w "$INSTALL_DIR" ]; then
        mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    else
        echo "Elevated permissions required to install to ${INSTALL_DIR}"
        sudo mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    fi
    chmod +x "${INSTALL_DIR}/${BINARY}"

    echo ""
    echo "Successfully installed ${BINARY} ${VERSION}"
    echo "Run '${BINARY} --help' to get started."
}

detect_platform
get_latest_version
install
