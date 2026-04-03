/**
 * TE-005: Unit tests for useLibraryStructure composable.
 *
 * Covers:
 * - moveNote 'before': inserts src before target in the same parent list
 * - moveNote 'after': inserts src after target in the same parent list
 * - moveNote 'inside': makes src a child of target, sets parents[src]
 * - moveNote 'root': detaches src from its parent, appends to top-level order
 * - moveNote no-op when src === target
 * - moveNote removes src from previous parent's childOrder
 * - moveNote creates childOrder entry when target has no children yet
 * - createSubNote: prepends new id to childOrder[pid], sets parents[id]
 * - createSubNote: adds new id to pendingNotes, sets currentNote
 * - createSubNote: calls saveStructure and onCreated callback
 * - createSubNote: uses English title when lang is 'en'
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { useLibraryStructure } from '../src/composables/useLibraryStructure'
import type { LibraryStructure } from '../src/composables/useLibrary'

// ── Mocks ────────────────────────────────────────────────────────────────────

const MOCK_NEW_ID = '20240101abcdefghijklmnop.md'

vi.mock('../src/composables/useLibrary', () => ({
  generateId: vi.fn(() => MOCK_NEW_ID),
  pendingNotes: {
    has: vi.fn(),
    add: vi.fn(),
    delete: vi.fn(),
  },
}))

// Re-import after mocking so the composable uses the mock
import { pendingNotes } from '../src/composables/useLibrary'

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Builds a minimal LibraryStructure with the given notes at root level. */
function makeStructure(order: string[] = [], extra: Partial<LibraryStructure> = {}): LibraryStructure {
  return {
    order,
    childOrder: {},
    parents: {},
    titles: {},
    tags: {},
    trash: [],
    ...extra,
  } as LibraryStructure
}

/** Creates a StructureDeps object wired to a ref wrapping the given structure. */
function makeDeps(s: LibraryStructure, lang = 'en') {
  const structure = ref<LibraryStructure>(s)
  const noteTitles: Record<string, string> = {}
  const currentNote = ref<string | null>(null)
  const saveStructure = vi.fn().mockResolvedValue(undefined)
  return {
    structure,
    noteTitles,
    currentNote,
    saveStructure,
    lang: ref(lang),
  }
}

// ── moveNote ─────────────────────────────────────────────────────────────────

describe('moveNote – root-level reordering', () => {
  const A = 'a.md', B = 'b.md', C = 'c.md'

  it('no-op when src === target', async () => {
    const s = makeStructure([A, B, C])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(A, A, 'before')
    expect(deps.structure.value.order).toEqual([A, B, C])
    expect(deps.saveStructure).not.toHaveBeenCalled()
  })

  it('moves A before C → [B, A, C]', async () => {
    const s = makeStructure([A, B, C])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(A, C, 'before')
    expect(deps.structure.value.order).toEqual([B, A, C])
    expect(deps.saveStructure).toHaveBeenCalledOnce()
  })

  it('moves C after A → [A, C, B]', async () => {
    const s = makeStructure([A, B, C])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(C, A, 'after')
    expect(deps.structure.value.order).toEqual([A, C, B])
  })

  it("moves B to 'root' (already at root) → order unchanged, no parents entry", async () => {
    const s = makeStructure([A, B, C])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(B, null, 'root')
    // B was at root; after move it should be appended (removed from index 1, pushed to end)
    expect(deps.structure.value.order).toEqual([A, C, B])
    expect(deps.structure.value.parents[B]).toBeUndefined()
  })
})

