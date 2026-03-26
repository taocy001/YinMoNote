import axios from 'axios'
import type { Ref } from 'vue'

const API_BASE = '/api'

/**
 * Dependencies required by the orphan cleanup composable.
 */
export interface OrphanCleanupDeps {
  /** The library structure ref (read-only access to order/parents/childOrder/trash). */
  structure: Ref<{
    order: string[]
    parents: Record<string, string>
    childOrder: Record<string, string[]>
    trash?: { id: string }[]
  }>
  /** The in-memory full-text content index (key = note ID, value = decrypted text). */
  contentIndex: Map<string, string>
  /** Whether a full index build is currently in progress. */
  isIndexing: Ref<boolean>
  /** Crypto state needed to guard against running GC on a locked library. */
  crypto: { isLibraryLocked: () => boolean }
}

/**
 * Manages garbage collection of orphaned assets (images/uploads) that are no
 * longer referenced by any note.
 *
 * After a note save or delete, images that are no longer referenced by any
 * note become orphans on the server. This GC sweep compares server-side
 * assets against all decrypted content in contentIndex and deletes the
 * unreferenced ones. Debounced so rapid saves don't spam the API.
 */
export function useOrphanCleanup(deps: OrphanCleanupDeps) {
  const { structure, contentIndex, isIndexing, crypto } = deps

  let _gcTimer: ReturnType<typeof setTimeout> | null = null

  /** Extract unique asset filenames referenced in a body of text. */
  const extractAssetRefs = (text: string): Set<string> => {
    const refs = new Set<string>()
    // Matches ./uploads/filename or /uploads/filename in markdown/HTML
    const re = /(?:\.\/|\/)?uploads\/([^\s"')>\]]+)/g
    let m: RegExpExecArray | null
    while ((m = re.exec(text)) !== null) refs.add(m[1])
    return refs
  }

  const _runOrphanGC = async () => {
    if (crypto.isLibraryLocked() || isIndexing.value) return
    // Safety: only run GC when the content index covers all known notes.
    // An incomplete index (e.g. due to bulk-fetch truncation) would cause
    // referenced assets to appear orphaned and be permanently deleted.
    // Skip GC entirely when the trash is non-empty — trashed notes are not
    // in contentIndex, so their image references cannot be verified and we
    // must not risk deleting assets that a trashed note still references.
    if ((structure.value.trash || []).length > 0) return

    const allNoteIds = new Set([
      ...(structure.value.order || []),
      ...Object.keys(structure.value.parents || {}),
      ...(Object.values(structure.value.childOrder || {}).flat() as string[]),
    ])
    if (allNoteIds.size > 0 && contentIndex.size < allNoteIds.size) return
    try {
      const res = await axios.get(`${API_BASE}/assets`)
      const serverAssets: string[] = res.data.assets || []
      if (serverAssets.length === 0) return

      // Collect all image filenames referenced across every indexed note
      const allRefs = new Set<string>()
      for (const [, text] of contentIndex) {
        for (const ref of extractAssetRefs(text)) allRefs.add(ref)
      }

      // Delete assets not referenced by any note
      for (const asset of serverAssets) {
        if (!allRefs.has(asset)) {
          await axios.delete(`${API_BASE}/uploads/${asset}`).catch(() => {})
        }
      }
    } catch { /* best-effort -- network errors are silently ignored */ }
  }

  /** Schedule an orphaned-asset cleanup after a short delay. */
  const scheduleOrphanCleanup = () => {
    if (_gcTimer) clearTimeout(_gcTimer)
    _gcTimer = setTimeout(() => { _gcTimer = null; _runOrphanGC() }, 5000)
  }

  return {
    extractAssetRefs,
    scheduleOrphanCleanup,
  }
}
