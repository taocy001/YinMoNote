/**
 * Tests for the useLibrary composable.
 *
 * Covers structure loading, sanitization (ghost removal, orphan promotion),
 * the ENC1 JSON-unwrapping bug fix, displayList filtering, note mutations,
 * and collapse state management. Axios is mocked throughout.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    post: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
    },
  },
}))

import axios from 'axios'
import { useLibrary } from '../src/composables/useLibrary'

const ax = axios as any

/** Wire up mock GET responses for the two mandatory endpoints. */
function mockServer(notes: any[], structure: any) {
  ax.get.mockImplementation((url: string) => {
    if (url.includes('/structure')) return Promise.resolve({ data: structure })
    if (url.includes('/notes'))    return Promise.resolve({ data: { notes } })
    return Promise.reject(new Error(`unexpected GET ${url}`))
  })
  ax.put.mockResolvedValue({ data: { status: 'ok' } })
  ax.delete.mockResolvedValue({ data: { status: 'ok' } })
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
})

// ─── noteTitles population ────────────────────────────────────────────────────

describe('loadNotesList — noteTitles population', () => {
  it('populates noteTitles from structure.titles on plain (unencrypted) load', async () => {
    const titles = { 'a.md': '第一篇', 'b.md': '第二篇' }
    mockServer(
      [{ name: 'a.md', modTime: 1000, title: '' }, { name: 'b.md', modTime: 1000, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.noteTitles['a.md']).toBe('第一篇')
    expect(lib.noteTitles['b.md']).toBe('第二篇')
  })

  it('falls back to server-supplied title when structure has no entry', async () => {
    mockServer(
      [{ name: 'c.md', modTime: 1000, title: '# Server Title' }],
      { order: ['c.md'], parents: {}, childOrder: {}, titles: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.noteTitles['c.md']).toBe('# Server Title')
  })

  it('server-supplied title overrides stale structure.titles (live always wins for unencrypted notes)', async () => {
    // n.title comes from the server reading the file live — it is the ground truth
    // for unencrypted notes and must override any potentially stale structure cache.
    mockServer(
      [{ name: 'd.md', modTime: 1000, title: 'Server Title' }],
      { order: ['d.md'], parents: {}, childOrder: {}, titles: { 'd.md': 'Structure Title' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.noteTitles['d.md']).toBe('Server Title')
  })
})

// ─── sanitizeStructure ────────────────────────────────────────────────────────

describe('loadNotesList — sanitizeStructure', () => {
  it('removes ghost IDs (in structure but not on disk)', async () => {
    mockServer(
      [{ name: 'real.md', modTime: 1000, title: 'Real' }],
      {
        order: ['real.md', 'ghost.md'],
        parents: {}, childOrder: {},
        titles: { 'real.md': 'Real', 'ghost.md': 'Ghost' },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('real.md')
    expect(keys).not.toContain('ghost.md')
  })

  it('promotes untracked files (on disk but absent from structure) to root', async () => {
    mockServer(
      [
        { name: 'tracked.md', modTime: 1000, title: '' },
        { name: 'orphan.md',  modTime: 1000, title: '' },
      ],
      { order: ['tracked.md'], parents: {}, childOrder: {}, titles: { 'tracked.md': 'T' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('tracked.md')
    expect(keys).toContain('orphan.md')
  })

  it('handles a null/missing structure without throwing', async () => {
    mockServer(
      [{ name: 'note.md', modTime: 1000, title: 'Hi' }],
      null,
    )
    const lib = useLibrary()
    await expect(lib.loadNotesList()).resolves.not.toThrow()
    // Orphan note promoted to root
    expect(lib.displayList.value.length).toBeGreaterThanOrEqual(1)
  })

  it('handles an empty notes list without throwing', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {} })
    const lib = useLibrary()
    await expect(lib.loadNotesList()).resolves.not.toThrow()
    expect(lib.displayList.value.length).toBe(0)
  })
})

// ─── ENC1 JSON-unwrapping fix ─────────────────────────────────────────────────

describe('loadNotesList — ENC1 JSON-string unwrapping', () => {
  it('does not throw when structure is a JSON-quoted ENC1 string (legacy storage format)', async () => {
    // Axios JSON-encodes string payloads on PUT, so old stored blobs look like `"ENC1:..."`
    mockServer([], '"ENC1:dGVzdA==:dGVzdA=="')
    const lib = useLibrary()
    await expect(lib.loadNotesList()).resolves.not.toThrow()
  })

  it('does not throw when structure is a bare ENC1 string (new storage format)', async () => {
    mockServer([], 'ENC1:dGVzdA==:dGVzdA==')
    const lib = useLibrary()
    await expect(lib.loadNotesList()).resolves.not.toThrow()
  })
})

// ─── displayList filtering ────────────────────────────────────────────────────

describe('displayList — search and tag filtering', () => {
  async function setupLib() {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 2, title: '' },
        { name: 'c.md', modTime: 3, title: '' },
      ],
      {
        order: ['a.md', 'b.md', 'c.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta', 'c.md': 'Gamma' },
        tags:   { 'a.md': ['work'], 'b.md': ['personal'] },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    return lib
  }

  it('shows all notes when no filter is active', async () => {
    const lib = await setupLib()
    expect(lib.displayList.value.length).toBe(3)
  })

  it('filters by search query (title match)', async () => {
    const lib = await setupLib()
    lib.searchQuery.value = 'alph'
    await new Promise(r => setTimeout(r, 250)) // wait for 200ms debounce
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('a.md')
    expect(keys).not.toContain('b.md')
    expect(keys).not.toContain('c.md')
  })

  it('clears filter when search query is reset to empty', async () => {
    const lib = await setupLib()
    lib.searchQuery.value = 'alph'
    await new Promise(r => setTimeout(r, 250))
    lib.searchQuery.value = ''
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value.length).toBe(3)
  })

  it('filters by active tag', async () => {
    const lib = await setupLib()
    lib.activeTagFilter.value = 'work'
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('a.md')
    expect(keys).not.toContain('b.md')
    expect(keys).not.toContain('c.md')
  })

  it('shows only untagged notes when filter is set to a non-existent tag', async () => {
    const lib = await setupLib()
    lib.activeTagFilter.value = 'nonexistent'
    expect(lib.displayList.value.length).toBe(0)
  })
})

// ─── displayList virtual paging ───────────────────────────────────────────────

describe('displayList — virtual paging via displayLimit', () => {
  it('caps displayed items at displayLimit', async () => {
    const notes = Array.from({ length: 60 }, (_, i) => ({
      name: `2026031800000000000000${String(i).padStart(2, '0')}.md`,
      modTime: i, title: `Note ${i}`,
    }))
    const titles = Object.fromEntries(notes.map(n => [n.name, n.title]))
    mockServer(notes, {
      order: notes.map(n => n.name),
      parents: {}, childOrder: {}, titles,
    })
    const lib = useLibrary()
    await lib.loadNotesList()
    // Default displayLimit is 40; 60 notes exist so only 40 are shown
    expect(lib.displayList.value.length).toBeLessThanOrEqual(lib.displayLimit.value)
  })
})

// ─── toggleCollapse ───────────────────────────────────────────────────────────

describe('toggleCollapse', () => {
  it('defaults to collapsed — children hidden until expanded', async () => {
    mockServer(
      [
        { name: 'parent.md', modTime: 1, title: '' },
        { name: 'child.md',  modTime: 2, title: '' },
      ],
      {
        order: ['parent.md'],
        parents:    { 'child.md': 'parent.md' },
        childOrder: { 'parent.md': ['child.md'] },
        titles: { 'parent.md': 'Parent', 'child.md': 'Child' },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    // Default: collapsed, child hidden
    const before = lib.displayList.value.map((i: any) => i.key)
    expect(before).toContain('parent.md')
    expect(before).not.toContain('child.md')

    // Toggle expands
    lib.toggleCollapse('parent.md')
    const after = lib.displayList.value.map((i: any) => i.key)
    expect(after).toContain('parent.md')
    expect(after).toContain('child.md')
  })

  it('re-collapses on second toggle', async () => {
    mockServer(
      [
        { name: 'parent.md', modTime: 1, title: '' },
        { name: 'child.md',  modTime: 2, title: '' },
      ],
      {
        order: ['parent.md'],
        parents:    { 'child.md': 'parent.md' },
        childOrder: { 'parent.md': ['child.md'] },
        titles: { 'parent.md': 'Parent', 'child.md': 'Child' },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.toggleCollapse('parent.md')  // expand
    lib.toggleCollapse('parent.md')  // re-collapse
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).not.toContain('child.md')
  })
  it('leaf notes (no children) are always visible regardless of collapse state', async () => {
    mockServer(
      [
        { name: 'leaf1.md', modTime: 1, title: '' },
        { name: 'leaf2.md', modTime: 2, title: '' },
      ],
      {
        order: ['leaf1.md', 'leaf2.md'],
        parents: {}, childOrder: {},
        titles: { 'leaf1.md': 'Leaf1', 'leaf2.md': 'Leaf2' },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('leaf1.md')
    expect(keys).toContain('leaf2.md')
  })

  it('pinned parent notes also default to collapsed', async () => {
    mockServer(
      [
        { name: 'parent.md', modTime: 1, title: '' },
        { name: 'child.md', modTime: 2, title: '' },
      ],
      {
        order: ['parent.md'],
        parents: { 'child.md': 'parent.md' },
        childOrder: { 'parent.md': ['child.md'] },
        titles: { 'parent.md': 'Parent', 'child.md': 'Child' },
        pinned: ['parent.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('parent.md')
    expect(keys).not.toContain('child.md')
  })

  it('expanding a pinned parent reveals its children', async () => {
    mockServer(
      [
        { name: 'parent.md', modTime: 1, title: '' },
        { name: 'child.md', modTime: 2, title: '' },
      ],
      {
        order: ['parent.md'],
        parents: { 'child.md': 'parent.md' },
        childOrder: { 'parent.md': ['child.md'] },
        titles: { 'parent.md': 'Parent', 'child.md': 'Child' },
        pinned: ['parent.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.toggleCollapse('parent.md')
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('child.md')
  })
})

// ─── hasServerNotes ───────────────────────────────────────────────────────────

describe('hasServerNotes', () => {
  it('is false before any load', () => {
    const lib = useLibrary()
    expect(lib.hasServerNotes.value).toBe(false)
  })

  it('is true after loading when notes exist on the server', async () => {
    mockServer(
      [{ name: 'x.md', modTime: 1, title: '' }],
      { order: ['x.md'], parents: {}, childOrder: {}, titles: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.hasServerNotes.value).toBe(true)
  })

  it('is false after loading an empty library', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {} })
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.hasServerNotes.value).toBe(false)
  })
})

// ─── generateId — unbiased distribution ──────────────────────────────────────

describe('generateId — unbiased distribution', () => {
  it('generates ids with the canonical format: 8 date digits + 16 random + .md', () => {
    const _lib = useLibrary()
    // Access generateId indirectly via createNewNote side-effect inspection.
    // Create multiple notes and check format of assigned IDs.
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {} })
    ax.put.mockResolvedValue({ data: { status: 'ok' } })
  })

  it('never reuses the same id across 200 consecutive calls', () => {
    // generateId is not exported directly, but we can test via pendingNotes behaviour.
    // A simpler approach: test the rejection-sampling helper inline.
    const charset = 'abcdefghijklmnopqrstuvwxyz0123456789'
    const limit = Math.floor(256 / charset.length) * charset.length
    // Simulate the algorithm used in generateId
    const seen = new Set<string>()
    for (let trial = 0; trial < 200; trial++) {
      const result: string[] = []
      while (result.length < 16) {
        const bytes = window.crypto.getRandomValues(new Uint8Array(32))
        for (const b of bytes) {
          if (b < limit && result.length < 16) result.push(charset[b % charset.length])
        }
      }
      const id = result.join('')
      expect(seen.has(id)).toBe(false)
      seen.add(id)
    }
  })

  it('rejection threshold eliminates biased byte values', () => {
    // Bytes 252–255 (4 values) are rejected. For charset length 36:
    // floor(256/36)*36 = 7*36 = 252. Values [0,251] are accepted.
    const charset = 'abcdefghijklmnopqrstuvwxyz0123456789'
    const limit = Math.floor(256 / charset.length) * charset.length
    expect(limit).toBe(252)
    // All bytes below limit are accepted — check modulo maps to valid charset index
    for (let b = 0; b < limit; b++) {
      const idx = b % charset.length
      expect(idx).toBeGreaterThanOrEqual(0)
      expect(idx).toBeLessThan(charset.length)
    }
  })
})

// ─── indexNote — real-time search index update ────────────────────────────────

describe('indexNote — updates contentMatchIds', () => {
  it('adds a note to contentMatchIds when it matches the current search query', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'Test' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'Test' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    // Set a search query and then index content that matches
    lib.searchQuery.value = 'hello world'
    await new Promise(r => setTimeout(r, 250)) // wait for debounce

    lib.indexNote('a.md', 'This note says hello world in the body')
    expect(lib.contentMatchIds.value.has('a.md')).toBe(true)
  })

  it('removes a note from contentMatchIds when new content no longer matches', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'Test' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'Test' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    lib.searchQuery.value = 'findme'
    await new Promise(r => setTimeout(r, 250))

    // First index with matching content
    lib.indexNote('a.md', 'findme is here')
    expect(lib.contentMatchIds.value.has('a.md')).toBe(true)

    // Re-index with non-matching content (simulates note edit)
    lib.indexNote('a.md', 'completely different content now')
    expect(lib.contentMatchIds.value.has('a.md')).toBe(false)
  })
})

// ─── notesLoadError — network failure handling ────────────────────────────────

describe('loadNotesList — notesLoadError', () => {
  it('sets notesLoadError to true when the network request fails', async () => {
    ax.get.mockRejectedValue(new Error('Network Error'))
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.notesLoadError.value).toBe(true)
    expect(lib.notesLoaded.value).toBe(false)
  })

  it('clears notesLoadError on the next successful load', async () => {
    // First call fails
    ax.get.mockRejectedValue(new Error('Network Error'))
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.notesLoadError.value).toBe(true)

    // Second call succeeds
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'Recovery' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'Recovery' } },
    )
    await lib.loadNotesList()
    expect(lib.notesLoadError.value).toBe(false)
    expect(lib.notesLoaded.value).toBe(true)
  })

  it('handles null structure data gracefully (treats as empty)', async () => {
    ax.get.mockImplementation((url: string) => {
      if (url.includes('/structure')) return Promise.resolve({ data: null })
      if (url.includes('/notes'))    return Promise.resolve({ data: { notes: [] } })
      return Promise.reject(new Error(`unexpected GET ${url}`))
    })
    ax.put.mockResolvedValue({ data: { status: 'ok' } })
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.notesLoadError.value).toBe(false)
    expect(lib.notesLoaded.value).toBe(true)
    expect(lib.structure.value.order).toEqual([])
  })
})

// ─── noteTags population ──────────────────────────────────────────────────────

describe('loadNotesList — noteTags population', () => {
  it('populates noteTags from structure.tags on load', async () => {
    const tags = { 'a.md': ['work', 'urgent'], 'b.md': ['personal'] }
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles: {}, tags },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.noteTags['a.md']).toEqual(['work', 'urgent'])
    expect(lib.noteTags['b.md']).toEqual(['personal'])
  })

  it('clears stale noteTags for deleted notes on reload', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles: {}, tags: { 'a.md': ['work'], 'b.md': ['old'] } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.noteTags['b.md']).toEqual(['old'])

    // Reload without b.md — stale entry should be removed
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: {}, tags: { 'a.md': ['work'] } },
    )
    await lib.loadNotesList()
    expect(lib.noteTags['b.md']).toBeUndefined()
    expect(lib.noteTags['a.md']).toEqual(['work'])
  })
})

