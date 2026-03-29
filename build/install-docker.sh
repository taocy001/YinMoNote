#!/bin/bash
# build/install-docker.sh [version]
#
# Interactive installer: loads the Docker image tar, prompts for notes directory,
# host port, and access mode, then starts the container service.
# Non-sensitive settings (NOTES_DIR, HOST_PORT, ACCESS_MODE) are persisted to
# ~/.yinmonote/docker.env so they are used as defaults on future upgrades.
# config.json is always at ~/.yinmonote/config.json (not configurable).
#
# Prerequisite: run make docker (or build/package-docker.sh) first.
#
# Usage:
#   ./build/install-docker.sh          # latest tar in dist/
#   ./build/install-docker.sh 1.2.0    # specific version

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUT_DIR="$PROJECT_ROOT/dist"
ENV_FILE="$HOME/.yinmonote/docker.env"
YINMONOTE_DIR="$HOME/.yinmonote"   # config.json is always here
DEFAULT_NOTES="$HOME/.yinmonote/notes"
DEFAULT_PORT="8080"

# ── Locate tar file ───────────────────────────────────────────────────────────
if [ -n "$1" ]; then
    TAR="$(ls -t "$OUT_DIR"/yinmonote-"$1"-docker-*.tar 2>/dev/null | head -1)"
else
    TAR="$(ls -t "$OUT_DIR"/yinmonote-*-docker-*.tar 2>/dev/null | head -1)"
fi

if [ -z "$TAR" ] || [ ! -f "$TAR" ]; then
    echo "Error: image tar file not found in dist/"
    echo "Please run first: make docker"
    exit 1
fi

# ── Read existing config as defaults when upgrading ──────────────────────────
EXISTING_NOTES=""
EXISTING_PORT=""
EXISTING_MODE=""
EXISTING_TLS_MODE=""
EXISTING_ACME_DOMAIN=""
EXISTING_EXTRA_IPS=""
EXISTING_WEBDAV_DISABLED=""

