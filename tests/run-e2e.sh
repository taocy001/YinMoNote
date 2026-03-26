#!/bin/bash
# Run the E2E test suite inside Docker.
#
# Usage:
#   ./tests/run-e2e.sh                          # run all specs (headless Chromium)
#   ./tests/run-e2e.sh --no-build               # skip rebuilding images (fastest re-run)
#   ./tests/run-e2e.sh specs/06 specs/07        # run only matching spec files
#   ./tests/run-e2e.sh --no-build specs/06      # combine flags
#
# Image rebuild strategy:
#   Full run (no spec filter):   build both app + e2e via docker compose build
#   Spec filter (e.g. specs/08): rebuild app always (catches App.vue / backend changes);
#                                patch e2e image from local base (catches test code changes)
#                                Both images use node:20-alpine which is always locally cached.
#   --no-build:                  skip all rebuilds (use when only re-running after a failure)
#
# After the run the Playwright HTML report is available at:
#   tests/e2e/playwright-report/index.html

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$( dirname "$SCRIPT_DIR" )"
COMPOSE="docker compose -f $PROJECT_ROOT/tests/docker-compose.e2e.yml"

# ── Cleanup: always runs on exit (normal, set -e abort, Ctrl-C) ───────────
cleanup() {
  $COMPOSE down -v 2>/dev/null || true
  # Remove E2E images — they are always rebuilt at the start of the next run.
  docker rmi yinmonote:e2e yinmonote-playwright:latest 2>/dev/null || true
  docker image prune -f >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo ""
echo "════════════════════════════════════════"
echo "  E2E — Playwright + Chromium in Docker"
echo "════════════════════════════════════════"

# Tear down any leftover containers and volumes from a previous run
$COMPOSE down -v 2>/dev/null || true

BUILD_IMAGES=true
SPEC_ARGS=""

for arg in "$@"; do
  if [ "$arg" = "--no-build" ]; then
    BUILD_IMAGES=false
    echo "  (skipping image rebuild)"
  else
    SPEC_ARGS="$SPEC_ARGS $arg"
  fi
done

EXIT_CODE=0

# ── Build images ──────────────────────────────────────────────────────────
if [ "$BUILD_IMAGES" = true ]; then
  if [ -n "$SPEC_ARGS" ]; then
    # Fast-path for spec-filtered runs: rebuild app (catches source changes) and
    # patch e2e image from its existing local base (catches test-code changes).
    # Both images are based on node:20-alpine which is always locally cached —
    # no Docker Hub access needed, so this works offline too.
    echo "  (rebuilding app image...)"
    docker build \
      -f "$PROJECT_ROOT/build/Dockerfile" \
      --build-arg VITE_PBKDF2_ITERATIONS=1000 \
      --no-cache-filter frontend-builder \
      -t yinmonote:e2e \
      "$PROJECT_ROOT" >/dev/null

    echo "  (patching playwright image with latest test code...)"
    # Dockerfile.playwright-patch: FROM yinmonote-playwright:latest + COPY specs/ helpers/
    # This avoids a full playwright image rebuild while still picking up test changes.
    docker build \
      -f "$PROJECT_ROOT/tests/Dockerfile.playwright-patch" \
      -t yinmonote-playwright:latest \
      "$PROJECT_ROOT" >/dev/null
  else
    # Full run: build both images via compose (standard path, same as CI)
    $COMPOSE build
  fi
fi

# ── Run tests ─────────────────────────────────────────────────────────────
if [ -n "$SPEC_ARGS" ]; then
  echo "  (running only:$SPEC_ARGS)"
  # Use 'docker compose run' so the e2e container gets the app service's
  # network and waits for app's healthcheck (depends_on is honoured).
  set +e
  $COMPOSE run --rm \
    -e APP_URL=https://app:8080 \
    e2e sh -c "npx playwright test $SPEC_ARGS --reporter=list,html"
  EXIT_CODE=$?
  set -e
else
  # --abort-on-container-exit: stop all containers when any exits
  # --exit-code-from e2e: propagate the Playwright exit code
  set +e
  $COMPOSE up \
    --abort-on-container-exit \
    --exit-code-from e2e
  EXIT_CODE=$?
  set -e
fi

# Note: cleanup is handled by the trap above (fires on exit $EXIT_CODE below)

echo ""
if [ "$EXIT_CODE" -eq 0 ]; then
  echo "════════════════════════════════════════"
  echo "  E2E PASSED ✓"
  echo "  Report: tests/e2e/playwright-report/index.html"
  echo "════════════════════════════════════════"
else
  echo "════════════════════════════════════════"
  echo "  E2E FAILED ✗  (exit code $EXIT_CODE)"
  echo "  Report: tests/e2e/playwright-report/index.html"
  echo "════════════════════════════════════════"
fi

exit $EXIT_CODE
