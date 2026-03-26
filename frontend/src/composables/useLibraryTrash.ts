import axios from 'axios'
import type { Ref } from 'vue'
import type { LibraryStructure } from './useLibrary'
import { pendingNotes } from './useLibrary'

/**
 * Dependencies required by the library trash composable.
 *
 * All reactive state is passed in by reference so trash operations
 * mutate the canonical copies owned by useLibrary.
 */
export interface TrashDeps {
  /** The library structure ref (mutated to move notes in/out of trash). */
  structure: Ref<LibraryStructure>
  /** Reactive record of note titles — entries deleted on permanent removal. */
  noteTitles: Record<string, string>
  /** Reactive record of note tags — entries deleted on permanent removal. */
  noteTags: Record<string, string[]>
  /** Currently open note — cleared when the deleted note is active. */
  currentNote: Ref<string | null>
  /** Persists the structure to the server after each mutation. */
  saveStructure: () => Promise<void>
  /** Reloads the full notes list from the server (used for error recovery). */
  loadNotesList: () => Promise<void>
  /** Whether the library is currently locked (encryption not yet unlocked). */
  isLibraryLocked: () => boolean
  /** Base URL for API calls. */
  API_BASE: string
}

/**
 * Encapsulates all trash-related operations: soft-delete, restore,
 * permanent delete, and empty trash.
 *
 * Extracted from useLibrary to reduce file size; logic is unchanged.
 */
export function useLibraryTrash(deps: TrashDeps) {
  const { structure, noteTitles, noteTags, currentNote, saveStructure, loadNotesList, isLibraryLocked, API_BASE } = deps

  /**
   * Soft-deletes a note by moving it to trash.
   * Children are promoted to top-level. The file stays on disk.
   */
  const deleteNote = async (k: string) => {
    try {
      const pid = structure.value.parents[k]
      if (pid) structure.value.childOrder[pid] = (structure.value.childOrder[pid] || []).filter(x => x !== k)
      else structure.value.order = (structure.value.order || []).filter(x => x !== k)

      // Promote children to top-level
      const children = structure.value.childOrder[k] || []
      for (const childId of children) {
        delete structure.value.parents[childId]
        structure.value.order.push(childId)
      }
      delete structure.value.childOrder[k]; delete structure.value.parents[k]
      // Remove from pinned
      if (structure.value.pinned) structure.value.pinned = structure.value.pinned.filter(x => x !== k)

      // Move to trash (keep title for display in trash list)
      if (!structure.value.trash) structure.value.trash = []
      structure.value.trash.push({ id: k, deletedAt: Date.now() })

      if (currentNote.value === k) currentNote.value = null
      await saveStructure()
    } catch (e) {
      console.error('[YinMo] Delete note failed, re-syncing:', e)
      if (!isLibraryLocked()) loadNotesList().catch(() => {})
    }
  }

  /** Restore a note from the trash back to the top-level order. */
  const restoreNote = async (k: string) => {
    if (!structure.value.trash) return
    structure.value.trash = structure.value.trash.filter(e => e.id !== k)
    if (!structure.value.order) structure.value.order = []
    structure.value.order.unshift(k)
    await saveStructure()
  }

  /** Permanently delete a single note (from trash). */
  const permanentDeleteNote = async (k: string) => {
    if (!structure.value.trash) return
    structure.value.trash = structure.value.trash.filter(e => e.id !== k)
    delete noteTitles[k]; delete noteTags[k]
    await saveStructure()
    if (!pendingNotes.has(k)) {
      axios.delete(`${API_BASE}/notes/${k}`).catch((err) => {
        console.error('[YinMo] Permanent delete failed:', k, err)
      })
    }
    pendingNotes.delete(k)
  }

  /** Empty the entire trash permanently. */
  const emptyTrash = async () => {
    const trashItems = structure.value.trash || []
    structure.value.trash = []
    for (const { id } of trashItems) {
      delete noteTitles[id]; delete noteTags[id]
    }
    await saveStructure()
    const deletePromises = trashItems
      .filter(({ id }) => !pendingNotes.has(id))
      .map(({ id }) => axios.delete(`${API_BASE}/notes/${id}`).catch((err) => {
        console.error('[YinMo] Empty trash delete failed:', id, err)
      }))
    await Promise.all(deletePromises)
    for (const { id } of trashItems) pendingNotes.delete(id)
  }

  return { deleteNote, restoreNote, permanentDeleteNote, emptyTrash }
}
