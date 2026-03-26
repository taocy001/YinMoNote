#!/bin/bash
# build/package-dmg.sh <version> <arch>
#
# Package the compiled macOS native binary into a .dmg disk image.
# DMG contents: yinmonote binary + Install.command (double-click to install) + QuickStart.txt
# Must be run on macOS (requires the built-in hdiutil utility).

set -e

VERSION="${1:?Usage: $0 <version> <arch>}"
ARCH="${2:?}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_ROOT/dist/yinmonote-darwin-$ARCH"
OUT_DIR="$PROJECT_ROOT/dist"
DMG_NAME="yinmonote-${VERSION}-macos-${ARCH}"

if [ "$(uname -s)" != "Darwin" ]; then
    echo "Error: .dmg packaging must be run on macOS"
    exit 1
fi

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found: $BINARY"
    echo "Please run first: ./build/build.sh darwin $ARCH"
    exit 1
fi

echo ""
echo "════════════════════════════════════════"
echo "  Packaging .dmg: $DMG_NAME.dmg"
echo "════════════════════════════════════════"

TMP="$(mktemp -d)"
trap "rm -rf '$TMP'" EXIT

STAGING="$TMP/YinMoNote"
mkdir -p "$STAGING"

# ── Binary ────────────────────────────────────────────────────────────────────
cp "$BINARY" "$STAGING/yinmonote"
chmod +x "$STAGING/yinmonote"

# ── Install.command (installation script that runs in Terminal on double-click) ──
cat > "$STAGING/Install.command" << 'INSTALLER'
#!/bin/bash
# YinMoNote install script — double-click to run in macOS Terminal
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BINARY="$SCRIPT_DIR/yinmonote"
BIN_DIR="/usr/local/bin"
PLIST_DIR="$HOME/Library/LaunchAgents"
PLIST="$PLIST_DIR/com.yinmonote.plist"
YINMONOTE_DIR="$HOME/.yinmonote"
DEFAULT_NOTES="$HOME/.yinmonote/notes"

echo ""
echo "════════════════════════════════════════"
echo "  YinMoNote Setup Wizard"
echo "════════════════════════════════════════"

# ── Read existing notes dir as default when upgrading ─────────────────────────
EXISTING_DATA=""
if [ -f "$PLIST" ]; then
    EXISTING_DATA=$(/usr/libexec/PlistBuddy -c "Print :EnvironmentVariables:DATA_DIR" "$PLIST" 2>/dev/null || true)
fi
DISPLAY_DEFAULT="${EXISTING_DATA:-$DEFAULT_NOTES}"

# ── Choose notes directory ────────────────────────────────────────────────────
CHOICE=$(osascript <<APPLESCRIPT
set defaultPath to "$DISPLAY_DEFAULT"
set msg to "Choose the directory where YinMoNote will store your notes." & return & return & "Default: " & defaultPath
set btn to button returned of (display dialog msg ¬
    buttons {"Custom…", "Use Default"} ¬
    default button "Use Default" ¬
    with title "YinMoNote Setup" ¬
    with icon note)
return btn
APPLESCRIPT
)

if [ "$CHOICE" = "Custom…" ]; then
    CHOSEN=$(osascript <<APPLESCRIPT 2>/dev/null || true
try
    set f to choose folder with prompt "Choose the folder to store your notes:"
    return POSIX path of f
on error
    return ""
end try
APPLESCRIPT
)
    DATA_DIR="${CHOSEN%/}"
    DATA_DIR="${DATA_DIR:-$DISPLAY_DEFAULT}"
else
    DATA_DIR="$DISPLAY_DEFAULT"
fi

# config.json is always at ~/.yinmonote/config.json regardless of notes location
CONFIG_FILE="$YINMONOTE_DIR/config.json"
LOG_DIR="$YINMONOTE_DIR/logs"

echo "  Notes directory: $DATA_DIR"

# ── Install binary ────────────────────────────────────────────────────────────
echo "==> Installing binary to $BIN_DIR/yinmonote ..."
sudo install -m 755 "$BINARY" "$BIN_DIR/yinmonote"

# ── Create directories ────────────────────────────────────────────────────────
mkdir -p "$DATA_DIR" "$YINMONOTE_DIR" "$LOG_DIR" "$PLIST_DIR"

# ── Stop old service (if already installed) ───────────────────────────────────
if [ -f "$PLIST" ]; then
    echo "==> Stopping old service..."
    launchctl unload "$PLIST" 2>/dev/null || true
fi

# ── Generate LaunchAgent plist ────────────────────────────────────────────────
echo "==> Creating LaunchAgent: $PLIST ..."
cat > "$PLIST" << PLISTEOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.yinmonote</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BIN_DIR/yinmonote</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
        <key>DATA_DIR</key>
        <string>$DATA_DIR</string>
        <key>CONFIG_FILE</key>
        <string>$CONFIG_FILE</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>$LOG_DIR/yinmonote.log</string>
    <key>StandardErrorPath</key>
    <string>$LOG_DIR/yinmonote.error.log</string>
</dict>
</plist>
PLISTEOF

# ── Start service ─────────────────────────────────────────────────────────────
echo "==> Starting service..."
launchctl load -w "$PLIST"

echo ""
echo "  ✓ Installation complete"
echo "  ✓ Service started and set to run at login"
echo ""
echo "  Open:   http://localhost:8080"
echo "  Notes:  $DATA_DIR"
echo "  Config: $CONFIG_FILE"
echo "  Logs:   $LOG_DIR/yinmonote.log"
echo ""
echo "  Manage service:"
echo "    launchctl stop  com.yinmonote   # stop"
echo "    launchctl start com.yinmonote   # start"
echo "════════════════════════════════════════"
INSTALLER
chmod +x "$STAGING/Install.command"

# ── Quick-start guide ─────────────────────────────────────────────────────────
cat > "$STAGING/QuickStart.txt" << EOF
YinMoNote $VERSION — macOS $ARCH
════════════════════════════════════════

Installation:
  Double-click Install.command and follow the prompts in Terminal.
  The wizard will help you choose a data directory and configure auto-start at login.

After installation:
  Open http://localhost:8080 to start using YinMoNote.

Manage the service:
  launchctl stop  com.yinmonote   # stop
  launchctl start com.yinmonote   # start

Uninstall:
  launchctl unload ~/Library/LaunchAgents/com.yinmonote.plist
  rm ~/Library/LaunchAgents/com.yinmonote.plist
  sudo rm /usr/local/bin/yinmonote
EOF

# ── Generate .dmg ─────────────────────────────────────────────────────────────
hdiutil create \
    -volname "YinMoNote $VERSION" \
    -srcfolder "$STAGING" \
    -ov \
    -format UDZO \
    "$OUT_DIR/$DMG_NAME.dmg"

echo ""
echo "  Package: dist/$DMG_NAME.dmg"
echo "  Usage:   open dist/$DMG_NAME.dmg  →  double-click Install.command"
echo "════════════════════════════════════════"
