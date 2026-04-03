<template>
  <Teleport to="body">
    <!-- ── Main toolbar ───────────────────────────────────────────────────── -->
    <div
      v-if="bubbleVisible"
      ref="toolbarEl"
      class="fixed z-[55] select-none flex items-center rounded-xl overflow-hidden anim-pop-in px-1"
      :style="{ top: bubbleTop + 'px', left: bubbleLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-md)' }"
    >
      <!-- Link-edit mode: replaces the whole toolbar -->
      <template v-if="bubbleLinkMode">
        <input
          ref="bubbleLinkInput"
          v-model="bubbleLinkUrl"
          :placeholder="t.linkPlaceholder"
          class="text-sm px-3 py-2.5 outline-none w-64 bg-transparent"
          style="color: var(--text-primary);"
          @keydown.enter.prevent="confirmLink"
          @keydown.esc.prevent="cancelLink"
        />
        <button
          class="px-3 py-2.5 text-xs font-medium"
          style="background: var(--accent); color: white;"
          @mousedown.prevent
          @click="confirmLink"
        >{{ t.confirm }}</button>
        <button
          class="px-2.5 py-2.5 text-xs transition-colors"
          style="color: var(--text-muted);"
          @mousedown.prevent
          @click="cancelLink"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >✕</button>
      </template>

      <!-- Normal mode: grouped buttons -->
      <template v-else>
        <!-- Group 1: Block type — hidden for code/callout/toggle where type change is destructive -->
        <template v-if="!isSpecialBlock">
          <button
            class="flex items-center gap-0.5 px-3 py-2.5 text-sm font-medium transition-colors rounded-lg"
            :style="typeMenuVisible ? 'background: var(--bg-hover);' : 'color: var(--text-secondary);'"
            @mousedown.prevent
            @click="toggleTypeMenu"
            @mouseenter="e => { showTooltip(e, t.blockType); (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
            @mouseleave="e => { hideTooltip(); if (!typeMenuVisible) (e.currentTarget as HTMLElement).style.background='transparent' }"
          >
            <span class="font-mono font-bold text-sm" style="min-width: 18px; text-align: center;">{{ currentTypeLabel }}</span>
            <ChevronDown :size="11" class="opacity-50" />
          </button>
          <div class="w-px h-5 shrink-0 mx-0.5" style="background: var(--border);"></div>
        </template>

        <!-- Group 2: Bold, Strike, Italic, Underline -->
        <button
          v-for="btn in inlineButtons"
          :key="btn.key"
          class="px-3 py-2.5 text-sm transition-colors rounded-lg"
          :style="btn.isActive() ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
          :class="btn.cls"
          @mousedown.prevent
          @click="btn.action()"
          @mouseenter="e => { showTooltip(e, btn.title); if (!btn.isActive()) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!btn.isActive()) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >{{ btn.label }}</button>

        <div class="w-px h-5 shrink-0 mx-0.5" style="background: var(--border);"></div>

        <!-- Group 3: Text color picker -->
        <button
          class="flex items-center gap-0.5 px-3 py-2.5 text-sm font-bold transition-colors rounded-lg"
          :style="colorMenuVisible ? 'background: var(--bg-hover);' : ''"
          @mousedown.prevent
          @click="toggleColorMenu"
          @mouseenter="e => { showTooltip(e, t.textColor); (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!colorMenuVisible) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >
          <!-- A with colored bar showing active text color -->
          <span class="relative inline-block leading-none pb-0.5" style="color: var(--text-secondary);">
            A
            <span
              class="absolute bottom-0 left-0 right-0 h-[3px] rounded-full"
              :style="{ background: activeTextColor ?? 'var(--text-secondary)' }"
            ></span>
          </span>
          <ChevronDown :size="11" class="opacity-50" style="color: var(--text-muted);" />
        </button>

        <div class="w-px h-5 shrink-0 mx-0.5" style="background: var(--border);"></div>

        <!-- Group 4: Link, inline code -->
        <button
          class="px-3 py-2.5 text-sm transition-colors rounded-lg"
          :style="props.editor?.isActive('link') ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
          @mousedown.prevent
          @click="openLinkMode"
          @mouseenter="e => { showTooltip(e, t.link); if (!props.editor?.isActive('link')) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!props.editor?.isActive('link')) (e.currentTarget as HTMLElement).style.background='transparent' }"
        ><Link :size="15" /></button>
        <button
          class="px-3 py-2.5 text-xs font-mono transition-colors rounded-lg"
          :style="props.editor?.isActive('code') ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
          @mousedown.prevent
          @click="props.editor?.chain().focus().toggleCode().run()"
          @mouseenter="e => { showTooltip(e, t.inlineCode); if (!props.editor?.isActive('code')) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!props.editor?.isActive('code')) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >&lt;/&gt;</button>

        <div class="w-px h-5 shrink-0 mx-0.5" style="background: var(--border);"></div>

        <!-- Group 5: Superscript, subscript -->
        <button
          class="px-3 py-2.5 text-xs transition-colors rounded-lg"
          :style="props.editor?.isActive('superscript') ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
          @mousedown.prevent
          @click="props.editor?.chain().focus().toggleSuperscript().run()"
          @mouseenter="e => { showTooltip(e, t.superscript); if (!props.editor?.isActive('superscript')) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!props.editor?.isActive('superscript')) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >X<sup>2</sup></button>
        <button
          class="px-3 py-2.5 text-xs transition-colors rounded-lg"
          :style="props.editor?.isActive('subscript') ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
          @mousedown.prevent
          @click="props.editor?.chain().focus().toggleSubscript().run()"
          @mouseenter="e => { showTooltip(e, t.subscript); if (!props.editor?.isActive('subscript')) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
          @mouseleave="e => { hideTooltip(); if (!props.editor?.isActive('subscript')) (e.currentTarget as HTMLElement).style.background='transparent' }"
        >X<sub>2</sub></button>
      </template>
    </div>

    <!-- ── Tooltip ─────────────────────────────────────────────────────────── -->
    <div
      v-if="tooltip && bubbleVisible"
      class="fixed z-[60] pointer-events-none px-2 py-1 text-xs rounded-md whitespace-nowrap"
      style="background: rgba(0,0,0,0.72); color: #fff; transform: translateX(-50%);"
      :style="{ top: tooltip.y + 'px', left: tooltip.centerX + 'px' }"
    >{{ tooltip.text }}</div>

    <!-- ── Backdrop closes open dropdowns ─────────────────────────────────── -->
    <div
      v-if="typeMenuVisible || colorMenuVisible"
      class="fixed inset-0 z-[56]"
      @mousedown.prevent
      @click="closeAllDropdowns"
    ></div>

    <!-- ── Block-type dropdown ────────────────────────────────────────────── -->
    <div
      v-if="typeMenuVisible"
      class="fixed z-[57] overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: dropdownTop + 'px', left: dropdownLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', minWidth: '148px' }"
    >
      <div
        v-for="opt in typeOptions"
        :key="opt.id"
        class="flex items-center gap-2.5 px-3 py-2 cursor-pointer text-sm transition-colors"
        :style="opt.id === currentTypeId ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'"
        @mousedown.prevent
        @click="applyType(opt)"
        @mouseenter="e => { if (opt.id !== currentTypeId) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (opt.id !== currentTypeId) (e.currentTarget as HTMLElement).style.background='transparent' }"
      >
        <span class="w-6 text-center font-mono text-xs font-bold shrink-0" style="color: var(--text-muted);">{{ opt.icon }}</span>
        <span>{{ opt.label }}</span>
      </div>
    </div>

    <!-- ── Color dropdown ─────────────────────────────────────────────────── -->
    <div
      v-if="colorMenuVisible"
      class="fixed z-[57] overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: dropdownTop + 'px', left: dropdownLeft + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', width: '176px' }"
    >
      <div class="px-3 pt-2.5 pb-1 text-xs font-medium" style="color: var(--text-muted);">{{ t.textColor }}</div>
      <div class="px-3 pb-2 flex items-center gap-1.5 flex-wrap">
        <button
          v-for="c in textColors"
          :key="c.value"
          class="w-5 h-5 rounded-full border-2 transition-transform hover:scale-110"
          :style="{ background: c.value, borderColor: activeTextColor === c.value ? 'var(--accent)' : 'transparent' }"
          :title="c.label"
          @mousedown.prevent
          @click="applyTextColor(c.value)"
        ></button>
        <button
          class="w-5 h-5 rounded-full border-2 transition-transform hover:scale-110 flex items-center justify-center"
          :style="{ borderColor: activeTextColor === null ? 'var(--accent)' : 'transparent', background: 'var(--bg-app)', color: 'var(--text-muted)' }"
          title="默认 / Default"
          @mousedown.prevent
          @click="applyTextColor(null)"
        ><Ban :size="10" /></button>
      </div>
      <div class="border-t mx-2" style="border-color: var(--border);"></div>
      <div class="px-3 pt-2 pb-1 text-xs font-medium" style="color: var(--text-muted);">{{ t.bgColor }}</div>
      <div class="px-3 pb-2.5 flex items-center gap-1.5 flex-wrap">
        <button
          v-for="c in bgColors"
          :key="c.value"
          class="w-5 h-5 rounded border-2 transition-transform hover:scale-110"
          :style="{ background: c.value, borderColor: activeBgColor === c.value ? 'var(--accent)' : 'transparent' }"
          :title="c.label"
          @mousedown.prevent
          @click="applyBgColor(c.value)"
        ></button>
        <button
          class="w-5 h-5 rounded border-2 transition-transform hover:scale-110 flex items-center justify-center"
          :style="{ borderColor: activeBgColor === null ? 'var(--accent)' : 'transparent', background: 'var(--bg-app)', color: 'var(--text-muted)' }"
          title="默认 / Default"
          @mousedown.prevent
          @click="applyBgColor(null)"
        ><Ban :size="10" /></button>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import { Link, ChevronDown, Ban } from 'lucide-vue-next'

