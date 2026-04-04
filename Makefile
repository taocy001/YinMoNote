# ──────────────────────────────────────────────────────────────────────────────
#  YinMoNote — build, test, package, and install entry point
#
#  Variables (can be overridden via environment or command line):
#    DOCKER    — set to 1 to force building inside Docker (default 0: native when Go+Node available, Docker otherwise)
#
#  Version number is read from the VERSION file (project root).
#
#  Quick start:
#
#    make                        # compile the binary for the current platform
#    make install                # install to /usr/local/bin/ (requires make first)
#    make docker                 # build Docker image (current architecture)
#    make install-docker         # package + deploy Docker container
#    make release                # build all packages for all platforms
#    make help                   # show all available targets
#    ./tests/test.sh             # run backend + frontend unit tests
#    ./tests/test.sh e2e         # run E2E tests (Docker + Playwright)
# ──────────────────────────────────────────────────────────────────────────────

VERSION  := $(shell cat VERSION 2>/dev/null | tr -d '[:space:]' || echo "dev")
ARCH     := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
OS       := $(shell uname -s | tr '[:upper:]' '[:lower:]')
DOCKER   ?= 0

.DEFAULT_GOAL := build
.PHONY: build release docker \
        install uninstall install-docker \
        clean help

# ── Build ──────────────────────────────────────────────────────────────────────

build: ## compile the binary for the current platform → dist/yinmonote-$(OS)-$(ARCH)
	DOCKER=$(DOCKER) ./build/build.sh $(OS) "$(ARCH)"
	@echo ""
	@echo "  Tip: run make help to see all available targets"

docker: ## build Docker image (host architecture) → dist/yinmonote-$(VERSION)-docker-$(ARCH).tar
	@command -v docker >/dev/null 2>&1 \
	    || { echo "Error: Docker is not installed"; exit 1; }
	./build/package-docker.sh "$(VERSION)" "$(ARCH)"

release: ## build all packages for all platforms → dist/
	@echo ""
	@echo "════════════════════════════════════════"
	@echo "  Release build v$(VERSION) (DOCKER=$(DOCKER))"
	@echo "════════════════════════════════════════"
	@echo ""
	@echo "── [1/5] Docker image linux/amd64 ──────"
	@if command -v docker >/dev/null 2>&1; then \
	    ./build/package-docker.sh "$(VERSION)" amd64 || echo "⚠ Docker linux/amd64 build failed, skipping"; \
	else \
	    echo "⚠ Skipping Docker image (Docker not installed)"; \
	fi
	@echo ""
	@echo "── [2/5] Docker image linux/arm64 ──────"
	@if command -v docker >/dev/null 2>&1; then \
	    ./build/package-docker.sh "$(VERSION)" arm64 || echo "⚠ Docker linux/arm64 build failed, skipping"; \
	else \
	    echo "⚠ Skipping Docker image (Docker not installed)"; \
	fi
	@echo ""
	@echo "── [3/5] Linux/amd64 .deb ─────────────"
	@DOCKER=$(DOCKER) ./build/build.sh linux amd64 \
	    && ./build/package-deb.sh "$(VERSION)" amd64 \
	    && rm -f dist/yinmonote-linux-amd64 \
	    || echo "⚠ linux/amd64 deb build failed, skipping"
	@echo ""
	@echo "── [4/5] Linux/arm64 .deb ─────────────"
	@DOCKER=$(DOCKER) ./build/build.sh linux arm64 \
	    && ./build/package-deb.sh "$(VERSION)" arm64 \
	    && rm -f dist/yinmonote-linux-arm64 \
	    || echo "⚠ linux/arm64 deb build failed, skipping"
	@echo ""
	@echo "── [5/5] macOS/arm64 .dmg ─────────────"
	@if [ "$(OS)" = "darwin" ]; then \
	    DOCKER=$(DOCKER) ./build/build.sh darwin arm64 \
	        && ./build/package-dmg.sh "$(VERSION)" arm64 \
	        && rm -f dist/yinmonote-darwin-arm64 \
	        || echo "⚠ darwin/arm64 dmg build failed, skipping"; \
	else \
	    echo "⚠ Skipping macOS dmg (must run on macOS)"; \
	fi
	@echo ""
	@echo "── [6/6] Windows/amd64 .zip ────────────"
	@DOCKER=$(DOCKER) ./build/build.sh windows amd64 \
	    && ./build/package-zip.sh "$(VERSION)" amd64 \
	    && rm -f dist/yinmonote-windows-amd64.exe \
	    || echo "⚠ windows/amd64 build failed, skipping"
	@echo ""
	@echo "════════════════════════════════════════"
	@echo "  Done. Artifacts in dist/"
	@echo "════════════════════════════════════════"
	@echo ""
	@ls -lh dist/ 2>/dev/null || true

# ── Install / Uninstall ────────────────────────────────────────────────────────

install: ## install/upgrade: prompts for data directory and port, starts service
	./build/install.sh dist/yinmonote-$(OS)-$(ARCH)

uninstall: ## uninstall the binary and stop the LaunchAgent service (macOS)
	@PLIST="$$HOME/Library/LaunchAgents/com.yinmonote.plist"; \
	if [ -f "$$PLIST" ]; then \
	    echo "==> Stopping and removing LaunchAgent..."; \
	    launchctl unload "$$PLIST" 2>/dev/null || true; \
	    rm -f "$$PLIST"; \
	fi
	sudo rm -f /usr/local/bin/yinmonote
	@echo "Uninstalled. Data directory preserved — remove ~/.yinmonote/ manually if desired."

install-docker: ## package Docker image and deploy container (interactive)
	@command -v docker >/dev/null 2>&1 \
	    || { echo "Error: Docker is not installed"; exit 1; }
	$(MAKE) docker
	./build/install-docker.sh "$(VERSION)"

# ── Clean ──────────────────────────────────────────────────────────────────────

clean: ## delete all build artifacts (dist/, backend/dist/)
	rm -rf dist/ backend/dist/
	mkdir -p backend/dist && touch backend/dist/.gitkeep
	@echo "Cleaned."

# ── Help ───────────────────────────────────────────────────────────────────────

help: ## show all available targets
	@printf "\nUsage: make [target] [DOCKER=0|1] [DATA_DIR=path]\n"
	@printf "Version: $(VERSION)   Arch: $(OS)/$(ARCH)\n\n"
	@awk 'BEGIN{FS=":.*?## "}/^[a-zA-Z0-9_-]+:.*?## /{printf "  \033[36m%-16s\033[0m %s\n",$$1,$$2}' \
	    $(MAKEFILE_LIST)
	@printf "\n"
