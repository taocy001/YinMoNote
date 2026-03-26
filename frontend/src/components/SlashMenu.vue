<template>
  <!-- Hover controls & Slash menu -->
  <Teleport to="body">
    <div
      v-if="hoverCtrlVisible"
      class="fixed flex items-center gap-0.5 z-[60] select-none"
      :style="{ top: hoverCtrlTop + 'px', left: hoverCtrlLeft + 'px', transform: 'translateY(-50%)' }"
      @mouseenter="keepHoverCtrl"
      @mouseleave="scheduleHideHoverCtrl"
    >
      <div
        draggable="true"
        @dragstart="onBlockDragStart"
        class="w-6 h-6 flex items-center justify-center rounded cursor-grab text-xs transition-colors"
        style="color: var(--text-muted);"
        :title="t.dragHandle"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      >
        ⠿
      </div>
      <button
        @click="openSlashMenuFromHover"
        class="w-6 h-6 flex items-center justify-center rounded text-sm transition-colors"
        style="color: var(--text-muted);"
        :title="t.insertBlockBtn"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      >
        +
      </button>
    </div>

    <div
      v-if="slashMenuVisible"
      class="fixed rounded-xl z-[70] w-60 overflow-hidden anim-pop-in"
      :style="{ top: slashMenuTop + 'px', left: slashMenuLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
    >
      <div class="px-3 py-2 border-b text-xs font-medium" style="border-color: var(--border); color: var(--text-muted);">{{ t.insertBlock }}</div>
      <div class="max-h-72 overflow-y-auto py-1">
        <div
          v-for="(cmd, i) in filteredSlashCommands"
          :key="cmd.id"
          @click="executeSlashCommand(cmd)"
          @mouseenter="slashSelectedIdx = i"
          class="flex items-center gap-2.5 px-3 py-1.5 cursor-pointer transition-colors"
          :style="i === slashSelectedIdx ? 'background: var(--accent-light);' : ''"
          @mouseleave="e => { if (slashSelectedIdx !== i) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >
          <span class="w-7 h-7 flex items-center justify-center rounded-lg text-xs font-bold shrink-0" style="background: var(--bg-app); color: var(--text-secondary);">{{ cmd.icon }}</span>
          <div class="min-w-0 text-left">
            <div class="text-sm font-medium truncate" style="color: var(--text-primary);">{{ cmd.title }}</div>
            <div class="text-xs truncate" style="color: var(--text-muted);">{{ cmd.desc }}</div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'

/** Shape of a slash command entry */
interface SlashCommand {
  id: string
  title: string
  desc: string
  icon: string
  action: (ed: TiptapEditor) => void
}

const props = defineProps<{
  /** TipTap editor instance */
  editor: TiptapEditor | undefined
  /** i18n translation object */
  t: Record<string, any>
}>()

// ── Hover controls state ──────────────────────────────────────────────────
const hoverCtrlVisible = ref(false)
const hoverCtrlTop = ref(0)
const hoverCtrlLeft = ref(0)
let hoverHideTimer: ReturnType<typeof setTimeout> | null = null

const scheduleHideHoverCtrl = () => {
  hoverHideTimer = setTimeout(() => { hoverCtrlVisible.value = false }, 300)
}
const keepHoverCtrl = () => {
  if (hoverHideTimer) clearTimeout(hoverHideTimer)
}
const onBlockDragStart = (e: DragEvent) => {
  if (e.dataTransfer) { e.dataTransfer.effectAllowed = 'move'; hoverCtrlVisible.value = false }
}

// ── Slash menu state ──────────────────────────────────────────────────────
const slashMenuVisible = ref(false)
const slashMenuTop = ref(0)
const slashMenuLeft = ref(0)
const slashQuery = ref('')
const slashSelectedIdx = ref(0)
let slashFromHover = false

const getSlashRange = (ed: TiptapEditor) => {
  const { selection } = ed.state
  const $from = ed.state.doc.resolve(selection.from)
  return { from: $from.start($from.depth), to: selection.from }
}

