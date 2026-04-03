<template>
  <Teleport to="body">
    <!-- ── Single hover handle ───────────────────────────────────────────── -->
    <div
      v-if="hoverCtrlVisible"
      class="fixed z-[60] select-none"
      :style="{ top: hoverCtrlTop + 'px', left: hoverCtrlLeft + 'px', transform: 'translateY(-50%)' }"
      @mouseenter="keepHoverCtrl"
      @mouseleave="scheduleHideHoverCtrl"
    >
      <!-- Empty block: [+] insert button — hover opens slash menu after short delay -->
      <button
        v-if="hoverBlockEmpty"
        class="w-6 h-6 flex items-center justify-center rounded text-base font-light transition-colors"
        :style="{ color: 'var(--text-muted)', background: plusHovered ? 'var(--bg-hover)' : 'transparent' }"
        :title="t.insertBlockBtn"
        @click="openSlashMenuFromClick"
        @mouseenter="onPlusMouseEnter"
        @mouseleave="onPlusMouseLeave"
      >+</button>
      <!-- Non-empty block: drag + block-menu handle -->
      <!-- Hover opens block menu after a short delay; mousedown + dragstart handles drag -->
      <div
        v-else
        draggable="true"
        class="w-6 h-6 flex items-center justify-center rounded cursor-grab transition-colors"
        :style="{ color: 'var(--text-muted)', background: handleHovered ? 'var(--bg-hover)' : 'transparent' }"
        :title="t.dragHandle"
        :aria-label="t.dragHandle"
        @mousedown="onHandleMousedown"
        @dragstart="onHandleDragStart"
        @dragend="onHandleDragEnd"
        @mouseenter="onHandleMouseEnter"
        @mouseleave="onHandleMouseLeave"
      ><GripVertical :size="15" aria-hidden="true" /></div>
    </div>

    <!-- ── Block menu (hover-triggered, no backdrop) ─────────────────────── -->
    <div
      v-if="blockMenuVisible"
      ref="blockMenuEl"
      class="fixed z-[69] w-56 overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: blockMenuTop + 'px', left: blockMenuLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
      @mouseenter="cancelCloseBlockMenu"
      @mouseleave="scheduleCloseBlockMenu"
    >
      <!-- 1. Block type grid: 2 rows × 6 icons -->
      <div class="px-2 pt-2 pb-1.5">
        <div class="grid grid-cols-6 gap-1">
          <button
            v-for="cmd in convertCommands"
            :key="cmd.id"
            class="h-9 flex items-center justify-center rounded text-sm font-mono transition-colors"
            :style="cmd.id === currentBlockTypeId
              ? 'background: var(--accent-light); color: var(--accent); font-weight: 600;'
              : 'color: var(--text-secondary);'"
            @click="executeConvert(cmd)"
            @mouseenter="e => { if (cmd.id !== currentBlockTypeId) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'; showTooltip(cmd.title, e) }"
            @mouseleave="e => { if (cmd.id !== currentBlockTypeId) (e.currentTarget as HTMLElement).style.background='transparent'; hideTooltip() }"
          >{{ cmd.icon }}</button>
        </div>
      </div>

      <div class="border-t mx-3" style="border-color: var(--border);"></div>

      <!-- 2. Alignment row → opens submenu -->
      <div class="px-1 pt-0.5">
        <div
          class="flex items-center gap-2 px-2 py-1.5 rounded text-sm cursor-pointer transition-colors"
          :style="activeSubmenu === 'align' ? 'background: var(--bg-hover);' : ''"
          style="color: var(--text-secondary);"
          @click="toggleSubmenu('align')"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => { if (activeSubmenu !== 'align') (e.currentTarget as HTMLElement).style.background='transparent' }"
        >
          <AlignLeft :size="15" class="shrink-0" style="color: var(--text-muted);" />
          <span>{{ t.alignLabel }}</span>
          <ChevronRight
            :size="14"
            class="ml-auto shrink-0 transition-transform duration-150"
            :class="{ 'rotate-90': activeSubmenu === 'align' }"
            :style="activeSubmenu === 'align' ? 'color: var(--accent);' : 'color: var(--text-muted);'"
          />
        </div>
        <!-- 3. Color row → opens submenu -->
        <div
          class="flex items-center gap-2 px-2 py-1.5 rounded text-sm cursor-pointer transition-colors"
          :style="activeSubmenu === 'color' ? 'background: var(--bg-hover);' : ''"
          style="color: var(--text-secondary);"
          @click="toggleSubmenu('color')"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => { if (activeSubmenu !== 'color') (e.currentTarget as HTMLElement).style.background='transparent' }"
        >
          <Palette :size="15" class="shrink-0" style="color: var(--text-muted);" />
          <span>{{ t.colorLabel }}</span>
          <ChevronRight
            :size="14"
            class="ml-auto shrink-0 transition-transform duration-150"
            :class="{ 'rotate-90': activeSubmenu === 'color' }"
            :style="activeSubmenu === 'color' ? 'color: var(--accent);' : 'color: var(--text-muted);'"
          />
        </div>
      </div>

      <div class="border-t mx-2 mt-0.5" style="border-color: var(--border);"></div>

      <!-- 4. Action rows: cut / copy / copy-md / delete -->
      <div class="px-1 py-1">
        <div
          v-for="act in actionCommands"
          :key="act.id"
          class="flex items-center gap-2 px-2 py-1.5 rounded text-sm cursor-pointer transition-colors"
          :style="act.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
          @click="executeAction(act)"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >
          <component :is="act.svgIcon" :size="14" class="shrink-0" />
          <span>{{ act.title }}</span>
        </div>
      </div>
    </div>

    <!-- ── Alignment submenu ──────────────────────────────────────────────── -->
    <div
      v-if="activeSubmenu === 'align'"
      class="fixed z-[70] w-40 overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: submenuTop + 'px', left: submenuLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
      @mouseenter="cancelCloseBlockMenu"
      @mouseleave="scheduleCloseBlockMenu"
    >
      <div class="px-3 pt-2 pb-1 text-xs font-medium" style="color: var(--text-muted);">{{ t.alignLabel }}</div>
      <div class="px-2 pb-2 flex items-center gap-1">
        <button
          v-for="a in alignOptions"
          :key="a.value"
          class="flex-1 h-7 flex items-center justify-center rounded transition-colors"
          :style="currentAlign === a.value ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-muted);'"
          :title="a.label"
          @click="applyAlign(a.value)"
          @mouseenter="e => { if (currentAlign !== a.value) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { if (currentAlign !== a.value) (e.currentTarget as HTMLElement).style.background='transparent' }"
        ><component :is="a.component" :size="15" /></button>
      </div>
    </div>

    <!-- ── Color submenu ──────────────────────────────────────────────────── -->
    <div
      v-if="activeSubmenu === 'color'"
      class="fixed z-[70] overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: submenuTop + 'px', left: submenuLeft + 'px', width: '176px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
      @mouseenter="cancelCloseBlockMenu"
      @mouseleave="scheduleCloseBlockMenu"
    >
      <div class="px-3 pt-2 pb-1 text-xs font-medium" style="color: var(--text-muted);">{{ t.textColor }}</div>
      <div class="px-3 pb-2 flex items-center gap-1.5 flex-wrap">
        <button
          v-for="c in textColors"
          :key="c.value"
          class="w-6 h-6 rounded-full border-2 transition-transform hover:scale-110"
          :style="{ background: c.value, borderColor: currentTextColor === c.value ? 'var(--accent)' : 'transparent' }"
          :title="c.label"
          @click="applyTextColor(c.value)"
        ></button>
        <button
          class="w-5 h-5 rounded-full border-2 transition-transform hover:scale-110 flex items-center justify-center"
          :style="{ borderColor: currentTextColor === null ? 'var(--accent)' : 'transparent', background: 'var(--bg-app)', color: 'var(--text-muted)' }"
          title="默认 / Default"
          @click="applyTextColor(null)"
        ><Ban :size="10" /></button>
      </div>
      <div class="border-t mx-3" style="border-color: var(--border);"></div>
      <div class="px-3 pt-2 pb-1 text-xs font-medium" style="color: var(--text-muted);">{{ t.bgColor }}</div>
      <div class="px-3 pb-2 flex items-center gap-1.5 flex-wrap">
        <button
          v-for="c in bgColors"
          :key="c.value"
          class="w-6 h-6 rounded border-2 transition-transform hover:scale-110"
          :style="{ background: c.value, borderColor: currentBgColor === c.value ? 'var(--accent)' : 'transparent' }"
          :title="c.label"
          @click="applyBgColor(c.value)"
        ></button>
        <button
          class="w-5 h-5 rounded border-2 transition-transform hover:scale-110 flex items-center justify-center"
          :style="{ borderColor: currentBgColor === null ? 'var(--accent)' : 'transparent', background: 'var(--bg-app)', color: 'var(--text-muted)' }"
          title="默认 / Default"
          @click="applyBgColor(null)"
        ><Ban :size="10" /></button>
      </div>
    </div>

    <!-- ── Callout type submenu ───────────────────────────────────────────── -->
    <div
      v-if="activeSubmenu === 'callout'"
      class="fixed z-[70] w-40 overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: submenuTop + 'px', left: submenuLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
      @mouseenter="cancelCloseBlockMenu"
      @mouseleave="scheduleCloseBlockMenu"
    >
      <div
        v-for="ct in calloutTypes"
        :key="ct.type"
        class="flex items-center gap-2 px-3 py-1.5 cursor-pointer transition-colors text-sm"
        style="color: var(--text-secondary);"
        @click="executeCalloutConvert(ct.type)"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      >
        <span>{{ ct.icon }}</span><span>{{ ct.label }}</span>
      </div>
    </div>

    <!-- ── Tooltip (block type grid, anchored to button center-bottom) ──────── -->
    <div
      v-if="tooltip"
      class="fixed z-[200] pointer-events-none px-2 py-1 rounded-md text-xs whitespace-nowrap"
      style="background: rgba(22,22,22,0.92); color: #fff; transform: translateX(-50%);"
      :style="{ top: tooltip.y + 'px', left: tooltip.centerX + 'px' }"
    >{{ tooltip.text }}</div>

    <!-- ── Insert slash menu ──────────────────────────────────────────────── -->
    <div
      v-if="slashMenuVisible"
      ref="slashMenuEl"
      class="fixed rounded-xl z-[70] w-60 overflow-hidden anim-pop-in"
      :style="{ top: slashMenuTop + 'px', left: slashMenuLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)' }"
      @mouseenter="cancelCloseSlashHover"
      @mouseleave="() => { if (slashFromHover) scheduleCloseSlashHover() }"
    >
      <div class="px-3 py-2 border-b text-xs font-medium" style="border-color: var(--border); color: var(--text-muted);">{{ t.insertBlock }}</div>
      <div class="max-h-72 overflow-y-auto py-1">
        <div
          v-for="(cmd, i) in filteredSlashCommands"
          :key="cmd.id"
          class="flex items-center gap-2.5 px-3 py-1.5 cursor-pointer transition-colors"
          :style="i === slashSelectedIdx ? 'background: var(--accent-light);' : ''"
          @click="executeSlashCommand(cmd)"
          @mouseenter="slashSelectedIdx = i"
          @mouseleave="e => { if (slashSelectedIdx !== i) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >
          <span class="w-8 h-8 flex items-center justify-center rounded-lg text-sm shrink-0" :style="slashIconStyle(cmd.id)">{{ cmd.icon }}</span>
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
import type { Component } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import {
  GripVertical, Scissors, Copy, FileText, Trash2,
  AlignLeft, AlignCenter, AlignRight, AlignJustify,
  Palette, ChevronRight, Ban,
} from 'lucide-vue-next'

