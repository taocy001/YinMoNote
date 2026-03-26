import { ref, watch, nextTick, type Ref, type ShallowRef } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import type { PluginKey } from '@tiptap/pm/state'
import type { Node as ProsemirrorNode } from '@tiptap/pm/model'

/** Position range of a single match within the document. */
interface MatchPosition {
  from: number
  to: number
}

/**
 * Scans the document for all case-insensitive occurrences of `term`.
 * Returns an array of `{ from, to }` positions suitable for ProseMirror
 * decorations and text selection.
 */
export function collectMatches(doc: ProsemirrorNode, term: string): MatchPosition[] {
  if (!term) return []
  const matches: MatchPosition[] = []
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

/**
 * Composable that encapsulates find-and-replace state and logic.
 *
 * Depends on a TipTap editor instance and a ProseMirror PluginKey used by the
 * SearchHighlight extension to render match decorations.
 */
export function useFindReplace(deps: {
  editor: Ref<TiptapEditor | undefined> | ShallowRef<TiptapEditor | undefined>
  searchHighlightKey: PluginKey
}) {
  const { editor, searchHighlightKey } = deps

  const showFindBar = ref(false)
  const findQuery = ref('')
  const replaceQuery = ref('')
  const isReplaceMode = ref(false)
  const findMatchCount = ref(0)
  const findCurrentIndex = ref(0)
  const findMatchPositions = ref<MatchPosition[]>([])
  const findInputRef = ref<HTMLInputElement | null>(null)

  // ── Decoration synchronisation ──────────────────────────────────────────

  function applyFindDecorations(): void {
    if (!editor.value) return
    const doc = editor.value.state.doc
    const matches = collectMatches(doc, findQuery.value)
    findMatchPositions.value = matches
    findMatchCount.value = matches.length
    if (findCurrentIndex.value >= matches.length && matches.length > 0) {
      findCurrentIndex.value = 0
    }
    const term = findQuery.value
    if (!term) {
      editor.value.view.dispatch(editor.value.state.tr.setMeta(searchHighlightKey, ''))
      return
    }
    // Encode find-replace state into the plugin meta so the SearchHighlight
    // extension can render both "all matches" and "current match" decorations.
    editor.value.view.dispatch(
      editor.value.state.tr.setMeta(searchHighlightKey, `__findreplace:${findCurrentIndex.value}:${term}`),
    )
  }

  watch(findQuery, () => {
    findCurrentIndex.value = 0
    if (showFindBar.value) applyFindDecorations()
  })

  watch(findCurrentIndex, () => {
    if (showFindBar.value) applyFindDecorations()
  })

  // ── Navigation ──────────────────────────────────────────────────────────

  function scrollToCurrentMatch(): void {
    if (!editor.value || findMatchPositions.value.length === 0) return
    const pos = findMatchPositions.value[findCurrentIndex.value]
    if (pos) {
      editor.value.commands.setTextSelection(pos.from)
      const dom = editor.value.view.domAtPos(pos.from)
      if (dom?.node) {
        const el = dom.node instanceof HTMLElement ? dom.node : dom.node.parentElement
        el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
      }
    }
  }

  function findNext(): void {
    if (findMatchCount.value === 0) return
    findCurrentIndex.value = (findCurrentIndex.value + 1) % findMatchCount.value
    scrollToCurrentMatch()
  }

  function findPrev(): void {
    if (findMatchCount.value === 0) return
    findCurrentIndex.value = (findCurrentIndex.value - 1 + findMatchCount.value) % findMatchCount.value
    scrollToCurrentMatch()
  }

  // ── Replace ─────────────────────────────────────────────────────────────

  /** Replace the current match with `replaceQuery` and re-scan. */
  function replaceOne(): void {
    if (!editor.value || findMatchPositions.value.length === 0) return
    const pos = findMatchPositions.value[findCurrentIndex.value]
    if (!pos) return
    // SEC-106: use schema.text() to ensure replacement is plain text
    const { tr } = editor.value.state
    tr.replaceWith(pos.from, pos.to, editor.value.state.schema.text(replaceQuery.value))
    editor.value.view.dispatch(tr)
    editor.value.commands.focus()
    nextTick(() => applyFindDecorations())
  }

  /** Replace all matches in a single transaction using mapping. */
  function replaceAllMatches(): void {
    if (!editor.value || findMatchPositions.value.length === 0) return
    // Mapping adjusts positions after each replacement within the same transaction
    // because replacements change document length, shifting subsequent positions.
    const matches = [...findMatchPositions.value]
    const { tr } = editor.value.state
    const replText = editor.value.state.schema.text(replaceQuery.value)
    for (const m of matches) {
      const from = tr.mapping.map(m.from)
      const to = tr.mapping.map(m.to)
      tr.replaceWith(from, to, replText)
    }
    editor.value.view.dispatch(tr)
    nextTick(() => applyFindDecorations())
  }

  // ── Bar visibility ──────────────────────────────────────────────────────

  /** Open the find bar (optionally in replace mode). */
  function openFindBar(replace = false): void {
    showFindBar.value = true
    isReplaceMode.value = replace
    nextTick(() => findInputRef.value?.focus())
    applyFindDecorations()
  }

  /** Close the find bar and clear all state + decorations. */
  function closeFindBar(): void {
    showFindBar.value = false
    findQuery.value = ''
    replaceQuery.value = ''
    findMatchPositions.value = []
    findMatchCount.value = 0
    findCurrentIndex.value = 0
    if (editor.value) {
      editor.value.view.dispatch(editor.value.state.tr.setMeta(searchHighlightKey, ''))
    }
  }

  return {
    showFindBar,
    findQuery,
    replaceQuery,
    isReplaceMode,
    findMatchCount,
    findCurrentIndex,
    findMatchPositions,
    findInputRef,
    findNext,
    findPrev,
    replaceOne,
    replaceAllMatches,
    openFindBar,
    closeFindBar,
  }
}
