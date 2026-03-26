/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Injected at build time from the repo-root VERSION file via vite.config.ts. */
  readonly VITE_APP_VERSION: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
