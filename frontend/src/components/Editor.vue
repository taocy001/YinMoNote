<template>
  <div
    ref="editorRoot"
    data-testid="editor-root"
    :class="['relative flex flex-col h-full', isFocusMode ? 'focus-mode' : '']"
    style="background: var(--bg-editor); color: var(--text-primary);"
    @mouseleave="slashMenuRef?.scheduleHideHoverCtrl()"
  >
    <!-- Desktop header — when TOC is open the left 192px shares the TOC panel background,
         creating a unified visual column instead of a color break at the panel seam. -->
    <div
class="hidden md:flex items-center justify-between px-4 py-2 shrink-0 focus-mode-hide"
      style="background: var(--bg-editor); border-bottom: 1px solid var(--border);">
      <div class="flex items-center gap-2">
        <!-- TOC button — leftmost -->
        <button
data-testid="toc-btn" class="w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" :style="showToc ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-muted);'"
          :title="t.toc"
          @click="toggleToc">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M2 3.5h10M2 7h7M2 10.5h5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        </button>
        <!-- Word count (subtle) -->
        <span v-if="wordStats.words > 0" class="ts-xs tabular-nums font-medium" style="color: var(--text-muted);">
          {{ wordStats.words }}{{ t.wordCount }} · {{ wordStats.readMin }}{{ t.readTime }}
        </span>
        <!-- Save status pill — clickable when dirty to trigger manual save -->
        <div
          data-testid="save-status"
          class="flex items-center gap-1.5 ts-xs font-medium transition-all rounded-md"
          :class="(saveStatus === 'dirty' || saveStatus === 'error') ? 'cursor-pointer px-1.5 py-0.5 active:scale-[0.97]' : ''"
          :style="saveStatus === 'dirty' ? saveStatusStyle + 'background: rgba(217,119,6,0.08);' : saveStatus === 'error' ? saveStatusStyle + 'background: rgba(220,38,38,0.08);' : saveStatusStyle"
          :title="saveStatus === 'error' ? lastSaveError : saveStatus === 'dirty' ? (t.unsaved as string) : undefined"
          @click="(saveStatus === 'dirty' || saveStatus === 'error') ? doSave() : undefined"
        >
          <div class="w-1.5 h-1.5 rounded-full" :class="saveStatus === 'saving' ? 'animate-pulse' : ''" :style="saveDotStyle"></div>
          <span>{{ statusText }}</span>
        </div>
      </div>
      <div class="flex items-center gap-0.5">
        <!-- Export dropdown -->
        <div ref="exportMenuRef" class="relative">
          <button
data-testid="export-btn" class="editor-toolbar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" :style="showExportMenu ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-muted);'"
            :title="t.exportMenu"
            @click="showExportMenu = !showExportMenu">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M7 1v8M4 6l3 3 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/><path d="M2 10v2h10v-2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
          <!-- Dropdown -->
          <div v-if="showExportMenu" class="absolute right-0 mt-1 w-40 rounded-xl overflow-hidden z-50 anim-pop-in" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-md);">
            <button
class="w-full text-left px-4 py-2.5 ts-sm transition-all" style="color: var(--text-secondary);" @click="exportHTML(); showExportMenu = false"
              @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'" @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">{{ t.exportHTML }}</button>
            <button
class="w-full text-left px-4 py-2.5 ts-sm transition-all" style="color: var(--text-secondary); border-top: 1px solid var(--border);" @click="exportPDF(); showExportMenu = false"
              @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'" @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">{{ t.exportPDF }}</button>
            <button
class="w-full text-left px-4 py-2.5 ts-sm transition-all" style="color: var(--text-secondary); border-top: 1px solid var(--border);" @click="exportMarkdown(); showExportMenu = false"
              @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'" @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">{{ t.exportMarkdown }}</button>
          </div>
        </div>
        <!-- History -->
        <button
data-testid="history-btn" class="editor-toolbar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97] relative" :style="showHistory ? 'background: var(--accent-light); color: var(--accent);' : historyLoadError ? 'color: var(--color-danger);' : 'color: var(--text-muted);'"
          :title="historyLoadError ? (t.historyLoadFailed ?? 'Failed to load history') : t.historyBtn"
          @click="toggleHistory">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="1.3"/><path d="M7 4v3.5l2 1.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <!-- Focus mode -->
        <button
data-testid="focus-mode-btn" class="editor-toolbar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" :style="isFocusMode ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-muted);'"
          :title="isFocusMode ? t.exitFocusMode : t.focusMode"
          @click="toggleFocusMode">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M1.5 4V2h2.5M10 2h2.5v2M12.5 10V12H10M3.5 12H1.5v-2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        </button>
        <!-- Keyboard shortcuts -->
        <button
data-testid="shortcuts-btn" class="editor-toolbar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" :style="showShortcuts ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-muted);'"
          :title="t.shortcutHelp"
          @click="showShortcuts = !showShortcuts">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><rect x="1.5" y="3.5" width="11" height="7" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4 6.5h.5M7 6.5h.5M10 6.5h.5M4 9h6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        </button>
        <!-- Settings -->
        <button