// SlashCommand uses icon: string (text/emoji label rendered in the command list icon box).
// ActionCommand uses svgIcon: Component (Lucide SVG rendered directly in the action row).
interface SlashCommand {
  id: string
  title: string
  desc: string
  icon: string
  action: (ed: TiptapEditor) => void
}
interface ActionCommand {
  id: string
  title: string
  svgIcon: Component
  danger?: boolean
  action: () => void
}

const props = defineProps<{
  editor: TiptapEditor | undefined
  t: Record<string, any>
}>()

// ── Hover control state ───────────────────────────────────────────────────
const hoverCtrlVisible = ref(false)
const hoverCtrlTop = ref(0)
const hoverCtrlLeft = ref(0)
const hoverBlockEmpty = ref(false)
const hoverBlockPos = ref(0)
let hoverHideTimer: ReturnType<typeof setTimeout> | null = null
let _isDragging = false

const scheduleHideHoverCtrl = () => {
  hoverHideTimer = setTimeout(() => { hoverCtrlVisible.value = false }, 300)
}
const keepHoverCtrl = () => {
  if (hoverHideTimer) clearTimeout(hoverHideTimer)
}

/**
 * Called from Editor.vue mousemove. blockPos is the depth-1 block start position.
 * Guard: no-op while dragging or while any menu is open (prevents flicker).
 */
