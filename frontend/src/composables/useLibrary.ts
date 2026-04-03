import { ref, reactive, computed, watch, nextTick } from 'vue'
import axios from 'axios'
import * as crypto from '../crypto'
import { useI18n } from '../i18n'
import { config } from '../config'
import { RELEASE_NOTE_VERSION, RELEASE_NOTE_EN, RELEASE_NOTE_ZH } from '../releaseNotes'
import { useContentIndex } from './useContentIndex'
import { useOrphanCleanup } from './useOrphanCleanup'
import { useLibraryTrash } from './useLibraryTrash'
import { useLibraryStructure } from './useLibraryStructure'
import { syncChannel } from './useEditorSave'

const RN_SEEN_KEY = 'yinmo_rn_seen'
const RN_ID_KEY   = 'yinmo_rn_id'

const API_BASE = '/api'

// Inject Bearer token on every request when a session token is available.
// The token is stored in sessionStorage by the unlock flow and cleared on lock.
axios.interceptors.request.use((axiosConfig) => {
  const token = crypto.getSessionToken()
  if (token) {
    axiosConfig.headers = axiosConfig.headers ?? {}
    axiosConfig.headers['Authorization'] = `Bearer ${token}`
  }
  return axiosConfig
})

/**
 * Generates a unique note filename of the form YYYYMMDD<16 random chars>.md.
 * Uses rejection sampling to avoid modulo bias.
 */
export const generateId = (): string => {
  const n = new Date()
  const d = n.getFullYear().toString() + (n.getMonth() + 1).toString().padStart(2, '0') + n.getDate().toString().padStart(2, '0')
  const charset = 'abcdefghijklmnopqrstuvwxyz0123456789'
  // Rejection sampling: discard bytes >= floor(256/charset.length)*charset.length to
  // eliminate modulo bias (256 % 36 = 4 -> chars 0-3 would be ~0.4% over-represented).
  const limit = Math.floor(256 / charset.length) * charset.length // 252 for length 36
  const result: string[] = []
  while (result.length < 16) {
    const bytes = window.crypto.getRandomValues(new Uint8Array(32)) // extra bytes reduce retries
    for (const b of bytes) {
      if (b < limit && result.length < 16) result.push(charset[b % charset.length])
    }
  }
  return d + result.join('') + '.md'
}

// Notes that have been added to structure but never uploaded to the server yet.
// Keyed by note filename. Cleared when the note is first successfully saved.
const _pendingNotes = new Set<string>()

/** Read-only view + mutation helpers for the pending-notes set.
 * Exported as an object so callers cannot swap out or clear the underlying Set directly. */
export const pendingNotes = {
  has:    (id: string) => _pendingNotes.has(id),
  add:    (id: string) => _pendingNotes.add(id),
  delete: (id: string) => _pendingNotes.delete(id),
} as const

// Server-reported modTime for each note (populated by loadNotesList).
// Used by buildContentIndex to decide whether cached content is still fresh.
// Values originate from the server's os.Stat().ModTime(), so client clock is irrelevant.
const _noteModTimes = new Map<string, number>()

/**
 * Single source of truth for all note library state.
 *
 * Design constraints:
 * - Components must never manipulate `structure` directly or call the structure API
 *   endpoint themselves. All mutations go through the methods returned here so that
 *   encryption decisions and local-backup logic are applied consistently.
 * - All write operations are optimistic: local state is updated immediately and
 *   persisted asynchronously. If persistence fails, the error handler calls
 *   `loadNotesList` to re-sync from the server.
 * - Structure, titles and tags are always persisted as a single atomic object in one
 *   `saveStructure` call, preventing partial-update inconsistencies on the server.
 */
/** Shape of the JSON blob stored in `_structure.json` on the server. */
export interface LibraryStructure {
  order: string[]
  parents: Record<string, string>
  childOrder: Record<string, string[]>
  titles: Record<string, string>
  tags: Record<string, string[]>
  dark?: boolean
  pinned?: string[]
  trash?: { id: string; deletedAt: number }[]
  commitLabels?: Record<string, string>
  vaultProxies?: string[]
}