data-testid="settings-btn" class="editor-toolbar-btn w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);"
          :title="t.settings"
          @click="emit('open-settings')">
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="2" stroke="currentColor" stroke-width="1.3"/><path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.6 2.6l1.1 1.1M10.3 10.3l1.1 1.1M2.6 11.4l1.1-1.1M10.3 3.7l1.1-1.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        </button>
      </div>
    </div>

    <!-- Floating format menu -->
    <FloatingMenu v-if="editor" :editor="editor" :tippy-options="{ duration: 100 }" class="hidden md:block">
      <div class="flex rounded-xl overflow-hidden p-1 gap-1" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-md);">
        <button
          v-for="(item, i) in formatButtons"
          :key="i"
          class="px-2 py-1 text-sm rounded-lg transition-colors"
          style="color: var(--text-secondary);"
          :class="[item.bold ? 'font-bold' : item.mono ? 'font-mono' : '']"
          @click="item.action()"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >
          {{ item.label }}
        </button>
      </div>
    </FloatingMenu>

    <!-- Main content -->
    <div class="flex-1 flex min-h-0">
      <!-- Table of Contents — background matches the header column above it for visual continuity -->
      <div
        v-if="showToc"
        class="w-48 shrink-0 border-r flex flex-col overflow-hidden"
        style="background: var(--bg-editor); border-color: var(--border);"
      >
        <div class="flex-1 overflow-y-auto pt-2 pb-2 px-2 space-y-1 text-left">
          <div
            v-for="h in tocItems"
            :key="h.pos"
            class="px-2 py-1 rounded-lg cursor-pointer text-sm truncate transition-colors"
            :class="[h.level === 1 ? 'font-bold' : h.level === 2 ? 'pl-4' : 'pl-7 text-xs opacity-80']"
            style="color: var(--text-secondary);"
            @click="jumpTo(h.pos)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          >
            {{ h.text }}
          </div>
          <div v-if="tocItems.length === 0" class="text-xs p-2 text-center" style="color: var(--text-muted);">{{ t.noHeadings }}</div>
        </div>
      </div>

      <!-- Editor viewport / Diff view — share the same flex slot -->
      <div class="flex-1 flex flex-col relative min-w-0 bg-transparent overflow-hidden">
        <!-- Loading / error overlays (only relevant when editor is visible) -->
        <div v-if="isLoading && !diffPayload" class="absolute inset-0 z-10 flex items-center justify-center backdrop-blur-[1px]" style="background: rgba(255,255,255,0.06);">
          <div class="flex flex-col items-center gap-2">
            <div class="w-6 h-6 border-2 border-t-transparent rounded-full animate-spin" style="border-color: var(--border); border-top-color: var(--accent);"></div>
            <span class="text-xs font-medium" style="color: var(--accent);">{{ t.loading }}</span>
          </div>
        </div>
        <div v-if="loadError && !isLoading && !diffPayload" class="absolute top-4 left-1/2 -translate-x-1/2 z-10 px-4 py-2 rounded-xl text-xs font-medium" style="background: var(--color-danger-light); color: var(--color-danger); border: 1px solid #FECACA;">{{ t.loadFailed ?? 'Failed to load note' }}</div>
        <div v-if="uploadError" class="absolute top-4 left-1/2 -translate-x-1/2 z-10 px-4 py-2 rounded-xl text-xs font-medium" style="background: var(--color-danger-light); color: var(--color-danger); border: 1px solid #FECACA;">{{ t.uploadFailed }}</div>

        <!-- Find & Replace bar -->
        <div v-if="showFindBar" class="shrink-0 flex flex-wrap items-center gap-2 px-4 py-2" style="background: var(--bg-sidebar); border-bottom: 1px solid var(--border);">
          <div class="flex items-center gap-1.5 flex-1 min-w-[200px]">
            <input
ref="findInputRef" v-model="findQuery" :placeholder="t.findPlaceholder" class="flex-1 px-2 py-1 ts-sm rounded-md border outline-none" style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit; min-width: 100px;" @keydown.enter.exact.prevent="findNext"
              @keydown.shift.enter.prevent="findPrev" @keydown.esc.prevent="closeFindBar" />
            <span class="ts-xs shrink-0 tabular-nums" style="color: var(--text-muted);">{{ findMatchCount > 0 ? t.matchCount(findCurrentIndex + 1, findMatchCount) : (findQuery ? t.noMatches : '') }}</span>
            <button class="w-6 h-6 flex items-center justify-center rounded transition-colors" style="color: var(--text-muted);" :title="t.findPrev" @click="findPrev"><svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2.5 7.5L6 4.5L9.5 7.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
            <button class="w-6 h-6 flex items-center justify-center rounded transition-colors" style="color: var(--text-muted);" :title="t.findNext" @click="findNext"><svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2.5 4.5L6 7.5L9.5 4.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg></button>
          </div>
          <div v-if="isReplaceMode" class="flex items-center gap-1.5 flex-1 min-w-[200px]">
            <input
v-model="replaceQuery" :placeholder="t.replacePlaceholder" class="flex-1 px-2 py-1 ts-sm rounded-md border outline-none" style="background: var(--bg-editor); border-color: var(--border); color: var(--text-primary); font-family: inherit; min-width: 100px;"
              @keydown.enter.prevent="replaceOne" @keydown.esc.prevent="closeFindBar" />
            <button class="px-2 py-1 ts-xs rounded-md transition-all" style="background: var(--accent-light); color: var(--accent);" @click="replaceOne">{{ t.replaceOne }}</button>
            <button class="px-2 py-1 ts-xs rounded-md transition-all" style="background: var(--accent-light); color: var(--accent);" @click="replaceAllMatches">{{ t.replaceAll }}</button>
          </div>
          <div class="flex items-center gap-1">
            <button v-if="!isReplaceMode" class="ts-xs px-1.5 py-0.5 rounded transition-all" style="color: var(--text-muted);" @click="isReplaceMode = true">{{ t.replaceOne }}…</button>
            <button class="w-6 h-6 flex items-center justify-center rounded transition-colors" style="color: var(--text-muted);" @click="closeFindBar"><svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg></button>
          </div>
        </div>

        <!-- Real editor — hidden (not destroyed) while diff mode is active -->
        <EditorContent
          v-show="!diffPayload"
          ref="editorScrollEl"
          :editor="editor"
          class="flex-1 overflow-y-auto outline-none pb-safe px-4 md:px-8 pt-6"
          :class="[editorWidth === 'full' ? '' : 'max-w-3xl mx-auto w-full']"
          :style="{ fontSize: fontSize + 'px' }"
          @scroll="onEditorScroll"
        />

        <!-- Feishu-style inline diff view -->
        <DiffView
          v-if="diffPayload"
          :old-content="diffPayload.oldContent"
          :new-content="diffPayload.newContent"
          @exit="exitDiff"
        />
      </div>

      <!-- History Panel -->
      <HistoryPanel
        :note-file-name="props.noteFileName"
        :show="showHistory"
        :save-if-dirty="saveIfDirty"
        :get-full-content="getFullContent"
        :commit-labels="props.commitLabels || {}"
        @close="showHistory = false"
        @apply-rollback="onApplyRollback"
        @load-error="historyLoadError = $event"
        @show-diff="onShowDiff"
        @set-label="(h, l) => emit('set-label', h, l)"
      />
    </div>

  <MobileToolbar :editor="editor" :t="t" />

    <BubbleToolbar ref="bubbleRef" :editor="editor" :t="t" />
    <TableOverlay :editor="editor" :t="t" />
    <ShortcutsModal :visible="showShortcuts" :shortcuts="shortcutList" :t="t" @close="showShortcuts = false" />
    <SlashMenu ref="slashMenuRef" :editor="editor" :t="t" />
  </div>