const showHoverCtrl = (top: number, left: number, isEmpty: boolean, blockPos: number) => {
  if (_isDragging || blockMenuVisible.value || slashMenuVisible.value) return
  if (hoverHideTimer) clearTimeout(hoverHideTimer)
  hoverCtrlTop.value = top
  hoverCtrlLeft.value = left
  hoverBlockEmpty.value = isEmpty
  hoverBlockPos.value = blockPos
  hoverCtrlVisible.value = true
}

// ── Handle visual state ───────────────────────────────────────────────────
const handleHovered = ref(false)
const plusHovered   = ref(false)

// ── Drag support ──────────────────────────────────────────────────────────

/**
 * Mousedown: select the depth-1 block node so dragstart gets the right slice.
 * This is the ONLY place setNodeSelection is called — not on hover — so the
 * user's cursor position is never disrupted by merely hovering the handle.
 */
const onHandleMousedown = (_e: MouseEvent) => {
  const ed = props.editor; if (!ed) return
  try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
}

/**
 * Bridge HTML5 drag into ProseMirror's internal drag mechanism so that PM
 * renders the drop cursor and inserts the slice at the correct position.
 */
const onHandleDragStart = (e: DragEvent) => {
  const ed = props.editor
  if (!ed || !e.dataTransfer) return
  _isDragging = true
  hoverCtrlVisible.value = false
  closeBlockMenu()

  const { state, view } = ed
  const sel = state.selection
  if (sel.empty) return

  const slice = sel.content()
  ;(view as any).dragging = { slice, move: true }

  e.dataTransfer.effectAllowed = 'move'
  e.dataTransfer.setData('text/plain', state.doc.cut(sel.from, sel.to).textContent)

  // Semi-transparent ghost image cloned from the block DOM node
  const domNode = view.nodeDOM(hoverBlockPos.value)
  if (domNode instanceof HTMLElement) {
    const ghost = domNode.cloneNode(true) as HTMLElement
    ghost.style.cssText = `position:fixed;top:-9999px;left:-9999px;opacity:0.6;pointer-events:none;max-width:400px;background:var(--bg-editor);padding:4px 8px;border-radius:8px;font-size:14px;`
    document.body.appendChild(ghost)
    e.dataTransfer.setDragImage(ghost, 0, 16)
    setTimeout(() => ghost.remove(), 0)
  }
}

