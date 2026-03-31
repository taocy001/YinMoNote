# ── Stage 1: compile Vue frontend (includes frontend unit tests) ──────────────
FROM node:20-alpine AS frontend-builder
# Use Tsinghua mirror for Alpine
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
WORKDIR /app
COPY frontend/package*.json ./
# Use npmmirror for NPM packages
RUN npm config set registry https://registry.npmmirror.com
# --mount=type=cache pins ~/.npm to a BuildKit persistent cache so npm packages
# are not re-downloaded when other layers are invalidated.
RUN --mount=type=cache,target=/root/.npm \
    npm install
# VERSION is at the repo root; copy it here so vite.config.ts can read it
# as ./VERSION (the ../VERSION path is not available inside the container).
COPY VERSION .
COPY frontend/ .
RUN npm run test
# VITE_PBKDF2_ITERATIONS: overridable at build time.  Defaults to 100 000
# (OWASP 2023 recommendation).  E2E test builds set this to 1 000 so
# Alpine Chromium's PBKDF2 completes in milliseconds, not tens of seconds.
ARG VITE_PBKDF2_ITERATIONS=100000
ENV VITE_PBKDF2_ITERATIONS=$VITE_PBKDF2_ITERATIONS
RUN npm run build

# ── Stage 2: compile Go backend (includes backend unit tests) ─────────────────
FROM golang:1.21-alpine AS backend-builder
# Use Tsinghua mirror for Alpine
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apk/repositories
# TARGETOS / TARGETARCH are injected automatically by Docker buildx --platform.
# Without --platform they default to the host's native platform (linux/amd64 or
# linux/arm64).  For extracting a native binary they can be overridden via
# --build-arg, e.g.: docker build --build-arg TARGETOS=darwin --build-arg TARGETARCH=arm64
ARG TARGETOS
ARG TARGETARCH
RUN apk add --no-cache ca-certificates
# Use goproxy.cn for Go modules
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
# --mount=type=cache pins Go module and build caches to BuildKit persistent storage
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go mod download
COPY backend/ .
# Copy compiled frontend for go:embed to bundle into the binary
COPY --from=frontend-builder /app/dist ./dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go test ./... -v
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o YinMoNote .

# ── Stage: frontend test runner ───────────────────────────────────────────────
# Inherits built deps from frontend-builder; CMD runs tests in a container
# so test.sh gets clean vitest output without Docker build noise.
FROM frontend-builder AS frontend-test
CMD ["npm", "run", "test"]

# ── Stage: backend test runner ────────────────────────────────────────────────
# Inherits compiled source + frontend dist from backend-builder; CMD runs
# go test so test.sh gets clean output via `docker run`.
FROM backend-builder AS backend-test
CMD ["go", "test", "./...", "-v"]

# ── Stage 3: production runtime ───────────────────────────────────────────────
FROM debian:bookworm
# Use Tsinghua mirror for Debian
RUN sed -i 's/deb.debian.org/mirrors.tuna.tsinghua.edu.cn/g' /etc/apt/sources.list.d/debian.sources
RUN apt-get update \
 && apt-get install -y --no-install-recommends tzdata git ca-certificates curl \
 && rm -rf /var/lib/apt/lists/*
# SEC-019: Run as a non-root user to limit container escape impact.
# Fixed UID/GID 1000 makes bind-mount ownership predictable on most Linux hosts.
# Data dir defaults to /home/yinmonote/.yinmonote; override with DATA_DIR env var.
RUN groupadd -r -g 1000 yinmonote \
 && useradd -r -u 1000 -g yinmonote -m -d /home/yinmonote -s /bin/sh yinmonote \
 && mkdir -p /data && chown yinmonote:yinmonote /data
WORKDIR /home/yinmonote
# Frontend is bundled into the binary via go:embed — no separate dist copy needed
COPY --from=backend-builder --chown=yinmonote:yinmonote /app/YinMoNote .
USER yinmonote

EXPOSE 8080
CMD ["./YinMoNote"]

