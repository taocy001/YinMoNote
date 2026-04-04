#!/bin/bash
# Build a self-contained native binary for Linux or macOS.
#
# Uses native Go + Node tools when available; auto-falls back to Docker
# when either tool is missing. Set DOCKER=1 to force Docker regardless.
#
# Usage:
#   ./build/build.sh                        # linux/amd64 (default)
#   ./build/build.sh linux arm64            # linux/arm64
#   ./build/build.sh darwin amd64           # macOS Intel
#   ./build/build.sh darwin arm64           # macOS Apple Silicon
#   ./build/build.sh windows amd64          # Windows x86-64 (.exe)
#   DOCKER=1 ./build/build.sh darwin arm64  # force Docker environment
#
# Output: dist/yinmonote-<os>-<arch>  (or .exe on Windows)
#
# Running the binary:
#   DATA_DIR=/path/to/data PORT=:8080 ./dist/yinmonote-linux-amd64

set -e

# Enable BuildKit for Docker build cache mounts (required on Debian/Ubuntu
# where docker.io package does not enable BuildKit by default).
export DOCKER_BUILDKIT=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

GOOS="${1:-linux}"
GOARCH="${2:-amd64}"
OUT_DIR="$PROJECT_ROOT/dist"
EXE_SUFFIX=""
[ "$GOOS" = "windows" ] && EXE_SUFFIX=".exe"
OUT_NAME="yinmonote-${GOOS}-${GOARCH}${EXE_SUFFIX}"

mkdir -p "$OUT_DIR"

echo ""
echo "════════════════════════════════════════"
echo "  Native binary: $GOOS/$GOARCH"
echo "════════════════════════════════════════"

# ── Docker build path ──────────────────────────────────────────────────────────
if [ "${DOCKER:-0}" = "1" ]; then
    if ! command -v docker >/dev/null 2>&1; then
        echo "Error: DOCKER=1 specified but Docker is not installed"
        exit 1
    fi

    # Fixed name so repeated builds overwrite rather than accumulate.
    TEMP_IMAGE="yinmonote-build:tmp"

    # Remove previous build image if present (ensures no stale layer confusion).
    docker rmi "$TEMP_IMAGE" 2>/dev/null || true

    docker build \
        --build-arg "TARGETOS=$GOOS" \
        --build-arg "TARGETARCH=$GOARCH" \
        -t "$TEMP_IMAGE" \
        -f "$PROJECT_ROOT/build/Dockerfile" \
        "$PROJECT_ROOT"

    CID=$(docker create "$TEMP_IMAGE")
    # Binary lives at /home/yinmonote/YinMoNote (non-root user, WORKDIR /home/yinmonote).
    # Windows cross-compilation produces a .exe; the Dockerfile outputs without suffix,
    # so we rename after copying.
    docker cp "$CID:/home/yinmonote/YinMoNote" "$OUT_DIR/$OUT_NAME"
    docker rm "$CID"
    docker rmi "$TEMP_IMAGE"
    docker image prune -f >/dev/null 2>&1 || true

# ── Native build path ──────────────────────────────────────────────────────────
else
    # Auto-fallback to Docker when native tools are unavailable
    if ! command -v go >/dev/null 2>&1 || ! command -v npm >/dev/null 2>&1; then
        if command -v docker >/dev/null 2>&1; then
            echo "Native tools not found, falling back to Docker..."
            DOCKER=1 "$0" "$GOOS" "$GOARCH"
            exit $?
        else
            echo "Error: Go/npm not found and Docker is not available."
            echo "Install Go 1.21+ and Node.js 18+, or install Docker."
            exit 1
        fi
    fi

    echo ""
    echo "==> Building frontend..."
    cd "$PROJECT_ROOT/frontend"
    npm ci --prefer-offline 2>/dev/null || npm install
    npm run build

    echo ""
    echo "==> Copying frontend dist to backend/dist..."
    rm -rf "$PROJECT_ROOT/backend/dist"
    cp -r "$PROJECT_ROOT/frontend/dist" "$PROJECT_ROOT/backend/dist"

    echo ""
    echo "==> Compiling backend binary ($GOOS/$GOARCH)..."
    cd "$PROJECT_ROOT/backend"
    export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"
    CGO_ENABLED=0 GOOS="$GOOS" GOARCH="$GOARCH" \
        go build -ldflags="-s -w" -o "$OUT_DIR/$OUT_NAME" .

    # Restore the backend/dist placeholder to keep git status clean.
    rm -rf "$PROJECT_ROOT/backend/dist"
    mkdir -p "$PROJECT_ROOT/backend/dist"
    touch "$PROJECT_ROOT/backend/dist/.gitkeep"
fi

# chmod +x is a no-op for .exe files but harmless; skip on Windows target to keep output clean.
[ "$GOOS" != "windows" ] && chmod +x "$OUT_DIR/$OUT_NAME"

echo ""
echo "════════════════════════════════════════"
echo "  Binary:  dist/$OUT_NAME"
echo ""
echo "  Run:"
echo "    DATA_DIR=/path/to/data \\"
echo "    PORT=:8080 \\"
echo "    $OUT_DIR/$OUT_NAME"
echo "════════════════════════════════════════"