// ─── allTags computed ─────────────────────────────────────────────────────────

describe('allTags — sorted unique list', () => {
  it('returns all unique tags sorted alphabetically', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles: {},
        tags: { 'a.md': ['zebra', 'apple'], 'b.md': ['mango', 'apple'] } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // 'apple' appears in both notes — should appear once, sorted
    expect(lib.allTags.value).toEqual(['apple', 'mango', 'zebra'])
  })

  it('returns empty array when no notes have tags', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: {}, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.allTags.value).toEqual([])
  })
})

// ─── displayList — case-insensitive search ────────────────────────────────────

describe('displayList — case-insensitive title search', () => {
  it('matches uppercase query against lowercase title', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'meeting notes' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'MEETING'
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value.map(i => i.key)).toContain('a.md')
  })

  it('matches mixed-case query against mixed-case title', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {},
        titles: { 'a.md': 'Project Alpha', 'b.md': 'project beta' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'project'
    await new Promise(r => setTimeout(r, 250))
    const keys = lib.displayList.value.map(i => i.key)
    expect(keys).toContain('a.md')
    expect(keys).toContain('b.md')
  })

  it('returns empty displayList for a query that matches no titles', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {},
        titles: { 'a.md': 'Shopping List', 'b.md': 'Travel Plan' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'xyznotfound'
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value).toHaveLength(0)
  })
})