const onHandleDragEnd = () => {
  _isDragging = false
  const ed = props.editor
  if (ed && (ed.view as any).dragging) {
    ;(ed.view as any).dragging = null
  }
}

// ── Block menu state ──────────────────────────────────────────────────────
const blockMenuVisible = ref(false)
const blockMenuTop = ref(0)
const blockMenuLeft = ref(0)
const blockMenuEl = ref<HTMLElement>()
const currentBlockTypeId = ref('')
const currentTextColor = ref<string | null>(null)
const currentBgColor = ref<string | null>(null)
const currentAlign = ref<string>('left')
// Stored text range used by color commands; set when the menu opens so that
// color operations always target the right block regardless of cursor shifts.
let _blockTextFrom = 0
let _blockTextTo = 0

// Active submenu: 'align' | 'color' | 'callout' | null
const activeSubmenu = ref<'align' | 'color' | 'callout' | null>(null)
const submenuTop = ref(0)
const submenuLeft = ref(0)

// Tooltip for block-type grid icons
// centerX is the horizontal center of the hovered button; used with translateX(-50%) in the template.
const tooltip = ref<{ text: string; centerX: number; y: number } | null>(null)

// ── Block menu close timer ────────────────────────────────────────────────
// Shared by handle, menu panel, and submenus so the mouse can travel between
// them without the menu closing (cancel on mouseenter, start on mouseleave).
let blockMenuCloseTimer: ReturnType<typeof setTimeout> | null = null
let blockMenuOpenTimer: ReturnType<typeof setTimeout> | null = null

const scheduleCloseBlockMenu = () => {
  blockMenuCloseTimer = setTimeout(closeBlockMenu, 150)
}
const cancelCloseBlockMenu = () => {
  if (blockMenuCloseTimer) clearTimeout(blockMenuCloseTimer)
}

// ── [⠿] handle hover handlers ────────────────────────────────────────────

const onHandleMouseEnter = () => {
  handleHovered.value = true
  cancelCloseBlockMenu()
  // Delay opening so that quickly passing the mouse over a block doesn't flash
  // the menu. Cancel if the mouse leaves before the delay fires.
  if (!blockMenuVisible.value) {
    blockMenuOpenTimer = setTimeout(() => {
      if (handleHovered.value) openBlockMenu()
    }, 200)
  }
}

const onHandleMouseLeave = () => {
  handleHovered.value = false
  if (blockMenuOpenTimer) clearTimeout(blockMenuOpenTimer)
  scheduleCloseBlockMenu()
}

// ── [+] button hover handlers ─────────────────────────────────────────────
let plusMenuOpenTimer: ReturnType<typeof setTimeout> | null = null

const onPlusMouseEnter = () => {
  plusHovered.value = true
  plusMenuOpenTimer = setTimeout(() => {
    if (plusHovered.value && !slashMenuVisible.value) openSlashMenuFromHoverHandle()
  }, 200)
}

const onPlusMouseLeave = () => {
  plusHovered.value = false
  if (plusMenuOpenTimer) clearTimeout(plusMenuOpenTimer)
  // If the slash menu was opened by hovering, schedule it to close unless the
  // mouse moves into the menu panel (which will cancel the timer).
  if (slashFromHover && slashMenuVisible.value) scheduleCloseSlashHover()
}

