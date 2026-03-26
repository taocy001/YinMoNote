#!/bin/bash
# tests/test.sh [backend|frontend|e2e|all]
#
# Run the test suite inside Docker. Defaults to "all" (backend + frontend unit tests).
# Unit tests run as Docker build stages; E2E tests use Docker Compose + Playwright.
#
# Usage:
#   ./tests/test.sh               # backend + frontend unit tests
#   ./tests/test.sh all           # same as above
#   ./tests/test.sh backend       # backend unit tests only
#   ./tests/test.sh frontend      # frontend unit tests only
#   ./tests/test.sh e2e           # E2E tests only
#   ./tests/test.sh e2e --no-build specs/06   # pass extra args to run-e2e.sh

set -e

# Enable BuildKit for Docker build cache mounts (required on Debian/Ubuntu
# where docker.io package does not enable BuildKit by default).
export DOCKER_BUILDKIT=1

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

TYPE="${1:-all}"
shift 2>/dev/null || true   # remaining args forwarded to e2e

if ! command -v docker >/dev/null 2>&1; then
    echo "Error: Docker is required to run tests"
    exit 1
fi

# ── helpers ───────────────────────────────────────────────────────────────────

# Build a named test image from a Dockerfile stage (silently, using cache),
# then run it to get clean test output uncontaminated by Docker build noise.
build_and_run() {
    local stage="$1"
    local tag="$2"
    docker build --target "$stage" --quiet \
        -f "$PROJECT_ROOT/build/Dockerfile" "$PROJECT_ROOT" \
        -t "$tag" >/dev/null
    docker run --rm "$tag"
}

run_backend() {
    echo ""
    echo "════════════════════════════════════════"
    echo "  Backend unit tests  (Go test)"
    echo "════════════════════════════════════════"
    build_and_run backend-test yinmonote-test-backend
}

run_frontend() {
    echo ""
    echo "════════════════════════════════════════"
    echo "  Frontend unit tests  (Vitest)"
    echo "════════════════════════════════════════"
    build_and_run frontend-test yinmonote-test-frontend
}

run_e2e() {
    "$SCRIPT_DIR/run-e2e.sh" "$@"
}

# ── dispatch ──────────────────────────────────────────────────────────────────

case "$TYPE" in
    backend)  run_backend ;;
    frontend) run_frontend ;;
    e2e)      run_e2e "$@" ;;
    all)
        BACKEND_EXIT=0
        FRONTEND_EXIT=0

        set +e
        run_backend;  BACKEND_EXIT=$?
        run_frontend; FRONTEND_EXIT=$?
        set -e

        echo ""
        echo "════════════════════════════════════════"
        echo "  Test Summary"
        echo "════════════════════════════════════════"
        if [ "$BACKEND_EXIT" -eq 0 ]; then
            echo "  Backend   ✓ PASSED"
        else
            echo "  Backend   ✗ FAILED (exit $BACKEND_EXIT)"
        fi
        if [ "$FRONTEND_EXIT" -eq 0 ]; then
            echo "  Frontend  ✓ PASSED"
        else
            echo "  Frontend  ✗ FAILED (exit $FRONTEND_EXIT)"
        fi
        echo "════════════════════════════════════════"

        [ "$BACKEND_EXIT" -eq 0 ] && [ "$FRONTEND_EXIT" -eq 0 ]
        ;;
    *)
        echo "Usage: $0 [backend|frontend|e2e|all]"
        echo "  backend   backend unit tests (Go test)"
        echo "  frontend  frontend unit tests (Vitest)"
        echo "  e2e       E2E tests (Docker + Playwright)"
        echo "  all       backend + frontend (default)"
        exit 1
        ;;
esac
