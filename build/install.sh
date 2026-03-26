#!/bin/bash
# build/install.sh <binary>
#
# Interactive installer: installs the binary, prompts for data directory and
# port, then configures and starts the background service.
#
# macOS : sets up a LaunchAgent (~/.yinmonote/ default, auto-start at login)
# Linux : installs a systemd user service

set -e

BINARY="${1:?Usage: $0 <binary-path>}"

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found: $BINARY"
    exit 1
fi

BIN_DIR="/usr/local/bin"
YINMONOTE_DIR="$HOME/.yinmonote"   # config.json is always here
DEFAULT_NOTES="$HOME/.yinmonote/notes"
DEFAULT_PORT=":8080"

# ── Detect existing installation and read config as defaults ──────────────────
IS_UPGRADE=false
EXISTING_DATA=""
EXISTING_PORT=""
EXISTING_STATUS=""
EXISTING_TLS_MODE=""   # "self" | "acme" | ""
EXISTING_ACME_DOMAIN=""
EXISTING_EXTRA_IPS=""
EXISTING_WEBDAV_DISABLED=""

if [ "$(uname -s)" = "Darwin" ]; then
    PLIST="$HOME/Library/LaunchAgents/com.yinmonote.plist"
    if [ -f "$PLIST" ]; then
        IS_UPGRADE=true
        EXISTING_DATA=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:DATA_DIR" "$PLIST" 2>/dev/null || true)
        EXISTING_PORT=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:PORT" "$PLIST" 2>/dev/null || true)
        _TLS_SELF=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:TLS_SELF" "$PLIST" 2>/dev/null || true)
        EXISTING_ACME_DOMAIN=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:ACME_DOMAIN" "$PLIST" 2>/dev/null || true)
        EXISTING_EXTRA_IPS=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:TLS_EXTRA_IPS" "$PLIST" 2>/dev/null || true)
        EXISTING_WEBDAV_DISABLED=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:WEBDAV_DISABLED" "$PLIST" 2>/dev/null || true)
        [ "$_TLS_SELF" = "1" ] && EXISTING_TLS_MODE="self"
        [ -n "$EXISTING_ACME_DOMAIN" ] && EXISTING_TLS_MODE="acme"
        if launchctl list 2>/dev/null | grep -q "com.yinmonote"; then
            EXISTING_STATUS="running"
        else
            EXISTING_STATUS="stopped"
        fi
    fi
else
    SERVICE="$HOME/.config/systemd/user/yinmonote.service"
    if [ -f "$SERVICE" ]; then
        IS_UPGRADE=true
        EXISTING_DATA=$(grep '^Environment=DATA_DIR=' "$SERVICE" 2>/dev/null | cut -d= -f3 || true)
        EXISTING_PORT=$(grep '^Environment=PORT=' "$SERVICE" 2>/dev/null | cut -d= -f3 || true)
        _TLS_SELF=$(grep '^Environment=TLS_SELF=' "$SERVICE" 2>/dev/null | cut -d= -f3 || true)
        EXISTING_ACME_DOMAIN=$(grep '^Environment=ACME_DOMAIN=' "$SERVICE" 2>/dev/null | cut -d= -f3 || true)
        EXISTING_EXTRA_IPS=$(grep '^Environment=TLS_EXTRA_IPS=' "$SERVICE" 2>/dev/null | cut -d= -f3- || true)
        EXISTING_WEBDAV_DISABLED=$(grep '^Environment=WEBDAV_DISABLED=' "$SERVICE" 2>/dev/null | cut -d= -f3 || true)
        [ "$_TLS_SELF" = "1" ] && EXISTING_TLS_MODE="self"
        [ -n "$EXISTING_ACME_DOMAIN" ] && EXISTING_TLS_MODE="acme"
        if systemctl --user is-active yinmonote >/dev/null 2>&1; then
            EXISTING_STATUS="running"
        else
            EXISTING_STATUS="stopped"
        fi
    fi
fi