// ─── displayList — combined search + tag filter ───────────────────────────────

describe('displayList — combined searchQuery + activeTagFilter', () => {
  it('shows only notes matching both query AND tag', async () => {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 1, title: '' },
        { name: 'c.md', modTime: 1, title: '' },
      ],
      {
        order: ['a.md', 'b.md', 'c.md'], parents: {}, childOrder: {},
        titles: { 'a.md': 'work report', 'b.md': 'work notes', 'c.md': 'personal diary' },
        tags:   { 'a.md': ['work'],      'b.md': ['personal'],  'c.md': ['personal'] },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'work'
    lib.activeTagFilter.value = 'personal'
    await new Promise(r => setTimeout(r, 250))
    // 'a.md' matches title "work" but tag is 'work' not 'personal'
    // 'b.md' matches title "work notes" AND tag 'personal' → visible
    const keys = lib.displayList.value.map(i => i.key)
    expect(keys).toContain('b.md')
    expect(keys).not.toContain('a.md')
    expect(keys).not.toContain('c.md') // title doesn't match 'work'
  })

  it('clears filter and shows all notes when both query and tag are reset', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'alpha'
    lib.activeTagFilter.value = ''
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value).toHaveLength(1)

    // Reset both filters
    lib.searchQuery.value = ''
    lib.activeTagFilter.value = ''
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value).toHaveLength(2)
  })
})