if [ -f "$ENV_FILE" ]; then
    EXISTING_NOTES=$(grep           '^NOTES_DIR='        "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_PORT=$(grep            '^HOST_PORT='        "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_MODE=$(grep            '^ACCESS_MODE='      "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_BIND=$(grep            '^HOST_BIND='        "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_TLS_MODE=$(grep        '^TLS_MODE='         "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_ACME_DOMAIN=$(grep     '^ACME_DOMAIN='      "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
    EXISTING_EXTRA_IPS=$(grep       '^TLS_EXTRA_IPS='    "$ENV_FILE" 2>/dev/null | cut -d= -f2- || true)
    EXISTING_WEBDAV_DISABLED=$(grep '^WEBDAV_DISABLED='  "$ENV_FILE" 2>/dev/null | cut -d= -f2  || true)
fi

# ── Detect existing container ─────────────────────────────────────────────────
IS_UPGRADE=false
EXISTING_STATUS=""

if docker ps -a --filter "name=^yinmonote$" --format "{{.Names}}" 2>/dev/null | grep -q "^yinmonote$"; then
    IS_UPGRADE=true
    if docker ps --filter "name=^yinmonote$" --format "{{.Names}}" 2>/dev/null | grep -q "^yinmonote$"; then
        EXISTING_STATUS="running"
    else
        EXISTING_STATUS="stopped"
    fi
fi

echo ""
echo "════════════════════════════════════════"
if [ "$IS_UPGRADE" = true ]; then
    echo "  YinMoNote Docker Upgrade"
else
    echo "  YinMoNote Docker Install"
fi
echo "  Image: $(basename "$TAR")"
echo "════════════════════════════════════════"

if [ "$IS_UPGRADE" = true ]; then
    echo ""
    echo "  Found existing container: yinmonote ($EXISTING_STATUS)"
    [ -n "$EXISTING_NOTES" ] && echo "    Notes:  $EXISTING_NOTES"
    [ -n "$EXISTING_PORT"  ] && echo "    Port:   $EXISTING_PORT"
    echo ""
    echo "  The container will be replaced with the new image."
    echo "  Your notes and config are not affected."
    echo ""
    printf "  Continue? [Y/n]: "
    read -r _CONFIRM
    case "$_CONFIRM" in
        [nN]*) echo "  Aborted."; exit 0 ;;
    esac
fi

PROMPT_NOTES="${EXISTING_NOTES:-$DEFAULT_NOTES}"
PROMPT_PORT="${EXISTING_PORT:-$DEFAULT_PORT}"

# ── Prompts ───────────────────────────────────────────────────────────────────
# Validate that the stored default is an absolute path; fall back to the
# built-in default if it looks like a previous accidental entry (e.g. "y").
case "$PROMPT_NOTES" in /*) ;; *) PROMPT_NOTES="$DEFAULT_NOTES" ;; esac

echo ""
while true; do
    printf "  Notes directory [%s]: " "$PROMPT_NOTES"
    read -r INPUT_NOTES
    NOTES_DIR="${INPUT_NOTES:-$PROMPT_NOTES}"
    case "$NOTES_DIR" in
        /*) break ;;
        *)  echo "  Error: please enter an absolute path (must start with /)" ;;
    esac
done

printf "  Host port [%s]: " "$PROMPT_PORT"
read -r INPUT_PORT
HOST_PORT="${INPUT_PORT:-$PROMPT_PORT}"
HOST_PORT="${HOST_PORT#:}"   # strip leading colon if present

echo ""
echo "  Who will access this instance?"
echo "    1) Localhost only — personal use on this machine"
echo "    2) LAN or public server — other devices / users access over the network"
if [ "$EXISTING_MODE" = "network" ]; then
    printf "  Choice [1/2] (default: 2): "
else
    printf "  Choice [1/2] (default: 1): "
fi
read -r INPUT_MODE
case "${INPUT_MODE:-${EXISTING_MODE:-1}}" in
    2|network) ACCESS_MODE="network"; HOST_BIND="0.0.0.0:" ;;
    *)         ACCESS_MODE="local";   HOST_BIND="127.0.0.1:" ;;
esac

# ── HTTPS ─────────────────────────────────────────────────────────────────────
echo ""
echo "── HTTPS ────────────────────────────────────────────"
echo "  1) No HTTPS — HTTP only (suitable for local/trusted network)"
echo "  2) Self-signed certificate — HTTPS without a domain name"
echo "     (download CA cert once per device from /ca.crt)"
echo "  3) Let's Encrypt — automatic HTTPS with a domain name"
echo "     (requires a valid domain pointing to this server)"

case "$EXISTING_TLS_MODE" in
    self) _TLS_DEFAULT=2 ;;
    acme) _TLS_DEFAULT=3 ;;
    *)    _TLS_DEFAULT=1 ;;
esac
printf "  Choice [%s]: " "$_TLS_DEFAULT"
read -r _TLS_CHOICE
_TLS_CHOICE="${_TLS_CHOICE:-$_TLS_DEFAULT}"

TLS_SELF=""
ACME_DOMAIN=""
TLS_EXTRA_IPS=""
case "$_TLS_CHOICE" in
    2)
        TLS_SELF="1"
        echo ""
        echo "  The certificate will include all IP addresses currently assigned to"
        echo "  this machine. If your public IP is on an upstream NAT gateway (common"
        echo "  on cloud VPS), it won't be detected automatically — enter it here."
        _EXTRA_PROMPT="${EXISTING_EXTRA_IPS:-}"
        if [ -n "$_EXTRA_PROMPT" ]; then
            printf "  Public/extra IPs, comma-separated [%s]: " "$_EXTRA_PROMPT"
        else
            printf "  Public/extra IPs (leave blank if not needed): "
        fi
        read -r _EXTRA_INPUT
        TLS_EXTRA_IPS="${_EXTRA_INPUT:-$_EXTRA_PROMPT}"
        TLS_MODE="self"
        ;;
    3)
        _ACME_PROMPT="${EXISTING_ACME_DOMAIN:-}"
        if [ -n "$_ACME_PROMPT" ]; then
            printf "  Domain name [%s]: " "$_ACME_PROMPT"
        else
            printf "  Domain name: "
        fi
        read -r _ACME_INPUT
        ACME_DOMAIN="${_ACME_INPUT:-$_ACME_PROMPT}"
        if [ -z "$ACME_DOMAIN" ]; then
            echo "  Error: domain name is required for Let's Encrypt. Falling back to HTTP."
        fi
        TLS_MODE="acme"
        ;;
    *) TLS_MODE="" ;;
esac

# ── WebDAV ────────────────────────────────────────────────────────────────────
echo ""
echo "── WebDAV ───────────────────────────────────────────"
echo "  Allows mobile apps (Obsidian, iA Writer, etc.) to access notes"
echo "  at /dav/ using the same password as the web app."

_DAV_DEFAULT="Y"
[ "$EXISTING_WEBDAV_DISABLED" = "1" ] && _DAV_DEFAULT="N"
printf "  Enable WebDAV? [%s]: " "$_DAV_DEFAULT"
read -r _DAV_INPUT
_DAV_INPUT="${_DAV_INPUT:-$_DAV_DEFAULT}"

WEBDAV_DISABLED=""
case "$_DAV_INPUT" in
    [nN]*) WEBDAV_DISABLED="1" ;;
esac

# ── Summary ───────────────────────────────────────────────────────────────────
echo ""
echo "  ── Configuration summary ─────────────────────────"
echo "  Image:   $(basename "$TAR")"
echo "  Config:  $YINMONOTE_DIR/config.json  (fixed)"
echo "  Notes:   $NOTES_DIR"
echo "  Port:    $HOST_PORT"
if [ "$ACCESS_MODE" = "network" ]; then
    echo "  Access:  LAN / public server"
else
    echo "  Access:  Localhost only"
fi
if   [ -n "$ACME_DOMAIN" ]; then
    echo "  HTTPS:   Let's Encrypt ($ACME_DOMAIN)"
elif [ "$TLS_SELF" = "1" ]; then
    [ -n "$TLS_EXTRA_IPS" ] \
        && echo "  HTTPS:   Self-signed certificate (extra IPs: $TLS_EXTRA_IPS)" \
        || echo "  HTTPS:   Self-signed certificate"
else
    echo "  HTTPS:   Disabled (HTTP only)"
fi
if [ "$WEBDAV_DISABLED" = "1" ]; then
    echo "  WebDAV:  Disabled"
else
    echo "  WebDAV:  Enabled (/dav/)"
fi
echo ""

# ── Persist config for future upgrades ───────────────────────────────────────
mkdir -p "$YINMONOTE_DIR"
cat > "$ENV_FILE" << EOF
NOTES_DIR=$NOTES_DIR
HOST_PORT=$HOST_PORT
HOST_BIND=$HOST_BIND
ACCESS_MODE=$ACCESS_MODE
TLS_MODE=$TLS_MODE
ACME_DOMAIN=$ACME_DOMAIN
TLS_EXTRA_IPS=$TLS_EXTRA_IPS
WEBDAV_DISABLED=$WEBDAV_DISABLED
EOF

# ── Create notes directory ────────────────────────────────────────────────────
mkdir -p "$NOTES_DIR"

# ── Stop and remove existing container ───────────────────────────────────────
docker rm -f yinmonote 2>/dev/null || true

# ── Load image ────────────────────────────────────────────────────────────────
echo "==> Loading image..."
docker load -i "$TAR"
# docker-compose.yml uses image: yinmonote (= yinmonote:latest).
# The tar contains yinmonote:<version>; re-tag so compose can find it.
TAR_VERSION=$(basename "$TAR" | sed 's/yinmonote-\(.*\)-docker-.*/\1/')
docker tag "yinmonote:$TAR_VERSION" yinmonote 2>/dev/null || true
docker image prune -f >/dev/null 2>&1 || true

# ── Start service ─────────────────────────────────────────────────────────────
echo "==> Starting service..."
YINMONOTE_DIR="$YINMONOTE_DIR" NOTES_DIR="$NOTES_DIR" \
    HOST_PORT="$HOST_PORT" HOST_BIND="$HOST_BIND" \
    TLS_SELF="$TLS_SELF" ACME_DOMAIN="$ACME_DOMAIN" \
    TLS_EXTRA_IPS="$TLS_EXTRA_IPS" WEBDAV_DISABLED="$WEBDAV_DISABLED" \
    docker compose -f "$PROJECT_ROOT/build/docker-compose.yml" \
    --project-directory "$PROJECT_ROOT" up -d --no-build

echo ""
echo "════════════════════════════════════════"
if [ "$IS_UPGRADE" = true ]; then
    echo "  ✓ Upgraded successfully — service restarted"
else
    echo "  ✓ Installed successfully — service started"
fi
echo ""
echo "  Notes:   $NOTES_DIR"
echo "  Config:  $YINMONOTE_DIR/config.json"
echo ""

# Determine base URL
_PORT_NUM="$HOST_PORT"
if   [ -n "$ACME_DOMAIN" ]; then
    _BASE_URL="https://$ACME_DOMAIN"
elif [ "$TLS_SELF" = "1" ]; then
    [ "$ACCESS_MODE" = "network" ] \
        && _BASE_URL="https://<server-ip>:${_PORT_NUM}" \
        || _BASE_URL="https://localhost:${_PORT_NUM}"
else
    [ "$ACCESS_MODE" = "network" ] \
        && _BASE_URL="http://<server-ip>:${_PORT_NUM}" \
        || _BASE_URL="http://localhost:${_PORT_NUM}"
fi

echo "  Open:    $_BASE_URL"
if [ "$TLS_SELF" = "1" ]; then
    echo ""
    echo "  HTTPS (self-signed TLS) — install the CA cert once per device:"
    echo "    $YINMONOTE_DIR/selfca/ca.crt"
    echo "  Remote devices:  $_BASE_URL/ca.crt"
fi
if [ "$WEBDAV_DISABLED" != "1" ]; then
    echo "  WebDAV:  ${_BASE_URL}/dav/"
fi
if [ "$ACCESS_MODE" = "network" ] && [ -z "$TLS_SELF" ] && [ -z "$ACME_DOMAIN" ]; then
    echo ""
    echo "  Note: running HTTP on a public server — consider enabling TLS."
    echo "        Re-run make install-docker to configure."
fi

echo "════════════════════════════════════════"