describe('moveNote – child ↔ root transitions', () => {
  const FOLDER = 'folder1', NOTE_A = 'a.md', NOTE_B = 'b.md'

  it("'inside': makes NOTE_A a child of FOLDER", async () => {
    const s = makeStructure([FOLDER, NOTE_A, NOTE_B])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(NOTE_A, FOLDER, 'inside')
    expect(deps.structure.value.parents[NOTE_A]).toBe(FOLDER)
    expect(deps.structure.value.childOrder[FOLDER]).toContain(NOTE_A)
    expect(deps.structure.value.order).not.toContain(NOTE_A)
  })

  it("'inside' creates childOrder entry when FOLDER had no children", async () => {
    const s = makeStructure([FOLDER, NOTE_A])
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(NOTE_A, FOLDER, 'inside')
    expect(deps.structure.value.childOrder[FOLDER]).toEqual([NOTE_A])
  })

  it("'root': detaches NOTE_A from its parent, appends to order", async () => {
    const s = makeStructure([FOLDER, NOTE_B], {
      childOrder: { [FOLDER]: [NOTE_A] },
      parents: { [NOTE_A]: FOLDER },
    })
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(NOTE_A, null, 'root')
    expect(deps.structure.value.parents[NOTE_A]).toBeUndefined()
    expect(deps.structure.value.childOrder[FOLDER]).not.toContain(NOTE_A)
    expect(deps.structure.value.order).toContain(NOTE_A)
  })

  it("removes src from previous parent's childOrder when moving to another folder", async () => {
    const FOLDER2 = 'folder2'
    const s = makeStructure([FOLDER, FOLDER2], {
      childOrder: { [FOLDER]: [NOTE_A], [FOLDER2]: [] },
      parents: { [NOTE_A]: FOLDER },
    })
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(NOTE_A, FOLDER2, 'inside')
    expect(deps.structure.value.childOrder[FOLDER]).not.toContain(NOTE_A)
    expect(deps.structure.value.childOrder[FOLDER2]).toContain(NOTE_A)
    expect(deps.structure.value.parents[NOTE_A]).toBe(FOLDER2)
  })
})

describe('moveNote – sibling reorder within a folder', () => {
  const FOLDER = 'folder1', A = 'a.md', B = 'b.md', C = 'c.md'

  it("'before': moves A before C in childOrder", async () => {
    const s = makeStructure([FOLDER], {
      childOrder: { [FOLDER]: [A, B, C] },
      parents: { [A]: FOLDER, [B]: FOLDER, [C]: FOLDER },
    })
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(A, C, 'before')
    expect(deps.structure.value.childOrder[FOLDER]).toEqual([B, A, C])
  })

  it("'after': moves A after C in childOrder", async () => {
    const s = makeStructure([FOLDER], {
      childOrder: { [FOLDER]: [A, B, C] },
      parents: { [A]: FOLDER, [B]: FOLDER, [C]: FOLDER },
    })
    const deps = makeDeps(s)
    const { moveNote } = useLibraryStructure(deps)
    await moveNote(A, C, 'after')
    expect(deps.structure.value.childOrder[FOLDER]).toEqual([B, C, A])
  })
})

// ── createSubNote ─────────────────────────────────────────────────────────────

describe('createSubNote', () => {
  beforeEach(() => {
    vi.mocked(pendingNotes.add).mockClear()
  })

  const FOLDER = 'folder1'

  it('prepends new note id to childOrder[pid]', async () => {
    const EXISTING = 'existing.md'
    const s = makeStructure([FOLDER], {
      childOrder: { [FOLDER]: [EXISTING] },
      parents: { [EXISTING]: FOLDER },
    })
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.structure.value.childOrder[FOLDER][0]).toBe(MOCK_NEW_ID)
    expect(deps.structure.value.childOrder[FOLDER]).toContain(EXISTING)
  })

  it('creates childOrder entry when parent had none', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.structure.value.childOrder[FOLDER]).toEqual([MOCK_NEW_ID])
  })

  it('sets parents[newId] = pid', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.structure.value.parents[MOCK_NEW_ID]).toBe(FOLDER)
  })

  it('adds new note to pendingNotes', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(pendingNotes.add).toHaveBeenCalledWith(MOCK_NEW_ID)
  })

  it('sets currentNote to new id', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.currentNote.value).toBe(MOCK_NEW_ID)
  })

  it('calls saveStructure', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.saveStructure).toHaveBeenCalledOnce()
  })

  it('calls onCreated callback after save', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s)
    const { createSubNote } = useLibraryStructure(deps)
    const onCreated = vi.fn()
    await createSubNote(FOLDER, onCreated)
    // nextTick fires in the same microtask queue; await one extra tick
    await Promise.resolve()
    expect(onCreated).toHaveBeenCalledOnce()
  })

  it('uses Chinese title when lang is zh', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s, 'zh')
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.noteTitles[MOCK_NEW_ID]).toBe('新建子文档')
  })

  it('uses English title when lang is en', async () => {
    const s = makeStructure([FOLDER])
    const deps = makeDeps(s, 'en')
    const { createSubNote } = useLibraryStructure(deps)
    await createSubNote(FOLDER)
    expect(deps.noteTitles[MOCK_NEW_ID]).toBe('New Sub-document')
  })
})
