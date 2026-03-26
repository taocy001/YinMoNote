<template>
  <!-- Bubble menu for selection -->
  <Teleport to="body">
    <div v-if="bubbleVisible" class="fixed z-[55] select-none" :style="{ top: bubbleTop + 'px', left: bubbleLeft + 'px' }">
      <div class="flex items-center rounded-xl overflow-hidden" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-md);">
        <template v-if="bubbleLinkMode">
          <input
            ref="bubbleLinkInput"
            v-model="bubbleLinkUrl"
            @keydown.enter.prevent="confirmLink"
            @keydown.esc.prevent="cancelLink"
            :placeholder="t.linkPlaceholder"
            class="text-sm px-3 py-2 outline-none w-64 bg-transparent"
            style="color: var(--text-primary);"
          />
          <button @click="confirmLink" class="px-3 py-2 text-xs font-medium transition-all" style="background: var(--accent); color: white;">{{ t.confirm }}</button>
          <button @click="cancelLink" class="px-2 py-2 text-xs transition-all" style="color: var(--text-muted);"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
          </button>
        </template>
        <template v-else>
          <button
            v-for="btn in bubbleButtons"
            :key="btn.key"
            @click="btn.action()"
            :title="btn.title"
            class="px-2.5 py-2 text-sm transition-colors border-r last:border-r-0"
            :style="`border-color: var(--border); ${btn.isActive?.() ? 'background: var(--accent-light); color: var(--accent);' : 'color: var(--text-secondary);'}`"
            :class="[btn.bold ? 'font-bold' : btn.mono ? 'font-mono' : '']"
            @mouseenter="e => { if (!btn.isActive?.()) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
            @mouseleave="e => { if (!btn.isActive?.()) (e.currentTarget as HTMLElement).style.background='transparent' }"
          >
            {{ btn.label }}
          </button>
        </template>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'

/** Shape of a bubble toolbar button */
interface BubbleButton {
  key: string
  label: string
  title: string
  bold: boolean
  mono: boolean
  action: () => void
  isActive: () => boolean | undefined
}

const SAFE_LINK_PROTO = /^(https?|mailto|tel):/i

const props = defineProps<{
  /** TipTap editor instance */
  editor: TiptapEditor | undefined
  /** i18n translation object */
  t: Record<string, any>
}>()

// ── Bubble state ──────────────────────────────────────────────────────────
const bubbleVisible = ref(false)
const bubbleTop = ref(0)
const bubbleLeft = ref(0)
const bubbleLinkMode = ref(false)
const bubbleLinkUrl = ref('')
const bubbleLinkInput = ref<HTMLInputElement>()

const bubbleButtons = computed<BubbleButton[]>(() => [
  { key: 'bold',        label: 'B',  title: 'Bold',        bold: true,  mono: false, action: () => props.editor?.chain().focus().toggleBold().run(),        isActive: () => props.editor?.isActive('bold') },
  { key: 'italic',      label: 'I',  title: 'Italic',      bold: false, mono: false, action: () => props.editor?.chain().focus().toggleItalic().run(),      isActive: () => props.editor?.isActive('italic') },
  { key: 'strike',      label: 'S',  title: 'Strikethrough', bold: false, mono: false, action: () => props.editor?.chain().focus().toggleStrike().run(),    isActive: () => props.editor?.isActive('strike') },
  { key: 'highlight',   label: '✦',  title: 'Highlight',   bold: false, mono: false, action: () => props.editor?.chain().focus().toggleHighlight().run(),   isActive: () => props.editor?.isActive('highlight') },
  { key: 'superscript', label: 'X²', title: 'Superscript', bold: false, mono: false, action: () => props.editor?.chain().focus().toggleSuperscript().run(), isActive: () => props.editor?.isActive('superscript') },
  { key: 'subscript',   label: 'X₂', title: 'Subscript',   bold: false, mono: false, action: () => props.editor?.chain().focus().toggleSubscript().run(),   isActive: () => props.editor?.isActive('subscript') },
  { key: 'link', label: '🔗', title: 'Link', bold: false, mono: false, action: () => { bubbleLinkUrl.value = props.editor?.getAttributes('link').href || ''; bubbleLinkMode.value = true; nextTick(() => bubbleLinkInput.value?.focus()) }, isActive: () => props.editor?.isActive('link') },
])

const confirmLink = () => {
  const url = bubbleLinkUrl.value.trim()
  if (url) {
    // Block dangerous protocols — only allow http(s), mailto, and tel.
    if (!SAFE_LINK_PROTO.test(url) && !url.startsWith('/') && !url.startsWith('#')) {
      bubbleLinkMode.value = false
      return
    }
    props.editor?.chain().focus().setLink({ href: url }).run()
  } else {
    props.editor?.chain().focus().unsetLink().run()
  }
  bubbleLinkMode.value = false
}

const cancelLink = () => (bubbleLinkMode.value = false)

/**
 * Called by the parent Editor.vue to update bubble position/visibility
 * based on the current selection state.
 */
const updateBubble = (ed: TiptapEditor) => {
  const { selection } = ed.state
  if (selection.empty) { bubbleVisible.value = false; return }
  const s = ed.view.coordsAtPos(selection.from)
  const e = ed.view.coordsAtPos(selection.to)
  bubbleLeft.value = (s.left + e.left) / 2
  bubbleTop.value = s.top - 40
  bubbleVisible.value = true
}

defineExpose({
  bubbleVisible,
  bubbleTop,
  bubbleLeft,
  updateBubble,
})
</script>
