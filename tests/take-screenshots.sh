#!/bin/bash
# Capture product screenshots and animated GIF for README documentation.
#
# Usage:
#   ./tests/take-screenshots.sh              # build images if needed, then capture
#   ./tests/take-screenshots.sh --no-build   # skip image rebuild (fastest re-run)
#
# Output: docs/images/
#   editor.png    — static hero screenshot
#   demo.gif      — animated GIF (9 frames showing live Markdown rendering)

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( dirname "$SCRIPT_DIR" )"
OUTPUT_DIR="$PROJECT_ROOT/docs/images"
COMPOSE="docker compose -f $PROJECT_ROOT/tests/docker-compose.e2e.yml"

# ── Parse flags ───────────────────────────────────────────────────────────────
BUILD_IMAGES=true
for arg in "$@"; do
  [ "$arg" = "--no-build" ] && BUILD_IMAGES=false
done

# ── Cleanup on exit ───────────────────────────────────────────────────────────
cleanup() {
  $COMPOSE down -v 2>/dev/null || true
}
trap cleanup EXIT

echo ""
echo "════════════════════════════════════════"
echo "  Screenshots — Playwright + Chromium"
echo "════════════════════════════════════════"

mkdir -p "$OUTPUT_DIR/frames"

$COMPOSE down -v 2>/dev/null || true

# ── Build images ──────────────────────────────────────────────────────────────
if [ "$BUILD_IMAGES" = true ]; then
  if ! docker image inspect yinmonote:e2e > /dev/null 2>&1; then
    echo "  (building app image — yinmonote:e2e)..."
    docker build \
      -f "$PROJECT_ROOT/build/Dockerfile" \
      --build-arg VITE_PBKDF2_ITERATIONS=1000 \
      -t yinmonote:e2e \
      "$PROJECT_ROOT"
  else
    echo "  (app image yinmonote:e2e already present, skipping build)"
  fi

  if ! docker image inspect yinmonote-playwright:latest > /dev/null 2>&1; then
    echo "  (building playwright image — yinmonote-playwright:latest)..."
    docker build \
      -f "$PROJECT_ROOT/tests/Dockerfile.playwright" \
      -t yinmonote-playwright:latest \
      "$PROJECT_ROOT"
  else
    echo "  (playwright image already present, skipping build)"
  fi
fi

# ── Start the app and wait for it to be healthy ───────────────────────────────
echo "  (starting app container)..."
$COMPOSE up -d app

echo "  (waiting for health check)..."
for i in $(seq 1 24); do
  if $COMPOSE exec -T app curl -fsk https://localhost:8080/api/config > /dev/null 2>&1; then
    echo "  (app is healthy)"
    break
  fi
  if [ "$i" -eq 24 ]; then
    echo "  ERROR: app failed to start within 2 minutes"
    exit 1
  fi
  sleep 5
done

# ── Run the screenshot spec ───────────────────────────────────────────────────
echo "  (running screenshot spec)..."
$COMPOSE run --rm \
  -v "$OUTPUT_DIR:/screenshots" \
  -v "$PROJECT_ROOT/tests/e2e/specs:/tests/specs" \
  -v "$PROJECT_ROOT/tests/e2e/helpers:/tests/helpers" \
  -v "$PROJECT_ROOT/tests/e2e/fixtures.ts:/tests/fixtures.ts" \
  -e APP_URL=http://app:8080 \
  -e SCREENSHOT_DIR=/screenshots \
  e2e sh -c "npx playwright test 99-screenshots --reporter=list"

# ── Assemble animated GIF from frames ────────────────────────────────────────
echo "  (assembling animated GIF)..."
docker run --rm \
  -v "$OUTPUT_DIR:/screenshots" \
  --entrypoint sh \
  yinmonote-playwright:latest \
  -c "ffmpeg -y -framerate 2 -pattern_type glob -i '/screenshots/frames/frame-*.png' \
        -vf 'scale=1440:-1:flags=lanczos,palettegen=max_colors=256' /tmp/palette.png && \
      ffmpeg -y -framerate 2 -pattern_type glob -i '/screenshots/frames/frame-*.png' \
        -i /tmp/palette.png \
        -lavfi 'scale=1440:-1:flags=lanczos[x];[x][1:v]paletteuse' \
        /screenshots/demo.gif 2>&1 | tail -5"

echo ""
echo "════════════════════════════════════════"
echo "  Done"
echo "  docs/images/editor.png"
echo "  docs/images/demo.gif"
echo "════════════════════════════════════════"
