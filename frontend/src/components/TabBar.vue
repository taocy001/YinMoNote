<template>
  <div v-if="tabs.length > 0" class="hidden md:flex items-end shrink-0 select-none overflow-x-auto tab-bar" @wheel.prevent="onWheel">
    <div
      v-for="(tab, idx) in tabs" :key="tab"
      class="group relative flex items-center gap-1.5 px-3.5 cursor-pointer shrink-0 transition-all duration-150 tab-item"
      :class="[
        tab === currentNote ? 'tab-active' : 'tab-inactive',
        tab === previewTab ? 'tab-preview' : ''
      ]"
      draggable="true"
      @click="emit('select', tab)"
      @dblclick="emit('pin', tab)"
      @mousedown.middle.prevent="emit('close', tab)"
      @dragstart="onDragStart($event, idx)"
      @dragover.prevent="onDragOver(idx)"
      @drop="onDrop(idx)"
      @dragend="dragIdx = -1; dropIdx = -1"
    >
      <span class="truncate max-w-[180px] ts-sm leading-none" :class="tab === previewTab ? 'italic' : ''">{{ titles[tab] || tab }}</span>
      <button
        class="w-5 h-5 flex items-center justify-center rounded-md opacity-0 group-hover:opacity-100 transition-all duration-150 tab-close-btn focus-ring"
        @click.stop="emit('close', tab)"
        @mousedown.stop
      >
        <svg width="10" height="10" viewBox="0 0 8 8" fill="none">
          <path d="M1 1l6 6M7 1l-6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
      </button>
      <!-- Active indicator bar -->
      <div v-if="tab === currentNote" class="absolute bottom-0 left-2 right-2 h-[2px] rounded-full" style="background: var(--accent);"></div>
      <!-- Drop position indicator -->
      <div v-if="dropIdx === idx" class="absolute left-0 top-2 bottom-2 w-0.5 rounded" style="background: var(--accent);"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  tabs: string[]
  currentNote: string | null
  titles: Record<string, string>
  previewTab: string | null
}>()

const emit = defineEmits<{
  select: [id: string]
  close: [id: string]
  pin: [id: string]
  reorder: [fromIdx: number, toIdx: number]
}>()

const dragIdx = ref(-1)
const dropIdx = ref(-1)

const onDragStart = (e: DragEvent, idx: number) => {
  dragIdx.value = idx
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', String(idx))
  }
}
const onDragOver = (idx: number) => { dropIdx.value = idx }
const onDrop = (toIdx: number) => {
  if (dragIdx.value >= 0 && dragIdx.value !== toIdx) emit('reorder', dragIdx.value, toIdx)
  dragIdx.value = -1; dropIdx.value = -1
}
const onWheel = (e: WheelEvent) => { (e.currentTarget as HTMLElement).scrollLeft += e.deltaY }
</script>

<style scoped>
.tab-bar {
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border);
  scrollbar-width: none;
  padding: 0 4px;
  gap: 1px;
}
.tab-bar::-webkit-scrollbar { display: none; }

.tab-item {
  padding-top: 8px;
  padding-bottom: 8px;
  border-radius: 8px 8px 0 0;
  min-height: 36px;
}

.tab-active {
  color: var(--accent);
  background: var(--bg-editor);
  font-weight: 600;
}
:global(.dark) .tab-active {
  box-shadow: 0 -1px 8px var(--accent-light);
}

.tab-inactive {
  color: var(--text-muted);
  background: transparent;
}
.tab-inactive:hover {
  color: var(--text-secondary);
  background: var(--bg-hover);
}

.tab-preview span {
  font-style: italic;
}

.tab-close-btn {
  color: var(--text-muted);
}
.tab-close-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}
</style>
