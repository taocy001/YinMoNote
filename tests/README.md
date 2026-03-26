# tests/

This directory contains all test infrastructure for YinMoNote.

---

## File structure

```
tests/
├── test.sh                   # Unified test entry point (backend / frontend / e2e)
├── run-e2e.sh                # E2E test runner (also called by test.sh)
├── gen-stress-data.sh        # Stress-test data generator
├── Dockerfile.playwright     # Playwright runner image (yinmonote-playwright:latest)
├── Dockerfile.playwright-patch  # Lightweight patch for quick spec-only re-runs
├── docker-compose.e2e.yml    # E2E two-container stack (app + playwright)
└── e2e/
    ├── package.json
    ├── playwright.config.ts
    ├── fixtures.ts
    ├── helpers/
    │   └── app.ts            # Shared helpers: freshPage, unlock, createNote, …
    └── specs/
        ├── 01-unlock.spec.ts          # 14 cases: first unlock, password / WebAuthn flow
        ├── 02-notes-crud.spec.ts      # 12 cases: create / edit / delete notes
        ├── 03-editor-features.spec.ts # 13 cases: formatting, code blocks, export
        ├── 04-sidebar-search.spec.ts  #  9 cases: title search, tag filter
        ├── 05-settings.spec.ts        # 16 cases: appearance / editor / quota settings
        ├── 06-history.spec.ts         #  9 cases: version history, diff, rollback
        ├── 07-lock-unlock.spec.ts     #  8 cases: lock / re-unlock
        ├── 08-server-encrypt.spec.ts  #  9 cases: serverEncrypt mode
        ├── 09-encryption-modes.spec.ts #  5 cases: encryption mode switching
        ├── 10-coverage.spec.ts        #  7 cases: empty search, multi-note persistence, save status
        └── 11-multi-device.spec.ts    # 13 cases: new-device login, concurrent sessions, re-lock
```

Unit tests live alongside their source:

```
frontend/tests/    # Vitest unit tests  (226 cases)
backend/*_test.go  # Go unit tests      (232 cases)
```

---

## Running tests

All tests run inside Docker containers to guarantee environment consistency.
Docker must be installed and running.

### Unit tests

```bash
# Backend + frontend (default)
./tests/test.sh

# Backend only (Go test)
./tests/test.sh backend

# Frontend only (Vitest)
./tests/test.sh frontend
```

`test.sh` uses a two-stage process: `docker build --target <stage> --quiet` to
build the test image (silent, uses BuildKit cache), then `docker run --rm` to
get clean test output without Docker build noise.

Unit tests also run automatically during `make docker` / `make build` — a
failing test aborts the image build.

### E2E tests

```bash
# Full run — build images + run all specs
./tests/test.sh e2e
./tests/run-e2e.sh          # equivalent

# Skip image rebuild (fastest re-run when only test code changed)
./tests/run-e2e.sh --no-build

# Run specific spec files
./tests/run-e2e.sh specs/06
./tests/run-e2e.sh --no-build specs/06 specs/07

# View the HTML report (screenshots, timeline)
open tests/e2e/playwright-report/index.html
```

**Docker images used:**
- `yinmonote:e2e` — the app under test (built with `VITE_PBKDF2_ITERATIONS=1000`)
- `yinmonote-playwright:latest` — the Playwright runner

Both images are automatically removed by the cleanup trap after every run.

---

## Stress-test data

Generate a large batch of notes that exercise every supported Markdown element
(H1–H6, inline formatting, code blocks, KaTeX math, Mermaid diagrams, callouts,
toggle blocks, tables, …):

```bash
./tests/gen-stress-data.sh --help

# Examples
./tests/gen-stress-data.sh                                   # 100 notes, 20–200 KB, mixed lang
./tests/gen-stress-data.sh --notes 1000                     # 1 000 notes
./tests/gen-stress-data.sh --notes 500 --lang zh            # Chinese content
./tests/gen-stress-data.sh --notes 200 --min-size 50 --max-size 300 --data-dir /tmp/stress
```

The script writes files directly into the target notes directory and updates
`config.json` quotas so the app accepts the batch.

---

See [docs/testing-guide.md](../docs/testing-guide.md) for architecture rationale,
coverage details, design principles, and test count summary.