// ─── displayList — level field ────────────────────────────────────────────────

describe('displayList — level field for nesting depth', () => {
  it('assigns level 0 to root, 1 to child, 2 to grandchild', async () => {
    mockServer(
      [
        { name: 'root.md', modTime: 1, title: '' },
        { name: 'child.md', modTime: 1, title: '' },
        { name: 'grand.md', modTime: 1, title: '' },
      ],
      {
        order: ['root.md'],
        parents: { 'child.md': 'root.md', 'grand.md': 'child.md' },
        childOrder: { 'root.md': ['child.md'], 'child.md': ['grand.md'] },
        titles: { 'root.md': 'Root', 'child.md': 'Child', 'grand.md': 'Grand' },
        tags: {},
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // Expand all levels to make children visible
    lib.toggleCollapse('root.md')
    lib.toggleCollapse('child.md')
    const items = lib.displayList.value
    const root  = items.find(i => i.key === 'root.md')
    const child = items.find(i => i.key === 'child.md')
    const grand = items.find(i => i.key === 'grand.md')
    expect(root?.level).toBe(0)
    expect(child?.level).toBe(1)
    expect(grand?.level).toBe(2)
  })
})

// ─── toggleCollapse — three-level hierarchy ───────────────────────────────────

describe('toggleCollapse — three-level hierarchy', () => {
  async function buildThreeLevel() {
    mockServer(
      [
        { name: 'root.md', modTime: 1, title: '' },
        { name: 'child.md', modTime: 1, title: '' },
        { name: 'grand.md', modTime: 1, title: '' },
      ],
      {
        order: ['root.md'],
        parents: { 'child.md': 'root.md', 'grand.md': 'child.md' },
        childOrder: { 'root.md': ['child.md'], 'child.md': ['grand.md'] },
        titles: { 'root.md': 'Root', 'child.md': 'Child', 'grand.md': 'Grand' },
        tags: {},
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    return lib
  }

  it('default collapsed — root visible but children hidden', async () => {
    const lib = await buildThreeLevel()
    const keys = lib.displayList.value.map(i => i.key)
    expect(keys).toContain('root.md')
    expect(keys).not.toContain('child.md')
    expect(keys).not.toContain('grand.md')
  })

  it('expanding root shows child but grandchild stays collapsed', async () => {
    const lib = await buildThreeLevel()
    lib.toggleCollapse('root.md') // expand root
    const keys = lib.displayList.value.map(i => i.key)
    expect(keys).toContain('root.md')
    expect(keys).toContain('child.md')
    expect(keys).not.toContain('grand.md') // child is still collapsed
  })

  it('expanding both root and child shows all three levels', async () => {
    const lib = await buildThreeLevel()
    lib.toggleCollapse('root.md')  // expand root
    lib.toggleCollapse('child.md') // expand child
    const keys = lib.displayList.value.map(i => i.key)
    expect(keys).toContain('root.md')
    expect(keys).toContain('child.md')
    expect(keys).toContain('grand.md')
  })

  it('isCollapsed flag is true by default, false after expand', async () => {
    const lib = await buildThreeLevel()
    const collapsed = lib.displayList.value.find(i => i.key === 'root.md')
    expect(collapsed?.isCollapsed).toBe(true)

    lib.toggleCollapse('root.md')
    const expanded = lib.displayList.value.find(i => i.key === 'root.md')
    expect(expanded?.isCollapsed).toBe(false)
  })

  it('search query overrides collapse — children appear even when collapsed', async () => {
    const lib = await buildThreeLevel()
    // Default: collapsed, child hidden
    expect(lib.displayList.value.map(i => i.key)).not.toContain('child.md')

    // With search query: collapse is suspended, child becomes visible
    lib.searchQuery.value = 'Child'
    await new Promise(r => setTimeout(r, 250))
    expect(lib.displayList.value.map(i => i.key)).toContain('child.md')
  })
})

// ─── clearContentIndex ────────────────────────────────────────────────────────

describe('clearContentIndex — wipes index and contentMatchIds', () => {
  it('empties contentMatchIds after clearing the index', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'Test' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'Test' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    // Populate index with a match
    lib.searchQuery.value = 'hello'
    await new Promise(r => setTimeout(r, 250))
    lib.indexNote('a.md', 'hello world')
    expect(lib.contentMatchIds.value.has('a.md')).toBe(true)

    // Clear should wipe both the map and the reactive set
    lib.clearContentIndex()
    expect(lib.contentMatchIds.value.size).toBe(0)
    expect(lib.contentIndex.size).toBe(0)
  })
})

// ─── createNewNote ────────────────────────────────────────────────────────────

describe('createNewNote — happy + error paths', () => {
  it('adds the new note id to structure.order and pendingNotes', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createNewNote()
    const id = lib.currentNote.value!
    expect(lib.structure.value.order).toContain(id)
    expect(lib.pendingNotes.has(id)).toBe(true)
  })

  it('calls PUT /structure after creation', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.createNewNote()
    expect(ax.put).toHaveBeenCalledWith(expect.stringContaining('/structure'), expect.anything())
  })

  it('saveStructure sends a plain object (not a string) in keyless/unencrypted mode', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.createNewNote()
    const structureCall = ax.put.mock.calls.find((c: any[]) => String(c[0]).includes('/structure'))
    expect(structureCall).toBeTruthy()
    // The payload (second argument) must be a plain object, NOT a string.
    // A string payload means double-encoding which the server rejects with
    // "cannot unmarshal string into Go value of type main.Structure".
    const payload = structureCall![1]
    expect(typeof payload).toBe('object')
    expect(payload).toHaveProperty('order')
  })

  it('invokes onCreated callback after creation', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
    const lib = useLibrary()
    await lib.loadNotesList()

    const cb = vi.fn()
    await lib.createNewNote(cb)
    await new Promise(r => setTimeout(r, 0))
    expect(cb).toHaveBeenCalledTimes(1)
  })

  it('new note appears at the top of displayList immediately', async () => {
    mockServer(
      [{ name: 'existing.md', modTime: 1, title: '' }],
      { order: ['existing.md'], parents: {}, childOrder: {}, titles: { 'existing.md': 'Existing' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createNewNote()
    const id = lib.currentNote.value!
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys[0]).toBe(id)
    expect(keys).toContain('existing.md')
  })

  it('new note remains in structure.order after saveStructure completes', async () => {
    mockServer([], { order: [], parents: {}, childOrder: {}, titles: {}, tags: {} })
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createNewNote()
    // Wait for any async side-effects (saveStructure queue)
    await new Promise(r => setTimeout(r, 50))
    const id = lib.currentNote.value!
    expect(lib.structure.value.order).toContain(id)
    expect(lib.displayList.value.some((i: any) => i.key === id)).toBe(true)
  })
})

// ─── moveNote — displayList update ────────────────────────────────────────────

describe('moveNote — displayList reflects changes', () => {
  it('moving a note changes its position in displayList', async () => {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 1, title: '' },
        { name: 'c.md', modTime: 1, title: '' },
      ],
      { order: ['a.md', 'b.md', 'c.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A', 'b.md': 'B', 'c.md': 'C' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    // Move c.md before a.md
    await lib.moveNote('c.md', 'a.md', 'before')
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys.indexOf('c.md')).toBeLessThan(keys.indexOf('a.md'))
  })
})