let slashHoverCloseTimer: ReturnType<typeof setTimeout> | null = null
const scheduleCloseSlashHover = () => {
  slashHoverCloseTimer = setTimeout(closeSlashMenu, 150)
}
const cancelCloseSlashHover = () => {
  if (slashHoverCloseTimer) clearTimeout(slashHoverCloseTimer)
}

// ── Tooltip helpers ───────────────────────────────────────────────────────

// Tooltip anchored to button center-bottom, not mouse cursor (avoids jitter on grid).
const showTooltip = (text: string, e: MouseEvent) => {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  tooltip.value = { text, centerX: rect.left + rect.width / 2, y: rect.bottom + 7 }
}
const hideTooltip = () => { tooltip.value = null }

// ── Color / align data ────────────────────────────────────────────────────
const textColors = [
  { value: '#ef4444', label: '红' }, { value: '#f97316', label: '橙' },
  { value: '#eab308', label: '黄' }, { value: '#22c55e', label: '绿' },
  { value: '#3b82f6', label: '蓝' }, { value: '#8b5cf6', label: '紫' },
  { value: '#ec4899', label: '粉' }, { value: '#6b7280', label: '灰' },
]
const bgColors = [
  { value: 'rgba(239,68,68,0.15)',    label: '红底' },
  { value: 'rgba(249,115,22,0.15)',   label: '橙底' },
  { value: 'rgba(234,179,8,0.15)',    label: '黄底' },
  { value: 'rgba(34,197,94,0.15)',    label: '绿底' },
  { value: 'rgba(59,130,246,0.15)',   label: '蓝底' },
  { value: 'rgba(139,92,246,0.15)',   label: '紫底' },
  { value: 'rgba(236,72,153,0.15)',   label: '粉底' },
  { value: 'rgba(107,114,128,0.15)',  label: '灰底' },
]
const alignOptions = [
  { value: 'left',    component: AlignLeft,    label: '左对齐' },
  { value: 'center',  component: AlignCenter,  label: '居中' },
  { value: 'right',   component: AlignRight,   label: '右对齐' },
  { value: 'justify', component: AlignJustify, label: '两端' },
]
const calloutTypes = [
  { type: 'info',    icon: '💡', label: '信息 / Info' },
  { type: 'warning', icon: '⚠️', label: '警告 / Warning' },
  { type: 'tip',     icon: '✅', label: '提示 / Tip' },
  { type: 'danger',  icon: '🚨', label: '危险 / Danger' },
]

// ── Convert commands: 12 types in a 2×6 grid ─────────────────────────────
// H5/H6 are omitted here; they remain available via the slash menu.
const convertCommands = computed<{ id: string; title: string; icon: string; action: (ed: TiptapEditor) => void }[]>(() => [
  { id: 'text',    title: props.t.cmdText,        icon: 'T',   action: (ed) => ed.chain().focus().setParagraph().run() },
  { id: 'h1',      title: props.t.cmdH1,          icon: 'H1',  action: (ed) => ed.chain().focus().setHeading({ level: 1 }).run() },
  { id: 'h2',      title: props.t.cmdH2,          icon: 'H2',  action: (ed) => ed.chain().focus().setHeading({ level: 2 }).run() },
  { id: 'h3',      title: props.t.cmdH3,          icon: 'H3',  action: (ed) => ed.chain().focus().setHeading({ level: 3 }).run() },
  { id: 'h4',      title: props.t.cmdH4,          icon: 'H4',  action: (ed) => ed.chain().focus().setHeading({ level: 4 }).run() },
  { id: 'ul',      title: props.t.cmdUL,          icon: '•',   action: (ed) => ed.chain().focus().toggleBulletList().run() },
  { id: 'ol',      title: props.t.cmdOL,          icon: '1.',  action: (ed) => ed.chain().focus().toggleOrderedList().run() },
  { id: 'todo',    title: props.t.cmdTodo,        icon: '☑',  action: (ed) => ed.chain().focus().toggleTaskList().run() },
  { id: 'code',    title: props.t.cmdCode,        icon: '</>', action: (ed) => ed.chain().focus().toggleCodeBlock().run() },
  { id: 'quote',   title: props.t.cmdQuote,       icon: '❝',  action: (ed) => ed.chain().focus().toggleBlockquote().run() },
  { id: 'callout', title: props.t.cmdCalloutInfo, icon: '💡', action: (_ed) => openSubmenu('callout') },
  { id: 'toggle',  title: props.t.cmdToggle,      icon: '▶',  action: (ed) => { ed.chain().focus().deleteSelection().insertContent({ type: 'toggleBlock', attrs: { open: true, title: 'Toggle' }, content: [{ type: 'paragraph' }] }).run() } },
])

