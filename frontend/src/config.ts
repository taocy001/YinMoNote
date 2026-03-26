/**
 * Runtime configuration for YinMoNote frontend.
 *
 * DEFAULTS is the single source of truth for every constant. At startup,
 * /config.json is fetched; any recognised keys found there override the
 * corresponding defaults. A missing file, a missing key, or a network error
 * all fall back silently to the defaults defined here.
 *
 * To customise a deployment, place a config.json at the server root with
 * only the keys you want to change, e.g.:
 *   { "resetCountdownSeconds": 5, "savedFadeDurationMs": 1500 }
 */

const DEFAULTS = {
  /** Seconds the user must wait before confirming a library reset. */
  resetCountdownSeconds: 10,
  /** Milliseconds the "Export key" success/error badge stays visible. */
  exportKeyFadeDurationMs: 4000,
  /** Milliseconds the "Saved" badge stays visible after a successful note save. */
  savedFadeDurationMs: 2500,
  /** Milliseconds debounce delay for the search input before querying the index. */
  searchDebounceMs: 200,
  /** Number of notes fetched concurrently during full-text index builds. */
  contentIndexBatchSize: 5,
  /** Milliseconds pause between note-fetch batches during full-text index builds. */
  contentIndexBatchPauseMs: 100,
  /** Notes shown per page in the sidebar (virtual pagination). */
  sidebarDisplayLimit: 40,
  /** Pixels from the bottom of the sidebar list that trigger loading the next page. */
  sidebarScrollThreshold: 100,
  /** Milliseconds of inactivity after the last edit before auto-saving the note. */
  autoSaveDebounceMs: 3000,
  /** Milliseconds of continuous editing before forcing a mid-session auto-save. */
  autoSaveIntervalMs: 10000,
}

export type AppConfig = typeof DEFAULTS

// Exported as a mutable object so that loadConfig() can overwrite individual
// keys in-place before Vue mounts. All modules import this same reference.
export const config: AppConfig = { ...DEFAULTS }

/**
 * Fetches /config.json and merges recognised keys into `config`.
 * Must be called (and awaited) in main.ts before createApp().mount().
 */
export async function loadConfig(): Promise<void> {
  try {
    const res = await fetch('/config.json')
    if (!res.ok) return
    const json: Partial<AppConfig> = await res.json()
    // Only copy keys that exist in DEFAULTS to prevent unknown properties from
    // leaking into config and to make the schema self-documenting.
    for (const key of Object.keys(DEFAULTS) as (keyof AppConfig)[]) {
      if (key in json && typeof json[key] === typeof DEFAULTS[key]) {
        Object.assign(config, { [key]: json[key] })
      }
    }
  } catch (e) {
    // Network error or JSON parse error — proceed with defaults.
    // Log so deployers can detect a broken config.json without a server restart.
    console.warn('[YinMo] Failed to load config.json, using defaults:', e)
  }
}
