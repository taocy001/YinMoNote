<template>
  <Teleport to="body">
    <Transition name="cmd-palette">
      <div v-if="modelValue" class="fixed inset-0 z-[500] flex items-start justify-center pt-[15vh]" style="background: rgba(0,0,0,0.4); backdrop-filter: blur(4px);" role="presentation" @click.self="emit('update:modelValue', false)">
        <div class="w-full max-w-lg rounded-2xl overflow-hidden anim-pop-in" role="dialog" aria-modal="true" aria-label="Command palette" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
          <!-- Search input -->
          <div class="flex items-center gap-3 px-4 py-3" style="border-bottom: 1px solid var(--border);">
            <svg class="shrink-0" width="16" height="16" viewBox="0 0 16 16" fill="none" style="color: var(--text-muted);" aria-hidden="true">
              <circle cx="7" cy="7" r="4.5" stroke="currentColor" stroke-width="1.4"/>
              <path d="M10.5 10.5L14 14" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>
            </svg>
            <input
              ref="inputRef"
              v-model="query"
              :placeholder="t.cmdPaletteSearch"
              class="flex-1 bg-transparent outline-none ts-base"
              style="color: var(--text-primary); font-family: inherit;"
              role="combobox"
              aria-autocomplete="list"
              aria-controls="cmd-palette-list"
              :aria-activedescendant="activeDescendantId"
              @keydown.down.prevent="moveSelection(1)"
              @keydown.up.prevent="moveSelection(-1)"
              @keydown.enter.prevent="executeSelected"
              @keydown.esc.prevent="emit('update:modelValue', false)"
            />
            <kbd class="shrink-0 ts-xs px-1.5 py-0.5 rounded-md font-mono" style="background: var(--bg-hover); color: var(--text-muted); border: 1px solid var(--border);" aria-hidden="true">ESC</kbd>
          </div>

          <!-- Results list -->
          <div id="cmd-palette-list" ref="listRef" class="max-h-[320px] overflow-y-auto py-1" role="listbox" style="scrollbar-width: thin; scrollbar-color: var(--border) transparent;">
            <!-- Section: Commands (when query starts with >) -->
            <template v-if="isCommandMode">
              <div
v-for="(cmd, idx) in filteredCommands" :id="`cmd-palette-item-${idx}`"
                :key="cmd.id"
                class="flex items-center gap-3 px-4 py-2.5 cursor-pointer transition-micro"
                role="option"
                :aria-selected="idx === selectedIdx"
                :style="idx === selectedIdx ? 'background: var(--bg-active); color: var(--accent);' : 'color: var(--text-secondary);'"
                @click="executeCommand(cmd)"
                @mouseenter="selectedIdx = idx"
              >
                <span class="w-7 h-7 flex items-center justify-center rounded-lg shrink-0 ts-sm" style="background: var(--bg-hover);" aria-hidden="true">{{ cmd.icon }}</span>
                <span class="ts-sm font-medium">{{ cmd.label }}</span>
              </div>
            </template>

            <!-- Section: Notes search -->
            <template v-else>
              <!-- Recent notes (when query is empty) -->
              <div v-if="!query && recentNotes.length > 0" class="px-4 pt-2 pb-1" role="presentation">
                <span class="ts-xs font-semibold uppercase tracking-wider" style="color: var(--text-muted);">{{ t.cmdPaletteRecent }}</span>
              </div>

              <div
v-for="(item, idx) in displayItems" :id="`cmd-palette-item-${idx}`"
                :key="item.key"
                class="flex items-center gap-3 px-4 py-2 cursor-pointer transition-micro"
                role="option"
                :aria-selected="idx === selectedIdx"
                :style="idx === selectedIdx ? 'background: var(--bg-active); color: var(--accent);' : 'color: var(--text-secondary);'"
                @click="selectNote(item.key)"
                @mouseenter="selectedIdx = idx"
              >
                <svg class="shrink-0" width="14" height="14" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);" aria-hidden="true">
                  <path d="M3 2h8v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2z" stroke="currentColor" stroke-width="1.2"/>
                  <path d="M5 5h4M5 7.5h3" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.5"/>
                </svg>
                <span class="ts-sm font-medium truncate">{{ titles[item.key] || item.key }}</span>
              </div>

              <!-- No results -->
              <div v-if="displayItems.length === 0" class="px-4 py-6 text-center ts-sm" style="color: var(--text-muted);" role="status">
                {{ t.cmdPaletteNoResults }}
              </div>
            </template>
          </div>

          <!-- Footer hint -->
          <div class="flex items-center gap-3 px-4 py-2" style="border-top: 1px solid var(--border);" aria-hidden="true">
            <span class="ts-xs" style="color: var(--text-muted);">
              <kbd class="px-1 py-0.5 rounded font-mono ts-xs" style="background: var(--bg-hover); border: 1px solid var(--border);">↑↓</kbd>
              <kbd class="px-1 py-0.5 rounded font-mono ts-xs ml-1" style="background: var(--bg-hover); border: 1px solid var(--border);">↵</kbd>
            </span>
            <span class="ts-xs" style="color: var(--text-muted);">
              &gt; {{ t.cmdSettings.toLowerCase() }}…
            </span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'