</template>

<script setup lang="ts">
/**
 * Editor.vue - High-performance Tiptap editor with incremental content rendering.
 * Optimizes long document experience by lazy-rendering blocks on scroll.
 */
import { ref, shallowRef, computed, watch, onMounted, onBeforeUnmount, nextTick, inject, type Ref } from 'vue'
import { useEditor, EditorContent, VueNodeViewRenderer } from '@tiptap/vue-3'
import { Extension, type Editor as TiptapEditor } from '@tiptap/core'
import { TextSelection, Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import type { Node as ProsemirrorNode } from 'prosemirror-model'
import { DOMSerializer } from 'prosemirror-model'
import type { MarkdownSerializerState } from 'prosemirror-markdown'
import { FloatingMenu } from '@tiptap/extension-floating-menu'
import StarterKit from '@tiptap/starter-kit'
import Paragraph from '@tiptap/extension-paragraph'
import Image from '@tiptap/extension-image'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import Heading from '@tiptap/extension-heading'
import { Markdown } from 'tiptap-markdown'
import Link from '@tiptap/extension-link'
import Underline from '@tiptap/extension-underline'
import Highlight from '@tiptap/extension-highlight'
import { TextStyle } from '@tiptap/extension-text-style'
import { Color } from '@tiptap/extension-color'
import TextAlign from '@tiptap/extension-text-align'
import Subscript from '@tiptap/extension-subscript'
import Superscript from '@tiptap/extension-superscript'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableHeader } from '@tiptap/extension-table-header'
import { TableCell } from '@tiptap/extension-table-cell'
import Typography from '@tiptap/extension-typography'
import Gapcursor from '@tiptap/extension-gapcursor'
import axios from 'axios'
import { pendingNotes } from '../composables/useLibrary'
import { all, createLowlight } from 'lowlight'
import { encryptText, decryptText } from '../crypto'
import ImageView from './ImageView.vue'
import CodeBlockView from './CodeBlockView.vue'
import HistoryPanel from './HistoryPanel.vue'
import DiffView from './DiffView.vue'
import MobileToolbar from './MobileToolbar.vue'
import ShortcutsModal from './ShortcutsModal.vue'
import SlashMenu from './SlashMenu.vue'
import BubbleToolbar from './BubbleToolbar.vue'
import TableOverlay from './TableOverlay.vue'
import { InlineMath } from './InlineMath'
import { Callout } from './Callout'
import { ToggleBlock } from './ToggleBlock'
import { useI18n } from '../i18n'
import { useWordStats } from '../composables/useWordStats'
import { useExport } from '../composables/useExport'
import { useFindReplace } from '../composables/useFindReplace'
import { useEditorSave } from '../composables/useEditorSave'

const lowlight = createLowlight(all)

// Sub-component refs
const slashMenuRef = ref<InstanceType<typeof SlashMenu>>()
const bubbleRef = ref<InstanceType<typeof BubbleToolbar>>()

const props = defineProps<{ noteFileName: string; isDark: boolean; searchHighlight?: string; searchOffset?: number; commitLabels?: Record<string, string> }>()
// title-changed carries the noteFileName so onTitleChanged in App.vue can verify
// the event belongs to the note that is currently selected, preventing a race
// condition where a previous note's late-resolving loadNote overwrites the new
// note's title when currentNote.value has already advanced.
const emit = defineEmits<{ 'title-changed': [key: string, title: string]; 'open-settings': []; 'set-label': [hash: string, label: string] }>()

// Injected settings
const serverEncrypt = inject<Ref<boolean>>('serverEncrypt', ref(true))
// Injected callback to update the full-text search index for this note after save.
const indexNote = inject<(id: string, plainText: string) => void>('indexNote', () => {})
const scheduleOrphanCleanup = inject<() => void>('scheduleOrphanCleanup', () => {})
const editorWidth = inject<Ref<string>>('editorWidth', ref('standard'))
const fontSize = inject<Ref<number>>('fontSize', ref(16))
const typewriterMode = inject<Ref<boolean>>('typewriterMode', ref(false))

// Ref to the editor-content component (scroll container is its $el)
const editorScrollEl = ref<any>(null)

const { t } = useI18n()
const API_BASE = '/api'

// Incremental Rendering State
const fullMarkdownParts = ref<string[]>([])
const loadedPartsCount = ref(0)
const CHUNK_SIZE = 100 // Lines per chunk
let _isLoadingChunk = false // guards onUpdate from treating chunk inserts as user edits
const isFullyLoaded = computed(() => loadedPartsCount.value >= fullMarkdownParts.value.length)

// ── Find & Replace (delegated to composable) ─────────────────────────────
// searchHighlightKey is defined below (near SearchHighlight extension) and
// shared between the find-replace composable and the decoration plugin.
// We forward-declare it here so the composable can be initialised eagerly;
// the actual PluginKey is assigned before the editor is created.
const searchHighlightKey = new PluginKey('searchHighlight')

// editor ref is declared later by useEditor; we create a standalone ref here
// so the composable can reference it, then assign the real editor in the
// useEditor block.  This avoids a circular dependency between the composable
// and the editor creation (which itself depends on searchHighlightKey).
const editorRef = shallowRef<TiptapEditor | undefined>(undefined)

const {
  showFindBar, findQuery, replaceQuery, isReplaceMode,
  findMatchCount, findCurrentIndex, findInputRef,
  findNext, findPrev, replaceOne, replaceAllMatches,
  openFindBar, closeFindBar,
} = useFindReplace({ editor: editorRef, searchHighlightKey })

