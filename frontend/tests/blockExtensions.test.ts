/**
 * TE-001: Unit tests for Callout and ToggleBlock node extensions.
 *
 * Tests the pure logic that does NOT require a live Tiptap/ProseMirror instance:
 *   - Markdown serializer: verifies the HTML written out for each block type
 *   - parseHTML getAttrs: verifies attribute extraction from raw HTML elements
 *   - HTML-escape in emoji/title attributes
 *
 * Tiptap's Node.create() returns an object with a `.config` property that holds
 * the raw configuration functions — accessible without spinning up an editor.
 */
import { describe, it, expect, vi } from 'vitest'
import { Callout, CALLOUT_DEFAULTS } from '../src/components/Callout'
import { ToggleBlock } from '../src/components/ToggleBlock'

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

/** Minimal markdown-serializer state mock. */
function makeState() {
  const written: string[] = []
  return {
    written,
    write: vi.fn((s: string) => { written.push(s) }),
    renderContent: vi.fn(),
    ensureNewLine: vi.fn(),
    closeBlock: vi.fn(),
    get output() { return written.join('') },
  }
}

/** Create a minimal ProseMirror-like node for serializer tests. */
function makeNode(attrs: Record<string, unknown>) {
  return { attrs }
}

/** Parse an HTML string and return the first matching element. */
function parseEl(html: string, selector: string): HTMLElement {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  return doc.querySelector(selector) as HTMLElement
}

// Access config functions without instantiating Tiptap
const calloutConfig   = (Callout as any).config
const toggleConfig    = (ToggleBlock as any).config

const calloutSerialize = calloutConfig.addStorage().markdown.serialize
const toggleSerialize  = toggleConfig.addStorage().markdown.serialize

const calloutParseRules = calloutConfig.parseHTML()
const toggleParseRules  = toggleConfig.parseHTML()

// ──────────────────────────────────────────────────────────────────
// Callout serializer
// ──────────────────────────────────────────────────────────────────

describe('Callout markdown serializer', () => {
  it('writes opening div with data-callout type', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'warning', emoji: '' }))
    expect(state.output).toContain('data-callout="warning"')
  })

  it('includes data-emoji attribute when emoji is set', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'info', emoji: '💡' }))
    expect(state.output).toContain('data-emoji="💡"')
  })

  it('omits data-emoji attribute when emoji is empty', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'tip', emoji: '' }))
    expect(state.output).not.toContain('data-emoji')
  })

  it('HTML-escapes double quotes in emoji attribute', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'danger', emoji: '"evil"' }))
    expect(state.output).toContain('&quot;evil&quot;')
    // The closing `>` of the opening tag should not be broken by unescaped quotes
    expect(state.output).not.toMatch(/data-emoji="[^"]*"[^"]*"/)
  })

  it('HTML-escapes ampersands in emoji', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'info', emoji: 'A&B' }))
    expect(state.output).toContain('&amp;')
  })

  it('calls state.renderContent to output block children', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'info', emoji: '' }))
    expect(state.renderContent).toHaveBeenCalled()
  })

  it('closes the div tag', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: 'info', emoji: '' }))
    expect(state.output).toContain('</div>')
  })

  it('defaults to info type when type attr is empty string', () => {
    const state = makeState()
    calloutSerialize(state, makeNode({ type: '', emoji: '' }))
    expect(state.output).toContain('data-callout="info"')
  })
})

describe('Callout parseHTML getAttrs', () => {
  const { getAttrs } = calloutParseRules[0]

  it('extracts type from data-callout attribute', () => {
    const el = parseEl('<div data-callout="warning"></div>', '[data-callout]')
    expect(getAttrs(el).type).toBe('warning')
  })

  it('defaults type to info when data-callout is empty', () => {
    const el = parseEl('<div data-callout=""></div>', '[data-callout]')
    expect(getAttrs(el).type).toBe('info')
  })

  it('extracts emoji from data-emoji attribute', () => {
    const el = parseEl('<div data-callout="tip" data-emoji="✅"></div>', '[data-callout]')
    expect(getAttrs(el).emoji).toBe('✅')
  })

  it('defaults emoji to empty string when attribute absent', () => {
    const el = parseEl('<div data-callout="info"></div>', '[data-callout]')
    expect(getAttrs(el).emoji).toBe('')
  })
})

describe('CALLOUT_DEFAULTS', () => {
  it('defines all four types', () => {
    expect(Object.keys(CALLOUT_DEFAULTS)).toEqual(
      expect.arrayContaining(['info', 'warning', 'tip', 'danger'])
    )
  })

  it('each type has a non-empty emoji and label', () => {
    for (const v of Object.values(CALLOUT_DEFAULTS)) {
      expect(v.emoji).toBeTruthy()
      expect(v.label).toBeTruthy()
    }
  })
})

// ──────────────────────────────────────────────────────────────────
// ToggleBlock serializer
// ──────────────────────────────────────────────────────────────────

describe('ToggleBlock markdown serializer', () => {
  it('writes <details open> when open=true', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: 'My Toggle' }))
    expect(state.output).toContain('<details open>')
  })

  it('writes <details> without open when open=false', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: false, title: 'My Toggle' }))
    expect(state.output).toContain('<details>')
    expect(state.output).not.toContain('<details open>')
  })

  it('writes <summary> with title text', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: 'Section A' }))
    expect(state.output).toContain('<summary>Section A</summary>')
  })

  it('defaults title to Toggle when attr is empty', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: '' }))
    expect(state.output).toContain('<summary>Toggle</summary>')
  })

  it('HTML-escapes < and > in title', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: '<script>' }))
    expect(state.output).toContain('&lt;script&gt;')
    expect(state.output).not.toContain('<script>')
  })

  it('calls state.renderContent to output block children', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: 'T' }))
    expect(state.renderContent).toHaveBeenCalled()
  })

  it('closes </details>', () => {
    const state = makeState()
    toggleSerialize(state, makeNode({ open: true, title: 'T' }))
    expect(state.output).toContain('</details>')
  })
})

describe('ToggleBlock parseHTML getAttrs', () => {
  const { getAttrs, contentElement } = toggleParseRules[0]

  it('extracts open=true when <details open>', () => {
    const el = parseEl('<details open><summary>Title</summary><p>body</p></details>', 'details')
    expect(getAttrs(el).open).toBe(true)
  })

  it('extracts open=false when <details> without open attribute', () => {
    const el = parseEl('<details><summary>Title</summary><p>body</p></details>', 'details')
    expect(getAttrs(el).open).toBe(false)
  })

  it('extracts title from <summary> text content', () => {
    const el = parseEl('<details><summary>My Section</summary></details>', 'details')
    expect(getAttrs(el).title).toBe('My Section')
  })

  it('defaults title to Toggle when no <summary>', () => {
    const el = parseEl('<details><p>body</p></details>', 'details')
    expect(getAttrs(el).title).toBe('Toggle')
  })

  it('contentElement excludes the <summary> child', () => {
    const el = parseEl('<details open><summary>Title</summary><p>content</p></details>', 'details')
    const content = contentElement(el)
    expect(content.querySelector('summary')).toBeNull()
    expect(content.querySelector('p')).not.toBeNull()
  })
})
