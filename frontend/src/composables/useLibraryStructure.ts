import { nextTick, type Ref } from 'vue'
import type { LibraryStructure } from './useLibrary'
import { generateId, pendingNotes } from './useLibrary'

/**
 * Dependencies required by the library structure manipulation composable.
 *
 * All reactive state is passed in by reference so mutations apply to the
 * canonical copies owned by useLibrary.
 */
export interface StructureDeps {
  /** The library structure ref (mutated during move/create-sub operations). */
  structure: Ref<LibraryStructure>
  /** Reactive record of note titles — mutated when creating sub-notes. */
  noteTitles: Record<string, string>
  /** Currently open note — set when creating a sub-note. */
  currentNote: Ref<string | null>
  /** Persists the structure to the server after each mutation. */
  saveStructure: () => Promise<void>
  /** Current UI language code (e.g. 'zh' or 'en'). */
  lang: Ref<string>
}

/**
 * Encapsulates note movement and sub-note creation.
 *
 * Extracted from useLibrary to reduce file size; logic is unchanged.
 */
export function useLibraryStructure(deps: StructureDeps) {
  const { structure, noteTitles, currentNote, saveStructure, lang } = deps

  /**
   * Moves a note to a new position within the tree, then persists the updated structure.
   *
   * The four position modes map to the drag-drop affordances in the sidebar:
   * - 'before' / 'after': sibling reordering within the same parent
   * - 'inside': make `src` a child of `target`
   * - 'root': detach from any parent and place at the top level
   *
   * Local state is mutated first (optimistic), then saved. No reload is needed.
   */
  const moveNote = async (src: string, target: string | null, pos: 'before' | 'after' | 'inside' | 'root') => {
    if (src === target) return
    const s = structure.value
    // Detach from current position before inserting elsewhere.
    const oldP = s.parents[src]; const oldArr = oldP ? s.childOrder[oldP] : s.order
    if (oldArr) {
      const idx = oldArr.indexOf(src); if (idx > -1) oldArr.splice(idx, 1)
    }
    // Insert at the new position.
    if (pos === 'root') {
      delete s.parents[src]; s.order.push(src)
    } else if (target && pos === 'inside') {
      s.parents[src] = target; if (!s.childOrder[target]) s.childOrder[target] = []; s.childOrder[target].push(src)
    } else if (target) {
      const newP = s.parents[target]; if (newP) s.parents[src] = newP; else delete s.parents[src]
      const newArr = newP ? s.childOrder[newP] : s.order; const targetIdx = newArr.indexOf(target)
      if (targetIdx === -1) { newArr.push(src) } else { newArr.splice(pos === 'before' ? targetIdx : targetIdx + 1, 0, src) }
    }
    await saveStructure()
  }

  /**
   * Creates a sub-note under the given parent and updates the structure atomically.
   *
   * The new note is prepended to the parent's childOrder so it appears first.
   * Marked as pending until the editor uploads actual content on first save.
   */
  const createSubNote = async (pid: string, onCreated?: () => void) => {
    const id = generateId()
    const title = lang.value === 'zh' ? '新建子文档' : 'New Sub-document'
    if (!structure.value.childOrder[pid]) structure.value.childOrder[pid] = []
    structure.value.childOrder[pid].unshift(id)
    structure.value.parents[id] = pid
    noteTitles[id] = title
    pendingNotes.add(id)
    currentNote.value = id
    await saveStructure()
    nextTick(() => onCreated?.())
  }

  return { moveNote, createSubNote }
}