const slashCommands = computed<SlashCommand[]>(() => [
  { id: 'h1',      title: props.t.cmdH1,      desc: props.t.cmdH1Desc,      icon: 'H1',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 1 }).run() },
  { id: 'h2',      title: props.t.cmdH2,      desc: props.t.cmdH2Desc,      icon: 'H2',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 2 }).run() },
  { id: 'h3',      title: props.t.cmdH3,      desc: props.t.cmdH3Desc,      icon: 'H3',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 3 }).run() },
  { id: 'h4',      title: props.t.cmdH4,      desc: props.t.cmdH4Desc,      icon: 'H4',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 4 }).run() },
  { id: 'h5',      title: props.t.cmdH5,      desc: props.t.cmdH5Desc,      icon: 'H5',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 5 }).run() },
  { id: 'h6',      title: props.t.cmdH6,      desc: props.t.cmdH6Desc,      icon: 'H6',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleHeading({ level: 6 }).run() },
  { id: 'ul',      title: props.t.cmdUL,      desc: props.t.cmdULDesc,      icon: '•',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleBulletList().run() },
  { id: 'ol',      title: props.t.cmdOL,      desc: props.t.cmdOLDesc,      icon: '1.',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleOrderedList().run() },
  { id: 'todo',    title: props.t.cmdTodo,    desc: props.t.cmdTodoDesc,    icon: '☑',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleTaskList().run() },
  { id: 'quote',   title: props.t.cmdQuote,   desc: props.t.cmdQuoteDesc,   icon: '❝',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleBlockquote().run() },
  { id: 'table',   title: props.t.cmdTable,   desc: props.t.cmdTableDesc,   icon: '⊞',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run() },
  { id: 'code',    title: props.t.cmdCode,    desc: props.t.cmdCodeDesc,    icon: '<>',  action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleCodeBlock().run() },
  { id: 'math',    title: props.t.cmdMath,    desc: props.t.cmdMathDesc,    icon: '∑',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleCodeBlock({ language: 'math' }).run() },
  { id: 'diagram', title: props.t.cmdDiagram, desc: props.t.cmdDiagramDesc, icon: '⬡',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).toggleCodeBlock({ language: 'mermaid' }).run() },
  { id: 'hr',           title: props.t.cmdHR,            desc: props.t.cmdHRDesc,            icon: '—',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).setHorizontalRule().run() },
  { id: 'callout-info',    title: props.t.cmdCalloutInfo,    desc: props.t.cmdCalloutInfoDesc,    icon: '💡',  action: (ed) => { const r = getSlashRange(ed); ed.chain().focus().deleteRange(r).insertContent({ type: 'callout', attrs: { type: 'info',    emoji: '' }, content: [{ type: 'paragraph' }] }).run() } },
  { id: 'callout-warning', title: props.t.cmdCalloutWarning, desc: props.t.cmdCalloutWarningDesc, icon: '⚠️',  action: (ed) => { const r = getSlashRange(ed); ed.chain().focus().deleteRange(r).insertContent({ type: 'callout', attrs: { type: 'warning', emoji: '' }, content: [{ type: 'paragraph' }] }).run() } },
  { id: 'callout-tip',     title: props.t.cmdCalloutTip,     desc: props.t.cmdCalloutTipDesc,     icon: '✅',  action: (ed) => { const r = getSlashRange(ed); ed.chain().focus().deleteRange(r).insertContent({ type: 'callout', attrs: { type: 'tip',     emoji: '' }, content: [{ type: 'paragraph' }] }).run() } },
  { id: 'callout-danger',  title: props.t.cmdCalloutDanger,  desc: props.t.cmdCalloutDangerDesc,  icon: '🚨',  action: (ed) => { const r = getSlashRange(ed); ed.chain().focus().deleteRange(r).insertContent({ type: 'callout', attrs: { type: 'danger',  emoji: '' }, content: [{ type: 'paragraph' }] }).run() } },
  { id: 'toggle',          title: props.t.cmdToggle,         desc: props.t.cmdToggleDesc,         icon: '▶',   action: (ed) => { const r = getSlashRange(ed); ed.chain().focus().deleteRange(r).insertContent({ type: 'toggleBlock', attrs: { open: true, title: 'Toggle' }, content: [{ type: 'paragraph' }] }).run() } },
])

const filteredSlashCommands = computed(() => {
  const q = slashQuery.value.toLowerCase()
  return slashCommands.value.filter(c => c.title.toLowerCase().includes(q) || c.id.includes(q))
})

const executeSlashCommand = (cmd: SlashCommand) => {
  if (props.editor) cmd.action(props.editor)
  closeSlashMenu()
}

const closeSlashMenu = () => {
  slashMenuVisible.value = false
  slashQuery.value = ''
  slashFromHover = false
}

const adjustSlashMenuPos = () => {
  const menu = document.querySelector('.z-\\[70\\]') as HTMLElement
  if (!menu) return
  const rect = menu.getBoundingClientRect()
  if (rect.bottom > window.innerHeight) slashMenuTop.value -= rect.height + 24
  if (rect.right > window.innerWidth) slashMenuLeft.value -= rect.right - window.innerWidth + 16
}

const openSlashMenuFromHover = () => {
  const ed = props.editor
  if (!ed) return
  const coords = ed.view.coordsAtPos(ed.state.selection.from)
  slashMenuTop.value = coords.bottom + 4
  slashMenuLeft.value = coords.left
  slashFromHover = true
  slashMenuVisible.value = true
}

/**
 * Called by the parent Editor.vue onUpdate handler to drive the slash menu
 * from typing context. This avoids the parent needing direct access to
 * internal refs like slashQuery, slashFromHover, etc.
 */
const handleSlashOnUpdate = (ed: TiptapEditor) => {
  const { selection } = ed.state
  if (!selection.empty) { if (!slashFromHover) closeSlashMenu(); return }
  const from = selection.from
  const $from = ed.state.doc.resolve(from)
  if ($from.parent.type.name !== 'paragraph') { if (!slashFromHover) closeSlashMenu(); return }
  const match = ed.state.doc.textBetween($from.start($from.depth), from).match(/^[/\\]([a-zA-Z0-9\u4e00-\u9fa5]*)$/)
  if (match) {
    slashQuery.value = match[1] ?? ''
    slashSelectedIdx.value = 0
    if (!slashMenuVisible.value) {
      const coords = ed.view.coordsAtPos(from)
      slashMenuTop.value = coords.bottom + 4
      slashMenuLeft.value = coords.left
      slashFromHover = false
      slashMenuVisible.value = true
      nextTick(adjustSlashMenuPos)
    }
  } else {
    if (!slashFromHover) closeSlashMenu()
  }
}

defineExpose({
  slashMenuVisible,
  closeSlashMenu,
  openSlashMenuFromHover,
  adjustSlashMenuPos,
  scheduleHideHoverCtrl,
  handleSlashOnUpdate,
  hoverCtrlVisible,
  hoverCtrlTop,
  hoverCtrlLeft,
})
</script>