/**
 * Handles editor scroll to trigger incremental content loading.
 */
const onEditorScroll = (e: Event) => {
  if (isFullyLoaded.value || isLoading.value) return
  const el = e.target as HTMLElement
  if (el.scrollHeight - el.scrollTop - el.clientHeight < 300) {
    loadNextChunk()
  }
}

/**
 * Appends the next chunk of content to the editor.
 *
 * loadedPartsCount is incremented BEFORE insertContentAt because
 * insertContentAt synchronously triggers onUpdate (via ProseMirror's
 * dispatchTransaction). If onUpdate's while-loop calls loadNextChunk
 * before the counter advances, the same chunk would be re-inserted
 * infinitely, causing the document to balloon.
 */
const loadNextChunk = () => {
  if (isFullyLoaded.value || !editor.value) return
  const start = loadedPartsCount.value
  const nextChunk = fullMarkdownParts.value.slice(start, start + CHUNK_SIZE).join('\n')
  loadedPartsCount.value = start + CHUNK_SIZE
  if (nextChunk) {
    _isLoadingChunk = true
    editor.value.commands.insertContentAt(editor.value.state.doc.content.size, nextChunk, {
      updateSelection: false,
      parseOptions: { preserveWhitespace: true }
    })
    _isLoadingChunk = false
  }
}

/**
 * Select All logic (Custom select block vs whole body)
 */
const CustomSelectAll = Extension.create({
  name: 'customSelectAll',
  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: new PluginKey('customSelectAll'),
        props: {
          handleKeyDown(view, event) {
            if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'a') {
              const { state, dispatch } = view; const { doc, selection } = state; const { $from } = selection
              if ($from.depth === 0) return false
              const blockStart = $from.start($from.depth); const blockEnd = $from.end($from.depth)
              const blockSel = TextSelection.create(doc, blockStart, blockEnd)
              const titleSize = doc.firstChild?.nodeSize || 0
              let bodySel = null; if (doc.content.size > titleSize) bodySel = TextSelection.create(doc, titleSize, doc.content.size)
              if (selection.from !== blockSel.from || selection.to !== blockSel.to) { dispatch(state.tr.setSelection(blockSel)); return true }
              else if (bodySel && (selection.from !== bodySel.from || selection.to !== bodySel.to)) { dispatch(state.tr.setSelection(bodySel)); return true }
            }
            return false
          },
        },
      }),
    ]
  },
})

// Search highlight: decorates all occurrences of the search term in the document.
// Activated when the user navigates to a note from the search results panel.
// searchHighlightKey is declared above (shared with useFindReplace composable).

function buildSearchDecorations(doc: ProsemirrorNode, term: string): DecorationSet {
  if (!term) return DecorationSet.empty
  // Handle find-replace mode: format is `__findreplace:<currentIdx>:<term>`
  if (term.startsWith('__findreplace:')) {
    const parts = term.match(/^__findreplace:(\d+):(.+)$/)
    if (!parts) return DecorationSet.empty
    const currentIdx = parseInt(parts[1], 10)
    const searchTerm = parts[2]
    const lower = searchTerm.toLowerCase()
    const decos: Decoration[] = []
    let matchIdx = 0
    doc.descendants((node, pos) => {
      if (!node.isText || !node.text) return
      const text = node.text.toLowerCase()
      let idx = 0
      while ((idx = text.indexOf(lower, idx)) !== -1) {
        const isCurrent = matchIdx === currentIdx
        decos.push(Decoration.inline(pos + idx, pos + idx + lower.length, {
          style: isCurrent
            ? 'background:rgba(234,88,12,0.55);border-radius:2px;padding:1px 0;outline:2px solid rgba(234,88,12,0.8)'
            : 'background:rgba(234,179,8,0.35);border-radius:2px;padding:1px 0',
        }))
        matchIdx++
        idx += lower.length
      }
    })
    return DecorationSet.create(doc, decos)
  }
  const decos: Decoration[] = []
  const lower = term.toLowerCase()
  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return
    const text = node.text.toLowerCase()
    let idx = 0
    while ((idx = text.indexOf(lower, idx)) !== -1) {
      decos.push(Decoration.inline(pos + idx, pos + idx + lower.length, {
        style: 'background:rgba(234,179,8,0.35);border-radius:2px;padding:1px 0',
      }))
      idx += lower.length
    }
  })
  return DecorationSet.create(doc, decos)
}

const SearchHighlight = Extension.create({
  name: 'searchHighlight',
  addProseMirrorPlugins() {
    const initialTerm = props.searchHighlight || ''
    return [
      new Plugin({
        key: searchHighlightKey,
        state: {
          init(_, { doc }) { return buildSearchDecorations(doc, initialTerm) },
          apply(tr, old) {
            const meta = tr.getMeta(searchHighlightKey)
            if (meta !== undefined) return buildSearchDecorations(tr.doc, meta)
            if (tr.docChanged) {
              // Remap existing decorations on doc change (typing clears stale positions)
              return old.map(tr.mapping, tr.doc)
            }
            return old
          },
        },
        props: {
          decorations(state) { return searchHighlightKey.getState(state) },
        },
      }),
    ]
  },
})

// Formatting and UI helpers (truncated for brevity, logic remains identical)
/**
 * Uploads an image file (or clipboard blob) to the server.
 * Derives a safe filename with correct extension from the MIME type when the
 * original name is missing or generic (e.g. clipboard pastes named "image").
 */
const uploadImage = async (file: File) => {
  const mimeToExt: Record<string, string> = { 'image/png': '.png', 'image/jpeg': '.jpg', 'image/gif': '.gif', 'image/webp': '.webp' }
  const ext = mimeToExt[file.type] ?? '.png'
  // Ensure the filename carries a valid extension so the backend can accept it.
  const safeName = file.name && /\.(png|jpg|jpeg|gif|webp)$/i.test(file.name) ? file.name : `image${ext}`
  const reader = new FileReader(); reader.readAsDataURL(file)
  reader.onerror = () => {
    uploadError.value = true
    setTimeout(() => { uploadError.value = false }, 4000)
  }
  reader.onload = async () => {
    const base64 = reader.result as string
    const content = serverEncrypt.value ? new Blob([await encryptText(base64)], { type: 'text/plain' }) : file
    const fd = new FormData(); fd.append('image', content, safeName)
    let uploaded = false
    try {
      const res = await axios.post(`${API_BASE}/upload`, fd)
      editor.value?.chain().focus().setImage({ src: res.data.preview_url, alt: res.data.markdown_url }).run()
      uploaded = true
    } catch (err) {
      console.error('[YinMo] Image upload failed:', err)
      uploadError.value = true
      setTimeout(() => { uploadError.value = false }, 4000)
    }
    if (uploaded) doSave()
  }
}