const executeCalloutConvert = (type: string) => {
  const ed = props.editor; if (!ed) return
  // setNodeSelection first so deleteSelection targets the right block
  try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
  ed.chain().focus().deleteSelection().insertContent({ type: 'callout', attrs: { type, emoji: '' }, content: [{ type: 'paragraph' }] }).run()
  closeBlockMenu()
}

/**
 * Execute a block-type conversion. Each action targets hoverBlockPos so the
 * cursor position in the editor is not disturbed until the user confirms an
 * action. setNodeSelection is called here, not on hover.
 */
const executeConvert = (cmd: { id: string; action: (ed: TiptapEditor) => void }) => {
  const ed = props.editor; if (!ed) return
  if (cmd.id === 'callout') { cmd.action(ed); return }
  // Select target block first so conversion applies to the right block
  try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
  cmd.action(ed)
  closeBlockMenu()
}

// ── Action commands ───────────────────────────────────────────────────────
// All actions derive their block range from hoverBlockPos directly so that
// they work correctly without relying on a pre-set ProseMirror selection.
const actionCommands = computed<ActionCommand[]>(() => [
  {
    id: 'cut', title: props.t.blockCut, svgIcon: Scissors,
    action: async () => {
      const ed = props.editor; if (!ed) return
      const bNode = ed.state.doc.nodeAt(hoverBlockPos.value)
      if (!bNode) return
      const text = ed.state.doc.textBetween(hoverBlockPos.value + 1, hoverBlockPos.value + bNode.nodeSize - 1, '\n')
      await navigator.clipboard.writeText(text).catch(() => {})
      try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
      ed.chain().focus().deleteSelection().run()
      closeBlockMenu()
    },
  },
  {
    id: 'copy', title: props.t.blockCopy, svgIcon: Copy,
    action: async () => {
      const ed = props.editor; if (!ed) return
      const bNode = ed.state.doc.nodeAt(hoverBlockPos.value)
      if (!bNode) return
      const text = ed.state.doc.textBetween(hoverBlockPos.value + 1, hoverBlockPos.value + bNode.nodeSize - 1, '\n')
      await navigator.clipboard.writeText(text).catch(() => {})
      closeBlockMenu()
    },
  },
  {
    id: 'copyMd', title: props.t.blockCopyMd, svgIcon: FileText,
    action: async () => {
      const ed = props.editor; if (!ed) return
      const bNode = ed.state.doc.nodeAt(hoverBlockPos.value)
      if (!bNode) return
      const text = ed.state.doc.textBetween(hoverBlockPos.value + 1, hoverBlockPos.value + bNode.nodeSize - 1, '\n')
      let md = text
      if (bNode.type.name === 'heading') {
        md = '#'.repeat(bNode.attrs.level) + ' ' + text
      } else if (bNode.type.name === 'blockquote') {
        md = text.split('\n').map((l: string) => '> ' + l).join('\n')
      } else if (bNode.type.name === 'codeBlock') {
        const lang = bNode.attrs.language ?? ''
        md = '```' + lang + '\n' + text + '\n```'
      }
      await navigator.clipboard.writeText(md).catch(() => {})
      closeBlockMenu()
    },
  },
  {
    id: 'delete', title: props.t.blockDelete, svgIcon: Trash2, danger: true,
    action: () => {
      const ed = props.editor; if (!ed) return
      try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
      ed.chain().focus().deleteSelection().run()
      closeBlockMenu()
    },
  },
])

const executeAction = (act: ActionCommand) => act.action()

// ── Format helpers ────────────────────────────────────────────────────────

const applyTextColor = (color: string | null) => {
  const ed = props.editor; if (!ed) return
  if (color) {
    ed.chain().focus().setTextSelection({ from: _blockTextFrom, to: _blockTextTo }).setColor(color).run()
  } else {
    ed.chain().focus().setTextSelection({ from: _blockTextFrom, to: _blockTextTo }).unsetColor().run()
  }
  currentTextColor.value = color
}

const applyBgColor = (color: string | null) => {
  const ed = props.editor; if (!ed) return
  if (color) {
    ed.chain().focus().setTextSelection({ from: _blockTextFrom, to: _blockTextTo }).setHighlight({ color }).run()
  } else {
    ed.chain().focus().setTextSelection({ from: _blockTextFrom, to: _blockTextTo }).unsetHighlight().run()
  }
  currentBgColor.value = color
}