const props = defineProps<{
  modelValue: boolean
  titles: Record<string, string>
  noteKeys: string[]
  recentNotes: string[]
  t: Record<string, any>
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  'select-note': [key: string]
  'new-note': []
  'open-settings': []
  'lock': []
  'toggle-theme': []
  'toggle-trash': []
}>()

const query = ref('')
const selectedIdx = ref(0)
const inputRef = ref<HTMLInputElement>()
const listRef = ref<HTMLElement>()

const isCommandMode = computed(() => query.value.startsWith('>'))

const activeDescendantId = computed(() => {
  const len = isCommandMode.value ? filteredCommands.value.length : displayItems.value.length
  return len > 0 ? `cmd-palette-item-${selectedIdx.value}` : undefined
})

const commands = computed(() => [
  { id: 'new-note', icon: '✏️', label: props.t.cmdNewNote, action: () => emit('new-note') },
  { id: 'settings', icon: '⚙️', label: props.t.cmdSettings, action: () => emit('open-settings') },
  { id: 'lock', icon: '🔒', label: props.t.cmdLock, action: () => emit('lock') },
  { id: 'theme', icon: '🎨', label: props.t.cmdToggleTheme, action: () => emit('toggle-theme') },
  { id: 'trash', icon: '🗑', label: props.t.cmdTrash, action: () => emit('toggle-trash') },
])

const filteredCommands = computed(() => {
  const q = query.value.slice(1).trim().toLowerCase()
  if (!q) return commands.value
  return commands.value.filter(c => c.label.toLowerCase().includes(q))
})

const filteredNotes = computed(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return []
  return props.noteKeys.filter(key => {
    const title = (props.titles[key] || key).toLowerCase()
    return title.includes(q)
  }).slice(0, 20)
})

const displayItems = computed(() => {
  if (query.value.trim()) {
    return filteredNotes.value.map(key => ({ key }))
  }
  return props.recentNotes.slice(0, 8).map(key => ({ key }))
})

const totalItems = computed(() =>
  isCommandMode.value ? filteredCommands.value.length : displayItems.value.length
)

watch(() => props.modelValue, (v) => {
  if (v) {
    query.value = ''
    selectedIdx.value = 0
    nextTick(() => inputRef.value?.focus())
  }
})

watch(query, () => { selectedIdx.value = 0 })

const scrollSelectedIntoView = () => {
  nextTick(() => {
    const el = listRef.value?.querySelector(`#cmd-palette-item-${selectedIdx.value}`)
    el?.scrollIntoView({ block: 'nearest' })
  })
}

const moveSelection = (delta: number) => {
  const len = totalItems.value
  if (len === 0) return
  selectedIdx.value = (selectedIdx.value + delta + len) % len
  scrollSelectedIntoView()
}

const executeSelected = () => {
  if (isCommandMode.value) {
    const cmd = filteredCommands.value[selectedIdx.value]
    if (cmd) executeCommand(cmd)
  } else {
    const item = displayItems.value[selectedIdx.value]
    if (item) selectNote(item.key)
  }
}

const executeCommand = (cmd: { action: () => void }) => {
  emit('update:modelValue', false)
  cmd.action()
}

const selectNote = (key: string) => {
  emit('update:modelValue', false)
  emit('select-note', key)
}
</script>

<style>
.cmd-palette-enter-active { transition: opacity 150ms var(--ease-micro); }
.cmd-palette-leave-active { transition: opacity 100ms var(--ease-micro); }
.cmd-palette-enter-from, .cmd-palette-leave-to { opacity: 0; }
</style>
