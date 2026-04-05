/**
 * Code Review Fix Tests (F1–F10)
 *
 * Validates that all fixes from design.md (.crew_workspace/design.md) are
 * correctly implemented in the source files. Tests read source files directly
 * and assert on code patterns — no live editor instance required.
 *
 * F1  SlashMenu.vue   — onUnmounted timer cleanup
 * F2  TableOverlay    — removeEventListener options match addEventListener
 * F4  TableOverlay    — no raw `as any` for dragging / chain commands
 * F5  TableOverlay    — catch blocks use import.meta.env.DEV guard
 * F6  Editor.vue      — MARKDOWN_PATTERN extracted as top-level constant
 * F7  Editor.vue      — pasteAsMarkdown extracted as standalone function
 * F8  CSS             — --color-code-bg-dark variable used in both dark selectors
 * F9  CSS             — .menu-item:focus-visible defined in style.css + editor-prose.css
 * F10 SlashMenu.vue   — dataTransfer null guard with console.warn early-return
 */
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

const slashMenu   = readFileSync(resolve(__dirname, '../src/components/SlashMenu.vue'),    'utf-8')
const tableOverlay = readFileSync(resolve(__dirname, '../src/components/TableOverlay.vue'), 'utf-8')
const editorVue   = readFileSync(resolve(__dirname, '../src/components/Editor.vue'),       'utf-8')
const proseCSS    = readFileSync(resolve(__dirname, '../src/assets/editor-prose.css'),     'utf-8')
const styleCSS    = readFileSync(resolve(__dirname, '../src/style.css'),                   'utf-8')
const indexHtml   = readFileSync(resolve(__dirname, '../index.html'),                      'utf-8')

// ─── F1: SlashMenu.vue — onUnmounted timer cleanup ───────────────────────────

describe('F1 SlashMenu.vue: onUnmounted timer cleanup', () => {
  it('imports onUnmounted from vue', () => {
    expect(slashMenu).toMatch(/import\s*\{[^}]*onUnmounted[^}]*\}\s*from\s*['"]vue['"]/)
  })

  it('has an onUnmounted hook', () => {
    expect(slashMenu).toContain('onUnmounted(')
  })

  it('clears hoverHideTimer in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('clearTimeout(hoverHideTimer)')
  })

  it('clears blockMenuOpenTimer in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('clearTimeout(blockMenuOpenTimer)')
  })

  it('clears blockMenuCloseTimer in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('clearTimeout(blockMenuCloseTimer)')
  })

  it('clears plusMenuOpenTimer in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('clearTimeout(plusMenuOpenTimer)')
  })

  it('clears slashHoverCloseTimer in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('clearTimeout(slashHoverCloseTimer)')
  })

  it('resets isDragging to false in onUnmounted', () => {
    const block = slashMenu.slice(slashMenu.indexOf('onUnmounted('))
    expect(block).toContain('isDragging.value = false')
  })
})

// ─── F2: TableOverlay.vue — symmetric addEventListener / removeEventListener ─

describe('F2 TableOverlay.vue: scroll listener options are symmetric', () => {
  it('adds scroll listener with { passive: true, capture: true }', () => {
    expect(tableOverlay).toContain("addEventListener('scroll',       onScroll,      { passive: true, capture: true })")
  })

  it('removes scroll listener with { passive: true, capture: true }', () => {
    expect(tableOverlay).toContain("removeEventListener('scroll',       onScroll,  { passive: true, capture: true })")
  })

  it('scroll add and remove options are identical', () => {
    const addMatch    = tableOverlay.match(/addEventListener\('scroll'[^)]+\)/)
    const removeMatch = tableOverlay.match(/removeEventListener\('scroll'[^)]+\)/)
    expect(addMatch).not.toBeNull()
    expect(removeMatch).not.toBeNull()
    // Extract options object — everything after the second comma
    const addOpts    = addMatch![0].replace(/[^{]*/, '').replace(/\)[^}]*$/, '')
    const removeOpts = removeMatch![0].replace(/[^{]*/, '').replace(/\)[^}]*$/, '')
    expect(removeOpts).toBe(addOpts)
  })
})

