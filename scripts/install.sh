#!/usr/bin/env sh
# Local Eml installer for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
#   curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh -s -- --version v0.0.1
#   sh install.sh --no-service --dir /usr/local/bin
#
# Flags:
#   --version <tag>   Specific release tag (default: latest)
#   --dir <path>      Install directory (default: ~/.local/bin)
#   --no-service      Skip the `local-eml install` service registration step
#   --port <n>        Port the service listens on (default: 7878; env LOCAL_EML_PORT)
#   -h | --help       Show this help

set -eu

REPO="hwhang0917/local-eml"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="latest"
RUN_SERVICE=1
PORT="${LOCAL_EML_PORT:-}"

usage() {
  sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)     VERSION="$2"; shift 2 ;;
    --version=*)   VERSION="${1#*=}"; shift ;;
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    --dir=*)       INSTALL_DIR="${1#*=}"; shift ;;
    --no-service)  RUN_SERVICE=0; shift ;;
    --port)        PORT="$2"; shift 2 ;;
    --port=*)      PORT="${1#*=}"; shift ;;
    -h|--help)     usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

OS=$(uname -s)
case "$OS" in
  Linux)  OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)    ARCH=amd64 ;;
  arm64|aarch64)   ARCH=arm64 ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

BINARY="local-eml-${OS}-${ARCH}"

if [ "$VERSION" = "latest" ]; then
  BASE="https://github.com/${REPO}/releases/latest/download"
else
  BASE="https://github.com/${REPO}/releases/download/${VERSION}"
fi
URL="${BASE}/${BINARY}"
SUMS_URL="${BASE}/SHA256SUMS"

printf 'Local Eml installer\n'
printf '  Target:  %s/%s\n' "$OS" "$ARCH"
printf '  Version: %s\n' "$VERSION"
printf '  URL:     %s\n' "$URL"
printf '  Dest:    %s/local-eml\n\n' "$INSTALL_DIR"

mkdir -p "$INSTALL_DIR"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

fetch() {
  url="$1"; dest="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$dest"
  else
    echo "curl or wget is required" >&2
    exit 1
  fi
}

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | awk '{print $1}'
  else
    echo "sha256sum or shasum is required" >&2
    exit 1
  fi
}

echo "Downloading binary..."
fetch "$URL"      "$TMP/local-eml"
fetch "$SUMS_URL" "$TMP/SHA256SUMS"

echo "Verifying checksum..."
EXPECTED=$(awk -v b="$BINARY" '$2==b {print $1}' "$TMP/SHA256SUMS")
if [ -z "$EXPECTED" ]; then
  echo "No checksum entry for $BINARY in SHA256SUMS" >&2
  exit 1
fi
ACTUAL=$(sha256 "$TMP/local-eml")
if [ "$EXPECTED" != "$ACTUAL" ]; then
  printf 'Checksum mismatch!\n  expected: %s\n  actual:   %s\n' "$EXPECTED" "$ACTUAL" >&2
  exit 1
fi

chmod +x "$TMP/local-eml"
mv "$TMP/local-eml" "$INSTALL_DIR/local-eml"
echo "Installed to $INSTALL_DIR/local-eml"

# Warn if not on PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    printf '\nNOTE: %s is not on your PATH. Add this to your shell rc:\n' "$INSTALL_DIR"
    printf '  export PATH="%s:$PATH"\n' "$INSTALL_DIR"
    ;;
esac

if [ "$RUN_SERVICE" = "1" ]; then
  echo
  set -- install
  [ -t 0 ] || set -- "$@" --yes
  [ -n "$PORT" ] && set -- "$@" --port "$PORT"
  "$INSTALL_DIR/local-eml" "$@"
fi
