import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// A base-36 timestamp embedded in the bundle at build time. The service worker
// registration URL carries it as ?v=<stamp>, and sw.js reads it back from
// self.location to use as the cache name. This ensures every deployment gets a
// fresh cache name and old entries are evicted on activate.
const BUILD_STAMP = Date.now().toString(36)

// Read the canonical version from the VERSION file.
// Primary path: repo root (local dev).  Fallback: same directory (Docker build
// copies VERSION into the frontend context when the repo root isn't mounted).
let APP_VERSION = '0.0.0'
for (const rel of ['../VERSION', './VERSION']) {
  try { APP_VERSION = readFileSync(resolve(__dirname, rel), 'utf-8').trim(); break } catch {}
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
  define: {
    __SW_VERSION__: JSON.stringify(BUILD_STAMP),
    // Expose via import.meta.env so Vitest's source-file transforms also see it.
    'import.meta.env.VITE_APP_VERSION': JSON.stringify(APP_VERSION),
  },
  test: {
    environment: 'happy-dom',
    // In Docker: COPY frontend/ . places frontend/tests/ at /app/tests/
    // Locally: tests/ is relative to this vite.config.ts (frontend/tests/)
    include: ['tests/**/*.test.ts'],
    exclude: ['**/node_modules/**'],
  },
} as any)