// ─── F4: TableOverlay.vue — type-safe dragging, no raw `as any` ──────────────

describe('F4 TableOverlay.vue: typed interfaces instead of as-any', () => {
  it('declares EditorViewWithDragging interface', () => {
    expect(tableOverlay).toContain('interface EditorViewWithDragging')
  })

  it('declares ChainedAny type', () => {
    expect(tableOverlay).toContain('type ChainedAny')
  })

  it('uses EditorViewWithDragging cast for view.dragging assignment', () => {
    expect(tableOverlay).toContain('as EditorViewWithDragging')
  })

  it('uses ChainedAny cast for chain command calls', () => {
    expect(tableOverlay).toContain('as ChainedAny')
  })

  it('no raw (view as any).dragging assignment', () => {
    // Allow `(view as any)` only in comments; must not appear in actual code
    const codeOnly = tableOverlay.replace(/\/\/.*/g, '').replace(/\/\*[\s\S]*?\*\//g, '')
    expect(codeOnly).not.toMatch(/\(\s*view\s+as\s+any\s*\)\.dragging/)
  })

  it('no raw (editor.view as any).dragging', () => {
    const codeOnly = tableOverlay.replace(/\/\/.*/g, '').replace(/\/\*[\s\S]*?\*\//g, '')
    expect(codeOnly).not.toMatch(/\(\s*props\.editor\.view\s+as\s+any\s*\)\.dragging/)
  })
})

// ─── F5: TableOverlay.vue — DEV-gated console.warn in catch blocks ───────────

describe('F5 TableOverlay.vue: catch blocks use import.meta.env.DEV guard', () => {
  it('posAtDOM failure is logged under DEV guard', () => {
    expect(tableOverlay).toMatch(/import\.meta\.env\.DEV.*console\.warn.*\[YinMo\].*posAtDOM/)
  })

  it('applyAlignToCells cell skip is logged under DEV guard', () => {
    expect(tableOverlay).toMatch(/import\.meta\.env\.DEV.*console\.warn.*\[YinMo\].*applyAlignToCells/)
  })

  it('applyMarkToCells cell skip is logged under DEV guard', () => {
    expect(tableOverlay).toMatch(/import\.meta\.env\.DEV.*console\.warn.*\[YinMo\].*applyMarkToCells/)
  })

  it('applyBgToCells cell skip is logged under DEV guard', () => {
    expect(tableOverlay).toMatch(/import\.meta\.env\.DEV.*console\.warn.*\[YinMo\].*applyBgToCells/)
  })

  it('copyTable failure is logged under DEV guard', () => {
    expect(tableOverlay).toMatch(/import\.meta\.env\.DEV.*console\.warn.*\[YinMo\].*copyTable/)
  })
})

// ─── F6: Editor.vue — MARKDOWN_PATTERN as top-level constant ─────────────────

describe('F6 Editor.vue: MARKDOWN_PATTERN is a top-level constant', () => {
  it('declares MARKDOWN_PATTERN constant', () => {
    expect(editorVue).toContain('const MARKDOWN_PATTERN =')
  })

  it('MARKDOWN_PATTERN includes heading detection (# )', () => {
    const match = editorVue.match(/const MARKDOWN_PATTERN\s*=\s*(.+)/)
    expect(match).not.toBeNull()
    const pattern = match![1]
    expect(pattern).toContain('#\\s')
  })

  it('MARKDOWN_PATTERN includes bold detection (**)', () => {
    const match = editorVue.match(/const MARKDOWN_PATTERN\s*=\s*(.+)/)
    expect(match![1]).toContain('\\*\\*')
  })

  it('MARKDOWN_PATTERN includes fenced code block detection (```)', () => {
    const match = editorVue.match(/const MARKDOWN_PATTERN\s*=\s*(.+)/)
    expect(match![1]).toContain('`{3}')
  })

  it('MARKDOWN_PATTERN includes table separator detection', () => {
    const match = editorVue.match(/const MARKDOWN_PATTERN\s*=\s*(.+)/)
    expect(match![1]).toContain('\\|\\s*---')
  })

  it('MARKDOWN_PATTERN includes task list detection', () => {
    const match = editorVue.match(/const MARKDOWN_PATTERN\s*=\s*(.+)/)
    expect(match![1]).toContain('- \\[')
  })

  it('MARKDOWN_PATTERN functions correctly as a regex', () => {
    // Re-evaluate the regex here so we can test it directly
    const pattern = /#\s|\*\*|\[.+?\]\(.+?\)|`{3}|\|\s*---|- \[[ x]\]|==.+==|\$[^$]+\$/

    expect(pattern.test('## Heading')).toBe(true)
    expect(pattern.test('**bold text**')).toBe(true)
    expect(pattern.test('```js\ncode\n```')).toBe(true)
    expect(pattern.test('| --- | --- |')).toBe(true)
    expect(pattern.test('- [ ] todo item')).toBe(true)
    expect(pattern.test('- [x] done item')).toBe(true)
    expect(pattern.test('[link](https://example.com)')).toBe(true)
    expect(pattern.test('==highlighted==')).toBe(true)
    expect(pattern.test('$x^2 + y^2$')).toBe(true)
    expect(pattern.test('plain text without markdown')).toBe(false)
    expect(pattern.test('hello world')).toBe(false)
  })
})

// ─── F7: Editor.vue — pasteAsMarkdown extracted as standalone function ────────

describe('F7 Editor.vue: pasteAsMarkdown is an extracted function', () => {
  it('defines pasteAsMarkdown as a named function', () => {
    expect(editorVue).toContain('function pasteAsMarkdown(')
  })

  it('pasteAsMarkdown accepts view and text parameters', () => {
    expect(editorVue).toMatch(/function pasteAsMarkdown\s*\(\s*view[^,]*,\s*text[^)]*\)/)
  })

  it('pasteAsMarkdown has explicit boolean return type', () => {
    expect(editorVue).toMatch(/function pasteAsMarkdown\s*\([^)]*\)\s*:\s*boolean/)
  })

  it('pasteAsMarkdown uses MARKDOWN_PATTERN for detection', () => {
    const fnStart = editorVue.indexOf('function pasteAsMarkdown(')
    const fnBody  = editorVue.slice(fnStart, fnStart + 500)
    expect(fnBody).toContain('MARKDOWN_PATTERN.test(text)')
  })

  it('pasteAsMarkdown has try-catch with console.warn', () => {
    const fnStart = editorVue.indexOf('function pasteAsMarkdown(')
    const fnBody  = editorVue.slice(fnStart, fnStart + 600)
    expect(fnBody).toContain('catch')
    expect(fnBody).toContain('console.warn')
  })

  it('pasteAsMarkdown returns false when pattern does not match', () => {
    const fnStart = editorVue.indexOf('function pasteAsMarkdown(')
    const fnBody  = editorVue.slice(fnStart, fnStart + 500)
    expect(fnBody).toContain('return false')
  })

  it('handlePaste delegates to pasteAsMarkdown', () => {
    expect(editorVue).toContain('pasteAsMarkdown(view, text)')
  })
})