const SAFE_LINK_PROTO = /^(https?|mailto|tel):/i

const props = defineProps<{
  editor: TiptapEditor | undefined
  t: Record<string, any>
}>()

// ── Tooltip ───────────────────────────────────────────────────────────────
// centerX is the horizontal center of the hovered button; used with translateX(-50%) in the template.
const tooltip = ref<{ text: string; centerX: number; y: number } | null>(null)

const showTooltip = (e: MouseEvent, text: string) => {
  if (!text) return
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  tooltip.value = { text, centerX: rect.left + rect.width / 2, y: rect.bottom + 7 }
}
const hideTooltip = () => { tooltip.value = null }

// ── Bubble visibility & position ──────────────────────────────────────────
const bubbleVisible = ref(false)
const bubbleTop  = ref(0)
const bubbleLeft = ref(0)
const toolbarEl  = ref<HTMLElement>()

// ── Dropdown state ────────────────────────────────────────────────────────
const typeMenuVisible  = ref(false)
const colorMenuVisible = ref(false)
const dropdownTop  = ref(0)
const dropdownLeft = ref(0)

const closeAllDropdowns = () => {
  typeMenuVisible.value  = false
  colorMenuVisible.value = false
}

const openDropdownAt = (e: MouseEvent, dropdownWidth: number) => {
  const btn = (e.currentTarget as HTMLElement).getBoundingClientRect()
  dropdownTop.value  = btn.bottom + 4
  dropdownLeft.value = Math.min(btn.left, window.innerWidth - dropdownWidth - 8)
}