echo ""
echo "════════════════════════════════════════"
if [ "$IS_UPGRADE" = true ]; then
    echo "  YinMoNote Upgrade"
else
    echo "  YinMoNote Install"
fi
echo "════════════════════════════════════════"

if [ "$IS_UPGRADE" = true ]; then
    echo ""
    echo "  Found existing installation:"
    echo "    Status:  $EXISTING_STATUS"
    [ -n "$EXISTING_DATA"  ] && echo "    Notes:   $EXISTING_DATA"
    [ -n "$EXISTING_PORT"  ] && echo "    Port:    $EXISTING_PORT"
    echo ""
    echo "  The binary will be replaced and the service restarted."
    echo "  Your notes and config are not affected."
    echo ""
    printf "  Continue? [Y/n]: "
    read -r _CONFIRM
    case "$_CONFIRM" in
        [nN]*) echo "  Aborted."; exit 0 ;;
    esac
fi

PROMPT_DATA="${EXISTING_DATA:-$DEFAULT_NOTES}"
# Extract bare port number from existing PORT which may be ":8080",
# "127.0.0.1:8080", or "localhost:8080".  The access mode prefix is
# handled separately by the access-mode prompt below.
_RAW_PORT="${EXISTING_PORT:-$DEFAULT_PORT}"
_RAW_PORT="${_RAW_PORT##*:}"          # keep only the part after the last colon
PROMPT_PORT="${_RAW_PORT:-8080}"