const applyAlign = (align: string) => {
  const ed = props.editor; if (!ed) return
  // setNodeSelection so alignment applies to the target block, not cursor block
  try { ed.commands.setNodeSelection(hoverBlockPos.value) } catch (_) {}
  ed.chain().focus().setTextAlign(align).run()
  currentAlign.value = align
}

// ── Submenu helpers ───────────────────────────────────────────────────────

/**
 * Open a side submenu panel to the right of the block menu. Falls back to
 * the left side if the right edge would overflow the viewport.
 */
const openSubmenu = (type: 'align' | 'color' | 'callout') => {
  if (!blockMenuEl.value) return
  const rect = blockMenuEl.value.getBoundingClientRect()
  const w = type === 'color' ? 176 : 160
  const leftCandidate = rect.right + 4
  submenuLeft.value = leftCandidate + w > window.innerWidth - 8 ? rect.left - w - 4 : leftCandidate
  submenuTop.value  = rect.top
  activeSubmenu.value = type
}

const toggleSubmenu = (type: 'align' | 'color') => {
  activeSubmenu.value === type ? (activeSubmenu.value = null) : openSubmenu(type)
}

// ── Block menu open / close ───────────────────────────────────────────────

/**
 * Open the block menu panel. Does NOT call setNodeSelection — the user's
 * cursor is never disturbed by hovering the handle. Format state (align/color)
 * is read directly from the block node's attributes and marks so no selection
 * change is needed.
 */
const openBlockMenu = () => {
  const ed = props.editor; if (!ed) return

  const blockNode = ed.state.doc.nodeAt(hoverBlockPos.value)
  const typeName = blockNode?.type.name ?? ''

  // Determine active block type for grid highlighting
  if      (typeName === 'heading')      currentBlockTypeId.value = 'h' + (blockNode?.attrs.level ?? '')
  else if (typeName === 'bulletList')   currentBlockTypeId.value = 'ul'
  else if (typeName === 'orderedList')  currentBlockTypeId.value = 'ol'
  else if (typeName === 'taskList')     currentBlockTypeId.value = 'todo'
  else if (typeName === 'blockquote')   currentBlockTypeId.value = 'quote'
  else if (typeName === 'codeBlock')    currentBlockTypeId.value = 'code'
  else if (typeName === 'callout')      currentBlockTypeId.value = 'callout'
  else if (typeName === 'toggleBlock')  currentBlockTypeId.value = 'toggle'
  else                                  currentBlockTypeId.value = 'text'

  // Store text range for color commands
  if (blockNode) {
    _blockTextFrom = hoverBlockPos.value + 1
    _blockTextTo   = hoverBlockPos.value + blockNode.nodeSize - 1
  } else {
    _blockTextFrom = _blockTextTo = hoverBlockPos.value
  }

  // Read alignment directly from block node attrs (no selection change needed)
  currentAlign.value = blockNode?.attrs?.textAlign ?? 'left'

  // Read text/bg color from the first text node's marks without changing selection
  let txtColor: string | null = null
  let bgColor:  string | null = null
  if (blockNode) {
    blockNode.descendants((node: any) => {
      if (node.isText && txtColor === null) {
        const ts = node.marks.find((m: any) => m.type.name === 'textStyle')
        const hl = node.marks.find((m: any) => m.type.name === 'highlight')
        txtColor = ts?.attrs?.color ?? null
        bgColor  = hl?.attrs?.color ?? null
        return false // stop after first text node
      }
    })
  }
  currentTextColor.value = txtColor
  currentBgColor.value   = bgColor

  blockMenuTop.value  = hoverCtrlTop.value
  blockMenuLeft.value = hoverCtrlLeft.value + 32
  blockMenuVisible.value = true
  nextTick(adjustBlockMenuPos)
}

const adjustBlockMenuPos = () => {
  if (!blockMenuEl.value) return
  const rect = blockMenuEl.value.getBoundingClientRect()
  if (rect.bottom > window.innerHeight - 8) blockMenuTop.value -= rect.bottom - window.innerHeight + 8
  if (rect.right  > window.innerWidth  - 8) blockMenuLeft.value -= rect.right - window.innerWidth + 8
  if (blockMenuTop.value < 8) blockMenuTop.value = 8
}

const closeBlockMenu = () => {
  blockMenuVisible.value = false
  activeSubmenu.value    = null
  tooltip.value          = null
}

// ── Slash insert menu ─────────────────────────────────────────────────────
const slashMenuVisible = ref(false)
const slashMenuTop = ref(0)
const slashMenuLeft = ref(0)
const slashMenuEl = ref<HTMLElement>()
const slashQuery = ref('')
const slashSelectedIdx = ref(0)
let slashFromHover = false