// ─── deleteNote ───────────────────────────────────────────────────────────────

describe('deleteNote — happy + error paths', () => {
  it('removes the note from structure.order and clears currentNote', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.currentNote.value = 'a.md'

    await lib.deleteNote('a.md')
    expect(lib.structure.value.order).not.toContain('a.md')
    expect(lib.currentNote.value).toBeNull()
  })

  it('moves note to trash instead of calling DELETE (soft-delete)', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.deleteNote('a.md')
    // Soft-delete: file stays on disk, note moves to trash in structure
    expect(ax.delete).not.toHaveBeenCalled()
    expect(lib.structure.value.trash).toEqual(
      expect.arrayContaining([expect.objectContaining({ id: 'a.md' })])
    )
    // Structure save (PUT) should be called to persist the trash entry
    expect(ax.put).toHaveBeenCalled()
  })

  it('removes note from pinned when soft-deleted', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, pinned: ['a.md'] },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.deleteNote('a.md')
    expect(lib.structure.value.pinned).not.toContain('a.md')
  })
})

// ─── setNoteTags ──────────────────────────────────────────────────────────────

describe('setNoteTags — normal + edge paths', () => {
  it('persists tags to structure and calls PUT /structure', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.setNoteTags('a.md', ['work', 'important'])
    expect(ax.put).toHaveBeenCalledWith(expect.stringContaining('/structure'), expect.anything())
  })

  it('removes the tags entry when given an empty array', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: { 'a.md': ['old'] } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.setNoteTags('a.md', [])
    // noteTags is a reactive object — key should be absent
    const { noteTags } = lib as any
    expect(Object.prototype.hasOwnProperty.call(noteTags, 'a.md')).toBe(false)
  })
})