// Format buttons for the floating menu (desktop only)
const formatButtons = computed(() => [{ label: 'H1', bold: true, mono: false, action: () => editor.value?.chain().focus().toggleHeading({ level: 1 }).run() }, { label: 'H2', bold: true, mono: false, action: () => editor.value?.chain().focus().toggleHeading({ level: 2 }).run() }, { label: 'List', bold: false, mono: false, action: () => editor.value?.chain().focus().toggleBulletList().run() }, { label: 'Code', bold: false, mono: true, action: () => editor.value?.chain().focus().toggleCodeBlock().run() }])

/**
 * Scrolls the editor viewport so the cursor line is vertically centered.
 * Only runs when typewriter mode is enabled.
 */
const applyTypewriterScroll = () => {
  if (!typewriterMode.value || !editor.value || !editorScrollEl.value) return
  const view = editor.value.view
  const { from } = view.state.selection
  const coords = view.coordsAtPos(from)
  // EditorContent is a Vue component; its root DOM node is the scrollable div
  const container: HTMLElement = editorScrollEl.value?.$el ?? editorScrollEl.value
  if (!container?.scrollTo) return
  const containerRect = container.getBoundingClientRect()
  const cursorOffsetFromTop = coords.top - containerRect.top
  const targetScroll = container.scrollTop + cursorOffsetFromTop - container.clientHeight * 0.45
  container.scrollTo({ top: Math.max(0, targetScroll), behavior: 'smooth' })
}

// Track whether search highlight decorations are active so we can clear them on first edit.
let _searchHighlightActive = !!props.searchHighlight

