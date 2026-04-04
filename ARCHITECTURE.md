# YinMoNote Architecture

Engineering reference. Facts only.

---

## System Diagram

```
┌─────────────────────────────┐
│  Browser (Vue 3 SPA)        │
│  Tiptap editor              │
│  IndexedDB (ENC1 cache)     │
└────────────┬────────────────┘
             │ HTTPS / HTTP (Bearer token)
             ▼
┌─────────────────────────────┐
│  Go HTTP Server (Gin)       │
│  Auth middleware            │
│  Rate limiter               │
│  WebDAV handler             │
│  MCP JSON-RPC 2.0           │
└────────────┬────────────────┘
             │ flat-file I/O + atomic writes
             ▼
┌─────────────────────────────┐
│  File System                │
│  ~/.yinmonote/config.json   │
│  DATA_DIR/  (notes + git)   │
└─────────────────────────────┘
```

---

## Backend File Responsibilities

| File | Responsibility |
|---|---|
| `server.go` | HTTP routing, auth middleware, rate limiting (cap: 1000 IPs, 500 ms/failure backoff, max 5 s) |
| `auth_srp.go` | SRP-6a handshake: `POST /api/auth/srp/init` + `POST /api/auth/srp/verify`; issues Bearer tokens (24 h TTL) |
| `library.go` / `library_*.go` | `NoteLibrary`: CRUD, folder structure, trash, git version history, utility helpers |
| `mcp.go` / `mcp_policy.go` | Model Context Protocol server (JSON-RPC 2.0); policy controls what the AI can read/write |
| `webdav.go` | WebDAV with title virtualization — exposes human-readable filenames derived from note titles |
| `config.go` | Config load + value clamping; persists to `~/.yinmonote/config.json` |
| `selfca.go` | Self-signed TLS CA + cert generation; CA downloadable at `/ca.crt` |

---

## Frontend State Model

No Pinia, no Vuex. Global state lives in composables surfaced through Vue's `provide`/`inject`.

- `useLibrary` — single source of truth for the note list, folder tree, and active note.
- `useEditor` — Tiptap editor instance and document state.
- `useAuth` — authentication state and token management.
- Components call composable functions directly; they do not own state.

---

## Key Invariants — Do Not Break

| Invariant | Why |
|---|---|
| `sync.Mutex` (not `sync.RWMutex`) on `NoteLibrary` | Write-heavy workload; RWMutex would give no benefit and adds complexity |
| `AtomicWrite`: write `.tmp` → `os.Rename` | Crash-safe on all target platforms; never write directly to the target path |
| `subtle.ConstantTimeCompare` for all token comparisons | Prevents timing-based token oracle attacks |
| IndexedDB stores ENC1 ciphertext only | Plaintext is never persisted to disk in encrypted mode |
| All colors via CSS variables | Never hardcode color values in components; theming must remain consistent |

---

## Request Flow

```
Browser  →  POST /api/auth/srp/init   →  server returns salt + server public key
Browser  →  POST /api/auth/srp/verify →  server returns Bearer token (24 h)
Browser  →  GET/POST /api/notes/...   →  Authorization: Bearer <token>  →  auth middleware validates
```

WebAuthn follows the same pattern: credential assertion → Bearer token.

---

## Encrypted Note Data Flow

```
Client side:
  plaintext  →  AES-256-GCM (PBKDF2 key, 100k iterations)  →  ENC1 blob

Upload:   PUT /api/notes/:id  body = ENC1 blob
Server:   stores blob blindly — never sees plaintext

Download: GET /api/notes/:id  →  ENC1 blob
Client:   ENC1 blob  →  AES-256-GCM decrypt  →  plaintext
```

Key derivation uses PBKDF2-SHA256 (100 000 iterations). WebAuthn PRF extension can supply the key material instead of a password.

---

## Build Pipeline

```
1. cd frontend && npm run build
   Output → backend/dist/   (static assets)

2. cd backend && go build
   go:embed backend/dist embeds the frontend into the binary

3. Single binary: dist/yinmonote-<os>-<arch>
   No separate static file serving required at runtime.
```

`make build` runs both steps. `make docker` additionally produces a `.tar` image in `dist/`.
