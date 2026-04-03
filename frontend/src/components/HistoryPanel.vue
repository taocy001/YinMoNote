<template>
  <div
    v-if="show"
    class="shrink-0 border-l flex flex-col overflow-hidden transition-all"
    :style="{ width: '288px', borderColor: 'var(--border)', background: 'var(--bg-sidebar)' }"
  >
    <div class="px-4 py-2 border-b flex items-center justify-between shrink-0" style="border-color: var(--border);">
      <span class="text-sm font-bold" style="color: var(--text-primary);">{{ t.historyTitle }}</span>
      <button class="transition-colors" style="color: var(--text-muted);" @click="emit('close')">
        <svg width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      </button>
    </div>

    <!-- History list -->
    <div class="flex-1 overflow-y-auto">
      <div
v-for="(rev, idx) in historyList" :key="rev.hash" class="p-4 border-b transition-colors group" style="border-color: var(--border);"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">
        <div class="flex justify-between items-start mb-1 text-left">
          <span class="text-xs font-mono" style="color: var(--text-muted);">{{ rev.hash.slice(0, 7) }}</span>
          <span class="text-xs" style="color: var(--text-muted);">{{ rev.date }}</span>
        </div>
        <!-- Version label -->
        <div v-if="editingLabelHash === rev.hash" class="flex gap-1 mb-2">
          <input
