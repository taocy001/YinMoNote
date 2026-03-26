/**
 * IndexedDB-backed note content cache for accelerating full-text index builds.
 *
 * On each unlock, buildContentIndex normally fetches every note from the server
 * to decrypt and index. This cache stores the raw server content (ENC1 ciphertext
 * or plain text for keyless mode) alongside the server-reported modTime so that
 * unchanged notes can be read from local cache instead of re-fetched.
 *
 * Design decisions:
 *  - IndexedDB stores raw server content only — NEVER decrypted plaintext.
 *  - modTime is treated as an opaque version token: both the cached value and the
 *    comparison value come from the same server's os.Stat().ModTime(), so client
 *    timezone or clock skew is irrelevant.
 *  - modTime metadata is kept in localStorage (tiny: ~40 bytes per note) so that
 *    cache-hit checks don't require deserialising full note content from IDB.
 *  - When IndexedDB is unavailable (e.g. happy-dom test env), all operations
 *    silently degrade to no-ops.
 *  - Encrypted mode: cache is cleared on library lock (clearCache).
 *  - Keyless mode: cache persists across sessions for maximum benefit.
 */

const DB_NAME = 'yinmo-index-cache'
const DB_VERSION = 1
const STORE = 'notes'
const META_KEY = 'yinmo_cache_meta'

const hasIDB = typeof indexedDB !== 'undefined'

// Singleton database connection — opened once per page session.
let _dbPromise: Promise<IDBDatabase> | null = null

function openDB(): Promise<IDBDatabase> {
  if (!_dbPromise) {
    _dbPromise = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, DB_VERSION)
      req.onupgradeneeded = () => {
        const db = req.result
        if (!db.objectStoreNames.contains(STORE)) {
          db.createObjectStore(STORE, { keyPath: 'id' })
        }
      }
      req.onsuccess = () => resolve(req.result)
      req.onerror = () => { _dbPromise = null; reject(req.error) }
    })
  }
  return _dbPromise
}

// ── Metadata (localStorage) ──────────────────────────────────────────────────

/** Returns { noteId → modTime } map from localStorage. */
export function getCacheMeta(): Record<string, number> {
  try {
    return JSON.parse(localStorage.getItem(META_KEY) || '{}')
  } catch { return {} }
}

function saveCacheMeta(meta: Record<string, number>): void {
  localStorage.setItem(META_KEY, JSON.stringify(meta))
}

// ── Content (IndexedDB) ─────────────────────────────────────────────────────

/** Read cached server content for a single note. Returns null on miss/error. */
export async function getCachedContent(id: string): Promise<string | null> {
  if (!hasIDB) return null
  try {
    const db = await openDB()
    return new Promise(resolve => {
      const tx = db.transaction(STORE, 'readonly')
      const req = tx.objectStore(STORE).get(id)
      req.onsuccess = () => resolve(req.result?.content ?? null)
      req.onerror = () => resolve(null)
    })
  } catch { return null }
}

/** Store a note's raw server content and update the modTime metadata. */
export async function cacheNote(id: string, content: string, modTime: number): Promise<void> {
  if (!hasIDB) return
  try {
    const db = await openDB()
    await new Promise<void>((resolve) => {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).put({ id, content })
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
    const meta = getCacheMeta()
    meta[id] = modTime
    saveCacheMeta(meta)
  } catch { /* best-effort */ }
}

/** Remove cache entries for notes that no longer exist on the server. */
export async function pruneCache(liveIds: Set<string>): Promise<void> {
  // Prune localStorage meta
  const meta = getCacheMeta()
  let changed = false
  for (const id of Object.keys(meta)) {
    if (!liveIds.has(id)) { delete meta[id]; changed = true }
  }
  if (changed) saveCacheMeta(meta)

  // Prune IndexedDB content
  if (!hasIDB) return
  try {
    const db = await openDB()
    const tx = db.transaction(STORE, 'readwrite')
    const store = tx.objectStore(STORE)
    const req = store.openKeyCursor()
    req.onsuccess = () => {
      const cursor = req.result
      if (cursor) {
        if (!liveIds.has(cursor.key as string)) store.delete(cursor.key)
        cursor.continue()
      }
    }
  } catch { /* best-effort */ }
}

/** Wipe all cached content and metadata. Called on lock in encrypted mode. */
export async function clearCache(): Promise<void> {
  localStorage.removeItem(META_KEY)
  if (!hasIDB) return
  try {
    const db = await openDB()
    await new Promise<void>(resolve => {
      const tx = db.transaction(STORE, 'readwrite')
      tx.objectStore(STORE).clear()
      tx.oncomplete = () => resolve()
      tx.onerror = () => resolve()
    })
  } catch { /* best-effort */ }
}