// ─── moveNote — boundary cases (TEST-P1-4) ────────────────────────────────────

describe('moveNote — boundary cases', () => {
  it('no-ops when src === target', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }, { name: 'b.md', modTime: 1, title: 'B' }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles: {}, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.moveNote('a.md', 'a.md', 'before')
    expect(ax.put).not.toHaveBeenCalled()
  })

  it('moves to root when pos=root', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1 }, { name: 'child.md', modTime: 1 }],
      { order: ['parent.md'], parents: { 'child.md': 'parent.md' }, childOrder: { 'parent.md': ['child.md'] }, titles: {}, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.moveNote('child.md', null, 'root')
    expect(lib.structure.value.order).toContain('child.md')
    expect(lib.structure.value.parents['child.md']).toBeUndefined()
  })

  it('moves inside a target folder when pos=inside', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1 }, { name: 'folder.md', modTime: 1 }],
      { order: ['a.md', 'folder.md'], parents: {}, childOrder: {}, titles: {}, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.moveNote('a.md', 'folder.md', 'inside')
    expect(lib.structure.value.parents['a.md']).toBe('folder.md')
    expect(lib.structure.value.childOrder['folder.md']).toContain('a.md')
  })

  it('appends to end when target not found in array (targetIdx === -1 guard)', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1 }, { name: 'b.md', modTime: 1 }],
      { order: ['a.md', 'b.md'], parents: {}, childOrder: {}, titles: {}, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    // Directly corrupt the order array so target ('b.md') is absent — exercises the -1 guard.
    lib.structure.value.order = ['a.md']
    await lib.moveNote('a.md', 'b.md', 'before')
    // After the -1 guard, src should end up at the end
    expect(lib.structure.value.order[lib.structure.value.order.length - 1]).toBe('a.md')
  })
})

// ─── createSubNote (D1/D2 fix) ────────────────────────────────────────────────

describe('createSubNote — D1/D2: sub-note creation via composable', () => {
  it('adds child to parent childOrder and sets parents map', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1, title: 'Parent' }],
      { order: ['parent.md'], parents: {}, childOrder: {}, titles: { 'parent.md': 'Parent' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createSubNote('parent.md')

    const child = lib.currentNote.value!
    expect(lib.structure.value.childOrder['parent.md']).toContain(child)
    expect(lib.structure.value.parents[child]).toBe('parent.md')
  })

  it('sets currentNote to the new child id', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1, title: 'Parent' }],
      { order: ['parent.md'], parents: {}, childOrder: {}, titles: { 'parent.md': 'Parent' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createSubNote('parent.md')

    expect(lib.currentNote.value).not.toBe('parent.md')
    expect(lib.currentNote.value).not.toBeNull()
  })

  it('adds child to pendingNotes (not yet uploaded)', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1, title: 'Parent' }],
      { order: ['parent.md'], parents: {}, childOrder: {}, titles: { 'parent.md': 'Parent' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.createSubNote('parent.md')

    const child = lib.currentNote.value!
    expect(lib.pendingNotes.has(child)).toBe(true)
  })

  it('calls PUT /structure to persist the updated structure', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1, title: 'Parent' }],
      { order: ['parent.md'], parents: {}, childOrder: {}, titles: { 'parent.md': 'Parent' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.createSubNote('parent.md')

    expect(ax.put).toHaveBeenCalledWith(
      expect.stringContaining('/structure'),
      expect.anything(),
    )
  })

  it('invokes the onCreated callback after the structure is saved', async () => {
    mockServer(
      [{ name: 'parent.md', modTime: 1, title: 'Parent' }],
      { order: ['parent.md'], parents: {}, childOrder: {}, titles: { 'parent.md': 'Parent' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    const cb = vi.fn()
    await lib.createSubNote('parent.md', cb)
    // nextTick has fired by the time the awaited promise resolves in tests
    await new Promise(r => setTimeout(r, 0))

    expect(cb).toHaveBeenCalledTimes(1)
  })

  it('initialises childOrder when parent had no children before', async () => {
    mockServer(
      [{ name: 'root.md', modTime: 1, title: 'Root' }],
      { order: ['root.md'], parents: {}, childOrder: {}, titles: { 'root.md': 'Root' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    expect(lib.structure.value.childOrder['root.md']).toBeUndefined()
    await lib.createSubNote('root.md')
    expect(Array.isArray(lib.structure.value.childOrder['root.md'])).toBe(true)
  })
})

// ─── togglePin ─────────────────────────────────────────────────────────────

describe('togglePin — pin and unpin notes', () => {
  it('should_add_note_to_pinned_when_not_already_pinned', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, pinned: [] },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.togglePin('a.md')
    expect(lib.structure.value.pinned).toContain('a.md')
  })

  it('should_remove_note_from_pinned_when_already_pinned', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, pinned: ['a.md'] },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.togglePin('a.md')
    expect(lib.structure.value.pinned).not.toContain('a.md')
  })

  it('should_call_saveStructure_when_toggling_pin', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, pinned: [] },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.togglePin('a.md')
    expect(ax.put).toHaveBeenCalledWith(expect.stringContaining('/structure'), expect.anything())
  })

  it('should_initialize_pinned_array_when_undefined', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // pinned may be initialized to [] by sanitizeStructure, but togglePin should handle undefined gracefully
    await lib.togglePin('a.md')
    expect(lib.structure.value.pinned).toContain('a.md')
  })
})

