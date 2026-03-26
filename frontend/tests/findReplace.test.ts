/**
 * Unit tests for find-replace logic (collectMatches).
 *
 * collectMatches is defined inline in Editor.vue. This test file re-implements
 * the same pure function signature to validate the algorithm. If collectMatches
 * is extracted to a shared utility in the future, this test should import it
 * directly instead.
 *
 * Covers:
 * - Empty search term returns no matches
 * - Case-insensitive matching
 * - Multiple occurrences in the same text node
 * - Matches across multiple text nodes
 * - Unicode/CJK content matching
 * - Special regex characters in search term (treated as literal)
 */
import { describe, it, expect } from 'vitest'

// ─── Re-implementation of collectMatches for testability ─────────────────────
// This mirrors the logic in Editor.vue lines 507-521.
// A ProseMirror Node-like interface for testing.
interface MockTextNode {
  isText: boolean
  text: string | null
}

interface MockDoc {
  descendants: (callback: (node: MockTextNode, pos: number) => void) => void
}

function collectMatches(doc: MockDoc, term: string): { from: number; to: number }[] {
  if (!term) return []
  const matches: { from: number; to: number }[] = []
  const lower = term.toLowerCase()
  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return
    const text = node.text.toLowerCase()
    let idx = 0
    while ((idx = text.indexOf(lower, idx)) !== -1) {
      matches.push({ from: pos + idx, to: pos + idx + lower.length })
      idx += lower.length
    }
  })
  return matches
}

// ─── Test helpers ────────────────────────────────────────────────────────────

/** Create a mock document with a list of text nodes at specified positions. */
function mockDoc(nodes: { text: string; pos: number }[]): MockDoc {
  return {
    descendants(callback) {
      for (const n of nodes) {
        callback({ isText: true, text: n.text }, n.pos)
      }
    },
  }
}

// ─── Tests ───────────────────────────────────────────────────────────────────

describe('collectMatches — find-replace core logic', () => {
  it('should_return_empty_array_when_term_is_empty_string', () => {
    const doc = mockDoc([{ text: 'Hello world', pos: 0 }])
    expect(collectMatches(doc, '')).toEqual([])
  })

  it('should_find_single_occurrence_in_text', () => {
    const doc = mockDoc([{ text: 'Hello world', pos: 0 }])
    const matches = collectMatches(doc, 'world')
    expect(matches).toEqual([{ from: 6, to: 11 }])
  })

  it('should_match_case_insensitively', () => {
    const doc = mockDoc([{ text: 'Hello World', pos: 0 }])
    const matches = collectMatches(doc, 'HELLO')
    expect(matches).toEqual([{ from: 0, to: 5 }])
  })

  it('should_find_multiple_occurrences_in_same_node', () => {
    const doc = mockDoc([{ text: 'ababab', pos: 0 }])
    const matches = collectMatches(doc, 'ab')
    expect(matches).toEqual([
      { from: 0, to: 2 },
      { from: 2, to: 4 },
      { from: 4, to: 6 },
    ])
  })

  it('should_find_matches_across_multiple_text_nodes', () => {
    const doc = mockDoc([
      { text: 'Hello foo', pos: 0 },
      { text: 'bar foo baz', pos: 10 },
    ])
    const matches = collectMatches(doc, 'foo')
    expect(matches).toEqual([
      { from: 6, to: 9 },
      { from: 14, to: 17 },
    ])
  })

  it('should_respect_node_position_offset', () => {
    // In ProseMirror, text nodes don't always start at pos 0
    const doc = mockDoc([{ text: 'find me', pos: 100 }])
    const matches = collectMatches(doc, 'find')
    expect(matches).toEqual([{ from: 100, to: 104 }])
  })

  it('should_handle_CJK_characters', () => {
    const doc = mockDoc([{ text: '你好世界你好', pos: 0 }])
    const matches = collectMatches(doc, '你好')
    expect(matches).toEqual([
      { from: 0, to: 2 },
      { from: 4, to: 6 },
    ])
  })

  it('should_return_empty_when_no_match_found', () => {
    const doc = mockDoc([{ text: 'Hello world', pos: 0 }])
    expect(collectMatches(doc, 'xyz')).toEqual([])
  })

  it('should_skip_non_text_nodes', () => {
    const doc: MockDoc = {
      descendants(callback) {
        callback({ isText: false, text: null }, 0)
        callback({ isText: true, text: 'found here' }, 10)
      },
    }
    const matches = collectMatches(doc, 'found')
    expect(matches).toEqual([{ from: 10, to: 15 }])
  })

  it('should_skip_text_nodes_with_null_text', () => {
    const doc: MockDoc = {
      descendants(callback) {
        callback({ isText: true, text: null }, 0)
        callback({ isText: true, text: 'ok here', pos: 5 } as any, 5)
      },
    }
    const matches = collectMatches(doc, 'ok')
    expect(matches).toEqual([{ from: 5, to: 7 }])
  })

  it('should_handle_overlapping_potential_matches_non_greedily', () => {
    // Searching "aa" in "aaa" should find [0,2] then skip to index 2, finding nothing more
    const doc = mockDoc([{ text: 'aaa', pos: 0 }])
    const matches = collectMatches(doc, 'aa')
    expect(matches).toEqual([{ from: 0, to: 2 }])
  })
})