const toggleTypeMenu = (e: MouseEvent) => {
  if (typeMenuVisible.value) { closeAllDropdowns(); return }
  colorMenuVisible.value = false
  openDropdownAt(e, 160)
  typeMenuVisible.value = true
}

const toggleColorMenu = (e: MouseEvent) => {
  if (colorMenuVisible.value) { closeAllDropdowns(); return }
  typeMenuVisible.value = false
  openDropdownAt(e, 176)
  colorMenuVisible.value = true
}

// ── Link mode ─────────────────────────────────────────────────────────────
const bubbleLinkMode  = ref(false)
const bubbleLinkUrl   = ref('')
const bubbleLinkInput = ref<HTMLInputElement>()

const openLinkMode = () => {
  bubbleLinkUrl.value  = props.editor?.getAttributes('link').href ?? ''
  bubbleLinkMode.value = true
  nextTick(() => bubbleLinkInput.value?.focus())
}

const confirmLink = () => {
  const url = bubbleLinkUrl.value.trim()
  if (url) {
    if (!SAFE_LINK_PROTO.test(url) && !url.startsWith('/') && !url.startsWith('#')) {
      bubbleLinkMode.value = false; return
    }
    props.editor?.chain().focus().setLink({ href: url }).run()
  } else {
    props.editor?.chain().focus().unsetLink().run()
  }
  bubbleLinkMode.value = false
}

const cancelLink = () => { bubbleLinkMode.value = false }

// ── Block-type detection (refs, updated in updateBubble) ──────────────────
const currentTypeId  = ref('text')
const isSpecialBlock = ref(false)

const typeOptions = computed(() => [
  { id: 'text', icon: 'T',  label: props.t.cmdText ?? '正文',    action: () => props.editor?.chain().focus().setParagraph().run() },
  { id: 'h1',   icon: 'H1', label: props.t.cmdH1   ?? '一级标题', action: () => props.editor?.chain().focus().setHeading({ level: 1 }).run() },
  { id: 'h2',   icon: 'H2', label: props.t.cmdH2   ?? '二级标题', action: () => props.editor?.chain().focus().setHeading({ level: 2 }).run() },
  { id: 'h3',   icon: 'H3', label: props.t.cmdH3   ?? '三级标题', action: () => props.editor?.chain().focus().setHeading({ level: 3 }).run() },
  { id: 'h4',   icon: 'H4', label: props.t.cmdH4   ?? '四级标题', action: () => props.editor?.chain().focus().setHeading({ level: 4 }).run() },
])

const currentTypeLabel = computed(() =>
  typeOptions.value.find(o => o.id === currentTypeId.value)?.icon ?? 'T'
)

const applyType = (opt: { action: () => void }) => {
  opt.action()
  closeAllDropdowns()
}

