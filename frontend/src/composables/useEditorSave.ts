/**
 * Composable that encapsulates the entire auto-save system for the note editor.
 *
 * Manages save status tracking, debounced/interval auto-save timers, and a
 * serialised promise chain that guarantees saves execute one at a time so the
 * last edit is always persisted.
 */
import { ref, computed, type Ref, type ShallowRef } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import axios from 'axios'
import { encryptText } from '../crypto'
import { pendingNotes } from './useLibrary'
import { config } from '../config'

// Cross-tab coordination via BroadcastChannel.
// After a save completes, notify other tabs so they can reload stale data.
let _channel: BroadcastChannel | null = null
try { _channel = new BroadcastChannel('yinmo-sync') } catch { /* SSR / unsupported */ }
export const syncChannel = _channel

/** Possible states of the save indicator. */
export type SaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

const API_BASE = '/api'

/** Dependencies injected by the host component. */
export interface EditorSaveDeps {
  editor: Ref<TiptapEditor | undefined> | ShallowRef<TiptapEditor | undefined>
  noteFileName: Ref<string>
  /** Whether the note content is empty (skip saving). */
  isContentEmpty: Ref<boolean>
  /** Whether server-side encryption is enabled. */
  serverEncrypt: Ref<boolean>
  /** Callback to update the full-text search index after a successful save. */
  indexNote: (id: string, plainText: string) => void
  /** Callback to schedule orphaned-image garbage collection after save. */
  scheduleOrphanCleanup: () => void
  /** i18n labels for status display. */
  t: Ref<Record<string, string>>
  /** When true, auto-save is skipped — the editor is in read-only mode. */
  isReadOnly: Ref<boolean>
}

/**
 * Extracts all save-related state and logic from Editor.vue.
 *
 * Returns reactive refs for UI binding, timer-management helpers for the
 * onUpdate callback, and cleanup/save-if-dirty helpers for onBeforeUnmount.
 */
