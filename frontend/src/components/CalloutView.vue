<template>
  <NodeViewWrapper class="callout-wrapper">
    <div
      class="callout-block"
      :data-callout-type="calloutType"
      :style="blockStyle"
    >
      <!-- Header: emoji + type selector (non-editable chrome) -->
      <div class="callout-header" contenteditable="false">
        <!-- Emoji picker trigger -->
        <div class="callout-emoji-wrap relative">
          <button
            class="callout-emoji-btn"
            :title="'Change emoji'"
            @click.stop="showEmojiPicker = !showEmojiPicker"
          >{{ displayEmoji }}</button>
          <!-- Inline emoji quick-picker -->
          <div v-if="showEmojiPicker" class="callout-emoji-picker" @click.stop>
            <button
              v-for="e in COMMON_EMOJIS" :key="e"
              class="callout-emoji-opt"
              @click.stop="setEmoji(e)"
            >{{ e }}</button>
          </div>
        </div>
        <!-- Type pill -->
        <div class="callout-type-row">
          <button
            v-for="t in TYPES" :key="t"
            class="callout-type-btn"
            :class="{ active: calloutType === t }"
            :style="calloutType === t ? activePillStyle(t) : ''"
            @click.stop="setType(t)"
          >{{ t }}</button>
        </div>
      </div>
      <!-- Content (editable via Tiptap) -->
      <NodeViewContent class="callout-content" />
    </div>
  </NodeViewWrapper>
</template>

<script setup lang="ts">
import { ref, computed, inject, onMounted, onBeforeUnmount, type Ref } from 'vue'
import { NodeViewWrapper, NodeViewContent, nodeViewProps } from '@tiptap/vue-3'
import { CALLOUT_DEFAULTS, type CalloutType } from './Callout'

const props  = defineProps(nodeViewProps)
const isDark = inject<Ref<boolean>>('isDark', ref(false))

const TYPES: CalloutType[] = ['info', 'warning', 'tip', 'danger']

const COMMON_EMOJIS = [
  '💡','ℹ️','⚠️','🚨','✅','📌','📝','🔥','🎯','💬',
  '🔑','🛡️','⚡','🌟','❓','🎉','🔔','📢','🧩','👀',
]

const COLORS: Record<CalloutType, { border: string; bg: string; bgDark: string; pill: string }> = {
  info:    { border: '#3B82F6', bg: 'rgba(59,130,246,0.06)',  bgDark: 'rgba(59,130,246,0.12)',  pill: '#3B82F6' },
  warning: { border: '#F59E0B', bg: 'rgba(245,158,11,0.06)',  bgDark: 'rgba(245,158,11,0.12)',  pill: '#D97706' },
  tip:     { border: '#10B981', bg: 'rgba(16,185,129,0.06)',  bgDark: 'rgba(16,185,129,0.12)',  pill: '#059669' },
  danger:  { border: '#EF4444', bg: 'rgba(239,68,68,0.06)',   bgDark: 'rgba(239,68,68,0.12)',   pill: '#DC2626' },
}

const showEmojiPicker = ref(false)

const calloutType = computed<CalloutType>(() => (props.node.attrs.type as CalloutType) || 'info')
const displayEmoji = computed(() => props.node.attrs.emoji || CALLOUT_DEFAULTS[calloutType.value]?.emoji || '💡')

const blockStyle = computed(() => {
  const c = COLORS[calloutType.value] || COLORS.info
  return {
    borderLeftColor: c.border,
    background: isDark.value ? c.bgDark : c.bg,
  }
})

const activePillStyle = (t: CalloutType) => {
  const c = COLORS[t] || COLORS.info
  return `background:${c.pill};color:#fff;border-color:${c.pill}`
}

const setType = (t: CalloutType) => props.updateAttributes({ type: t })
const setEmoji = (e: string) => { props.updateAttributes({ emoji: e }); showEmojiPicker.value = false }

// Close emoji picker on outside click
const onOutside = (e: MouseEvent) => {
  if (showEmojiPicker.value && !(e.target as HTMLElement)?.closest('.callout-emoji-wrap')) {
    showEmojiPicker.value = false
  }
}
onMounted(() => document.addEventListener('click', onOutside))
onBeforeUnmount(() => document.removeEventListener('click', onOutside))
</script>

<style scoped>
.callout-block {
  border-left: 3px solid var(--accent);
  border-radius: 0 8px 8px 0;
  padding: 10px 14px 10px 14px;
  margin: 12px 0;
  transition: background 0.2s;
}

.callout-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  user-select: none;
}

.callout-emoji-wrap {
  position: relative;
}

.callout-emoji-btn {
  font-size: 18px;
  line-height: 1;
  padding: 2px;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
}
.callout-emoji-btn:hover {
  background: rgba(0,0,0,0.06);
}

.callout-emoji-picker {
  position: absolute;
  top: 28px;
  left: 0;
  z-index: 100;
  display: flex;
  flex-wrap: wrap;
  width: 200px;
  padding: 6px;
  gap: 2px;
  border-radius: 10px;
  background: var(--bg-editor, #fff);
  border: 1px solid var(--border, #e5e7eb);
  box-shadow: 0 8px 24px rgba(0,0,0,0.12);
}

.callout-emoji-opt {
  font-size: 18px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  cursor: pointer;
  transition: background 0.12s;
}
.callout-emoji-opt:hover {
  background: rgba(0,0,0,0.07);
}

.callout-type-row {
  display: flex;
  gap: 4px;
}

.callout-type-btn {
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  padding: 2px 7px;
  border-radius: 20px;
  border: 1px solid var(--border, #e5e7eb);
  color: var(--text-muted, #9ca3af);
  cursor: pointer;
  transition: all 0.15s;
}
.callout-type-btn:hover {
  opacity: 0.8;
}

.callout-content :deep(p:first-child) {
  margin-top: 0;
}
.callout-content :deep(p:last-child) {
  margin-bottom: 0;
}
</style>
