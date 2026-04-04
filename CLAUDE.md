# CLAUDE.md — YinMoNote Project Context

## Project Overview
YinMoNote: self-hosted, privacy-first, single-user notes app. Vue 3 SPA + Go backend. E2EE optional.

## Language Rules
- **Communication / docs / comments**: English in code, Chinese in conversation with user
- **Commit messages**: English only

## Build Process (order matters)
```bash
cd frontend && npm run build
cd .. && rm -rf backend/dist && cp -r frontend/dist backend/dist
cd backend && go build -o yinmonote .
```
Shortcuts: `make build` / `make install` / `make docker`

## Deployment
Target: `vpc_tengxun` via passwordless SSH.
SSH socket: `SSH_AUTH_SOCK=$(ls /tmp/ssh-*/agent.* 2>/dev/null | head -1)`

## Running Tests
```bash
./tests/test.sh backend    # Go unit tests
./tests/test.sh frontend   # Vitest
./tests/test.sh e2e        # Playwright (requires Docker)
./tests/test.sh            # All
```
`TableOverlay.vue` has no unit tests — test via E2E (Playwright) only.

## Key File Locations
```
frontend/src/
  components/          # Vue components (TableOverlay.vue, etc.)
  composables/         # useLibrary.ts (central state hub), useEditorSave.ts
  assets/editor-prose.css  # ProseMirror + table CSS
  style.css            # Global styles + CSS design token utilities
  i18n.ts              # All UI strings (zh + en)
backend/
  server.go            # HTTP routing, auth, rate limiting
  auth_srp.go          # SRP-6a authentication
  library*.go          # NoteLibrary CRUD, trash, git, util
  mcp.go / mcp_policy.go
  webdav.go
docs/
  design.md            # Full architecture + feature matrix (Chinese)
  security.md          # Security model (Chinese)
  testing-guide.md     # Test layers and coverage (Chinese)
```

## Tiptap v3 Gotchas (v3.20.2)
- `selectRow()` and `selectColumn()` were **removed** in v3 — use direct ProseMirror transactions instead.
- `posAtDOM(el, 0)` returns position of the element's *open token*. Use `posAtDOM(el, 0) + 1` to place the cursor at the first content position inside the element.

## Frontend Conventions
- **State management**: No Pinia/Vuex. Use composables + `provide`/`inject`.
- **Colors**: Always use CSS variables (`var(--color-*)`) — no hardcoded color values in components.
- **Touch detection**: `matchMedia('(pointer: coarse)')` — do NOT use `navigator.maxTouchPoints`.
- **Clipboard**: Use `ClipboardItem` API only (HTTPS). `execCommand` fallback is intentionally removed for security.

## Backend Conventions
- `sync.Mutex` (not `RWMutex`) — write-heavy workload makes RWMutex counterproductive.
- `AtomicWrite`: writes to `.tmp` then renames — preserves crash safety. Always use this pattern for file writes.
- All token comparisons use `subtle.ConstantTimeCompare` — never use `==` for secrets.
- `authFailures` map in `server.go` is capped at 1000 entries (`maxAuthFailureEntries`) to prevent unbounded memory growth.
- Auth protocol: SRP-6a (RFC 5054, 2048-bit) — not Basic Auth.

## Security / Encryption
- IndexedDB caches **ENC1 ciphertext**, not plaintext, in encrypted mode.
- SRP-6a means the server never sees the user's password — keep this invariant intact.