// Editor Setup
const editor = useEditor({
  extensions: [
    StarterKit.configure({ codeBlock: false, heading: false, paragraph: false }),
    // Custom Paragraph: serialize empty paragraphs as <p></p> so multiple blank
    // lines survive the markdown round-trip (plain blank lines collapse to one).
    Paragraph.extend({ addStorage() { return { markdown: { serialize(state: MarkdownSerializerState, node: ProsemirrorNode) { if (!node.childCount) { state.write('<p></p>'); state.closeBlock(node) } else { state.renderInline(node); state.closeBlock(node) } } } } } }),
    CustomSelectAll, Typography, Gapcursor,
    Heading.configure({ levels: [1, 2, 3, 4, 5, 6] }),
    CodeBlockLowlight.extend({ addNodeView() { return VueNodeViewRenderer(CodeBlockView) } }).configure({ lowlight }),
    Image.extend({ inline: false, group: 'block', addAttributes() { return { ...this.parent?.(), width: { default: null, parseHTML: el => (el.getAttribute('width') ? parseInt(el.getAttribute('width')!) : null), renderHTML: attrs => (attrs.width ? { width: attrs.width } : {}) } } }, parseHTML() { return [{ tag: 'img[src]' }] }, addStorage() { return { markdown: { serialize(state: MarkdownSerializerState, node: ProsemirrorNode) { const { src, alt, width } = node.attrs as { src: string; alt?: string; width?: number | null }; if (width) state.write(`<img src="${src}"${alt ? ` alt="${alt}"` : ''} width="${width}">`); else state.write(`![${(alt || '').replace(/\[/g, '\\[').replace(/\]/g, '\\]')}](${src})`) } } } }, addNodeView() { return VueNodeViewRenderer(ImageView) } }),
    Markdown.configure({ html: true, transformPastedText: true, transformCopiedText: true }),
    Underline,
    TextStyle,
    Color,
    Highlight.configure({ multicolor: true }),
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    Subscript,
    Superscript,
    TaskList,
    TaskItem.configure({ nested: true }),
    InlineMath,
    Callout,
    ToggleBlock,
    Link.configure({ openOnClick: false, autolink: true }),
    // Serialize tables as HTML so that column widths (colwidth attribute) are
    // preserved across save/load. tiptap-markdown's GFM pipe-table format drops
    // colwidth; HTML round-trips it correctly because TableCell.addAttributes()
    // emits colwidth as an HTML attribute, and parseHTML reads it back.
    Table.configure({ resizable: true }).extend({
      addStorage() {
        return {
          markdown: {
            serialize(state: any, node: any) {
              const domSerializer = DOMSerializer.fromSchema(node.type.schema)
              const dom = domSerializer.serializeNode(node)
              const wrapper = document.createElement('div')
              wrapper.appendChild(dom as Node)
              state.write(wrapper.innerHTML)
              state.closeBlock(node)
            },
          },
        }
      },
    }),
    TableRow, TableHeader, TableCell,
    SearchHighlight,
  ],
  content: '',
  editorProps: {
    attributes: { class: 'focus:outline-none' },
    handleDOMEvents: {
      mousemove: (view, event) => {
        if (!slashMenuRef.value) return false
        // Desktop-only: hide on narrow viewports
        if (window.innerWidth < 768) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const e = event as MouseEvent
        const pos = view.posAtCoords({ left: e.clientX, top: e.clientY })
        if (!pos) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const $pos = view.state.doc.resolve(pos.pos)
        if ($pos.depth === 0) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        // Resolve to depth-1 ancestor (top-level block).
        // For list items the depth-1 ancestor is the list itself (bulletList/orderedList/taskList),
        // which is the correct drag/convert unit.
        const blockStart = $pos.before(1)
        // Skip pos 0 (document root) and position 0 which is the title H1
        if (blockStart === 0) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const blockNode = view.state.doc.nodeAt(blockStart)
        if (!blockNode) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const domNode = view.nodeDOM(blockStart)
        if (!(domNode instanceof HTMLElement)) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const rect = domNode.getBoundingClientRect()
        // Anchor control to the block's own left edge — independent of sidebar width
        const left = rect.left - 32
        if (left < 4) { slashMenuRef.value.scheduleHideHoverCtrl(); return false }
        const top = rect.top + rect.height / 2
        // Empty block: no text content (ignores structural nodes like image, hr)
        const textTypes = new Set(['paragraph','heading','blockquote','callout','toggleBlock'])
        const isEmpty = textTypes.has(blockNode.type.name) && blockNode.textContent.trim() === ''
        slashMenuRef.value.showHoverCtrl(top, left, isEmpty, blockStart)
        return false
      },
    },
    transformPastedHTML(html) {
      // Sanitise HTML pasted from external sources to prevent XSS.
      return sanitizePastedHtml(html)
    },
    handlePaste: (view, e) => {
      // Upload ALL pasted images (not just the first one)
      const files = e.clipboardData?.files; let hasImage = false
      if (files && files.length > 0) { for (let i = 0; i < files.length; i++) { if (files[i].type.startsWith('image/')) { uploadImage(files[i]); hasImage = true } } }
      if (!hasImage) { const items = Array.from(e.clipboardData?.items || []); for (const item of items) { if (item.type.startsWith('image/')) { const file = item.getAsFile(); if (file) { uploadImage(file); hasImage = true } } } }
      if (hasImage) return true
      const text = e.clipboardData?.getData('text/plain') || ''; const html = e.clipboardData?.getData('text/html') || ''; const isMarkdown = /#\s|\*\*|\[.+?\]\(.+?\)|`{3}|\|\s*---|- \[[ x]\]|==.+==|\$[^$]+\$/.test(text)
      if (text && isMarkdown && (!html || html.includes('data-mime="text/x-markdown"'))) { try { const parser = (editor.value?.storage as any).markdown?.parser; if (!parser) return false; const node = parser.parse(text); if (node) { view.dispatch(view.state.tr.replaceSelectionWith(node)); return true } } catch (err) { console.warn('[YinMo] Markdown paste parse failed:', err) } }
      return false
    },
    handleDrop: (_view, e) => {
      const files = e.dataTransfer?.files
      if (!files || files.length === 0) return false
      let handled = false
      for (let i = 0; i < files.length; i++) {
        if (files[i].type.startsWith('image/')) { uploadImage(files[i]); handled = true }
      }
      if (handled) { e.preventDefault(); return true }
      return false
    },
  },
  onUpdate: ({ editor: ed }) => {
    if (isLoading.value || _isLoadingChunk) return
    // Clear search highlight decorations on first user edit
    if (_searchHighlightActive) {
      _searchHighlightActive = false
      ed.view.dispatch(ed.state.tr.setMeta(searchHighlightKey, ''))
    }
    // If the user edits before all chunks are rendered, load the rest in one
    // bulk insert to prevent save-boundary drift. A single insertContentAt
    // avoids the recursive onUpdate → loadNextChunk → onUpdate loop that
    // would occur if we called loadNextChunk in a while-loop (since
    // insertContentAt synchronously fires onUpdate via dispatchTransaction).
    if (!isFullyLoaded.value && editor.value) {
      const remaining = fullMarkdownParts.value.slice(loadedPartsCount.value).join('\n')
      loadedPartsCount.value = fullMarkdownParts.value.length
      if (remaining) {
        editor.value.commands.insertContentAt(editor.value.state.doc.content.size, remaining, {
          updateSelection: false,
          parseOptions: { preserveWhitespace: true }
        })
      }
    }
    onContentChanged()
    const firstNode = ed.state.doc.firstChild
    if (firstNode && firstNode.type.name !== 'heading' && firstNode.textContent.trim().length > 0) { ed.chain().command(({ tr }) => { tr.setNodeMarkup(0, ed.schema.nodes.heading, { level: 1 }); return true }).run() }
    emit('title-changed', props.noteFileName, ed.state.doc.firstChild?.textContent?.trim() ?? '')
    bubbleRef.value?.updateBubble(ed); updateToc(ed); updateWordStats(ed)
    slashMenuRef.value?.handleSlashOnUpdate(ed)
    applyTypewriterScroll()
  },
  onSelectionUpdate: ({ editor: ed }) => { bubbleRef.value?.updateBubble(ed); applyTypewriterScroll() },
})

// Keep the find-replace composable's editor ref in sync with the real editor.
watch(editor, (ed) => { editorRef.value = ed ?? undefined }, { immediate: true })

// Lifecycle and Methods
const uploadError = ref(false)

// True when the note has no meaningful content (empty or only an empty heading).
// Declared early so the save composable can reference it.
const isContentEmpty = computed(() => {
  if (!editor.value) return true
  const md: string = (editor.value.storage as any).markdown?.getMarkdown() ?? ''
  return md.trim() === '' || /^#+\s*$/.test(md.trim())
})

// ── Save system (extracted to composable) ─────────────────────────────────
const {
  saveStatus, lastSaveError,
  statusText, saveStatusStyle, saveDotStyle,
  doSave, saveIfDirty, onContentChanged,
  clearTimers, resetForLoad,
} = useEditorSave({
  editor: editorRef,
  noteFileName: computed(() => props.noteFileName),
  isContentEmpty,
  serverEncrypt,
  indexNote,
  scheduleOrphanCleanup,
  t: t as unknown as Ref<Record<string, string>>,
})