# ── Prompts ───────────────────────────────────────────────────────────────────
# Validate that the stored default is an absolute path; fall back to the
# built-in default if it looks like a previous accidental entry (e.g. "y").
case "$PROMPT_DATA" in /*) ;; *) PROMPT_DATA="$DEFAULT_NOTES" ;; esac

while true; do
    printf "Notes directory [%s]: " "$PROMPT_DATA"
    read -r INPUT_DATA
    DATA_DIR="${INPUT_DATA:-$PROMPT_DATA}"
    case "$DATA_DIR" in
        /*) break ;;
        *)  echo "  Error: please enter an absolute path (must start with /)" ;;
    esac
done

printf "Port [%s]: " "$PROMPT_PORT"
read -r INPUT_PORT
PORT="${INPUT_PORT:-$PROMPT_PORT}"
PORT="${PORT##*:}"   # extract bare port number (strips any address prefix or leading colon)

# ── Access mode ──────────────────────────────────────────────────────────────
echo ""
echo "── Access Mode ──────────────────────────────────────"
echo "  1) Localhost only — personal use on this machine"
echo "  2) LAN or public server — other devices access over the network"
# Infer existing mode from PORT: 127.0.0.1 prefix → local, else network
_EXISTING_ACCESS=""
case "$EXISTING_PORT" in localhost:*|127.0.0.1:*) _EXISTING_ACCESS="local" ;; :?*) _EXISTING_ACCESS="network" ;; "") ;; *) _EXISTING_ACCESS="network" ;; esac
if [ "$_EXISTING_ACCESS" = "network" ]; then
    printf "  Choice [1/2] (default: 2): "
else
    printf "  Choice [1/2] (default: 1): "
fi
read -r _ACCESS_INPUT
case "${_ACCESS_INPUT:-${_EXISTING_ACCESS:-1}}" in
    2|network) PORT=":$PORT" ;;           # 0.0.0.0:PORT — all interfaces
    *)         PORT="localhost:$PORT" ;;   # loopback only (IPv4+IPv6)
esac

# ── HTTPS ─────────────────────────────────────────────────────────────────────
echo ""
echo "── HTTPS ────────────────────────────────────────────"
echo "  1) No HTTPS — HTTP only (suitable for local/trusted network)"
echo "  2) Self-signed certificate — HTTPS without a domain name"
echo "     (download CA cert once per device from /ca.crt)"
echo "  3) Let's Encrypt — automatic HTTPS with a domain name"
echo "     (requires a valid domain pointing to this server)"

# Default selection from existing installation
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
        ;;
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

# config.json is always at ~/.yinmonote/config.json regardless of notes location
CONFIG_FILE="$YINMONOTE_DIR/config.json"
LOG_DIR="$YINMONOTE_DIR/logs"

echo ""
echo "  Binary:    $BIN_DIR/yinmonote"
echo "  Notes:     $DATA_DIR"
echo "  Config:    $CONFIG_FILE"
echo "  Port:      $PORT"
case "$PORT" in localhost:*|127.0.0.1:*) echo "  Access:    Localhost only" ;; *) echo "  Access:    LAN / public server" ;; esac
if   [ -n "$ACME_DOMAIN" ]; then
    echo "  HTTPS:     Let's Encrypt ($ACME_DOMAIN)"
elif [ "$TLS_SELF" = "1" ]; then
    if [ -n "$TLS_EXTRA_IPS" ]; then
        echo "  HTTPS:     Self-signed certificate (extra IPs: $TLS_EXTRA_IPS)"
    else
        echo "  HTTPS:     Self-signed certificate"
    fi
else
    echo "  HTTPS:     Disabled (HTTP only)"
fi
if [ "$WEBDAV_DISABLED" = "1" ]; then
    echo "  WebDAV:    Disabled"
else
    echo "  WebDAV:    Enabled (/dav/)"
fi
echo ""

# ── Install binary ────────────────────────────────────────────────────────────
echo "==> Installing binary..."
sudo install -m 755 "$BINARY" "$BIN_DIR/yinmonote"

# ── Create directories ────────────────────────────────────────────────────────
mkdir -p "$DATA_DIR" "$YINMONOTE_DIR" "$LOG_DIR"

# ── macOS: LaunchAgent ────────────────────────────────────────────────────────
if [ "$(uname -s)" = "Darwin" ]; then
    PLIST_DIR="$HOME/Library/LaunchAgents"
    PLIST="$PLIST_DIR/com.yinmonote.plist"
    mkdir -p "$PLIST_DIR"

    if [ -f "$PLIST" ]; then
        echo "==> Stopping existing service..."
        launchctl unload "$PLIST" 2>/dev/null || true
    fi

    echo "==> Creating LaunchAgent: $PLIST ..."

    # Build optional env-var entries for TLS and WebDAV
    _PLIST_EXTRA=""
    [ "$TLS_SELF"        = "1" ] && _PLIST_EXTRA="${_PLIST_EXTRA}        <key>TLS_SELF</key><string>1</string>\n"
    [ -n "$ACME_DOMAIN"        ] && _PLIST_EXTRA="${_PLIST_EXTRA}        <key>ACME_DOMAIN</key><string>${ACME_DOMAIN}</string>\n"
    [ -n "$TLS_EXTRA_IPS"      ] && _PLIST_EXTRA="${_PLIST_EXTRA}        <key>TLS_EXTRA_IPS</key><string>${TLS_EXTRA_IPS}</string>\n"
    [ "$WEBDAV_DISABLED" = "1" ] && _PLIST_EXTRA="${_PLIST_EXTRA}        <key>WEBDAV_DISABLED</key><string>1</string>\n"

    printf '%s' '<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.yinmonote</string>
    <key>ProgramArguments</key>
    <array>
        <string>'"$BIN_DIR/yinmonote"'</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>DATA_DIR</key>
        <string>'"$DATA_DIR"'</string>
        <key>CONFIG_FILE</key>
        <string>'"$CONFIG_FILE"'</string>
        <key>PORT</key>
        <string>'"$PORT"'</string>
' > "$PLIST"
    printf '%b' "$_PLIST_EXTRA" >> "$PLIST"
    printf '%s\n' '    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>'"$LOG_DIR/yinmonote.log"'</string>
    <key>StandardErrorPath</key>
    <string>'"$LOG_DIR/yinmonote.error.log"'</string>
</dict>
</plist>' >> "$PLIST"

    echo "==> Starting service..."
    launchctl load -w "$PLIST"

# ── Linux: systemd user service ───────────────────────────────────────────────
else
    SYSTEMD_DIR="$HOME/.config/systemd/user"
    SERVICE="$SYSTEMD_DIR/yinmonote.service"
    mkdir -p "$SYSTEMD_DIR"

    {
        echo "[Unit]"
        echo "Description=YinMoNote"
        echo "After=network.target"
        echo ""
        echo "[Service]"
        echo "ExecStart=$BIN_DIR/yinmonote"
        echo "Environment=DATA_DIR=$DATA_DIR"
        echo "Environment=CONFIG_FILE=$CONFIG_FILE"
        echo "Environment=PORT=$PORT"
        [ "$TLS_SELF"        = "1" ] && echo "Environment=TLS_SELF=1"
        [ -n "$ACME_DOMAIN"        ] && echo "Environment=ACME_DOMAIN=$ACME_DOMAIN"
        [ -n "$TLS_EXTRA_IPS"      ] && echo "Environment=TLS_EXTRA_IPS=$TLS_EXTRA_IPS"
        [ "$WEBDAV_DISABLED" = "1" ] && echo "Environment=WEBDAV_DISABLED=1"
        echo "Restart=always"
        echo ""
        echo "[Install]"
        echo "WantedBy=default.target"
    } > "$SERVICE"

    if systemctl --user is-active yinmonote >/dev/null 2>&1; then
        echo "==> Stopping existing service..."
        systemctl --user stop yinmonote
    fi

    echo "==> Enabling and starting service..."
    systemctl --user daemon-reload
    systemctl --user enable --now yinmonote
fi

# ── Done ──────────────────────────────────────────────────────────────────────
# Extract port number from PORT (which may be "127.0.0.1:8080" or ":8080")
_PORT_NUM="${PORT##*:}"

# Determine the access URL based on TLS mode and access mode
if [ -n "$ACME_DOMAIN" ]; then
    _BASE_URL="https://$ACME_DOMAIN"
elif [ "$TLS_SELF" = "1" ]; then
    case "$PORT" in localhost:*|127.0.0.1:*) _BASE_URL="https://localhost:${_PORT_NUM}" ;; *) _BASE_URL="https://<server-ip>:${_PORT_NUM}" ;; esac
else
    case "$PORT" in localhost:*|127.0.0.1:*) _BASE_URL="http://localhost:${_PORT_NUM}" ;; *) _BASE_URL="http://<server-ip>:${_PORT_NUM}" ;; esac
fi

echo ""
if [ "$IS_UPGRADE" = true ]; then
    echo "  ✓ Upgraded successfully — service restarted"
else
    echo "  ✓ Installation complete, service started"
fi
echo ""
echo "  Open:   $_BASE_URL"
if [ "$TLS_SELF" = "1" ]; then
    echo ""
    echo "  HTTPS (self-signed TLS) — install the CA cert once per device:"
    echo ""
    echo "  This machine:"
    echo "    $YINMONOTE_DIR/selfca/ca.crt"
    if [ "$(uname -s)" = "Darwin" ]; then
        echo "    Run:  open $YINMONOTE_DIR/selfca/ca.crt"
        echo "    Then: System Settings → Privacy & Security → Certificate Trust Settings → trust it"
    else
        echo "    Double-click the file, or: sudo cp it to /usr/local/share/ca-certificates/ && update-ca-certificates"
    fi
    echo ""
    echo "  Remote devices (phone, tablet):"
    echo "    Visit $_BASE_URL/ca.crt in the browser"
    echo "    Click through the security warning once (Advanced → Proceed)"
    echo "    Install the downloaded file:"
    echo "    iOS:     Settings → General → VPN & Device Management → trust it"
    echo "    Android: Settings → Security → Install a certificate → CA Certificate"
fi
if [ "$WEBDAV_DISABLED" != "1" ]; then
    echo "  WebDAV: ${_BASE_URL}/dav/"
fi
echo ""
echo "  Notes:  $DATA_DIR"
echo "  Config: $CONFIG_FILE"
echo "  Logs:   $LOG_DIR/yinmonote.log"
echo "════════════════════════════════════════"
