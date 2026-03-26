#!/bin/bash
# build/package-docker.sh <version> [arch]
#
# Build a Docker image and save it as a tar file for offline distribution.
# Output: dist/yinmonote-<version>.tar
#
# Architecture defaults to the host architecture and can be overridden by the second argument (amd64 | arm64).

set -e
export DOCKER_BUILDKIT=1

VERSION="${1:?Usage: $0 <version> [arch]}"
ARCH="${2:-$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
OUT_DIR="$PROJECT_ROOT/dist"
IMAGE="yinmonote:$VERSION"
OUT_FILE="$OUT_DIR/yinmonote-$VERSION-docker-$ARCH.tar"

mkdir -p "$OUT_DIR"

echo ""
echo "════════════════════════════════════════"
echo "  Packaging Docker image: $IMAGE (linux/$ARCH)"
echo "════════════════════════════════════════"

# Remove existing image with the same tag before rebuilding to avoid stale layers.
docker rmi "$IMAGE" 2>/dev/null || true

docker build \
    -f "$PROJECT_ROOT/build/Dockerfile" \
    --platform "linux/$ARCH" \
    --build-arg TARGETOS=linux \
    --build-arg TARGETARCH="$ARCH" \
    -t "$IMAGE" \
    "$PROJECT_ROOT"

# Also tag as :latest for convenience (replaces any previous :latest).
docker tag "$IMAGE" yinmonote:latest

docker save "$IMAGE" -o "$OUT_FILE"

# Remove the image after saving to tar — the tar is the distribution artifact.
# Keep :latest only if it points to this build (same ID).
docker rmi "$IMAGE" 2>/dev/null || true
docker image prune -f >/dev/null 2>&1 || true

echo ""
echo "  Saved to: dist/yinmonote-$VERSION-docker-$ARCH.tar"
echo "════════════════════════════════════════"