// Start as true so that the initial onUpdate fired during Tiptap's DOM mount
// (before onMounted calls loadNote) does not emit title-changed with empty content.
// loadNote() sets it to true again, then clears it after nextTick (DOM update).
const isLoading = ref(true)
const loadError = ref(false)
const loadNote = async () => {
  if (!props.noteFileName) return
  resetForLoad()
  isLoading.value = true; loadError.value = false
  // Pending notes haven't been uploaded yet — a GET would return 404.
  // Leave the editor empty; doSave() will create the note on first keystroke.
  if (pendingNotes.has(props.noteFileName)) {
    editor.value?.commands.setContent({ type: 'doc', content: [{ type: 'heading', attrs: { level: 1 }, content: [] }] })
    editor.value?.commands.focus('start')
    await nextTick(); isLoading.value = false
    return
  }
  try {
    const res = await axios.get(`${API_BASE}/notes/${props.noteFileName}`)
    const plain = await decryptText(res.data.content)

    // Split content for incremental rendering
    fullMarkdownParts.value = plain.trim() ? plain.split('\n') : []
    loadedPartsCount.value = Math.min(fullMarkdownParts.value.length, CHUNK_SIZE)

    const initialContent = fullMarkdownParts.value.slice(0, loadedPartsCount.value).join('\n')
    editor.value?.commands.setContent(initialContent || { type: 'doc', content: [{ type: 'heading', attrs: { level: 1 }, content: [] }] })

    await nextTick()
    if (editor.value) { updateToc(editor.value); updateWordStats(editor.value) }
    // Only emit title-changed if the note has actual content — skip for empty/pending
    // notes so we don't overwrite "New Document" with "Untitled" on a blank note.
    const loadedTitle = editor.value?.state.doc.firstChild?.textContent?.trim() ?? ''
    if (loadedTitle) emit('title-changed', props.noteFileName, loadedTitle)
    // If opened from search results with a char offset, load all remaining
    // content before attempting to scroll, then jump to the match closest to
    // the requested offset.
    if (props.searchHighlight && props.searchOffset != null && props.searchOffset >= 0) {
      if (!isFullyLoaded.value && editor.value) {
        const remaining = fullMarkdownParts.value.slice(loadedPartsCount.value).join('\n')
        loadedPartsCount.value = fullMarkdownParts.value.length
        if (remaining) {
          editor.value.commands.insertContentAt(editor.value.state.doc.content.size, remaining, {
            updateSelection: false,
            parseOptions: { preserveWhitespace: true }
          })
        }
      }
      await nextTick()
      scrollToSearchOffset(props.searchOffset, props.searchHighlight)
    } else {
      editor.value?.commands.focus('start')
    }
  } catch {
    loadError.value = true
    editor.value?.commands.setContent({ type: 'doc', content: [{ type: 'heading', attrs: { level: 1 }, content: [] }] })
  } finally { await nextTick(); isLoading.value = false }
}

const onKey = (e: KeyboardEvent) => {
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') { e.preventDefault(); if (saveStatus.value === 'dirty') doSave(); return }
  // Ctrl+F: open find bar; Ctrl+H: open find+replace bar
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'f') { e.preventDefault(); openFindBar(false); return }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'h') { e.preventDefault(); openFindBar(true); return }
  // Ctrl/⌘+Alt+0: paragraph; Ctrl/⌘+Alt+1-6: heading level N
  if ((e.metaKey || e.ctrlKey) && e.altKey && !e.shiftKey) {
    if (e.key === '0') { e.preventDefault(); editor.value?.chain().focus().setParagraph().run(); return }
    const lvl = parseInt(e.key)
    if (lvl >= 1 && lvl <= 6) { e.preventDefault(); editor.value?.chain().focus().setHeading({ level: lvl as 1|2|3|4|5|6 }).run(); return }
  }
  // Escape closes find bar, focus mode, shortcuts
  if (e.key === 'Escape') { if (showFindBar.value) { closeFindBar(); return } if (isFocusMode.value) { isFocusMode.value = false; return } if (showShortcuts.value) { showShortcuts.value = false; return } }
  // Toggle shortcut help panel with ? (no modifier, not in input/textarea)
  const tag = (e.target as HTMLElement).tagName
  if (e.key === '?' && tag !== 'INPUT' && tag !== 'TEXTAREA' && !e.ctrlKey && !e.metaKey) { e.preventDefault(); showShortcuts.value = !showShortcuts.value; return }
  // Toggle focus mode with F key (no modifier, not typing in editor prose)
  if (e.key === 'f' && !e.ctrlKey && !e.metaKey && !e.altKey && (e.target as HTMLElement).classList.contains('ProseMirror') === false && tag !== 'INPUT' && tag !== 'TEXTAREA') { toggleFocusMode(); return }
}
onMounted(() => { window.addEventListener('keydown', onKey, true); document.addEventListener('click', onDocClick); loadNote() })
onBeforeUnmount(async () => {
  window.removeEventListener('keydown', onKey, true)
  document.removeEventListener('click', onDocClick)
  clearTimers()
  // Last resort save attempt
  if (saveStatus.value === 'dirty') await doSave()
  editor.value?.destroy()
})

// TOC and Bubble Menu logic
const showToc = ref(false)
const tocItems = ref<{ level: number; text: string; pos: number }[]>([])
const toggleToc = () => (showToc.value = !showToc.value)
const jumpTo = (pos: number) => editor.value?.chain().focus().setTextSelection(pos).scrollIntoView().run()

/**
 * Scrolls to the search match closest to a given plaintext character offset.
 *
 * The char offset (from SearchResults) counts characters in the decrypted
 * plaintext. ProseMirror positions include structural overhead (nodes, marks),
 * so we cannot use the offset directly. Instead, we find all matches of the
 * search term in the document and pick the one whose surrounding plaintext
 * char index is closest to the requested offset.
 */
const scrollToSearchOffset = (charOffset: number, term: string) => {
  if (!editor.value || !term) return
  const doc = editor.value.state.doc
  const lower = term.toLowerCase()
  // Collect all match positions and their cumulative plaintext char index
  const matches: { from: number; charIdx: number }[] = []
  let charCount = 0
  doc.descendants((node, pos) => {
    if (!node.isText || !node.text) return
    const text = node.text
    const textLower = text.toLowerCase()
    let idx = 0
    while ((idx = textLower.indexOf(lower, idx)) !== -1) {
      matches.push({ from: pos + idx, charIdx: charCount + idx })
      idx += lower.length
    }
    charCount += text.length
  })
  if (matches.length === 0) return
  // Find the match closest to the requested char offset
  let best = matches[0]
  let bestDist = Math.abs(best.charIdx - charOffset)
  for (let i = 1; i < matches.length; i++) {
    const dist = Math.abs(matches[i].charIdx - charOffset)
    if (dist < bestDist) { best = matches[i]; bestDist = dist }
  }
  // Scroll to that match
  editor.value.commands.setTextSelection(best.from)
  const dom = editor.value.view.domAtPos(best.from)
  if (dom?.node) {
    const el = dom.node instanceof HTMLElement ? dom.node : dom.node.parentElement
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }
}
const updateToc = (ed: TiptapEditor) => { const items: { level: number; text: string; pos: number }[] = []; ed.state.doc.descendants((n, pos) => { if (n.type.name === 'heading') items.push({ level: n.attrs.level as number, text: n.textContent, pos }) }); tocItems.value = items }