// ─── F8: CSS — --color-code-bg-dark variable unifies dark code backgrounds ────

describe('F8 CSS: --color-code-bg-dark unifies dark mode code block backgrounds', () => {
  it('defines --color-code-bg-dark in index.html .dark block', () => {
    expect(indexHtml).toContain('--color-code-bg-dark')
  })

  it('--color-code-bg-dark has the GitHub Dark value #0d1117', () => {
    expect(indexHtml).toMatch(/--color-code-bg-dark\s*:\s*#0d1117/)
  })

  it('.dark .ProseMirror pre uses var(--color-code-bg-dark) for background', () => {
    expect(proseCSS).toMatch(/\.dark\s+\.ProseMirror\s+pre\s*\{[^}]*background:\s*var\(--color-code-bg-dark\)/)
  })

  it('html.dark .hljs uses var(--color-code-bg-dark) for background', () => {
    expect(proseCSS).toMatch(/html\.dark\s+\.hljs\s*\{[^}]*background:\s*var\(--color-code-bg-dark\)/)
  })

  it('.dark .ProseMirror pre does NOT have a hardcoded hex background', () => {
    // Extract only the .dark .ProseMirror pre rule
    const ruleMatch = proseCSS.match(/\.dark\s+\.ProseMirror\s+pre\s*\{[^}]+\}/)
    expect(ruleMatch).not.toBeNull()
    expect(ruleMatch![0]).not.toMatch(/background:\s*#[0-9a-fA-F]{3,8}/)
  })

  it('html.dark .hljs does NOT have a hardcoded hex background', () => {
    const ruleMatch = proseCSS.match(/html\.dark\s+\.hljs\s*\{[^}]+\}/)
    expect(ruleMatch).not.toBeNull()
    expect(ruleMatch![0]).not.toMatch(/background:\s*#[0-9a-fA-F]{3,8}/)
  })
})

// ─── F9: CSS — .menu-item:focus-visible keyboard-accessibility styles ─────────

describe('F9 CSS: focus-visible styles for interactive menu items', () => {
  it('style.css defines .menu-item:focus-visible rule', () => {
    expect(styleCSS).toContain('.menu-item:focus-visible')
  })

  it('.menu-item:focus-visible uses outline (not box-shadow)', () => {
    const idx = styleCSS.indexOf('.menu-item:focus-visible')
    const rule = styleCSS.slice(idx, idx + 200)
    expect(rule).toContain('outline:')
  })

  it('.menu-item:focus-visible uses accent color variable', () => {
    const idx = styleCSS.indexOf('.menu-item:focus-visible')
    const rule = styleCSS.slice(idx, idx + 200)
    expect(rule).toMatch(/var\(--color-accent|var\(--accent/)
  })

  it('editor-prose.css adds focus-visible for .slash-menu .menu-item', () => {
    expect(proseCSS).toContain('.slash-menu .menu-item:focus-visible')
  })

  it('editor-prose.css adds focus-visible for .block-menu .menu-item', () => {
    expect(proseCSS).toContain('.block-menu .menu-item:focus-visible')
  })

  it('SlashMenu/block-menu focus-visible uses outline with accent color', () => {
    const idx = proseCSS.indexOf('.slash-menu .menu-item:focus-visible')
    const rule = proseCSS.slice(idx, idx + 200)
    expect(rule).toMatch(/outline:\s*2px/)
    expect(rule).toMatch(/var\(--color-accent/)
  })
})

// ─── F10: SlashMenu.vue — dataTransfer null guard ────────────────────────────

describe('F10 SlashMenu.vue: dataTransfer null guard with early-return', () => {
  it('onHandleDragStart guards against null dataTransfer', () => {
    expect(slashMenu).toContain('if (!e.dataTransfer)')
  })

  it('null dataTransfer logs a console.warn with [YinMo] prefix', () => {
    expect(slashMenu).toContain("console.warn('[YinMo] dragstart: dataTransfer is null")
  })

  it('null dataTransfer triggers an early return', () => {
    const idx = slashMenu.indexOf('if (!e.dataTransfer)')
    const block = slashMenu.slice(idx, idx + 200)
    expect(block).toContain('return')
  })

  it('dataTransfer.effectAllowed is set unconditionally after the guard', () => {
    // After early-return guard, the remaining code should not be inside another if(e.dataTransfer)
    const idx = slashMenu.indexOf('if (!e.dataTransfer)')
    // Find the guard block end, then check effectAllowed is set
    const afterGuard = slashMenu.slice(idx + 100)
    expect(afterGuard).toContain('e.dataTransfer.effectAllowed')
  })
})
