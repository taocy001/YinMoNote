import { ref } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'

/**
 * Word and character statistics composable.
 */
export function useWordStats() {
  const wordStats = ref({ words: 0, chars: 0, readMin: 0 })

  /**
   * Counts words and characters from the current editor plain text.
   * Updates wordStats reactively.
   */
  const updateWordStats = (ed: TiptapEditor) => {
    const text = ed.state.doc.textContent || ''
    // CJK characters each count as one word.
    // Uses Unicode property escapes (ES2018) to cover ALL CJK ranges including
    // supplementary plane characters (Extension B–I, U+20000+), plus Hiragana,
    // Katakana, and Korean Hangul. The `u` flag enables astral plane support.
    const cjkMatches = text.match(
      /[\u3400-\u4dbf\u4e00-\u9fff\uf900-\ufaff\u3040-\u309f\u30a0-\u30ff\uac00-\ud7af\u{20000}-\u{2a6df}\u{2a700}-\u{2ceaf}\u{2ceb0}-\u{2ebef}\u{2f800}-\u{2fa1f}\u{30000}-\u{323af}]/gu,
    ) || []
    // Latin words: only alphanumeric sequences (ignores punctuation including CJK punctuation)
    const latinWords = text.match(/[a-zA-Z0-9]+/g) || []
    const words = cjkMatches.length + latinWords.length
    const chars = text.length
    // CJK: ~300 chars/min; Latin: ~200 words/min — use the dominant type for estimation
    const readMin = Math.max(1, Math.round(words / (cjkMatches.length > latinWords.length ? 300 : 200)))
    wordStats.value = { words, chars, readMin }
  }

  return { wordStats, updateWordStats }
}
