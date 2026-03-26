# build/

Build scripts, Dockerfiles, and service configuration for YinMoNote.

---

## File structure

| File | Purpose |
|---|---|
| `Dockerfile` | Multi-stage image: frontend build → backend build → production runtime |
| `docker-compose.yml` | Production container definition (used by `install-docker.sh`) |
| `build.sh` | Compile a native binary for Linux or macOS (auto-falls back to Docker) |
| `package-docker.sh` | Build a Docker image and save it as a distributable `.tar` |
| `package-deb.sh` | Package the Linux binary as a `.deb` installer |
| `package-dmg.sh` | Package the macOS binary as a `.dmg` installer |
| `install.sh` | Interactive native installer (data dir + port + systemd / LaunchAgent) |
| `install-docker.sh` | Interactive Docker installer (load image, start container) |
| `yinmonote.service` | systemd user service template (used by `install.sh` on Linux) |

---

## Build commands

All `make` targets delegate to the scripts in this directory.

```bash
make build            # native binary for the current platform → dist/yinmonote-<os>-<arch>
make docker           # Docker image → dist/yinmonote-<version>-docker-<arch>.tar
make release          # all platforms: docker ×2 + .deb ×2 + .dmg → dist/
make install          # build + interactive native install (systemd / LaunchAgent)
make install-docker   # make docker + interactive Docker install
make clean            # remove dist/ and backend/dist/
```

Force Docker for native builds (useful when Go / Node are not installed locally):

```bash
DOCKER=1 make build
DOCKER=1 make build   # cross-compile
```

Cross-compile via `build.sh` directly:

```bash
./build/build.sh linux  amd64   # Linux x86-64
./build/build.sh linux  arm64   # Linux ARM64 (Raspberry Pi, Graviton)
./build/build.sh darwin arm64   # macOS Apple Silicon
```

---

## Dockerfile — multi-stage architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│ Stage: frontend-builder                                             │
│   Base: node:20-alpine (Aliyun mirror)                              │
│   npm install (npmmirror) → npm run test → npm run build            │
│   Output: /app/dist  (bundled Vue SPA)                              │
├─────────────────────────────────────────────────────────────────────┤
│ Stage: backend-builder                                              │
│   Base: golang:1.21-alpine (Aliyun mirror)                          │
│   GOPROXY=goproxy.cn                                                │
│   go mod download → go test ./... → go build                        │
│   Embeds /app/dist via go:embed → single self-contained binary      │
│   Output: /app/YinMoNote                                            │
├─────────────────────────────────────────────────────────────────────┤
│ Stage: frontend-test  (extends frontend-builder, CMD: npm run test) │
│ Stage: backend-test   (extends backend-builder,  CMD: go test)      │
│   Used by tests/test.sh for isolated, noise-free test output        │
├─────────────────────────────────────────────────────────────────────┤
│ Stage: production runtime                                           │
│   Base: debian:bookworm (Aliyun apt mirror)                         │
│   Runs as non-root user yinmonote (UID/GID 1000)                    │
│   Copies /app/YinMoNote → /home/yinmonote/YinMoNote                 │
│   EXPOSE 8080,  CMD ./YinMoNote                                     │
└─────────────────────────────────────────────────────────────────────┘
```

Unit tests are embedded in the build: a test failure in either stage aborts
the image build, making `make docker` an implicit CI gate.

---

## Docker image naming

| Image | Built by | Lifecycle |
|---|---|---|
| `yinmonote:<version>` / `yinmonote:latest` | `package-docker.sh` | Persistent — replaced on each new build |
| `yinmonote:e2e` | `tests/run-e2e.sh` | Ephemeral — removed after every E2E run |
| `yinmonote-playwright:latest` | `tests/run-e2e.sh` | Ephemeral — removed after every E2E run |
| `yinmonote-test-backend:latest` | `tests/test.sh` | Reused across runs (same tag overwrites) |
| `yinmonote-test-frontend:latest` | `tests/test.sh` | Reused across runs (same tag overwrites) |
| `yinmonote-build:tmp` | `build.sh` (DOCKER=1) | Deleted immediately after binary is extracted |

`package-docker.sh` removes the old image with the same tag before rebuilding,
and removes the new image after saving to tar — the `.tar` is the distribution
artifact, not the local image.

---

## App environment variables

These are passed to the container or the native binary at runtime (not at build time).

| Variable | Default | Description |
|---|---|---|
| `DATA_DIR` | `~/.yinmonote/notes` | Notes, assets, and Git history |
| `CONFIG_FILE` | `~/.yinmonote/config.json` | App config (quotas, appearance, …) |
| `PORT` | `:8080` | Listen address, format `:port` |
| `TZ` | — | Timezone (e.g. `Asia/Shanghai`) |
| `SYNC_COMMIT` | — | Set to `1` to commit on every save (used in E2E tests) |
| `ACME_DOMAIN` | — | Enable automatic TLS via Let's Encrypt (needs port 443) |
| `TLS_CERT` / `TLS_KEY` | — | Custom TLS: path to PEM certificate and private key |
| `TLS_SELF` | — | Set to `1` to generate a self-signed certificate (LAN / intranet). **Note:** biometric/fingerprint unlock (WebAuthn) requires a domain name and is automatically hidden when accessed via IP address. Use `ACME_DOMAIN` or a reverse proxy with a real domain to enable biometric unlock. |
| `TLS_EXTRA_IPS` | — | Extra SAN IPs for self-signed cert, comma-separated |
| `WEBDAV_DISABLED` | — | Set to `1` to disable the WebDAV endpoint (`/dav/`) |
| `ALLOWED_ORIGIN` | — | CORS origin (only needed when frontend and backend run on different origins) |

`VITE_PBKDF2_ITERATIONS` is a **build-time** arg (default 100 000; set to 1 000
in E2E builds via `docker-compose.e2e.yml` to prevent test timeouts).

---

## Build output

All artifacts land in `dist/`:

| File | Platform |
|---|---|
| `yinmonote-linux-amd64` | Linux x86-64 native binary |
| `yinmonote-linux-arm64` | Linux ARM64 native binary |
| `yinmonote-darwin-arm64` | macOS Apple Silicon native binary |
| `yinmonote-<ver>-docker-amd64.tar` | Docker image (linux/amd64) |
| `yinmonote-<ver>-docker-arm64.tar` | Docker image (linux/arm64) |
| `yinmonote_<ver>_amd64.deb` | Debian/Ubuntu package (x86-64) |
| `yinmonote_<ver>_arm64.deb` | Debian/Ubuntu package (ARM64) |
| `yinmonote-<ver>-macos-arm64.dmg` | macOS installer |

---

For quick start instructions see the project [README.md](../README.md).