const getSlashRange = (ed: TiptapEditor) => {
  const { selection } = ed.state
  const $from = ed.state.doc.resolve(selection.from)
  return { from: $from.start($from.depth), to: selection.from }
}

const slashCommands = computed<SlashCommand[]>(() => [
  { id: 'text',    title: props.t.cmdText,    desc: props.t.cmdTextDesc,    icon: 'T',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).setParagraph().run() },
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
  { id: 'hr',      title: props.t.cmdHR,      desc: props.t.cmdHRDesc,      icon: '—',   action: (ed) => ed.chain().focus().deleteRange(getSlashRange(ed)).setHorizontalRule().run() },
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

// Returns background + text color for each slash command category icon box.
const slashIconStyle = (id: string): string => {
  if (/^h[1-6]$/.test(id))      return 'background:rgba(59,130,246,0.12);color:#3b82f6'
  if (id === 'text' || id === 'hr') return 'background:var(--bg-hover);color:var(--text-secondary)'
  if (id === 'ul' || id === 'ol' || id === 'todo') return 'background:rgba(34,197,94,0.12);color:#22c55e'
  if (id === 'code' || id === 'math' || id === 'diagram') return 'background:rgba(107,114,128,0.14);color:var(--text-secondary)'
  if (id === 'callout-info')    return 'background:rgba(59,130,246,0.12);color:#3b82f6'
  if (id === 'callout-warning') return 'background:rgba(249,115,22,0.12);color:#f97316'
  if (id === 'callout-tip')     return 'background:rgba(34,197,94,0.12);color:#22c55e'
  if (id === 'callout-danger')  return 'background:rgba(239,68,68,0.12);color:#ef4444'
  if (id === 'quote')  return 'background:rgba(139,92,246,0.12);color:#8b5cf6'
  if (id === 'toggle') return 'background:rgba(249,115,22,0.12);color:#f97316'
  if (id === 'table')  return 'background:rgba(59,130,246,0.10);color:#3b82f6'
  return 'background:var(--bg-hover);color:var(--text-secondary)'
}

const adjustSlashMenuPos = () => {
  if (!slashMenuEl.value) return
  const rect = slashMenuEl.value.getBoundingClientRect()
  if (rect.bottom > window.innerHeight) slashMenuTop.value -= rect.height + 24
  if (rect.right  > window.innerWidth)  slashMenuLeft.value -= rect.right - window.innerWidth + 16
}

/** Opened by clicking the [+] button (cursor is already in the empty block). */
const openSlashMenuFromClick = () => {
  const ed = props.editor; if (!ed) return
  const coords = ed.view.coordsAtPos(ed.state.selection.from)
  slashMenuTop.value  = coords.bottom + 4
  slashMenuLeft.value = coords.left
  slashFromHover = true
  slashMenuVisible.value = true
  nextTick(adjustSlashMenuPos)
}

/**
 * Opened by hovering the [+] handle. Moves the cursor into the empty block
 * first so that slash commands insert at the right position.
 */
const openSlashMenuFromHoverHandle = () => {
  const ed = props.editor; if (!ed) return
  const pos = hoverBlockPos.value + 1
  try { ed.chain().focus().setTextSelection(pos).run() } catch (_) {}
  const coords = ed.view.coordsAtPos(pos)
  slashMenuTop.value  = coords.bottom + 4
  slashMenuLeft.value = coords.left
  slashFromHover = true
  slashMenuVisible.value = true
  nextTick(adjustSlashMenuPos)
}

// Keep backward-compat alias used by Editor.vue keyboard handler
const openSlashMenuFromHover = openSlashMenuFromClick

/**
 * Called by Editor.vue onUpdate to drive the slash menu from keyboard input.
 */
const handleSlashOnUpdate = (ed: TiptapEditor) => {
  const { selection } = ed.state
  if (!selection.empty) { if (!slashFromHover) closeSlashMenu(); return }
  const from = selection.from
  const $from = ed.state.doc.resolve(from)
  const parentTypeName = $from.parent.type.name
  if (parentTypeName !== 'paragraph' && parentTypeName !== 'heading') { if (!slashFromHover) closeSlashMenu(); return }
  const match = ed.state.doc.textBetween($from.start($from.depth), from).match(/^[/\\]([a-zA-Z0-9\u4e00-\u9fa5]*)$/)
  if (match) {
    slashQuery.value = match[1] ?? ''
    slashSelectedIdx.value = 0
    if (!slashMenuVisible.value) {
      const coords = ed.view.coordsAtPos(from)
      slashMenuTop.value  = coords.bottom + 4
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
  showHoverCtrl,
  hoverCtrlVisible,
  hoverCtrlTop,
  hoverCtrlLeft,
})
</script>