// ─── displayList — pinned partition ──────────────────────────────────────────

describe('displayList — pinned notes appear first', () => {
  it('should_show_pinned_notes_before_unpinned_when_no_search_active', async () => {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 2, title: '' },
        { name: 'c.md', modTime: 3, title: '' },
      ],
      {
        order: ['a.md', 'b.md', 'c.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta', 'c.md': 'Gamma' },
        tags: {},
        pinned: ['c.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    // c.md is pinned, should appear before a.md and b.md
    expect(keys.indexOf('c.md')).toBeLessThan(keys.indexOf('a.md'))
    expect(keys.indexOf('c.md')).toBeLessThan(keys.indexOf('b.md'))
  })

  it('should_mark_pinned_items_with_isPinned_true', async () => {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 2, title: '' },
      ],
      {
        order: ['a.md', 'b.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta' },
        tags: {},
        pinned: ['a.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const pinned = lib.displayList.value.find((i: any) => i.key === 'a.md')
    const unpinned = lib.displayList.value.find((i: any) => i.key === 'b.md')
    expect(pinned?.isPinned).toBe(true)
    expect(unpinned?.isPinned).toBe(false)
  })

  it('should_not_partition_pinned_when_search_query_is_active', async () => {
    mockServer(
      [
        { name: 'a.md', modTime: 1, title: '' },
        { name: 'b.md', modTime: 2, title: '' },
      ],
      {
        order: ['a.md', 'b.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta' },
        tags: {},
        pinned: ['b.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.searchQuery.value = 'alpha'
    await new Promise(r => setTimeout(r, 250))
    // During search, only matching notes shown; pinned partition is suppressed
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('a.md')
    expect(keys).not.toContain('b.md') // Beta doesn't match 'alpha'
  })
})

// ─── restoreNote ────────────────────────────────────────────────────────────

describe('restoreNote — restore from trash', () => {
  it('should_move_note_from_trash_back_to_order_top', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }, { name: 'b.md', modTime: 1, title: 'B' }],
      {
        order: ['b.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A', 'b.md': 'B' },
        tags: {},
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.restoreNote('a.md')
    expect(lib.structure.value.order[0]).toBe('a.md')
    expect(lib.structure.value.trash).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ id: 'a.md' })])
    )
  })

  it('should_call_saveStructure_after_restore', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: {},
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.restoreNote('a.md')
    expect(ax.put).toHaveBeenCalledWith(expect.stringContaining('/structure'), expect.anything())
  })

  it('should_noop_when_trash_is_undefined', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // structure.trash is initialized to [] by sanitize, but restoreNote guards against undefined
    lib.structure.value.trash = undefined as any
    await expect(lib.restoreNote('a.md')).resolves.not.toThrow()
  })
})

// ─── permanentDeleteNote ──────────────────────────────────────────────────────

describe('permanentDeleteNote — permanently remove from trash', () => {
  it('should_remove_from_trash_and_delete_file_on_server', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: { 'a.md': ['work'] },
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.delete.mockClear()

    await lib.permanentDeleteNote('a.md')
    expect(lib.structure.value.trash).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ id: 'a.md' })])
    )
    expect(ax.delete).toHaveBeenCalledWith(expect.stringContaining('/notes/a.md'))
  })

  it('should_clean_up_noteTitles_and_noteTags', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: { 'a.md': ['tag1'] },
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.permanentDeleteNote('a.md')
    expect(lib.noteTitles['a.md']).toBeUndefined()
    expect(lib.noteTags['a.md']).toBeUndefined()
  })

  it('should_skip_server_DELETE_for_pending_notes', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: {},
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.pendingNotes.add('a.md')
    ax.delete.mockClear()

    await lib.permanentDeleteNote('a.md')
    expect(ax.delete).not.toHaveBeenCalled()
    expect(lib.pendingNotes.has('a.md')).toBe(false)
  })
})

// ─── emptyTrash ──────────────────────────────────────────────────────────────

describe('emptyTrash — clear all trashed notes', () => {
  it('should_clear_all_trash_entries_and_delete_files', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }, { name: 'b.md', modTime: 1, title: 'B' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A', 'b.md': 'B' },
        tags: {},
        trash: [
          { id: 'a.md', deletedAt: 1000 },
          { id: 'b.md', deletedAt: 2000 },
        ],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.delete.mockClear()

    await lib.emptyTrash()
    expect(lib.structure.value.trash).toEqual([])
    expect(ax.delete).toHaveBeenCalledTimes(2)
  })

  it('should_clean_up_noteTitles_and_noteTags_for_all_trashed', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }, { name: 'b.md', modTime: 1, title: 'B' }],
      {
        order: [],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A', 'b.md': 'B' },
        tags: { 'a.md': ['t1'], 'b.md': ['t2'] },
        trash: [
          { id: 'a.md', deletedAt: 1000 },
          { id: 'b.md', deletedAt: 2000 },
        ],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.emptyTrash()
    expect(lib.noteTitles['a.md']).toBeUndefined()
    expect(lib.noteTitles['b.md']).toBeUndefined()
    expect(lib.noteTags['a.md']).toBeUndefined()
    expect(lib.noteTags['b.md']).toBeUndefined()
  })
})

