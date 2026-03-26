/**
 * Unit tests for batch import pure functions.
 *
 * Covers:
 * - titleFromPath: filename to title conversion
 * - dirParts: directory hierarchy extraction
 * - stripCommonPrefix: ZIP root folder stripping
 * - resolveRelativePath: relative path resolution from md file location
 * - rewriteImageRefs: markdown image reference rewriting
 */
import { describe, it, expect } from 'vitest'
import {
  titleFromPath,
  dirParts,
  stripCommonPrefix,
  resolveRelativePath,
  rewriteImageRefs,
  type ParsedEntry,
} from '../src/composables/useBatchImport'

// ── titleFromPath ───────────────────────────────────────────────────────────

describe('titleFromPath', () => {
  it('strips .md extension', () => {
    expect(titleFromPath('Chapter 1.md')).toBe('Chapter 1')
  })

  it('strips .txt extension', () => {
    expect(titleFromPath('notes.txt')).toBe('notes')
  })

  it('handles nested paths, uses only filename', () => {
    expect(titleFromPath('docs/sub/README.md')).toBe('README')
  })

  it('returns full name if no extension', () => {
    expect(titleFromPath('Makefile')).toBe('Makefile')
  })

  it('handles dotfiles correctly', () => {
    expect(titleFromPath('.gitignore')).toBe('.gitignore')
  })

  it('handles multiple dots', () => {
    expect(titleFromPath('my.notes.2024.md')).toBe('my.notes.2024')
  })
})

// ── dirParts ────────────────────────────────────────────────────────────────

describe('dirParts', () => {
  it('returns empty array for flat file', () => {
    expect(dirParts('README.md')).toEqual([])
  })

  it('returns single directory', () => {
    expect(dirParts('docs/README.md')).toEqual(['docs'])
  })

  it('returns nested directories', () => {
    expect(dirParts('project/src/utils/helper.md')).toEqual(['project', 'src', 'utils'])
  })

  it('filters empty segments', () => {
    expect(dirParts('a//b/file.md')).toEqual(['a', 'b'])
  })
})

// ── stripCommonPrefix ───────────────────────────────────────────────────────

describe('stripCommonPrefix', () => {
  it('strips single root folder from all entries', () => {
    const entries: ParsedEntry[] = [
      { relativePath: 'root/a.md', content: 'a', isAsset: false },
      { relativePath: 'root/b.md', content: 'b', isAsset: false },
      { relativePath: 'root/sub/c.md', content: 'c', isAsset: false },
    ]
    const result = stripCommonPrefix(entries)
    expect(result.map(e => e.relativePath)).toEqual(['a.md', 'b.md', 'sub/c.md'])
  })

  it('strips multi-level common prefix', () => {
    const entries: ParsedEntry[] = [
      { relativePath: 'a/b/c/file1.md', content: '1', isAsset: false },
      { relativePath: 'a/b/c/file2.md', content: '2', isAsset: false },
    ]
    const result = stripCommonPrefix(entries)
    expect(result.map(e => e.relativePath)).toEqual(['file1.md', 'file2.md'])
  })

  it('does not strip when no common prefix', () => {
    const entries: ParsedEntry[] = [
      { relativePath: 'a/file1.md', content: '1', isAsset: false },
      { relativePath: 'b/file2.md', content: '2', isAsset: false },
    ]
    const result = stripCommonPrefix(entries)
    expect(result.map(e => e.relativePath)).toEqual(['a/file1.md', 'b/file2.md'])
  })

  it('does not strip if it would eliminate the filename', () => {
    // All files are in the same directory — the prefix IS the only directory
    const entries: ParsedEntry[] = [
      { relativePath: 'notes/a.md', content: 'a', isAsset: false },
      { relativePath: 'notes/b.md', content: 'b', isAsset: false },
    ]
    const result = stripCommonPrefix(entries)
    expect(result.map(e => e.relativePath)).toEqual(['a.md', 'b.md'])
  })

  it('returns empty array for empty input', () => {
    expect(stripCommonPrefix([])).toEqual([])
  })

  it('handles single entry', () => {
    const entries: ParsedEntry[] = [
      { relativePath: 'root/file.md', content: 'x', isAsset: false },
    ]
    const result = stripCommonPrefix(entries)
    expect(result.map(e => e.relativePath)).toEqual(['file.md'])
  })

  it('preserves content and isAsset flag', () => {
    const entries: ParsedEntry[] = [
      { relativePath: 'root/note.md', content: 'hello', isAsset: false },
      { relativePath: 'root/img.png', content: new Uint8Array([1, 2, 3]), isAsset: true },
    ]
    const result = stripCommonPrefix(entries)
    expect(result[0].content).toBe('hello')
    expect(result[0].isAsset).toBe(false)
    expect(result[1].isAsset).toBe(true)
  })
})