// ── Inline format buttons ─────────────────────────────────────────────────
const inlineButtons = computed(() => [
  { key: 'bold',      label: 'B', title: props.t.bold      ?? 'Bold',          cls: 'font-bold',    isActive: () => !!props.editor?.isActive('bold'),      action: () => props.editor?.chain().focus().toggleBold().run() },
  { key: 'strike',    label: 'S', title: props.t.strike    ?? 'Strikethrough', cls: 'line-through', isActive: () => !!props.editor?.isActive('strike'),    action: () => props.editor?.chain().focus().toggleStrike().run() },
  { key: 'italic',    label: 'I', title: props.t.italic    ?? 'Italic',        cls: 'italic',       isActive: () => !!props.editor?.isActive('italic'),    action: () => props.editor?.chain().focus().toggleItalic().run() },
  { key: 'underline', label: 'U', title: props.t.underline ?? 'Underline',     cls: 'underline',    isActive: () => !!props.editor?.isActive('underline'), action: () => props.editor?.chain().focus().toggleUnderline().run() },
])

// ── Color state (refs, updated in updateBubble) ───────────────────────────
const activeTextColor = ref<string | null>(null)
const activeBgColor   = ref<string | null>(null)

const textColors = [
  { value: '#ef4444', label: '红' }, { value: '#f97316', label: '橙' },
  { value: '#eab308', label: '黄' }, { value: '#22c55e', label: '绿' },
  { value: '#3b82f6', label: '蓝' }, { value: '#8b5cf6', label: '紫' },
  { value: '#ec4899', label: '粉' }, { value: '#6b7280', label: '灰' },
]
const bgColors = [
  { value: 'rgba(239,68,68,0.15)',   label: '红底' }, { value: 'rgba(249,115,22,0.15)',  label: '橙底' },
  { value: 'rgba(234,179,8,0.15)',   label: '黄底' }, { value: 'rgba(34,197,94,0.15)',   label: '绿底' },
  { value: 'rgba(59,130,246,0.15)',  label: '蓝底' }, { value: 'rgba(139,92,246,0.15)', label: '紫底' },
  { value: 'rgba(236,72,153,0.15)',  label: '粉底' }, { value: 'rgba(107,114,128,0.15)', label: '灰底' },
]

const applyTextColor = (color: string | null) => {
  if (color) props.editor?.chain().focus().setColor(color).run()
  else        props.editor?.chain().focus().unsetColor().run()
}

const applyBgColor = (color: string | null) => {
  if (color) props.editor?.chain().focus().setHighlight({ color }).run()
  else        props.editor?.chain().focus().unsetHighlight().run()
}

// ── Position & visibility (called from Editor.vue on every update) ────────
const updateBubble = (ed: TiptapEditor) => {
  const { selection } = ed.state
  if (selection.empty) {
    bubbleVisible.value = false
    closeAllDropdowns()
    bubbleLinkMode.value = false
    hideTooltip()
    return
  }

  if      (ed.isActive('heading', { level: 1 })) currentTypeId.value = 'h1'
  else if (ed.isActive('heading', { level: 2 })) currentTypeId.value = 'h2'
  else if (ed.isActive('heading', { level: 3 })) currentTypeId.value = 'h3'
  else if (ed.isActive('heading', { level: 4 })) currentTypeId.value = 'h4'
  else                                            currentTypeId.value = 'text'

  isSpecialBlock.value = ed.isActive('codeBlock') || ed.isActive('callout') || ed.isActive('toggleBlock')

  activeTextColor.value = ed.getAttributes('textStyle').color ?? null
  activeBgColor.value   = ed.getAttributes('highlight').color ?? null

  bubbleVisible.value = true

  nextTick(() => {
    if (!toolbarEl.value) return
    const { width, height } = toolbarEl.value.getBoundingClientRect()

    let midX: number, selTop: number, selBottom: number

    const domSel = window.getSelection()
    if (domSel && domSel.rangeCount > 0) {
      const rect = domSel.getRangeAt(0).getBoundingClientRect()
      if (rect.width > 0 || rect.height > 0) {
        midX      = rect.left + rect.width / 2
        selTop    = rect.top
        selBottom = rect.bottom
      } else {
        const c = ed.view.coordsAtPos(selection.from)
        midX = c.left; selTop = c.top; selBottom = c.bottom
      }
    } else {
      const c = ed.view.coordsAtPos(selection.from)
      midX = c.left; selTop = c.top; selBottom = c.bottom
    }

    let left = midX - width / 2
    let top  = selTop - height - 10

    left = Math.max(8, Math.min(left, window.innerWidth - width - 8))
    if (top < 8) top = selBottom + 10

    bubbleLeft.value = left
    bubbleTop.value  = top
  })
}

defineExpose({
  bubbleVisible,
  bubbleTop,
  bubbleLeft,
  updateBubble,
})
</script>
