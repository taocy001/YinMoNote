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

if [ -f "$ENV_FILE" ]; then
    EXISTING_NOTES=$(grep '^NOTES_DIR='   "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
    EXISTING_PORT=$(grep  '^HOST_PORT='   "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
    EXISTING_MODE=$(grep  '^ACCESS_MODE=' "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
    EXISTING_BIND=$(grep  '^HOST_BIND='   "$ENV_FILE" 2>/dev/null | cut -d= -f2 || true)
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
echo ""

# ── Persist non-sensitive config for future upgrades ─────────────────────────
mkdir -p "$YINMONOTE_DIR"
cat > "$ENV_FILE" << EOF
NOTES_DIR=$NOTES_DIR
HOST_PORT=$HOST_PORT
HOST_BIND=$HOST_BIND
ACCESS_MODE=$ACCESS_MODE
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
YINMONOTE_DIR="$YINMONOTE_DIR" NOTES_DIR="$NOTES_DIR" HOST_PORT="$HOST_PORT" HOST_BIND="$HOST_BIND" \
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

if [ "$ACCESS_MODE" = "network" ]; then
    echo "  Open:    http://<server-ip>:$HOST_PORT"
    echo ""
    echo "  ── Security checklist for network access ──────"
    echo "  1. Set an access password: Settings → Security"
    echo "     (PBKDF2-derived; only a hash is stored on disk)"
    echo ""
    echo "  2. Enable TLS (required for public internet):"
    echo "       Auto certificate (needs a domain):"
    echo "         ACME_DOMAIN=your.domain.com"
    echo "       Self-signed certificate (LAN / intranet):"
    echo "         TLS_SELF=1"
    echo "     Add the variable to build/docker-compose.yml,"
    echo "     then run: make install-docker"
    echo ""
    echo "  3. Restrict firewall to trusted sources on port $HOST_PORT"
else
    echo "  Open:    http://localhost:$HOST_PORT"
    echo ""
    echo "  Tip: to share access over the network, re-run the"
    echo "       installer and choose option 2."
fi

echo "════════════════════════════════════════"
