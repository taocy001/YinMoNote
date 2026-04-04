#!/bin/bash
# build/package-zip.sh <version> <arch>
#
# Package the compiled Windows binary and installer into a ZIP archive.
# Output: dist/yinmonote-<version>-windows-<arch>.zip
#
# Contents:
#   yinmonote-windows-<arch>/
#     yinmonote.exe   -- the binary
#     Install.bat     -- double-click to install
#     Install.ps1     -- PowerShell installer
#     QuickStart.txt  -- quick reference
#
# Prerequisite: dist/yinmonote-windows-<arch>.exe must already exist
# (produced by build.sh with GOOS=windows).

set -e

VERSION="${1:?Usage: $0 <version> <arch>}"
ARCH="${2:-amd64}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_ROOT/dist/yinmonote-windows-$ARCH.exe"
OUT_DIR="$PROJECT_ROOT/dist"
PKG_NAME="yinmonote-${VERSION}-windows-${ARCH}"
INNER_DIR="yinmonote-windows-${ARCH}"

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found: $BINARY"
    echo "Please run first: ./build/build.sh windows $ARCH"
    exit 1
fi

echo ""
echo "================================================"
echo "  Packaging ZIP: $PKG_NAME.zip"
echo "================================================"

TMP="$(mktemp -d)"
trap "rm -rf '$TMP'" EXIT

PKG_DIR="$TMP/$INNER_DIR"
mkdir -p "$PKG_DIR"

# Binary
cp "$BINARY" "$PKG_DIR/yinmonote.exe"

# Installer files
cp "$SCRIPT_DIR/Install.bat" "$PKG_DIR/Install.bat"
cp "$SCRIPT_DIR/Install.ps1" "$PKG_DIR/Install.ps1"

# QuickStart.txt
cat > "$PKG_DIR/QuickStart.txt" << 'EOF'
YinMoNote - Windows Quick Start
================================

INSTALL
  Double-click Install.bat (recommended)
  Or: right-click Install.ps1 -> Run with PowerShell

FIRST USE
  1. Open the URL shown at the end of installation
  2. Set a password on first launch
  3. If using self-signed HTTPS: install the CA cert (instructions shown during install)

MANAGE
  Task Scheduler mode (no admin):
    Start:  schtasks /Run /TN YinMoNote
    Stop:   schtasks /End /TN YinMoNote
    Status: schtasks /Query /TN YinMoNote

  Windows Service mode (admin):
    Start:  sc start YinMoNote
    Stop:   sc stop YinMoNote
    Status: sc query YinMoNote

UNINSTALL
  Task Scheduler mode:
    Run: Unregister-ScheduledTask -TaskName "YinMoNote" -Confirm:$false
    Then delete: %LOCALAPPDATA%\YinMoNote\

  Windows Service mode:
    Run: sc stop YinMoNote && sc delete YinMoNote
    Then delete: C:\Program Files\YinMoNote\

DATA
  Default location: %USERPROFILE%\.yinmonote\notes\
  Config:           %USERPROFILE%\.yinmonote\config.json

SUPPORT
  https://github.com/taocy001/YinMoNote
EOF

mkdir -p "$OUT_DIR"

# Prefer system zip; fall back to Python (available on most platforms)
if command -v zip >/dev/null 2>&1; then
    (cd "$TMP" && zip -r "$OUT_DIR/$PKG_NAME.zip" "$INNER_DIR")
elif command -v python3 >/dev/null 2>&1; then
    python3 -c "
import zipfile, os, sys
src = sys.argv[1]
dst = sys.argv[2]
with zipfile.ZipFile(dst, 'w', zipfile.ZIP_DEFLATED) as zf:
    for root, dirs, files in os.walk(src):
        for f in files:
            abs_path = os.path.join(root, f)
            arc_name = os.path.relpath(abs_path, os.path.dirname(src))
            zf.write(abs_path, arc_name)
" "$PKG_DIR" "$OUT_DIR/$PKG_NAME.zip"
else
    echo "Error: neither 'zip' nor 'python3' found -- cannot create ZIP archive"
    exit 1
fi

echo ""
echo "  Package: dist/$PKG_NAME.zip"
echo ""
echo "  Contents:"
echo "    $INNER_DIR/yinmonote.exe"
echo "    $INNER_DIR/Install.bat"
echo "    $INNER_DIR/Install.ps1"
echo "    $INNER_DIR/QuickStart.txt"
echo ""
echo "  To install on Windows:"
echo "    1. Extract the ZIP"
echo "    2. Double-click Install.bat"
echo "================================================"
