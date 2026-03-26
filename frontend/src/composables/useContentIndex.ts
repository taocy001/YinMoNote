import { ref, type Ref } from 'vue'
import axios from 'axios'
import { getCacheMeta, getCachedContent, cacheNote, pruneCache } from '../indexCache'

const API_BASE = '/api'

/**
 * Dependencies required by the content index composable.
 *
 * All reactive state is passed in by reference so the index can read and
 * mutate the canonical copies owned by useLibrary.
 */
export interface ContentIndexDeps {
  /** The library structure ref (read-only access to order/parents). */
  structure: Ref<{
    order: string[]
    parents: Record<string, string>
    childOrder: Record<string, string[]>
  }>
  /** Reactive record of note titles — mutated when decrypted first lines are extracted. */
  noteTitles: Record<string, string>
  /** Server-reported modTime per note, used to validate IndexedDB cache freshness. */
  noteModTimes: Map<string, number>
  /** Crypto module — only the subset of functions actually used by the index. */
  crypto: {
    isLibraryLocked: () => boolean
    decryptText: (cipher: string) => Promise<string>
  }
  /** Tuning knobs pulled from the app config. */
  config: {
    contentIndexBatchSize: number
    contentIndexBatchPauseMs: number
  }
}

/**
 * Manages the in-memory full-text search index built by decrypting all notes.
 *
 * The index is a plain Map (not reactive) because it can hold hundreds of
 * entries and we only need to trigger UI updates through `contentMatchIds`,
 * not the Map itself.
 */
export function useContentIndex(deps: ContentIndexDeps) {
  const { structure, noteTitles, noteModTimes, crypto, config } = deps

  // Key = note ID, value = decrypted plain text.
  const contentIndex = new Map<string, string>()
  const isIndexing = ref(false)
  // Derived set of IDs whose full content matched the last search query.
  const contentMatchIds = ref(new Set<string>())

  /**
   * Decrypt server content, update contentIndex and noteTitles.
   * @returns true on successful decryption and indexing.
   */
  const _indexOneNote = async (id: string, serverContent: string): Promise<boolean> => {
    const plain = await crypto.decryptText(serverContent)
    if (!plain || plain === '[Decryption Error]' || plain === '[Locked]') return false
    contentIndex.set(id, plain)
    const firstLine = plain.split('\n').find((l: string) => l.trim())?.replace(/^#+\s*/, '').trim()
    if (firstLine && noteTitles[id] !== firstLine) noteTitles[id] = firstLine
    return true
  }

  /**
   * Builds an in-memory full-text search index by decrypting all notes.
   *
   * Fetching strategy (fastest path first):
   *  1. IndexedDB cache hit (modTime unchanged) -- decrypt locally, no network
   *  2. Bulk fetch (GET /api/notes/bulk) -- one HTTP request for all cache-missed notes
   *  3. Individual fetch fallback -- batched GET /api/notes/:id (if bulk unavailable)
   *
   * modTime is treated as an opaque version token -- both sides come from the same
   * server's os.Stat().ModTime(), so client timezone/clock skew is irrelevant.
   *
   * IndexedDB only stores server-original content (ENC1 or keyless plain text)
   * -- NEVER the decrypted plaintext index.
   */
  const buildContentIndex = async () => {
    if (isIndexing.value || crypto.isLibraryLocked()) return
    isIndexing.value = true
    try {
      const ids = [...new Set([
        ...(structure.value.order || []),
        ...Object.keys(structure.value.parents || {})
      ])]
      const cacheMeta = getCacheMeta()

      // -- Phase 1: resolve cache hits from IndexedDB ----------------------------
      const cacheMissIds: string[] = []
      for (const id of ids) {
        if (crypto.isLibraryLocked()) break
        if (contentIndex.has(id)) continue
        const serverMod = noteModTimes.get(id)
        if (serverMod !== undefined && cacheMeta[id] === serverMod) {
          try {
            const cached = await getCachedContent(id)
            if (cached !== null && await _indexOneNote(id, cached)) continue
          } catch { /* fall through to network fetch */ }
        }
        cacheMissIds.push(id)
      }

      // -- Phase 2: fetch all cache-missed notes in one bulk request -------------
      if (cacheMissIds.length > 0 && !crypto.isLibraryLocked()) {
        let bulkHandled = false
        try {
          const res = await axios.get(`${API_BASE}/notes/bulk`)
          const bulkNotes: Record<string, { content: string; modTime: number }> = res.data.notes || {}
          for (const id of cacheMissIds) {
            if (crypto.isLibraryLocked()) break
            const entry = bulkNotes[id]
            if (!entry) continue
            try {
              if (await _indexOneNote(id, entry.content)) {
                cacheNote(id, entry.content, entry.modTime).catch(() => {})
              }
            } catch { /* skip individual decrypt failures */ }
          }
          // Only mark bulk as handled if all cache-miss notes were actually returned.
          // When the server truncates the response (50 MB cap), some notes will be
          // missing -- Phase 3 must fetch them individually to avoid an incomplete index.
          bulkHandled = !res.data.truncated && cacheMissIds.every(id => contentIndex.has(id))
        } catch { /* bulk endpoint unavailable -- fall back to individual requests */ }

        // -- Phase 3: fallback to batched individual requests --------------------
        if (!bulkHandled) {
          const remaining = cacheMissIds.filter(id => !contentIndex.has(id))
          const BATCH = config.contentIndexBatchSize
          for (let i = 0; i < remaining.length; i += BATCH) {
            if (crypto.isLibraryLocked()) break
            const batch = remaining.slice(i, i + BATCH)
            await Promise.all(batch.map(async (id) => {
              try {
                const res = await axios.get(`${API_BASE}/notes/${id}`)
                const serverContent: string = res.data.content
                const serverMod = noteModTimes.get(id)
                if (await _indexOneNote(id, serverContent) && serverMod !== undefined) {
                  cacheNote(id, serverContent, serverMod).catch(() => {})
                }
              } catch { /* skip */ }
            }))
            await new Promise(r => setTimeout(r, config.contentIndexBatchPauseMs))
          }
        }
      }
      // Remove cached entries for notes that no longer exist on the server
      pruneCache(new Set(ids)).catch(() => {})
    } catch { /* graceful degradation */ } finally {
      isIndexing.value = false
    }
  }

  /**
   * Updates the content index for a single note after it is saved.
   * Called by Editor.vue on each successful save so the full-text search index
   * reflects the current content without requiring a full index rebuild.
   *
   * @param debouncedSearch - current debounced search query, used to re-evaluate matches.
   * @param updateMatchIds - callback to set the updated contentMatchIds.
   */
  const indexNote = (id: string, plainText: string) => {
    contentIndex.set(id, plainText)
  }

  /**
   * Clears the in-memory full-text search index and match set.
   * Must be called on library lock so stale decrypted content from a previous
   * key session cannot leak into a subsequent unlock with a different key.
   *
   * The IndexedDB cache (raw server content: ENC1 ciphertext or keyless plain
   * text) is intentionally NOT cleared here. The cached data is identical to
   * what the server stores -- no new attack surface is introduced. Keeping it
   * allows both encrypted and keyless users to benefit from fast re-indexing
   * on the next unlock. The cache is only wiped on library reset or encryption
   * mode switch.
   */
  const clearContentIndex = () => {
    contentIndex.clear()
    contentMatchIds.value = new Set()
  }

  return {
    contentIndex,
    isIndexing,
    contentMatchIds,
    buildContentIndex,
    indexNote,
    clearContentIndex,
  }
}
