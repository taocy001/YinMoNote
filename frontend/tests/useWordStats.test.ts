/**
 * TE-003: Unit tests for useWordStats composable.
 *
 * Covers:
 * - Pure Latin word counting
 * - CJK character counting (each CJK char = 1 word)
 * - Mixed CJK + Latin text
 * - Korean Hangul characters (added in CJK regex fix)
 * - CJK Extension A and Compatibility Ideographs
 * - Empty text
 * - Hiragana and Katakana
 * - readMin estimation: CJK-dominant vs Latin-dominant
 * - Character count (total string length)
 */
import { describe, it, expect } from 'vitest'
import { useWordStats } from '../src/composables/useWordStats'

/** Creates a minimal mock TiptapEditor whose textContent is the provided string. */
function mockEditor(text: string) {
  return {
    state: {
      doc: { textContent: text },
    },
  } as any
}

describe('useWordStats', () => {
  it('counts Latin words', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('Hello world foo'))
    expect(wordStats.value.words).toBe(3)
  })

  it('counts alphanumeric sequences as single words', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('foo123 bar baz42'))
    expect(wordStats.value.words).toBe(3)
  })

  it('ignores punctuation in word count', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('Hello, world! How are you?'))
    expect(wordStats.value.words).toBe(5)
  })

  it('counts CJK unified ideographs (each as one word)', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('你好世界'))
    expect(wordStats.value.words).toBe(4)
  })

  it('counts Hiragana characters', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('はろー')) // 3 hiragana
    expect(wordStats.value.words).toBe(3)
  })

  it('counts Katakana characters', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('ハロー')) // 3 katakana
    expect(wordStats.value.words).toBe(3)
  })

  it('counts Korean Hangul syllables', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('안녕하세요')) // 5 Hangul syllables (U+AC00–U+D7AF range)
    expect(wordStats.value.words).toBe(5)
  })

  it('counts CJK Extension A characters (U+3400–U+4DBF)', () => {
    const { wordStats, updateWordStats } = useWordStats()
    // U+3400 is the first character of CJK Extension A
    const extA = '\u3400\u3401\u3402'
    updateWordStats(mockEditor(extA))
    expect(wordStats.value.words).toBe(3)
  })

  it('counts CJK Compatibility Ideographs (U+F900–U+FAFF)', () => {
    const { wordStats, updateWordStats } = useWordStats()
    const compat = '\uF900\uF901'
    updateWordStats(mockEditor(compat))
    expect(wordStats.value.words).toBe(2)
  })

  it('handles mixed CJK and Latin text', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('Hello 世界 world'))
    // 1 (Hello) + 2 (世界) + 1 (world) = 4
    expect(wordStats.value.words).toBe(4)
  })

  it('counts chars as total string length', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('abc def'))
    expect(wordStats.value.chars).toBe(7)
  })

  it('returns 0 words and 0 chars for empty text', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor(''))
    expect(wordStats.value.words).toBe(0)
    expect(wordStats.value.chars).toBe(0)
  })

  it('readMin is at least 1', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('hi'))
    expect(wordStats.value.readMin).toBeGreaterThanOrEqual(1)
  })

  it('readMin uses CJK rate (300/min) when CJK dominates', () => {
    // 300 CJK chars → 1 minute at 300 chars/min
    const text = '字'.repeat(300)
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor(text))
    expect(wordStats.value.readMin).toBe(1)
  })

  it('readMin uses Latin rate (200/min) when Latin dominates', () => {
    // 200 single-char "words" → 1 minute at 200 words/min
    const words = Array.from({ length: 200 }, (_, i) => 'w' + i).join(' ')
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor(words))
    expect(wordStats.value.readMin).toBe(1)
  })

  it('wordStats ref updates reactively on each call', () => {
    const { wordStats, updateWordStats } = useWordStats()
    updateWordStats(mockEditor('one two'))
    expect(wordStats.value.words).toBe(2)
    updateWordStats(mockEditor('one two three four'))
    expect(wordStats.value.words).toBe(4)
  })
})
