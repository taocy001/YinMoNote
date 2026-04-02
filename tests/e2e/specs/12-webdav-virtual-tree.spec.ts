/**
 * 12 – WebDAV Virtual Tree
 *
 * Verifies that notes with children in _structure.json appear as virtual
 * WebDAV directories, and that child notes are accessible inside those
 * directories via PROPFIND and GET.
 *
 * Design notes:
 *   - Canonical note IDs (8-digit date + 16 lowercase alphanumeric) are used
 *     so that PUT /api/notes/:id can CREATE the file (non-canonical names are
 *     update-only and return 404 when the file doesn't exist).
 *   - Titles are set explicitly in st.Titles so the WebDAV virtual tree uses
 *     the correct display name without needing to parse note content.
 *   - serverEncrypt is forced off before note creation so plaintext content
 *     is accepted; the original config is restored after each test.
 *   - Each test reads the live structure, merges in its hierarchy, saves,
 *     runs assertions, then removes its entries and deletes the test notes.
 */
import { test, expect, type Page } from '../fixtures'

const BASE = process.env.APP_URL ?? 'http://localhost:8080'
const API  = `${BASE}/api`
const DAV  = `${BASE}/dav`

// ─── Helpers ─────────────────────────────────────────────────────────────────

/** Generate a canonical note ID: YYYYMMDD + 16 lowercase alphanumeric chars. */
function makeNoteId(): string {
  const d = new Date()
  const date = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let tail = ''
  for (let i = 0; i < 16; i++) tail += chars[Math.floor(Math.random() * 36)]
  return `${date}${tail}.md`
}

interface Structure {
  order:      string[]
  parents:    Record<string, string>
  childOrder: Record<string, string[]>
  titles?:    Record<string, string>
  [key: string]: unknown
}

async function getStructure(page: Page): Promise<Structure> {
  const r = await page.request.get(`${API}/structure`)
  try {
    return JSON.parse(await r.text()) as Structure
  } catch {
    return { order: [], parents: {}, childOrder: {}, titles: {} }
  }
}

async function putStructure(page: Page, st: Structure): Promise<void> {
  const r = await page.request.put(`${API}/structure`, { data: st })
  if (!r.ok()) throw new Error(`putStructure failed: ${r.status()} ${await r.text()}`)
}

/**
 * Merge test hierarchy into the live structure and save.
 * Returns the original structure so it can be restored in cleanup.
 */
async function installHierarchy(
  page: Page,
  parentId: string, parentTitle: string,
  childId:  string, childTitle:  string,
): Promise<Structure> {
  const orig = await getStructure(page)
  const next: Structure = {
    ...orig,
    order:      [...(orig.order ?? []), parentId],
    parents:    { ...(orig.parents    ?? {}), [childId]:  parentId },
    childOrder: { ...(orig.childOrder ?? {}), [parentId]: [childId] },
    titles:     { ...(orig.titles     ?? {}), [parentId]: parentTitle, [childId]: childTitle },
  }
  await putStructure(page, next)
  return orig
}

/**
 * Remove test hierarchy entries from the live structure and delete the notes.
 */
async function cleanupHierarchy(
  page: Page,
  orig: Structure,
  parentId: string,
  childId:  string,
): Promise<void> {
  await putStructure(page, orig)
  await page.request.delete(`${API}/notes/${parentId}`)
  await page.request.delete(`${API}/notes/${childId}`)
}

// ─── Tests ────────────────────────────────────────────────────────────────────