v-model="editingLabelValue" :placeholder="t.labelPlaceholder" autofocus class="flex-1 px-2 py-1 text-xs rounded-md border outline-none" style="background: var(--bg-app); border-color: var(--border); color: var(--text-primary); font-family: inherit;"
            @keydown.enter="saveLabel" @keydown.esc="cancelEditLabel" />
          <button class="px-2 py-1 text-xs rounded-md" style="background: var(--accent); color: white;" @click="saveLabel">OK</button>
          <button class="px-1 py-1 text-xs rounded-md" style="color: var(--text-muted);" @click="cancelEditLabel">✕</button>
        </div>
        <div v-else-if="commitLabels[rev.hash]" class="flex items-center gap-1 mb-1.5">
          <span class="inline-flex items-center px-2 py-0.5 rounded-md ts-xs font-semibold cursor-pointer" style="background: var(--accent-light); color: var(--accent);" @click="startEditLabel(rev.hash)">🏷 {{ commitLabels[rev.hash] }}</span>
        </div>
        <p class="text-sm mb-1 break-all text-left" style="color: var(--text-secondary);">{{ rev.message }}</p>
        <!-- Add/edit label link -->
        <button v-if="editingLabelHash !== rev.hash && !commitLabels[rev.hash]" class="ts-xs mb-2 opacity-0 group-hover:opacity-100 transition-opacity" style="color: var(--text-muted);" @click="startEditLabel(rev.hash)">+ {{ t.addLabel }}</button>
        <div v-else class="mb-2"></div>
        <!-- Inline revert confirmation -->
        <div v-if="pendingRollbackHash === rev.hash" class="flex gap-2 items-center rounded-lg px-3 py-2 mb-1" style="background: var(--accent-light); border: 1px solid var(--accent);">
          <span class="text-xs flex-1" style="color: var(--text-primary);">{{ t.confirmRevert }}</span>
          <button class="px-2 py-1 text-xs rounded-lg history-revert-btn" @click="confirmRollback">{{ t.confirm }}</button>
          <button class="px-2 py-1 text-xs rounded-lg" style="background: var(--bg-hover); color: var(--text-secondary);" @click="cancelRollback">{{ t.libResetModalCancel }}</button>
        </div>
        <div v-else class="flex gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
          <button
            class="flex-1 py-1 text-xs rounded-lg transition-all history-diff-btn relative"
            :disabled="diffLoadingHash === rev.hash"
            @click="requestDiff(rev.hash, historyList[idx + 1]?.hash)"
          >
            <span v-if="diffLoadingHash === rev.hash" class="flex items-center justify-center gap-1">
              <span class="w-2.5 h-2.5 border border-t-transparent rounded-full animate-spin" style="border-color: var(--accent); border-top-color: transparent;"></span>
            </span>
            <span v-else>{{ t.diffView }}</span>
          </button>
          <button class="flex-1 py-1 text-xs rounded-lg transition-all history-revert-btn" @click="requestRollback(rev.hash)">{{ t.revertTo }}</button>
        </div>
      </div>
      <div v-if="rollbackError" class="text-xs px-4 py-2 mx-4 mb-2 rounded-lg" style="background: var(--color-danger-light); color: var(--color-danger);">{{ t.revertFailed }}</div>
      <div v-if="historyList.length === 0" class="text-xs p-8 text-center" style="color: var(--text-muted);">{{ t.noHistory }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import axios from 'axios'
import { decryptText } from '../crypto'
import { useI18n } from '../i18n'

const props = defineProps<{
  noteFileName: string
  show: boolean
  saveIfDirty: () => Promise<void>
  getFullContent: () => string
  commitLabels: Record<string, string>
}>()

const emit = defineEmits<{
  close: []
  'apply-rollback': [content: string]
  'load-error': [value: boolean]
  'show-diff': [payload: { newContent: string; oldContent: string }]
  'set-label': [hash: string, label: string]
}>()

const { t } = useI18n()
const API_BASE = '/api'

interface HistoryEntry { hash: string; message: string; author: string; date: string }

const historyList = ref<HistoryEntry[]>([])
const pendingRollbackHash = ref<string | null>(null)
const rollbackError = ref(false)
const diffLoadingHash = ref<string | null>(null)
const editingLabelHash = ref<string | null>(null)
const editingLabelValue = ref('')

const startEditLabel = (hash: string) => {
  editingLabelHash.value = hash
  editingLabelValue.value = props.commitLabels[hash] || ''
}
const saveLabel = () => {
  if (editingLabelHash.value) {
    emit('set-label', editingLabelHash.value, editingLabelValue.value)
  }
  editingLabelHash.value = null
}
const cancelEditLabel = () => { editingLabelHash.value = null }

watch(() => props.show, async (opened) => {
  if (!opened) { pendingRollbackHash.value = null; return }
  emit('load-error', false)
  try {
    historyList.value = (await axios.get(`${API_BASE}/notes/${props.noteFileName}/history`)).data.history ?? []
  } catch {
    emit('load-error', true)
    emit('close')
  }
})

const requestRollback = (h: string) => { pendingRollbackHash.value = h }
const cancelRollback = () => { pendingRollbackHash.value = null }

const confirmRollback = async () => {
  const h = pendingRollbackHash.value
  if (!h) return
  pendingRollbackHash.value = null
  rollbackError.value = false
  try {
    await props.saveIfDirty()
    const res = await axios.post(`${API_BASE}/notes/${props.noteFileName}/rollback`, { hash: h })
    const content = await decryptText(res.data.content)
    emit('apply-rollback', content)
    emit('close')
  } catch {
    rollbackError.value = true
  }
}

/**
 * Fetches both versions and emits show-diff so the editor area renders
 * the inline diff view (Feishu-style) instead of a sidebar code-diff.
 */
const requestDiff = async (hash: string, prevHash?: string) => {
  diffLoadingHash.value = hash
  try {
    const newRes = await axios.get(`${API_BASE}/notes/${props.noteFileName}/version/${hash}`)
    const newContent = await decryptText(newRes.data.content)
    let oldContent = ''
    if (prevHash) {
      const oldRes = await axios.get(`${API_BASE}/notes/${props.noteFileName}/version/${prevHash}`)
      oldContent = await decryptText(oldRes.data.content)
    } else {
      oldContent = props.getFullContent()
    }
    emit('show-diff', { newContent, oldContent })
  } catch {
    // silently ignore — user can retry
  } finally {
    diffLoadingHash.value = null
  }
}
</script>

<style>
.history-diff-btn { background: var(--accent-light); color: var(--accent); }
.history-diff-btn:hover:not(:disabled) { background: var(--accent); color: white; }
.history-diff-btn:disabled { opacity: 0.6; cursor: default; }
.history-revert-btn { background: rgba(239,68,68,0.1); color: var(--color-danger); }
.history-revert-btn:hover { background: var(--color-danger); color: white; }
</style>