/** A single item in the flat sidebar display list, derived from LibraryStructure. */
interface DisplayItem {
  key: string
  label: string
  level: number
  hasChildren: boolean
  isCollapsed: boolean
  contentMatch: boolean
  tags: string[]
  isPinned?: boolean
}

export function useLibrary() {
  const { lang } = useI18n()

  // The hierarchical structure of the library (order, parent/child relationships).
  // Kept as a plain ref so that deep-mutation watchers in components are explicit.
  const structure = ref<LibraryStructure>({ order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
  const noteTitles = reactive<Record<string, string>>({})
  // Stored separately from structure so components can react to tag changes without
  // observing the entire structure object.
  const noteTags = reactive<Record<string, string[]>>({})
  const currentNote = ref<string | null>(localStorage.getItem('yinmo_current_note'))
  const isLibraryLocked = ref(true)
  const displayLimit = ref(config.sidebarDisplayLimit)
  // Gates the "import key" UI path -- only meaningful after the first server round-trip.
  const notesLoaded = ref(false)
  // Set to true when loadNotesList fails so callers can surface an error to the user.
  // Reset to false at the start of each attempt so transient failures clear on retry.
  const notesLoadError = ref(false)
  const hasServerNotes = computed(() => notesLoaded.value && (structure.value.order?.length > 0 || Object.keys(structure.value.parents || {}).length > 0))
  const searchQuery = ref('')
  const debouncedSearch = ref('')
  // Empty string means "show all" -- avoids a separate boolean and simplifies the filter logic.
  const activeTagFilter = ref('')
  let searchTimer: ReturnType<typeof setTimeout> | null = null

  // ── Delegate full-text index to useContentIndex ──────────────────────────
  const {
    contentIndex,
    isIndexing,
    contentMatchIds,
    buildContentIndex,
    indexNote: _rawIndexNote,
    clearContentIndex,
  } = useContentIndex({
    structure,
    noteTitles,
    noteModTimes: _noteModTimes,
    crypto: {
      isLibraryLocked: () => crypto.isLibraryLocked(),
      decryptText: (cipher: string) => crypto.decryptText(cipher),
    },
    config: {
      contentIndexBatchSize: config.contentIndexBatchSize,
      contentIndexBatchPauseMs: config.contentIndexBatchPauseMs,
    },
  })

  /**
   * Wraps the raw indexNote from useContentIndex to also re-evaluate
   * contentMatchIds against the current debounced search query.
   */
  const indexNote = (id: string, plainText: string) => {
    _rawIndexNote(id, plainText)
    const q = debouncedSearch.value.toLowerCase().trim()
    if (q) {
      const updated = new Set(contentMatchIds.value)
      if (plainText.toLowerCase().includes(q)) updated.add(id)
      else updated.delete(id)
      contentMatchIds.value = updated
    }
  }

  // ── Delegate orphan cleanup to useOrphanCleanup ──────────────────────────
  const { scheduleOrphanCleanup } = useOrphanCleanup({
    structure,
    contentIndex,
    isIndexing,
    crypto: { isLibraryLocked: () => crypto.isLibraryLocked() },
  })

  // Derived list of all unique tags across all notes, sorted for stable display order.
  const allTags = computed(() => {
    const set = new Set<string>()
    Object.values(noteTags).forEach(tags => tags.forEach(t => set.add(t)))
    return Array.from(set).sort()
  })

  // Debounce search input to avoid triggering expensive index traversal on every keystroke.
  watch(searchQuery, (v) => {
    if (searchTimer) clearTimeout(searchTimer)
    searchTimer = setTimeout(() => {
      debouncedSearch.value = v
      // Update content match set synchronously inside the debounce so that displayList
      // re-computation sees a consistent (query, matchSet) pair.
      const q = v.toLowerCase().trim()
      if (q && contentIndex.size > 0) {
        const matches = new Set<string>()
        for (const [id, text] of contentIndex) {
          if (text.toLowerCase().includes(q)) matches.add(id)
        }
        contentMatchIds.value = matches
      } else {
        contentMatchIds.value = new Set()
      }
    }, config.searchDebounceMs)
  })
  // Persist the last-opened note across page reloads so the user returns to where they left off.
  watch(currentNote, (v) => { if (v) localStorage.setItem('yinmo_current_note', v) })

  /**
   * Self-heals the structure after loading from the server.
   *
   * The server is the authority on which files actually exist. The structure metadata
   * (stored in _structure.json) can drift from disk reality if a note was deleted
   * outside the UI or if a previous save was interrupted. This function reconciles the
   * two: removes ghost IDs from the structure and promotes untracked files to the top
   * level so they remain accessible rather than silently hidden.
   */
  const sanitizeStructure = (s: Partial<LibraryStructure> | null, notes: { name: string }[]): LibraryStructure => {
    const safe: LibraryStructure = s?.order
      ? { order: s.order, parents: s.parents ?? {}, childOrder: s.childOrder ?? {}, titles: s.titles ?? {}, tags: s.tags ?? {}, pinned: s.pinned ?? [], trash: s.trash ?? [], commitLabels: s.commitLabels ?? {}, vaultProxies: s.vaultProxies ?? [] }
      : { order: [], parents: {}, childOrder: {}, titles: s?.titles ?? {}, tags: {}, pinned: [], trash: [], commitLabels: {}, vaultProxies: [] }
    const idSet = new Set(notes.map(n => n.name))
    // Guard: if the server returned zero notes but the structure has entries,
    // this is almost certainly a transient error (e.g. volume not mounted).
    // Refuse to sanitize -- returning the un-modified structure prevents a
    // catastrophic wipe of all folder hierarchy, titles, and tags.
    if (idSet.size === 0 && safe.order.length > 0) {
      console.error('[YinMo] sanitizeStructure: server returned 0 notes but structure has', safe.order.length, 'entries — skipping sanitization')
      return safe
    }
    // Remove references to files that no longer exist on disk.
    safe.order = safe.order.filter(id => idSet.has(id))
    Object.keys(safe.parents).forEach(k => { if (!idSet.has(k) || !idSet.has(safe.parents[k])) delete safe.parents[k] })
    Object.keys(safe.childOrder).forEach(pk => {
      if (!idSet.has(pk)) delete safe.childOrder[pk]
      else safe.childOrder[pk] = safe.childOrder[pk].filter(ck => idSet.has(ck))
    })
    // Surface any files present on disk but absent from the structure at the top level.
    const trashIds = new Set((safe.trash || []).map(e => e.id))
    const tracked = new Set([...safe.order, ...Object.keys(safe.parents), ...(Object.values(safe.childOrder).flat() as string[]), ...trashIds])
    idSet.forEach(id => { if (!tracked.has(id)) safe.order.push(id) })
    // Clean up pinned: remove IDs that no longer exist or are trashed.
    if (safe.pinned) safe.pinned = safe.pinned.filter(id => idSet.has(id) && !trashIds.has(id))
    // Clean up trash: remove entries whose files no longer exist on disk (already permanently deleted).
    if (safe.trash) safe.trash = safe.trash.filter(e => idSet.has(e.id))
    return safe
  }

  // Collapse state is UI-only, not persisted. Default: all collapsed.
  // Items are added to expandedItems when the user explicitly expands them.
  const expandedItems = ref(new Set<string>())
  const toggleCollapse = (id: string) => {
    const next = new Set(expandedItems.value)
    if (next.has(id)) next.delete(id); else next.add(id)
    expandedItems.value = next
  }

  /**
   * Derives the flat ordered list of visible items for the sidebar.
   *
   * Collapse state is suspended when a search query or tag filter is active so that
   * matching items nested inside collapsed folders are still reachable. The displayLimit
   * cap prevents the DOM from growing unbounded on large libraries -- additional items
   * are revealed incrementally via scroll.
   */
  const displayList = computed<DisplayItem[]>(() => {
    const res: DisplayItem[] = []
    if (!structure.value || !structure.value.order) return res
    const q = debouncedSearch.value.toLowerCase().trim()
    const tagFilter = activeTagFilter.value
    const limit = displayLimit.value
    const pinnedSet = new Set(structure.value.pinned || [])
    const trashIds = new Set((structure.value.trash || []).map(e => e.id))
    const traverse = (ids: string[], level: number, forcePinned?: boolean) => {
      for (const id of ids) {
        if (res.length >= limit) return
        if (trashIds.has(id)) continue
        const label = noteTitles[id] || id
        const children = (structure.value.childOrder[id] || []).filter(c => !trashIds.has(c))
        const hasChildren = children.length > 0
        const isCollapsed = !q && !tagFilter && hasChildren && !expandedItems.value.has(id)
        const matchTitle = !q || label.toLowerCase().includes(q)
        const matchContent = q ? contentMatchIds.value.has(id) : false
        const matchQ = matchTitle || matchContent
        const matchTag = !tagFilter || (noteTags[id] || []).includes(tagFilter)
        const match = matchQ && matchTag
        if (match) res.push({ key: id, label, level, hasChildren, isCollapsed, tags: noteTags[id] || [], contentMatch: matchContent && !matchTitle, isPinned: forcePinned || pinnedSet.has(id) })
        if (hasChildren && !isCollapsed) traverse(children, level + 1)
      }
    }
    // Show pinned notes first (only at top level, skip during search/filter)
    if (!q && !tagFilter && pinnedSet.size > 0) {
      const pinnedIds = (structure.value.pinned || []).filter(id => !trashIds.has(id))
      traverse(pinnedIds, 0, true)
      // Then traverse order, skipping pinned ones
      const remaining = structure.value.order.filter(id => !pinnedSet.has(id))
      traverse(remaining, 0)
    } else {
      traverse(structure.value.order, 0)
    }
    return res
  })

  /**
   * Creates or updates a release-notes note in the library, then sets it as the
   * current note so the user sees it immediately.
   *
   * Runs once per version upgrade (tracked via localStorage). Skipped when the
   * library is locked (encryption enabled but not yet unlocked) because we cannot
   * encrypt or upload note content without the master key.
   *
   * If a release-notes note from a previous version still exists in the structure,
   * its content is updated in-place rather than creating a duplicate.
   */
  const seedReleaseNote = async () => {
    const content = lang.value === 'zh' ? RELEASE_NOTE_ZH : RELEASE_NOTE_EN
    const title   = lang.value === 'zh'
      ? `YinMoNote v${RELEASE_NOTE_VERSION} 发布说明`
      : `What's New in YinMoNote v${RELEASE_NOTE_VERSION}`

    // Reuse the existing release-note ID if it is still in the structure.
    const existingId = localStorage.getItem(RN_ID_KEY)
    const allIds = new Set([
      ...(structure.value.order ?? []),
      ...Object.keys(structure.value.parents ?? {}),
    ])
    const reuseExisting = !!existingId && allIds.has(existingId)
    const id = reuseExisting ? existingId! : generateId()

    // Upload content, encrypted when server encryption is active.
    const uploadContent = crypto.shouldEncrypt() ? await crypto.encryptText(content) : content
    await axios.put(`${API_BASE}/notes/${id}`, { content: uploadContent })

    if (!reuseExisting) {
      // Prepend the new note to the top-level order so it appears first in the sidebar.
      if (!structure.value.order) structure.value.order = []
      structure.value.order.unshift(id)
      noteTitles[id] = title
      await saveStructure()
      localStorage.setItem(RN_ID_KEY, id)
    }

    currentNote.value = id
    localStorage.setItem(RN_SEEN_KEY, RELEASE_NOTE_VERSION)
  }

  const loadNotesList = async () => {
    if (_isLoadingNotes) return
    _isLoadingNotes = true
    notesLoadError.value = false
    try {
      const [nRes, sRes] = await Promise.all([axios.get(`${API_BASE}/notes`), axios.get(`${API_BASE}/structure`)])
      const notes = nRes.data.notes || []; let rawData = sRes.data
      // Axios JSON-encodes string payloads on PUT, so legacy encrypted structures may
      // be stored as a JSON-quoted string: `"ENC1:..."`. Unwrap before detection.
      if (typeof rawData === 'string' && rawData.startsWith('"')) {
        try { const p = JSON.parse(rawData); if (typeof p === 'string') rawData = p } catch (_) { /* not a JSON-quoted string */ }
      }
      // Decrypt the structure if the library is currently unlocked; otherwise the
      // encrypted blob will be used as-is and titles will show as IDs until unlock.
      if (typeof rawData === 'string' && rawData.startsWith('ENC1:')) {
        const decrypted = await crypto.decryptObject(rawData); if (decrypted) rawData = decrypted
      }
      structure.value = sanitizeStructure(rawData, notes)
      notesLoaded.value = true
      // Remove stale title entries for notes that no longer exist in the structure.
      const liveIds = new Set([
        ...(structure.value.order || []),
        ...Object.keys(structure.value.parents || {}),
        ...(Object.values(structure.value.childOrder || {}).flat() as string[]),
        ...(structure.value.trash || []).map(e => e.id),
      ])
      Object.keys(noteTitles).forEach(id => { if (!liveIds.has(id)) delete noteTitles[id] })
      Object.keys(noteTags).forEach(id => { if (!liveIds.has(id)) delete noteTags[id] })
      if (structure.value.titles) Object.assign(noteTitles, structure.value.titles)
      if (structure.value.tags) Object.assign(noteTags, structure.value.tags)
      // The server reads the first line of each file live, so n.title is ground truth
      // for unencrypted notes. Always override the structure cache with the live value
      // to fix stale titles (e.g. file renamed externally or structure not yet saved).
      // For encrypted notes n.title is empty -- the structure cache remains as-is until
      // buildContentIndex decrypts the content and updates noteTitles below.
      _noteModTimes.clear()
      for (const n of notes) {
        _noteModTimes.set(n.name, n.modTime)
        if (n.title) noteTitles[n.name] = n.title
      }
      if (!crypto.isLibraryLocked()) {
        const cipher = await crypto.encryptObject({ ...structure.value, titles: { ...noteTitles } })
        localStorage.setItem('yinmo_structure_backup_v2', cipher)
      }
      // If the previously open note no longer exists in the structure (e.g. it was
      // deleted externally), clear it so the editor doesn't show a ghost 404 note.
      // Exception: pending notes (created locally but not yet uploaded) are NOT in
      // liveIds yet -- do not null them out here or the editor unmounts mid-creation,
      // causing a race condition with the concurrent saveStructure PUT request.
      if (currentNote.value) {
        if (!liveIds.has(currentNote.value) && !pendingNotes.has(currentNote.value)) currentNote.value = null
      }
      // Seed release notes on first run and after version upgrades. Runs only when the
      // library is accessible (unlocked or keyless) so content can be uploaded correctly.
      if (!crypto.isLibraryLocked() && localStorage.getItem(RN_SEEN_KEY) !== RELEASE_NOTE_VERSION) {
        try {
          await seedReleaseNote()
        } catch (e) {
          console.error('[YinMo] Release note write failed:', e)
          if (!currentNote.value && displayList.value.length) currentNote.value = displayList.value[0].key
        }
      } else if (!currentNote.value && displayList.value.length) {
        currentNote.value = displayList.value[0].key
      }
    } catch (e) {
      console.error('[YinMo] Note list load failed:', e)
      notesLoadError.value = true
    } finally { _isLoadingNotes = false }
  }

  // Guards against concurrent loadNotesList invocations (e.g. rapid unlock events)
  // that could produce inconsistent intermediate state or duplicate server requests.
  let _isLoadingNotes = false

  // Serialises all saveStructure calls so concurrent operations (drag-drop, tag edit,
  // note creation, etc.) never race each other on the PUT /structure endpoint.
  // Each call snapshots the current state at enqueue time and waits its turn.
  let _saveQueue: Promise<void> = Promise.resolve()

  const saveStructure = (): Promise<void> => {
    // Snapshot reactive state immediately so late-arriving callers don't overwrite
    // an earlier snapshot that is already in flight.
    const payloadObj = { ...structure.value, titles: { ...noteTitles }, tags: { ...noteTags } }
    _saveQueue = _saveQueue.then(async () => {
      let payload: LibraryStructure | string = payloadObj
      // Encrypt the payload before uploading unless the user has explicitly disabled
      // server-side encryption.
      if (crypto.isServerEncryptEnabled() && !crypto.isLibraryLocked()) {
        const encrypted = await crypto.encryptObject(payloadObj); if (encrypted) payload = encrypted
      }
      // ENC1 blobs are strings that must be JSON-quoted so the server receives
      // `"ENC1:..."` instead of raw `ENC1:...`. Keyless mode's encryptObject
      // returns a plain JSON string (not ENC1) — send it as-is (axios will
      // JSON-encode the object for us).
      if (typeof payload === 'string' && payload.startsWith('ENC1:')) {
        await axios.put(`${API_BASE}/structure`, JSON.stringify(payload), { headers: { 'Content-Type': 'application/json' } })
      } else if (typeof payload === 'string') {
        // Keyless mode: encryptObject returned JSON.stringify(obj) — parse it
        // back so axios sends a proper JSON object, not a double-encoded string.
        await axios.put(`${API_BASE}/structure`, JSON.parse(payload))
      } else {
        await axios.put(`${API_BASE}/structure`, payload)
      }
      // Notify other tabs of structure change
      syncChannel?.postMessage({ type: 'structure-saved' })
      // Keep a local encrypted backup so the structure can be recovered if the server
      // is unavailable or if a future loadNotesList call fails to decrypt the server copy.
      // Wrapped in try-catch: localStorage quota errors must not roll back the save.
      try {
        if (!crypto.isLibraryLocked()) {
          const cipher = await crypto.encryptObject(payloadObj)
          localStorage.setItem('yinmo_structure_backup_v2', cipher)
        }
      } catch { /* best-effort backup */ }
    }).catch((err) => {
      const detail = err?.response?.status ? `HTTP ${err.response.status}: ${err.response.data?.error || ''}` : String(err)
      console.error('[YinMo] Structure save failed:', detail, err)
      if (!crypto.isLibraryLocked()) loadNotesList()
    })
    return _saveQueue
  }

  // generateId is defined as a module-level export above; reference it here for local use.

  // onCreated is an optional callback invoked after the structure is saved and the
  // next tick has fired. The caller (App.vue) is responsible for any DOM interactions
  // such as focusing the editor -- composables must not access the DOM directly (D2 fix).
  const createNewNote = async (onCreated?: () => void) => {
    const id = generateId(); const title = lang.value === 'zh' ? '新建文档' : 'New Document'
    if (!structure.value.order) structure.value.order = []
    // Prepend to order so the new note appears at the top of the sidebar immediately.
    structure.value.order.unshift(id); noteTitles[id] = title
    // Don't upload yet -- wait for the user to type actual content.
    // The editor's doSave will handle the first upload when content is non-empty.
    pendingNotes.add(id)
    currentNote.value = id  // Set immediately so rapid re-clicks see this as the current pending note
    await saveStructure()
    nextTick(() => onCreated?.())
  }

  // Silently remove an empty note (no confirm, no currentNote change).
  // If the note was never uploaded (pending), skips the server DELETE entirely.
  const silentDeleteNote = (k: string) => {
    const pid = structure.value.parents[k]
    if (pid) structure.value.childOrder[pid] = (structure.value.childOrder[pid] || []).filter(x => x !== k)
    else structure.value.order = (structure.value.order || []).filter(x => x !== k)
    delete structure.value.childOrder[k]; delete structure.value.parents[k]
    delete noteTitles[k]; delete noteTags[k]
    saveStructure().catch((err) => { console.error('[YinMo] Silent delete: structure save failed:', err) })
    if (!pendingNotes.has(k)) axios.delete(`${API_BASE}/notes/${k}`).catch((err) => { console.error('[YinMo] Silent delete: server delete failed:', err) })
    pendingNotes.delete(k)
  }

  // ── Delegate trash operations to useLibraryTrash ──────────────────────────
  const { deleteNote, restoreNote, permanentDeleteNote, emptyTrash } = useLibraryTrash({
    structure,
    noteTitles,
    noteTags,
    currentNote,
    saveStructure,
    loadNotesList,
    isLibraryLocked: () => crypto.isLibraryLocked(),
    API_BASE,
  })

  // ── Delegate structure manipulation to useLibraryStructure ───────────────
  const { moveNote, createSubNote } = useLibraryStructure({
    structure,
    noteTitles,
    currentNote,
    saveStructure,
    lang,
  })

  /**
   * Updates the tags for a note and persists the full structure atomically.
   * Tags are stored inside the structure payload so they receive the same
   * E2EE protection as the rest of the metadata.
   */
  const setNoteTags = async (id: string, tags: string[]) => {
    if (tags.length === 0) delete noteTags[id]
    else noteTags[id] = tags
    await saveStructure()
  }

  // ── Pinned notes ──────────────────────────────────────────────────────────
  const togglePin = async (id: string) => {
    if (!structure.value.pinned) structure.value.pinned = []
    const idx = structure.value.pinned.indexOf(id)
    if (idx >= 0) structure.value.pinned.splice(idx, 1)
    else structure.value.pinned.push(id)
    await saveStructure()
  }

  // ── Commit labels (version history naming) ────────────────────────────────
  const COMMIT_LABEL_MAX_LEN = 100
  const COMMIT_LABELS_MAX_COUNT = 500
  const setCommitLabel = async (hash: string, label: string) => {
    if (!structure.value.commitLabels) structure.value.commitLabels = {}
    const trimmed = label.trim().slice(0, COMMIT_LABEL_MAX_LEN)
    if (trimmed) {
      // Enforce entry count limit -- evict oldest entries (by insertion order) if needed
      const keys = Object.keys(structure.value.commitLabels)
      while (keys.length >= COMMIT_LABELS_MAX_COUNT && !structure.value.commitLabels[hash]) {
        delete structure.value.commitLabels[keys.shift()!]
      }
      structure.value.commitLabels[hash] = trimmed
    } else {
      delete structure.value.commitLabels[hash]
    }
    await saveStructure()
  }

  return {
    structure, noteTitles, noteTags, currentNote, isLibraryLocked,
    searchQuery, debouncedSearch, displayList, displayLimit,
    notesLoaded, hasServerNotes, notesLoadError,
    expandedItems, toggleCollapse,
    activeTagFilter, allTags,
    isIndexing, contentIndex, contentMatchIds,
    loadNotesList, saveStructure, createNewNote, createSubNote, deleteNote, silentDeleteNote, moveNote, setNoteTags, buildContentIndex, clearContentIndex, indexNote, scheduleOrphanCleanup, pendingNotes,
    togglePin, restoreNote, permanentDeleteNote, emptyTrash, setCommitLabel
  }
}