test.describe('WebDAV – virtual directory tree', () => {

  test('root PROPFIND lists parent note as virtual directory', async ({ unlockedPage: page }) => {
    // Ensure server-side encryption is off so plaintext notes can be saved.
    expect((await page.request.put(`${API}/config`, { data: { serverEncrypt: false } })).status()).toBe(200)

    const parentId = makeNoteId()
    const childId  = makeNoteId()

    expect((await page.request.put(`${API}/notes/${parentId}`, { data: { content: '# VT1 Parent' } })).status()).toBe(200)
    expect((await page.request.put(`${API}/notes/${childId}`,  { data: { content: '# VT1 Child'  } })).status()).toBe(200)

    const orig = await installHierarchy(page, parentId, 'VT1 Parent', childId, 'VT1 Child')
    try {
      const resp = await page.request.fetch(`${DAV}/`, {
        method: 'PROPFIND',
        headers: { 'Depth': '1' },
      })
      expect(resp.status()).toBe(207)
      const body = await resp.text()
      // Parent note must appear as a virtual directory with its display title
      expect(body).toContain('VT1 Parent')
      // Child note must NOT appear at root depth (it is nested inside the virtual dir)
      expect(body).not.toContain('VT1 Child')
    } finally {
      await cleanupHierarchy(page, orig, parentId, childId)
    }
  })

  test('PROPFIND virtual directory lists child note', async ({ unlockedPage: page }) => {
    expect((await page.request.put(`${API}/config`, { data: { serverEncrypt: false } })).status()).toBe(200)

    const parentId = makeNoteId()
    const childId  = makeNoteId()

    expect((await page.request.put(`${API}/notes/${parentId}`, { data: { content: '# VT2 Parent' } })).status()).toBe(200)
    expect((await page.request.put(`${API}/notes/${childId}`,  { data: { content: '# VT2 Child'  } })).status()).toBe(200)

    const orig = await installHierarchy(page, parentId, 'VT2 Parent', childId, 'VT2 Child')
    try {
      const resp = await page.request.fetch(`${DAV}/VT2%20Parent/`, {
        method: 'PROPFIND',
        headers: { 'Depth': '1' },
      })
      expect(resp.status()).toBe(207)
      const body = await resp.text()
      // Child note must appear inside the virtual directory
      expect(body).toContain('VT2 Child')
    } finally {
      await cleanupHierarchy(page, orig, parentId, childId)
    }
  })

  test('GET child note via virtual path returns its content', async ({ unlockedPage: page }) => {
    expect((await page.request.put(`${API}/config`, { data: { serverEncrypt: false } })).status()).toBe(200)

    const parentId = makeNoteId()
    const childId  = makeNoteId()

    expect((await page.request.put(`${API}/notes/${parentId}`, { data: { content: '# VT3 Parent' } })).status()).toBe(200)
    expect((await page.request.put(`${API}/notes/${childId}`,  { data: { content: '# VT3 Child\n\nVT3 body content' } })).status()).toBe(200)

    const orig = await installHierarchy(page, parentId, 'VT3 Parent', childId, 'VT3 Child')
    try {
      // Access the child note via its virtual path inside the parent directory
      const resp = await page.request.get(`${DAV}/VT3%20Parent/VT3%20Child.md`)
      expect(resp.status()).toBe(200)
      const body = await resp.text()
      expect(body).toContain('VT3 body content')
    } finally {
      await cleanupHierarchy(page, orig, parentId, childId)
    }
  })

  test('notes without children appear as flat files at root', async ({ unlockedPage: page }) => {
    expect((await page.request.put(`${API}/config`, { data: { serverEncrypt: false } })).status()).toBe(200)

    const noteId = makeNoteId()
    expect((await page.request.put(`${API}/notes/${noteId}`, { data: { content: '# VT4 Solo Note' } })).status()).toBe(200)

    // Add solo note to the live structure without any children
    const orig = await getStructure(page)
    const next: Structure = {
      ...orig,
      order:  [...(orig.order ?? []), noteId],
      titles: { ...(orig.titles ?? {}), [noteId]: 'VT4 Solo Note' },
    }
    await putStructure(page, next)

    try {
      const resp = await page.request.fetch(`${DAV}/`, {
        method: 'PROPFIND',
        headers: { 'Depth': '1' },
      })
      expect(resp.status()).toBe(207)
      const body = await resp.text()
      // Solo note must appear in root listing as a flat file
      expect(body).toContain('VT4 Solo Note')
    } finally {
      await putStructure(page, orig)
      await page.request.delete(`${API}/notes/${noteId}`)
    }
  })
})
