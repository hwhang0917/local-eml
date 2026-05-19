#!/usr/bin/env sh
# Local Eml uninstaller for Linux and macOS.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh
#   sh uninstall.sh --purge --dir /usr/local/bin
#
# Flags:
#   --dir <path>      Install directory (default: ~/.local/bin)
#   --keep-binary     Unregister the service but leave the binary in place
#   --purge           Also delete ~/.local-eml/ (EMLs, database, logs)
#   -h | --help       Show this help

set -eu

INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
KEEP_BINARY=0
PURGE=0

usage() {
  sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
}

while [ $# -gt 0 ]; do
  case "$1" in
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    --dir=*)       INSTALL_DIR="${1#*=}"; shift ;;
    --keep-binary) KEEP_BINARY=1; shift ;;
    --purge)       PURGE=1; shift ;;
    -h|--help)     usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done

BIN="$INSTALL_DIR/local-eml"
if [ ! -x "$BIN" ] && command -v local-eml >/dev/null 2>&1; then
  BIN=$(command -v local-eml)
fi

printf 'Local Eml uninstaller\n'
printf '  Binary:  %s\n' "$BIN"
printf '  Purge:   %s\n' "$([ "$PURGE" = "1" ] && echo "yes (~/.local-eml/ will be deleted)" || echo "no (data preserved)")"
printf '\n'

# Unregister the system service (best-effort if the binary still exists).
if [ -x "$BIN" ]; then
  if [ -t 0 ]; then
    "$BIN" uninstall || echo "(service unregister returned non-zero; continuing)"
  else
    "$BIN" uninstall --yes || echo "(service unregister returned non-zero; continuing)"
  fi
fi

# Remove binary.
if [ "$KEEP_BINARY" = "0" ] && [ -e "$BIN" ]; then
  rm -f "$BIN"
  echo "Removed $BIN"
fi

# Purge data.
if [ "$PURGE" = "1" ]; then
  DATA="$HOME/.local-eml"
  if [ -d "$DATA" ]; then
    rm -rf "$DATA"
    echo "Removed $DATA"
  fi
fi

echo
echo "Done."
