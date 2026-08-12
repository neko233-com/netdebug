#!/bin/sh
set -eu

REPO="${NETDEBUG_REPO:-neko233-com/netdebug}"
VERSION="${NETDEBUG_VERSION:-latest}"
INSTALL_DIR="${NETDEBUG_INSTALL_DIR:-}"

say() { printf '%s\n' "$*"; }
die() { printf 'netdebug installer: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar >/dev/null 2>&1 || die "tar is required"

route_urls() {
  ORIGINAL_URL="$1"
  printf '%s\n' "$ORIGINAL_URL"
  [ "${NETDEBUG_DIRECT_ONLY:-0}" = "1" ] && return 0
  MIRRORS="${NETDEBUG_UPDATE_MIRRORS-https://gh-proxy.com/,https://ghfast.top/,https://ghproxy.net/}"
  OLD_IFS="$IFS"
  IFS=,
  for MIRROR in $MIRRORS; do
    [ -n "$MIRROR" ] || continue
    printf '%s\n' "${MIRROR%/}/$ORIGINAL_URL"
  done
  IFS="$OLD_IFS"
}

fastest_route() {
  TARGET_URL="$1"
  BEST_URL=""
  BEST_TIME="999999"
  while IFS= read -r CANDIDATE; do
    [ -n "$CANDIDATE" ] || continue
    PROBE="$(curl -fsSL -L --range 0-0 --max-time 5 -o /dev/null -w '%{http_code} %{time_total}' "$CANDIDATE" 2>/dev/null || true)"
    CODE="${PROBE%% *}"
    ELAPSED="${PROBE#* }"
    case "$CODE" in
      2*|3*)
        if awk "BEGIN { exit !($ELAPSED < $BEST_TIME) }"; then
          BEST_URL="$CANDIDATE"
          BEST_TIME="$ELAPSED"
        fi
        ;;
    esac
  done <<EOF
$(route_urls "$TARGET_URL")
EOF
  [ -n "$BEST_URL" ] || return 1
  printf '%s\n' "$BEST_URL"
}

OS="$(uname -s 2>/dev/null || true)"
ARCH="$(uname -m 2>/dev/null || true)"
case "$OS" in
  Linux) OS_NAME=linux ;;
  Darwin) OS_NAME=darwin ;;
  *) die "unsupported OS: $OS (use install.ps1 on Windows)" ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH_NAME=amd64 ;;
  aarch64|arm64) ARCH_NAME=arm64 ;;
  *) die "unsupported architecture: $ARCH" ;;
esac

if [ -z "$INSTALL_DIR" ]; then
  if [ "$(id -u)" -eq 0 ] || [ -w /usr/local/bin ]; then
    INSTALL_DIR=/usr/local/bin
  else
    INSTALL_DIR="${HOME:-}/.local/bin"
  fi
fi
[ -n "$INSTALL_DIR" ] || die "cannot determine install directory"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t netdebug)"
cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT HUP INT TERM

if [ "$VERSION" = latest ]; then
  API_URL="https://api.github.com/repos/$REPO/releases/latest"
  API_ROUTE="$(fastest_route "$API_URL")" || die "no reachable GitHub metadata route"
  VERSION="$(curl -fsSL --max-time 15 "$API_ROUTE" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
fi
case "$VERSION" in
  v*) ;;
  *) VERSION="v$VERSION" ;;
esac
VERSION_NUMBER="${VERSION#v}"
ASSET="netdebug_${VERSION_NUMBER}_${OS_NAME}_${ARCH_NAME}.tar.gz"
BASE_URL="https://github.com/$REPO/releases/download/$VERSION"

say "Installing netdebug $VERSION ($OS_NAME/$ARCH_NAME)"
ASSET_ROUTE="$(fastest_route "$BASE_URL/$ASSET")" || die "no reachable release asset route: $ASSET"
ROUTE_BASE="${ASSET_ROUTE%/$ASSET}"
curl -fsSL --max-time 120 "$ROUTE_BASE/$ASSET" -o "$TMP_DIR/$ASSET" || die "release asset not found: $ASSET"
curl -fsSL --max-time 30 "$ROUTE_BASE/checksums.txt" -o "$TMP_DIR/checksums.txt" || die "checksum file not found"

EXPECTED="$(awk -v file="$ASSET" '$2 == file { print $1; exit }' "$TMP_DIR/checksums.txt")"
[ -n "$EXPECTED" ] || die "checksum missing for $ASSET"
ACTUAL="$(sha256sum "$TMP_DIR/$ASSET" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}')"
[ "$EXPECTED" = "$ACTUAL" ] || die "checksum verification failed"

tar -xzf "$TMP_DIR/$ASSET" -C "$TMP_DIR"
BIN="$TMP_DIR/netdebug"
[ -f "$BIN" ] || die "release archive has no netdebug binary"
mkdir -p "$INSTALL_DIR"
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$BIN" "$INSTALL_DIR/netdebug"
elif command -v sudo >/dev/null 2>&1; then
  sudo install -m 0755 "$BIN" "$INSTALL_DIR/netdebug"
else
  die "install directory is not writable: $INSTALL_DIR"
fi

say "Installed: $INSTALL_DIR/netdebug"

if [ -z "${NETDEBUG_PROFILE_FILE:-}" ]; then
  if [ "$(id -u)" -eq 0 ] && [ -d /etc/profile.d ] && [ -w /etc/profile.d ]; then
    NETDEBUG_PROFILE_FILE=/etc/profile.d/netdebug.sh
  else
    NETDEBUG_PROFILE_FILE="${HOME:-}/.profile"
  fi
fi
[ -n "$NETDEBUG_PROFILE_FILE" ] || die "cannot determine shell profile"
PROFILE_MARKER="# netdebug installer managed environment"
PROFILE_BLOCK="$(printf '%s\n' \
  "$PROFILE_MARKER" \
  "export NETDEBUG_HOME=$(printf '%s' "$INSTALL_DIR" | sed \"s/'/'\\\\''/g\")" \
  'case ":${PATH:-}:" in' \
  "  *:$INSTALL_DIR:*) ;;" \
  "  *) export PATH=\"$INSTALL_DIR:\$PATH\" ;;" \
  'esac')"

if [ ! -f "$NETDEBUG_PROFILE_FILE" ] || ! grep -Fq "$PROFILE_MARKER" "$NETDEBUG_PROFILE_FILE"; then
  PROFILE_DIR="$(dirname "$NETDEBUG_PROFILE_FILE")"
  if [ -d "$PROFILE_DIR" ] && [ -w "$PROFILE_DIR" ]; then
    mkdir -p "$PROFILE_DIR"
    printf '\n%s\n' "$PROFILE_BLOCK" >> "$NETDEBUG_PROFILE_FILE"
  elif command -v sudo >/dev/null 2>&1; then
    printf '\n%s\n' "$PROFILE_BLOCK" | sudo tee -a "$NETDEBUG_PROFILE_FILE" >/dev/null
  else
    die "cannot update shell profile: $NETDEBUG_PROFILE_FILE"
  fi
fi

# Activate installer process and make subsequent login shells global/idempotent.
export NETDEBUG_HOME="$INSTALL_DIR"
case ":${PATH:-}:" in
  *":$INSTALL_DIR:"*) ;;
  *) export PATH="$INSTALL_DIR:${PATH:-}" ;;
esac
say "Environment registered: NETDEBUG_HOME=$INSTALL_DIR"
say "New shells load: $NETDEBUG_PROFILE_FILE"
