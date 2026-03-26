<template>
  <!-- Keyboard shortcut help panel (modal overlay) -->
  <Teleport to="body">
    <div v-if="visible" class="fixed inset-0 z-[200] flex items-center justify-center" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" @click.self="emit('close')">
      <div class="w-full max-w-sm rounded-2xl overflow-hidden" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
        <div class="px-5 py-3 border-b flex items-center justify-between" style="border-color: var(--border);">
          <span class="font-bold text-sm" style="color: var(--text-primary);">{{ t.shortcutHelpTitle }}</span>
          <button @click="emit('close')" style="color: var(--text-muted);">
            <svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
          </button>
        </div>
        <div class="p-4 space-y-1.5">
          <div v-for="s in shortcuts" :key="s.key" class="flex items-center justify-between gap-4">
            <span class="text-sm" style="color: var(--text-secondary);">{{ s.desc }}</span>
            <kbd class="text-xs font-mono px-2 py-0.5 rounded-md border whitespace-nowrap" style="background: var(--bg-app); border-color: var(--border); color: var(--text-muted);">{{ s.key }}</kbd>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
defineProps<{
  /** Whether the modal is visible */
  visible: boolean
  /** Array of shortcut definitions to display */
  shortcuts: { key: string; desc: string }[]
  /** i18n translation object */
  t: Record<string, any>
}>()

const emit = defineEmits<{ close: [] }>()
</script>
