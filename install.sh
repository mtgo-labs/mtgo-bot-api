#!/bin/sh
# install.sh — install mtgo-bot-api from GitHub Releases
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | sh
#   wget -qO- https://raw.githubusercontent.com/mtgo-labs/mtgo-bot-api/main/install.sh | sh
#
# Options (set as env vars before piping):
#   INSTALL_DIR  destination directory             (default: ~/.local/bin)
#   VERSION      release tag to install            (default: latest)
#   REPO         github owner/repo                 (default: mtgo-labs/mtgo-bot-api)
#
# If no prebuilt release binary is available, the script falls back to
# `go install` (requires Go 1.26+ in PATH).

set -eu

REPO="${REPO:-mtgo-labs/mtgo-bot-api}"
VERSION="${VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-${HOME}/.local/bin}"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

info()  { printf '  \033[1;34m→\033[0m %s\n' "$*"; }
ok()    { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
err()   { printf '  \033[1;31m✗\033[0m %s\n' "$*" >&2; }

# ---------------------------------------------------------------------------
# Detect OS and architecture
# ---------------------------------------------------------------------------

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux*)         OS="linux" ;;
    darwin*)        OS="darwin" ;;
    *)              err "Unsupported OS: $(uname -s)"; exit 1 ;;
esac

case "$ARCH" in
    x86_64|amd64)   ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)              err "Unsupported architecture: $ARCH"; exit 1 ;;
esac

info "Detected: ${OS}/${ARCH}"

# ---------------------------------------------------------------------------
# Resolve version
# ---------------------------------------------------------------------------

if [ "$VERSION" = "latest" ]; then
    info "Resolving latest version…"
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep -m1 '"tag_name"' \
        | sed -E 's/.*"([^"]+)".*/\1/')"
    if [ -z "$VERSION" ]; then
        err "Could not resolve latest release."
        VERSION=""
    fi
fi

# ---------------------------------------------------------------------------
# Try downloading a prebuilt binary from GitHub Releases
# ---------------------------------------------------------------------------

BIN_NAME="mtgo-bot-api"

if [ -n "$VERSION" ]; then
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${BIN_NAME}_${OS}_${ARCH}"

    info "Downloading ${VERSION} from GitHub Releases…"
    TMPFILE="$(mktemp)"
    if curl -fSL "$URL" -o "$TMPFILE" 2>/dev/null; then
        mkdir -p "$INSTALL_DIR"
        chmod +x "$TMPFILE"
        mv "$TMPFILE" "${INSTALL_DIR}/${BIN_NAME}"
        ok "Installed ${BIN_NAME} ${VERSION} → ${INSTALL_DIR}/${BIN_NAME}"
        printf '\n'
        info "Add ${INSTALL_DIR} to your PATH if not already:"
        printf '    export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
        info "Run:"
        printf '    %s --version\n' "${INSTALL_DIR}/${BIN_NAME}"
        exit 0
    fi
    rm -f "$TMPFILE"
    err "No prebuilt binary for ${OS}/${ARCH} at ${VERSION}."
fi

# ---------------------------------------------------------------------------
# Fallback: go install
# ---------------------------------------------------------------------------

if command -v go >/dev/null 2>&1; then
    info "Falling back to 'go install'…"
    go install "github.com/${REPO}/cmd/mtgo-bot-api@latest"
    ok "Installed via go install."
    printf '\n'
    info "Ensure $(go env GOPATH)/bin is in your PATH:"
    printf '    export PATH="$(go env GOPATH)/bin:$PATH"\n\n'
    info "Run:"
    printf '    mtgo-bot-api --version\n'
    exit 0
fi

err "No prebuilt binary and Go is not installed."
err "Install Go 1.26+ from https://go.dev/dl/ then re-run this script."
exit 1
