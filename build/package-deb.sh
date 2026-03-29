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

# DEBIAN/conffiles — tells dpkg this file is user-editable:
#   - fresh install : template is installed as-is
#   - upgrade, file unchanged : template is silently updated
#   - upgrade, file modified  : dpkg prompts (keep/replace/diff)
#   - unattended upgrade      : dpkg -o Dpkg::Options::=--force-confold -i *.deb
cat > "$TMP/DEBIAN/conffiles" << 'EOF'
/etc/systemd/system/yinmonote.service
EOF

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

# DEBIAN/postinst — called by dpkg after files are unpacked
#   $1 = "configure"   $2 = "" (fresh install) | previous-version (upgrade)
cat > "$TMP/DEBIAN/postinst" << 'EOF'
#!/bin/bash
set -e

case "$1" in
    configure)
        # Create system user (no-op if already exists)
        useradd --system --shell /bin/false --no-create-home yinmonote 2>/dev/null || true

        # Initialise data directory (no-op if already exists)
        mkdir -p /var/lib/yinmonote/notes
        chown -R yinmonote:yinmonote /var/lib/yinmonote
        chmod 700 /var/lib/yinmonote /var/lib/yinmonote/notes

        systemctl daemon-reload
        systemctl enable yinmonote

        if [ -z "$2" ]; then
            # ── Fresh install ─────────────────────────────────
            echo ""
            echo "YinMoNote installed successfully."
            echo "Edit the service file to set PORT, TLS options, and auth credentials:"
            echo "  sudo nano /etc/systemd/system/yinmonote.service"
            echo "Then start the service:"
            echo "  sudo systemctl start yinmonote"
        else
            # ── Upgrade from version $2 ───────────────────────
            if systemctl is-active --quiet yinmonote; then
                echo "==> Restarting yinmonote service..."
                systemctl restart yinmonote
                echo "YinMoNote upgraded from $2 and service restarted."
            else
                echo "YinMoNote upgraded from $2. Service is stopped; start manually:"
                echo "  sudo systemctl start yinmonote"
            fi
        fi
        ;;
esac
EOF
chmod 755 "$TMP/DEBIAN/postinst"

# DEBIAN/prerm — called by dpkg before files are removed/replaced
#   $1 = "upgrade $new-ver" | "remove" | "deconfigure"
cat > "$TMP/DEBIAN/prerm" << 'EOF'
#!/bin/bash
case "$1" in
    remove|deconfigure)
        # Full removal: stop and disable
        systemctl stop yinmonote 2>/dev/null || true
        systemctl disable yinmonote 2>/dev/null || true
        ;;
    upgrade)
        # Upgrade: only stop; postinst will restart after new files are in place
        systemctl stop yinmonote 2>/dev/null || true
        ;;
esac
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
echo "  Fresh install:"
echo "    sudo dpkg -i dist/$PKG_NAME.deb"
echo "    sudo nano /etc/systemd/system/yinmonote.service  # configure port / TLS / auth"
echo "    sudo systemctl start yinmonote"
echo ""
echo "  Upgrade (keeps your existing service config):"
echo "    sudo dpkg --force-confold -i dist/$PKG_NAME.deb"
echo "════════════════════════════════════════"
