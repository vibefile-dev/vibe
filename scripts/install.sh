#!/bin/sh
set -e

# ─────────────────────────────────────────────
# Vibefile installer
#
# Downloads the latest vibe binary from GitHub
# and installs it to /usr/local/bin by default.
#
# Usage:
#   curl -fsSL https://get.vibefile.dev | sh
#
# Want a specific version? Set VIBE_VERSION:
#   VIBE_VERSION=v0.0.3 ./install.sh
#
# Want to install somewhere else? Set VIBE_INSTALL_DIR:
#   VIBE_INSTALL_DIR=~/.local/bin ./install.sh
# ─────────────────────────────────────────────

REPO="vibefile-dev/vibe"
BINARY_NAME="vibe"
DEFAULT_INSTALL_DIR="/usr/local/bin"

#It's an exit handler or trap that ensures idempotency by cleaning up state regardless of the termination signal
cleanup() { [ -n "$TMP_DIR" ] && rm -rf "$TMP_DIR"; }
trap cleanup EXIT

# ── Pretty output helpers
# These just print colored lines so the output is easy to read.

info()  { printf "  \033[34m[info]\033[0m  %s\n" "$1"; }
ok()    { printf "  \033[32m[ok]\033[0m    %s\n" "$1"; }
err()   { printf "  \033[31m[error]\033[0m %s\n" "$1" >&2; exit 1; }

# ── Figure out what OS we're on
# We only support Linux and macOS for now.
# The OS name is used to build the download filename later.

detect_os() {
    OS="$(uname -s)"
    case "$OS" in
        Linux)  OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)      err "Unsupported OS: $OS. Only Linux and macOS are supported." ;;
    esac
}

# ── Figure out the CPU architecture
# Normalise the various names people use for the same thing
# so it matches what GoReleaser puts in the filename.
# e.g. x86_64 and amd64 are the same thing — we always use amd64.

detect_arch() {
    ARCH="$(uname -m)"
    case "$ARCH" in
        x86_64 | amd64)   ARCH="amd64" ;;
        arm64  | aarch64) ARCH="arm64" ;;
        *)                err "Unsupported architecture: $ARCH." ;;
    esac
}

# ── Work out which version to install
# If the user set VIBE_VERSION we use that.
# Otherwise we ask the GitHub API what the latest release is.

get_latest_version() {
    if [ -n "$VIBE_VERSION" ]; then
        VERSION="$VIBE_VERSION"
        info "Using pinned version: $VERSION"
        return
    fi

    info "Fetching latest version from GitHub..."

    # We send a User-Agent header because GitHub's API returns 403 without one.
    if command -v curl > /dev/null 2>&1; then
        VERSION="$(curl -fsSL -H "User-Agent: vibe-installer" \
            "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' \
            | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
    elif command -v wget > /dev/null 2>&1; then
        VERSION="$(wget -qO- --header="User-Agent: vibe-installer" \
            "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' \
            | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
    else
        err "Neither curl nor wget found. Please install one and try again."
    fi

    # If VERSION is still empty something went wrong (rate limit, no internet, etc.)
    if [ -z "$VERSION" ]; then
        err "Could not determine latest version. Try setting VIBE_VERSION manually and retry."
    fi

    info "Latest version: $VERSION"
}

# ── Download the archive and install the binary
# The release filenames look like: vibe_0.0.3_linux_amd64.tar.gz
# Note: the tag is v0.0.3 but the filename uses 0.0.3 (no leading v).

install_vibe() {
    INSTALL_DIR="${VIBE_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

    # Strip the leading 'v' from the version tag for the filename.
    # e.g. v0.0.3 → 0.0.3
    VERSION_NUM="${VERSION#v}"

    ARCHIVE="${BINARY_NAME}_${VERSION_NUM}_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE}"

    # We extract into a temp folder so we never leave a half-installed binary lying around.
    TMP_DIR="$(mktemp -d)"

    info "Downloading $ARCHIVE..."

    # 2>/dev/null silences curl/wget's own error output — we handle errors ourselves below.
    if command -v curl > /dev/null 2>&1; then
        curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$ARCHIVE" 2>/dev/null || {
            info "Check available releases at: https://github.com/${REPO}/releases"
            err "Download failed. Version $VERSION may not exist or has no binary for $OS/$ARCH."
        }
    elif command -v wget > /dev/null 2>&1; then
        wget -qO "$TMP_DIR/$ARCHIVE" "$DOWNLOAD_URL" 2>/dev/null || {
            info "Check available releases at: https://github.com/${REPO}/releases"
            err "Download failed. Version $VERSION may not exist or has no binary for $OS/$ARCH."
        }
    fi

    info "Extracting..."
    tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR"

    # Make sure the binary actually ended up in the archive.
    BINARY_PATH="$TMP_DIR/$BINARY_NAME"
    if [ ! -f "$BINARY_PATH" ]; then
        err "Binary not found after extracting the archive. This is unexpected — please open an issue."
    fi

    chmod +x "$BINARY_PATH"

    info "Installing to $INSTALL_DIR..."

    # Try to copy without sudo first.
    # If that fails (no write permission) we retry with sudo.
    if mkdir -p "$INSTALL_DIR" 2>/dev/null && cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null; then
        ok "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME"
    else
        info "Permission denied — retrying with sudo..."
        sudo mkdir -p "$INSTALL_DIR"
        sudo cp "$BINARY_PATH" "$INSTALL_DIR/$BINARY_NAME"
        ok "Installed $BINARY_NAME to $INSTALL_DIR/$BINARY_NAME (via sudo)"
    fi

}

# ── Check the binary is actually reachable after install
# If the install directory isn't in PATH we tell the user how to fix that.

verify_install() {
    INSTALL_DIR="${VIBE_INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"
    if ! command -v "$BINARY_NAME" > /dev/null 2>&1; then
        info "Heads up: $INSTALL_DIR is not in your PATH."
        info "Add this line to your shell profile (~/.bashrc, ~/.zshrc, etc.) and restart your terminal:"
        info "  export PATH=\"\$PATH:$INSTALL_DIR\""
    else
        ok "$BINARY_NAME is ready: $(vibe --version 2>/dev/null || echo "$VERSION")"
    fi
}

# ── Let's go

main() {
    echo ""
    echo "  Vibefile installer"
    echo "  ──────────────────"
    echo ""

    detect_os
    detect_arch
    info "Detected: $OS / $ARCH"

    get_latest_version
    install_vibe
    verify_install

    echo ""
    ok "Done! Run 'vibe --help' to get started."
    echo ""
}

main