// ── resolveRelativePath ─────────────────────────────────────────────────────

describe('resolveRelativePath', () => {
  it('resolves sibling reference', () => {
    expect(resolveRelativePath('docs/note.md', 'image.png')).toBe('docs/image.png')
  })

  it('resolves parent directory reference', () => {
    expect(resolveRelativePath('docs/chapter1/note.md', '../assets/logo.png')).toBe('docs/assets/logo.png')
  })

  it('resolves ./ prefix', () => {
    expect(resolveRelativePath('docs/note.md', './images/pic.png')).toBe('docs/images/pic.png')
  })

  it('resolves deeply nested ../', () => {
    expect(resolveRelativePath('a/b/c/note.md', '../../img.png')).toBe('a/img.png')
  })

  it('resolves root-level md file', () => {
    expect(resolveRelativePath('note.md', 'assets/img.png')).toBe('assets/img.png')
  })

  it('handles multiple ../ going to root', () => {
    expect(resolveRelativePath('a/b/note.md', '../../img.png')).toBe('img.png')
  })
})

// ── rewriteImageRefs ────────────────────────────────────────────────────────

describe('rewriteImageRefs', () => {
  const assetMap = new Map([
    ['assets/logo.png', './uploads/abc123.png'],
    ['images/photo.jpg', './uploads/def456.jpg'],
    ['logo.png', './uploads/abc123.png'],
  ])

  it('rewrites markdown image syntax ![alt](path)', () => {
    const md = '# Title\n\n![logo](assets/logo.png)\n\nSome text.'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toContain('![logo](./uploads/abc123.png)')
    expect(result).not.toContain('assets/logo.png')
  })

  it('rewrites HTML img tag', () => {
    const md = '<img src="assets/logo.png" alt="logo">'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toContain('src="./uploads/abc123.png"')
  })

  it('does not rewrite external URLs', () => {
    const md = '![ext](https://example.com/img.png)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toBe(md)
  })

  it('does not rewrite data URIs', () => {
    const md = '![data](data:image/png;base64,abc)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toBe(md)
  })

  it('resolves relative paths from md file location', () => {
    // md is at docs/chapter1/note.md, references ../assets/logo.png
    // which resolves to docs/assets/logo.png — need that in the map
    const map = new Map([['docs/assets/logo.png', './uploads/xyz.png']])
    const md = '![](../assets/logo.png)'
    const result = rewriteImageRefs(md, 'docs/chapter1/note.md', map)
    expect(result).toBe('![](./uploads/xyz.png)')
  })

  it('falls back to filename-only matching', () => {
    const md = '![](some/unknown/path/logo.png)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toBe('![](./uploads/abc123.png)')
  })

  it('leaves unmatched references unchanged', () => {
    const md = '![](missing.svg)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toBe('![](missing.svg)')
  })

  it('handles multiple images in one document', () => {
    const md = '![a](assets/logo.png) text ![b](images/photo.jpg)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    expect(result).toContain('![a](./uploads/abc123.png)')
    expect(result).toContain('![b](./uploads/def456.jpg)')
  })

  it('does not corrupt non-image text', () => {
    const md = '# Title\n\nSome text with assets/logo.png mentioned.\n\n![](assets/logo.png)'
    const result = rewriteImageRefs(md, 'note.md', assetMap)
    // The plain text mention should NOT be replaced
    expect(result).toContain('Some text with assets/logo.png mentioned.')
    // Only the image ref should be replaced
    expect(result).toContain('![](./uploads/abc123.png)')
  })
})
