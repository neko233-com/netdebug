#!/bin/sh
set -eu

REPO="${NETDEBUG_REPO:-neko233-com/netdebug}"
VERSION="${NETDEBUG_VERSION:-latest}"
INSTALL_DIR="${NETDEBUG_INSTALL_DIR:-}"
RUN_AFTER_INSTALL=0

say() { printf '%s\n' "$*"; }
die() { printf 'netdebug installer: %s\n' "$*" >&2; exit 1; }

while [ "$#" -gt 0 ]; do
  case "$1" in
    --run|-y) RUN_AFTER_INSTALL=1 ;;
    --help|-h)
      say "Usage: install.sh [--run|-y]"
      say "  --run, -y  run netdebug immediately after installation"
      exit 0
      ;;
    *) die "unknown argument: $1" ;;
  esac
  shift
done

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
    PROBE="$(curl -fsSL -L --range 0-65535 --max-time 8 -o /dev/null -w '%{http_code} %{time_total}' "$CANDIDATE" 2>/dev/null || true)"
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
SELECTED_BASE="${ASSET_ROUTE%/$ASSET}"
ROUTE_ORDER="$(printf '%s\n' "$SELECTED_BASE"; route_urls "$BASE_URL" | sed "s#/$ASSET\$##" | grep -Fv -x "$SELECTED_BASE" || true)"
RELEASE_READY=0
while IFS= read -r ROUTE_BASE; do
  [ -n "$ROUTE_BASE" ] || continue
  rm -f "$TMP_DIR/$ASSET" "$TMP_DIR/checksums.txt"
  if ! curl -fsSL --max-time 120 "$ROUTE_BASE/$ASSET" -o "$TMP_DIR/$ASSET" 2>/dev/null; then
    continue
  fi
  if ! curl -fsSL --max-time 30 "$ROUTE_BASE/checksums.txt" -o "$TMP_DIR/checksums.txt" 2>/dev/null; then
    continue
  fi
  EXPECTED="$(awk -v file="$ASSET" '$2 == file { print $1; exit }' "$TMP_DIR/checksums.txt")"
  [ -n "$EXPECTED" ] || continue
  ACTUAL="$(sha256sum "$TMP_DIR/$ASSET" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$TMP_DIR/$ASSET" | awk '{print $1}')"
  if [ "$EXPECTED" = "$ACTUAL" ]; then
    RELEASE_READY=1
    break
  fi
done <<EOF
$ROUTE_ORDER
EOF
[ "$RELEASE_READY" -eq 1 ] || die "could not download and verify release asset: $ASSET"

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
INSTALL_DIR_QUOTED="$(printf '%s' "$INSTALL_DIR" | sed "s/'/'\\\\''/g; 1s/^/'/; \$s/\$/\'/")"
PROFILE_BLOCK="$(printf '%s\n' \
  "$PROFILE_MARKER" \
  "export NETDEBUG_HOME=$INSTALL_DIR_QUOTED" \
  'case ":${PATH:-}:" in' \
  "  *:\"$INSTALL_DIR\":*) ;;" \
  "  *) export PATH=\"$INSTALL_DIR:\$PATH\" ;;" \
  'esac')"

PROFILE_DIR="$(dirname "$NETDEBUG_PROFILE_FILE")"
PROFILE_EXPECTED="export NETDEBUG_HOME=$INSTALL_DIR_QUOTED"
if [ -f "$NETDEBUG_PROFILE_FILE" ] && grep -Fq "$PROFILE_MARKER" "$NETDEBUG_PROFILE_FILE" && grep -Fq "$PROFILE_EXPECTED" "$NETDEBUG_PROFILE_FILE"; then
  :
elif [ -d "$PROFILE_DIR" ] && [ -w "$PROFILE_DIR" ]; then
  mkdir -p "$PROFILE_DIR"
  if [ -f "$NETDEBUG_PROFILE_FILE" ] && grep -Fq "$PROFILE_MARKER" "$NETDEBUG_PROFILE_FILE"; then
    PROFILE_TMP="$(mktemp "$PROFILE_DIR/.netdebug-profile.XXXXXX")"
    awk -v marker="$PROFILE_MARKER" 'BEGIN { skip=0 } $0 == marker { skip=1; next } skip && $0 == "esac" { skip=0; next } !skip { print }' "$NETDEBUG_PROFILE_FILE" > "$PROFILE_TMP"
    printf '\n%s\n' "$PROFILE_BLOCK" >> "$PROFILE_TMP"
    mv "$PROFILE_TMP" "$NETDEBUG_PROFILE_FILE"
  else
    printf '\n%s\n' "$PROFILE_BLOCK" >> "$NETDEBUG_PROFILE_FILE"
  fi
elif command -v sudo >/dev/null 2>&1; then
  printf '\n%s\n' "$PROFILE_BLOCK" | sudo tee -a "$NETDEBUG_PROFILE_FILE" >/dev/null
else
  die "cannot update shell profile: $NETDEBUG_PROFILE_FILE"
fi

# Activate installer process and make subsequent login shells global/idempotent.
export NETDEBUG_HOME="$INSTALL_DIR"
case ":${PATH:-}:" in
  *":$INSTALL_DIR:"*) ;;
  *) export PATH="$INSTALL_DIR:${PATH:-}" ;;
esac
say "Environment registered: NETDEBUG_HOME=$INSTALL_DIR"
say "New shells load: $NETDEBUG_PROFILE_FILE"

if [ "$RUN_AFTER_INSTALL" -eq 1 ]; then
  say "Running netdebug report"
  "$INSTALL_DIR/netdebug"
fi
