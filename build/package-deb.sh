#!/bin/bash
# build/package-deb.sh <version> <arch>
#
# Package the compiled Linux native binary into a .deb installer.
# Uses a Debian Docker container to run dpkg-deb, so it works on both Linux and macOS.
#
# Prerequisite: dist/yinmonote-linux-<arch> must already exist (produced by build.sh).

set -e

VERSION="${1:?Usage: $0 <version> <arch>}"
ARCH="${2:?}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BINARY="$PROJECT_ROOT/dist/yinmonote-linux-$ARCH"
OUT_DIR="$PROJECT_ROOT/dist"
PKG_NAME="yinmonote_${VERSION}_${ARCH}"

if [ ! -f "$BINARY" ]; then
    echo "Error: binary not found: $BINARY"
    echo "Please run first: ./build/build.sh linux $ARCH"
    exit 1
fi

echo ""
echo "════════════════════════════════════════"
echo "  Packaging .deb: $PKG_NAME.deb"
echo "════════════════════════════════════════"

# Create the deb directory structure in a temp directory
TMP="$(mktemp -d)"
trap "rm -rf '$TMP'" EXIT

mkdir -p "$TMP/DEBIAN"
mkdir -p "$TMP/opt/yinmonote"
mkdir -p "$TMP/etc/systemd/system"
mkdir -p "$TMP/var/lib/yinmonote"

# Binary
cp "$BINARY" "$TMP/opt/yinmonote/yinmonote"
chmod 755 "$TMP/opt/yinmonote/yinmonote"

# Systemd service
cp "$SCRIPT_DIR/yinmonote.service" "$TMP/etc/systemd/system/"

# DEBIAN/control
cat > "$TMP/DEBIAN/control" << EOF
Package: yinmonote
Version: $VERSION
Section: utils
Priority: optional
Architecture: $ARCH
Maintainer: YinMoNote
Description: Secure personal note library
 A self-hosted, encrypted note-taking application with Git-based version history.
 Supports client-side AES-GCM encryption, Markdown editing, and file attachments.
 Single self-contained binary with no runtime dependencies.
EOF

# DEBIAN/postinst — executed after install: create user, directories, enable service
cat > "$TMP/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e
# Create system user (ignore if already exists)
useradd --system --shell /bin/false --no-create-home yinmonote 2>/dev/null || true

# Initialise data directory
mkdir -p /var/lib/yinmonote
chown yinmonote:yinmonote /var/lib/yinmonote
chmod 700 /var/lib/yinmonote

# Register the service (do not start automatically — let the admin finish configuration first)
systemctl daemon-reload
systemctl enable yinmonote

echo ""
echo "YinMoNote installed successfully."
echo "Config file: /etc/systemd/system/yinmonote.service"
echo "  → Set AUTH_USER / AUTH_PASS / PORT as needed"
echo "Start service: systemctl start yinmonote"
EOF
chmod 755 "$TMP/DEBIAN/postinst"

# DEBIAN/prerm — executed before removal: stop and disable the service
cat > "$TMP/DEBIAN/prerm" << 'EOF'
#!/bin/bash
systemctl stop yinmonote 2>/dev/null || true
systemctl disable yinmonote 2>/dev/null || true
EOF
chmod 755 "$TMP/DEBIAN/prerm"

# Prefer the host's dpkg-deb; fall back to a Debian Docker container
if command -v dpkg-deb >/dev/null 2>&1; then
    dpkg-deb --root-owner-group --build "$TMP" "$OUT_DIR/$PKG_NAME.deb"
elif command -v docker >/dev/null 2>&1; then
    docker run --rm \
        -v "$TMP:/pkg" \
        -v "$OUT_DIR:/out" \
        debian:bookworm-slim \
        dpkg-deb --root-owner-group --build /pkg "/out/$PKG_NAME.deb"
    docker container prune -f >/dev/null 2>&1 || true
else
    echo "Error: neither dpkg-deb nor Docker found — cannot package .deb"
    echo "Please install dpkg-dev (Linux) or Docker (any platform)"
    exit 1
fi

echo ""
echo "  Package: dist/$PKG_NAME.deb"
echo ""
echo "  Install:"
echo "    sudo dpkg -i dist/$PKG_NAME.deb"
echo "    sudo nano /etc/systemd/system/yinmonote.service  # configure auth"
echo "    sudo systemctl start yinmonote"
echo "════════════════════════════════════════"
