<template>
  <node-view-wrapper class="toggle-wrapper">
    <div class="toggle-block" :class="{ 'toggle-closed': !isOpen }">
      <!-- Toggle header: arrow + editable title -->
      <div class="toggle-header" contenteditable="false">
        <button
          class="toggle-arrow"
          @click.stop="toggleOpen"
          :title="isOpen ? 'Collapse' : 'Expand'"
          :aria-expanded="isOpen"
        >
          <svg
            width="12" height="12" viewBox="0 0 12 12" fill="none"
            :style="{ transform: isOpen ? 'rotate(90deg)' : 'rotate(0deg)', transition: 'transform 0.2s' }"
          >
            <path d="M4.5 2.5L7.5 6L4.5 9.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </button>
        <!-- Editable title — uses a contenteditable span, not a ProseMirror node -->
        <span
          ref="titleEl"
          class="toggle-title"
          contenteditable="true"
          :data-placeholder="'Toggle'"
          @input="onTitleInput"
          @keydown.enter.prevent="focusContent"
          @keydown.backspace="onTitleBackspace"
          >{{ props.node.attrs.title }}</span
        >
      </div>
      <!-- Collapsible content -->
      <div v-show="isOpen" class="toggle-content">
        <node-view-content />
      </div>
    </div>
  </node-view-wrapper>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { NodeViewWrapper, NodeViewContent, nodeViewProps } from '@tiptap/vue-3'

const props = defineProps(nodeViewProps)
const titleEl = ref<HTMLElement | null>(null)

const isOpen = computed(() => props.node.attrs.open !== false)

const toggleOpen = () => props.updateAttributes({ open: !isOpen.value })

// Sync the contenteditable title to the node attribute on every keystroke.
// We write to the DOM directly on mount to avoid cursor-jumping on re-render.
const onTitleInput = (e: Event) => {
  const text = (e.target as HTMLElement).textContent || ''
  props.updateAttributes({ title: text })
}

// Enter inside the title → move cursor into the content area
const focusContent = () => {
  if (!isOpen.value) props.updateAttributes({ open: true })
  nextTick(() => {
    const contentEl = titleEl.value?.closest('.toggle-block')?.querySelector('.toggle-content .ProseMirror, .toggle-content [contenteditable="true"]') as HTMLElement | null
    if (contentEl) {
      contentEl.focus()
      // Place cursor at start
      const sel = window.getSelection()
      const range = document.createRange()
      range.setStart(contentEl, 0)
      range.collapse(true)
      sel?.removeAllRanges()
      sel?.addRange(range)
    }
  })
}

// Backspace on empty title → remove the toggle block
const onTitleBackspace = (e: KeyboardEvent) => {
  const text = (e.target as HTMLElement).textContent || ''
  if (text === '') {
    e.preventDefault()
    props.deleteNode()
  }
}

// Set initial DOM content without triggering re-render loop
onMounted(() => {
  if (titleEl.value) {
    titleEl.value.textContent = props.node.attrs.title || 'Toggle'
  }
})
</script>

<style scoped>
.toggle-block {
  margin: 8px 0;
  border-radius: 6px;
}

.toggle-header {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  cursor: default;
}

.toggle-arrow {
  flex-shrink: 0;
  width: 20px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  color: var(--text-muted, #9ca3af);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
  margin-top: 1px;
}
.toggle-arrow:hover {
  background: var(--bg-hover, rgba(0,0,0,0.06));
  color: var(--text-secondary, #6b7280);
}

.toggle-title {
  flex: 1;
  min-width: 0;
  font-weight: 600;
  font-size: 1em;
  line-height: 1.5;
  color: var(--text-primary, #111);
  outline: none;
  border: none;
  background: transparent;
  word-break: break-word;
  cursor: text;
}
.toggle-title:empty::before {
  content: attr(data-placeholder);
  color: var(--text-muted, #9ca3af);
  pointer-events: none;
}

.toggle-content {
  padding-left: 26px;
  margin-top: 2px;
}
.toggle-content :deep(> .ProseMirror),
.toggle-content :deep(> [data-node-view-content]) {
  padding: 0;
}
/* Visually tighten the first child paragraph */
.toggle-content :deep(p:first-child) {
  margin-top: 2px;
}
</style>
