import js from '@eslint/js'
import pluginVue from 'eslint-plugin-vue'
import tsParser from '@typescript-eslint/parser'
import tsPlugin from '@typescript-eslint/eslint-plugin'
import prettier from 'eslint-config-prettier'

// All browser + Node globals used across the project.
const browserGlobals = {
  // Core
  window: 'readonly',
  document: 'readonly',
  navigator: 'readonly',
  location: 'readonly',
  history: 'readonly',
  screen: 'readonly',
  // Storage
  localStorage: 'readonly',
  sessionStorage: 'readonly',
  indexedDB: 'readonly',
  IDBDatabase: 'readonly',
  // Fetch / network
  fetch: 'readonly',
  URL: 'readonly',
  URLSearchParams: 'readonly',
  Headers: 'readonly',
  Request: 'readonly',
  Response: 'readonly',
  AbortController: 'readonly',
  AbortSignal: 'readonly',
  // Files / blobs
  Blob: 'readonly',
  File: 'readonly',
  FileReader: 'readonly',
  FormData: 'readonly',
  DataTransfer: 'readonly',
  FileSystemEntry: 'readonly',
  FileSystemFileEntry: 'readonly',
  FileSystemDirectoryEntry: 'readonly',
  FileSystemFileHandle: 'readonly',
  // DOM events
  Event: 'readonly',
  CustomEvent: 'readonly',
  EventTarget: 'readonly',
  KeyboardEvent: 'readonly',
  MouseEvent: 'readonly',
  WheelEvent: 'readonly',
  DragEvent: 'readonly',
  MessageEvent: 'readonly',
  BeforeUnloadEvent: 'readonly',
  MediaQueryListEvent: 'readonly',
  // DOM elements
  HTMLElement: 'readonly',
  HTMLInputElement: 'readonly',
  HTMLTextAreaElement: 'readonly',
  HTMLAnchorElement: 'readonly',
  HTMLImageElement: 'readonly',
  HTMLSelectElement: 'readonly',
  Element: 'readonly',
  Node: 'readonly',
  NodeList: 'readonly',
  NodeFilter: 'readonly',
  DOMParser: 'readonly',
  // Observers
  MutationObserver: 'readonly',
  ResizeObserver: 'readonly',
  IntersectionObserver: 'readonly',
  // Timers / animation
  requestAnimationFrame: 'readonly',
  cancelAnimationFrame: 'readonly',
  setTimeout: 'readonly',
  clearTimeout: 'readonly',
  setInterval: 'readonly',
  clearInterval: 'readonly',
  // Crypto / encoding
  crypto: 'readonly',
  CryptoKey: 'readonly',
  TextEncoder: 'readonly',
  TextDecoder: 'readonly',
  atob: 'readonly',
  btoa: 'readonly',
  // Misc browser APIs
  performance: 'readonly',
  BroadcastChannel: 'readonly',
  getComputedStyle: 'readonly',
  matchMedia: 'readonly',
  alert: 'readonly',
  confirm: 'readonly',
  prompt: 'readonly',
  console: 'readonly',
  // Node/Vite
  process: 'readonly',
  __dirname: 'readonly',
}

const tsRules = {
  ...tsPlugin.configs.recommended.rules,
  // Warn on `any` rather than error — avoids blocking CI on AI-generated code.
  '@typescript-eslint/no-explicit-any': 'warn',
  // Unused variables are almost always bugs.
  '@typescript-eslint/no-unused-vars': [
    'error',
    { argsIgnorePattern: '^_', varsIgnorePattern: '^_', caughtErrorsIgnorePattern: '^_' },
  ],
  '@typescript-eslint/no-non-null-assertion': 'warn',
  // Allow empty catch blocks with a comment.
  'no-empty': ['error', { allowEmptyCatch: false }],
}

export default [
  js.configs.recommended,

  // TypeScript source files
  {
    files: ['**/*.ts', '**/*.tsx'],
    languageOptions: {
      parser: tsParser,
      parserOptions: { ecmaVersion: 'latest', sourceType: 'module' },
      globals: browserGlobals,
    },
    plugins: { '@typescript-eslint': tsPlugin },
    rules: tsRules,
  },

  // Vue SFC files — must come after the TS block so Vue rules take precedence.
  ...pluginVue.configs['flat/recommended'].map((cfg) => ({
    ...cfg,
    files: ['**/*.vue'],
  })),
  {
    files: ['**/*.vue'],
    languageOptions: {
      parser: (await import('vue-eslint-parser')).default,
      parserOptions: {
        parser: tsParser,
        ecmaVersion: 'latest',
        sourceType: 'module',
        extraFileExtensions: ['.vue'],
      },
      globals: browserGlobals,
    },
    plugins: { '@typescript-eslint': tsPlugin },
    rules: {
      ...tsRules,
      'vue/component-name-in-template-casing': ['error', 'PascalCase'],
      'vue/multi-word-component-names': 'off',
      // v-html is used deliberately for rendered content.
      'vue/no-v-html': 'off',
      // TypeScript-typed props don't need default values.
      'vue/require-default-prop': 'off',
    },
  },

  // Disable all formatting rules — Prettier owns formatting.
  prettier,

  {
    ignores: ['dist/', 'node_modules/', 'coverage/'],
  },
]