// Word count stats derived from editor content
// Word stats (see useWordStats composable)
const { wordStats, updateWordStats } = useWordStats()

// Focus / Zen mode state
const isFocusMode = ref(false)
const toggleFocusMode = () => { isFocusMode.value = !isFocusMode.value }

// Keyboard shortcut help panel state
const showShortcuts = ref(false)

const showHistory = ref(false)
const historyLoadError = ref(false)
const toggleHistory = () => { showHistory.value = !showHistory.value }

// Feishu-style inline diff: when set, DiffView replaces the editor viewport
const diffPayload = ref<{ newContent: string; oldContent: string } | null>(null)
const onShowDiff = (payload: { newContent: string; oldContent: string }) => {
  diffPayload.value = payload
}
const exitDiff = () => { diffPayload.value = null }
/** Returns the full current note content including any unloaded (not-yet-rendered) lines. */
const getFullContent = (): string => {
  const renderedPart = (editor.value?.storage as any)?.markdown?.getMarkdown() ?? ''
  const unloadedPart = !isFullyLoaded.value
    ? fullMarkdownParts.value.slice(loadedPartsCount.value).join('\n')
    : ''
  return unloadedPart ? renderedPart + '\n' + unloadedPart : renderedPart
}

/** Resets incremental-rendering state and sets editor content after a rollback. */
const onApplyRollback = (content: string) => {
  // Reset incremental-rendering state to match the rolled-back content.
  // Without this, `fullMarkdownParts` and `loadedPartsCount` would still
  // reflect the pre-rollback document: a subsequent doSave would append the
  // old unrendered tail to the new content, silently corrupting the note.
  fullMarkdownParts.value = content.trim() ? content.split('\n') : []
  loadedPartsCount.value = fullMarkdownParts.value.length  // mark fully loaded
  editor.value?.commands.setContent(content)
}

// Export functions (see useExport composable)
const showExportMenu = ref(false)
const exportMenuRef = ref<HTMLElement>()
const { exportHTML, exportPDF, exportMarkdown } = useExport(editor as any, getFullContent)

/**
 * Sanitises HTML pasted from external sources (B1 fix).
 * Removes script/iframe/form tags, on* event handlers, and javascript: links.
 * Uses a DOM template element so the browser's own HTML parser does the heavy lifting.
 */
// HTML sanitization — whitelist-based, see utils/sanitizeHtml.ts
import { sanitizePastedHtml } from '../utils/sanitizeHtml'

// Close export menu on outside click
const onDocClick = (e: MouseEvent) => {
  if (showExportMenu.value && exportMenuRef.value && !exportMenuRef.value.contains(e.target as Node)) {
    showExportMenu.value = false
  }
}

// Keyboard shortcut definitions for the help panel
const shortcutList = computed(() => [
  { key: 'Ctrl/⌘ + S', desc: t.value.saving },
  { key: 'Ctrl/⌘ + B', desc: 'Bold' },
  { key: 'Ctrl/⌘ + I', desc: 'Italic' },
  { key: 'Ctrl/⌘ + U', desc: 'Underline' },
  { key: 'Ctrl/⌘ + Z', desc: 'Undo' },
  { key: 'Ctrl/⌘ + Shift + Z', desc: 'Redo' },
  { key: 'Ctrl/⌘ + Alt + 0', desc: t.value.cmdText },
  { key: 'Ctrl/⌘ + Alt + 1', desc: t.value.cmdH1 },
  { key: 'Ctrl/⌘ + Alt + 2', desc: t.value.cmdH2 },
  { key: 'Ctrl/⌘ + Alt + 3', desc: t.value.cmdH3 },
  { key: '/', desc: t.value.insertBlockBtn },
  { key: 'Ctrl/⌘ + A', desc: 'Select block / all' },
  { key: 'Ctrl/⌘ + F', desc: t.value.findPlaceholder },
  { key: 'Ctrl/⌘ + H', desc: t.value.replacePlaceholder },
  { key: 'F', desc: t.value.focusMode },
  { key: '?', desc: t.value.shortcutHelp },
])

// Expose the ref for editorRoot (needed for focus-mode CSS selector)
const editorRoot = ref<HTMLElement>()

defineExpose({ doSave, loadNote, saveStatus, isContentEmpty, toggleHistory, exportHTML, exportPDF, exportMarkdown })
</script>

<style>
/* @import must be first — CSS spec requirement */
@import '../assets/editor-prose.css';

.pb-safe { padding-bottom: max(env(safe-area-inset-bottom), 0.5rem); }

/* Editor toolbar button hover */
.editor-toolbar-btn:hover {
  background: var(--bg-hover);
}

/* Focus mode: fade header chrome on idle, hide panels */
.focus-mode > .focus-mode-hide { opacity: 0; pointer-events: none; transition: opacity 0.3s; }
.focus-mode:hover > .focus-mode-hide { opacity: 1; pointer-events: auto; }
.focus-mode .w-48, .focus-mode .w-72 { display: none; }
.focus-mode [style*="width: 480px"] { display: none; }
.focus-mode .editor-content, .focus-mode .flex-1.flex.flex-col.relative { max-width: 680px; margin: 0 auto; }

/* Mobile format toolbar buttons */
.mobile-fmt-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;   /* 40px — wide enough to tap comfortably */
  height: 2.5rem;  /* 40px */
  border-radius: 0.5rem;
  transition: background 0.15s, color 0.15s;
  flex-shrink: 0;
}
.mobile-fmt-btn:active { opacity: 0.7; }
</style>