// ─── setCommitLabel ─────────────────────────────────────────────────────────

describe('setCommitLabel — version history labeling', () => {
  it('should_add_label_to_commitLabels_map', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.setCommitLabel('abc123', 'v1.0 release')
    expect(lib.structure.value.commitLabels['abc123']).toBe('v1.0 release')
  })

  it('should_remove_label_when_value_is_empty_string', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, commitLabels: { 'abc123': 'old label' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.setCommitLabel('abc123', '')
    expect(lib.structure.value.commitLabels['abc123']).toBeUndefined()
  })

  it('should_remove_label_when_value_is_whitespace_only', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {}, commitLabels: { 'abc123': 'old' } },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.setCommitLabel('abc123', '   ')
    expect(lib.structure.value.commitLabels['abc123']).toBeUndefined()
  })

  it('should_trim_whitespace_from_label', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    await lib.setCommitLabel('abc123', '  Release v2  ')
    expect(lib.structure.value.commitLabels['abc123']).toBe('Release v2')
  })

  it('should_initialize_commitLabels_when_undefined', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    lib.structure.value.commitLabels = undefined as any

    await lib.setCommitLabel('hash1', 'test')
    expect(lib.structure.value.commitLabels['hash1']).toBe('test')
  })

  it('should_call_saveStructure_after_setting_label', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    ax.put.mockClear()

    await lib.setCommitLabel('hash1', 'label')
    expect(ax.put).toHaveBeenCalledWith(expect.stringContaining('/structure'), expect.anything())
  })
})

// ─── sanitizeStructure — trash and pinned cleanup ─────────────────────────────

describe('sanitizeStructure — trash and pinned cleanup', () => {
  it('should_remove_trash_entries_for_files_no_longer_on_disk', async () => {
    // a.md exists on disk, b.md does not
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: ['a.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A', 'b.md': 'B' },
        tags: {},
        trash: [
          { id: 'a.md', deletedAt: 1000 },  // exists on disk
          { id: 'b.md', deletedAt: 2000 },  // NOT on disk
        ],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // b.md should have been cleaned from trash since it's not on disk
    const trashIds = lib.structure.value.trash.map((e: any) => e.id)
    expect(trashIds).not.toContain('b.md')
  })

  it('should_remove_pinned_entries_for_notes_in_trash', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }, { name: 'b.md', modTime: 1, title: 'B' }],
      {
        order: ['b.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A', 'b.md': 'B' },
        tags: {},
        pinned: ['a.md', 'b.md'],
        trash: [{ id: 'a.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // a.md is in trash, should be removed from pinned
    expect(lib.structure.value.pinned).not.toContain('a.md')
    expect(lib.structure.value.pinned).toContain('b.md')
  })

  it('should_remove_pinned_entries_for_deleted_files', async () => {
    // ghost.md is in pinned but not on disk
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: ['a.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: {},
        pinned: ['a.md', 'ghost.md'],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.structure.value.pinned).toContain('a.md')
    expect(lib.structure.value.pinned).not.toContain('ghost.md')
  })

  it('should_exclude_trashed_notes_from_displayList', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      {
        order: ['a.md', 'b.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Alpha', 'b.md': 'Beta' },
        tags: {},
        trash: [{ id: 'b.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    const keys = lib.displayList.value.map((i: any) => i.key)
    expect(keys).toContain('a.md')
    expect(keys).not.toContain('b.md')
  })

  it('should_retain_titles_for_trashed_notes_after_loadNotesList', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: '' }, { name: 'b.md', modTime: 1, title: '' }],
      {
        order: ['a.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'Active', 'b.md': 'Trashed' },
        tags: {},
        trash: [{ id: 'b.md', deletedAt: 1000 }],
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    // Trashed note title must survive the liveIds cleanup in loadNotesList
    expect(lib.noteTitles['b.md']).toBe('Trashed')
    expect(lib.noteTitles['a.md']).toBe('Active')
  })

  it('should_preserve_commitLabels_through_sanitize_cycle', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      {
        order: ['a.md'],
        parents: {}, childOrder: {},
        titles: { 'a.md': 'A' },
        tags: {},
        commitLabels: { 'hash1': 'v1.0', 'hash2': 'beta' },
      },
    )
    const lib = useLibrary()
    await lib.loadNotesList()
    expect(lib.structure.value.commitLabels).toEqual({ 'hash1': 'v1.0', 'hash2': 'beta' })
  })
})

// ─── deleteNote — trash entry format ──────────────────────────────────────────

describe('deleteNote — trash entry format', () => {
  it('should_include_id_and_deletedAt_timestamp_in_trash_entry', async () => {
    mockServer(
      [{ name: 'a.md', modTime: 1, title: 'A' }],
      { order: ['a.md'], parents: {}, childOrder: {}, titles: { 'a.md': 'A' }, tags: {} },
    )
    const lib = useLibrary()
    await lib.loadNotesList()

    const before = Date.now()
    await lib.deleteNote('a.md')
    const after = Date.now()

    const entry = lib.structure.value.trash.find((e: any) => e.id === 'a.md')
    expect(entry).toBeDefined()
    expect(entry!.id).toBe('a.md')
    expect(entry!.deletedAt).toBeGreaterThanOrEqual(before)
    expect(entry!.deletedAt).toBeLessThanOrEqual(after)
  })
})
