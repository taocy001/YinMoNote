package main

import "embed"

// staticFiles embeds the compiled Vue frontend into the binary at build time.
// The dist/ directory is populated by the build pipeline:
//   - Docker:  COPY --from=frontend-builder in backend-builder stage
//   - Native:  build/build.sh copies frontend/dist → backend/dist
//
// backend/dist/.gitkeep is committed so that go test works without a prior
// frontend build; the real dist/ is copied in by the build scripts.
//
// "all:" includes hidden files (e.g. .gitkeep) so the embed works even when
// dist/ contains only the placeholder.
//
//go:embed all:dist
var staticFiles embed.FS