export function useEditorSave(deps: EditorSaveDeps) {
  const {
    editor, noteFileName, isContentEmpty,
    serverEncrypt, indexNote, scheduleOrphanCleanup, t, isReadOnly,
  } = deps

  // ── Reactive state ──────────────────────────────────────────────────────
  const saveStatus = ref<SaveStatus>('idle')
  const lastSaveError = ref('')
  const lastSavedTime = ref<Date | null>(null)

  // ── Timers (non-reactive, no need for ref) ──────────────────────────────
  let savedFadeTimer: ReturnType<typeof setTimeout> | null = null
  let autoSaveTimer: ReturnType<typeof setTimeout> | null = null
  let autoSaveIntervalTimer: ReturnType<typeof setTimeout> | null = null

  // ── Promise queue for serialised saves ──────────────────────────────────
  // Concurrent calls are enqueued rather than silently discarded, so the
  // last edit is always persisted (A1 fix).
  let _saveChain: Promise<void> = Promise.resolve()

  // ── Core save logic ─────────────────────────────────────────────────────

  /**
   * Performs a single save: reads markdown from the editor, optionally
   * encrypts, PUTs to the server, updates the search index, and triggers
   * orphan cleanup.
   */
  const _doSaveOnce = async (f?: string) => {
    const n = f ?? noteFileName.value
    if (!editor.value || !n) return
    // Never upload empty content -- App.vue will discard the note via
    // silentDeleteNote if the user switches away without typing anything.
    if (isContentEmpty.value) return
    try {
      // At save time all chunks must already be loaded (ensured by the
      // onUpdate handler). Use getMarkdown() on the complete document.
      let c = (editor.value.storage as any).markdown.getMarkdown()
      const plainText = c // capture before encryption for search index
      if (serverEncrypt.value) c = await encryptText(c)
      saveStatus.value = 'saving'
      await axios.put(`${API_BASE}/notes/${n}`, { content: c })
      // First successful save -- remove from pending set.
      pendingNotes.delete(n)
      // Update in-memory full-text search index immediately.
      indexNote(n, plainText)
      scheduleOrphanCleanup()
      // Notify other tabs that this note was saved
      syncChannel?.postMessage({ type: 'note-saved', note: n })
      saveStatus.value = 'saved'
      lastSavedTime.value = new Date()
      if (savedFadeTimer) clearTimeout(savedFadeTimer)
      savedFadeTimer = setTimeout(() => {
        saveStatus.value = 'idle'
      }, config.savedFadeDurationMs)
    } catch (err: unknown) {
      saveStatus.value = 'error'
      const axiosErr = err as { response?: { status?: number; data?: { error?: string } }; message?: string }
      const status = axiosErr?.response?.status
      const serverMsg = axiosErr?.response?.data?.error
      lastSaveError.value = serverMsg
        ? `${status ?? 'Error'}: ${serverMsg}`
        : (axiosErr?.message || String(err))
    }
  }

  /** Enqueues a save onto the serial promise chain. */
  const doSave = (f?: string): Promise<void> => {
    _saveChain = _saveChain.then(() => _doSaveOnce(f)).catch((err) => {
      console.error('[YinMo] doSave unexpected error:', err)
      saveStatus.value = 'error'
    })
    return _saveChain
  }

  // ── Computed display helpers ─────────────────────────────────────────────

  const statusText = computed(() => {
    if (saveStatus.value === 'dirty') return t.value.unsaved
    if (saveStatus.value === 'saving') return t.value.saving
    if (saveStatus.value === 'saved') return t.value.saved
    if (saveStatus.value === 'error') return t.value.saveError
    return ''
  })

  const saveStatusStyle = computed(() => {
    if (saveStatus.value === 'dirty') return 'color: var(--color-warning);'
    if (saveStatus.value === 'saving') return 'color: var(--accent);'
    if (saveStatus.value === 'saved') return 'color: var(--color-success);'
    if (saveStatus.value === 'error') return 'color: var(--color-danger);'
    return 'color: var(--text-muted);'
  })

  const saveDotStyle = computed(() => {
    if (saveStatus.value === 'dirty') return 'background: var(--color-warning);'
    if (saveStatus.value === 'saving') return 'background: var(--accent);'
    if (saveStatus.value === 'saved') return 'background: var(--color-success);'
    if (saveStatus.value === 'error') return 'background: var(--color-danger);'
    return 'background: transparent;'
  })

  // ── Auto-save timer management ──────────────────────────────────────────

  /**
   * Called by the editor's onUpdate handler whenever content changes.
   * Sets saveStatus to dirty and manages debounce + interval auto-save timers.
   */
  const onContentChanged = () => {
    if (isReadOnly.value) return
    saveStatus.value = 'dirty'
    // Debounce: save after a pause in editing; cancels interval to avoid double-save.
    if (autoSaveTimer) clearTimeout(autoSaveTimer)
    autoSaveTimer = setTimeout(() => {
      autoSaveTimer = null
      if (autoSaveIntervalTimer) {
        clearTimeout(autoSaveIntervalTimer)
        autoSaveIntervalTimer = null
      }
      doSave()
    }, config.autoSaveDebounceMs)
    // Interval: if editing continuously, force-save periodically;
    // cancels debounce to avoid double-save.
    if (!autoSaveIntervalTimer) {
      autoSaveIntervalTimer = setTimeout(() => {
        autoSaveIntervalTimer = null
        if (autoSaveTimer) {
          clearTimeout(autoSaveTimer)
          autoSaveTimer = null
        }
        if (saveStatus.value === 'dirty') doSave()
      }, config.autoSaveIntervalMs)
    }
  }

  /**
   * Clears all active timers. Must be called from onBeforeUnmount and
   * before loading a new note.
   */
  const clearTimers = () => {
    if (savedFadeTimer) { clearTimeout(savedFadeTimer); savedFadeTimer = null }
    if (autoSaveTimer) { clearTimeout(autoSaveTimer); autoSaveTimer = null }
    if (autoSaveIntervalTimer) { clearTimeout(autoSaveIntervalTimer); autoSaveIntervalTimer = null }
  }

  /** Resets save state when a new note is loaded. */
  const resetForLoad = () => {
    clearTimers()
    saveStatus.value = 'idle'
    lastSavedTime.value = null
  }

  /** Saves only if the current status is dirty. Used by HistoryPanel and onBeforeUnmount. */
  const saveIfDirty = (): Promise<void> =>
    saveStatus.value === 'dirty' ? doSave() : Promise.resolve()

  return {
    // State
    saveStatus,
    lastSaveError,
    lastSavedTime,
    // Computed display
    statusText,
    saveStatusStyle,
    saveDotStyle,
    // Actions
    doSave,
    saveIfDirty,
    onContentChanged,
    // Lifecycle helpers
    clearTimers,
    resetForLoad,
  }
}
