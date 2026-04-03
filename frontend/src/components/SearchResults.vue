<template>
  <div class="h-full flex flex-col" style="background: var(--bg-editor);">
    <!-- Header -->
    <div class="flex items-center justify-between px-5 py-3 shrink-0" style="border-bottom: 1px solid var(--border);">
      <div class="flex items-center gap-2">
        <svg width="15" height="15" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><circle cx="6" cy="6" r="4.5" stroke="currentColor" stroke-width="1.3"/><path d="M9.5 9.5L13 13" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
        <span class="text-sm font-semibold" style="color: var(--text-primary);">{{ t.searchResultsTitle }}</span>
      </div>
      <span class="text-xs tabular-nums" style="color: var(--text-muted);">
        {{ t.searchStats(noteCount, matchCount) }}
      </span>
    </div>

    <!-- Results -->
    <div class="flex-1 overflow-y-auto">
      <div v-if="results.length === 0" class="flex items-center justify-center h-full">
        <p class="text-sm" style="color: var(--text-muted);">{{ t.searchNoResults }}</p>
      </div>

      <div v-else class="py-2">
        <div v-for="item in results" :key="item.id" class="group">
          <!-- Note title row -->
          <button
            class="w-full text-left px-5 py-2.5 flex items-center gap-2 transition-all"
            :style="'color: var(--text-primary);'"
            @click="emit('open-note', item.id, item.snippets[0]?.offset ?? 0)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background = 'var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background = 'transparent'"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted); flex-shrink: 0;"><path d="M3 1.5h5.5L11 4v8.5H3z" stroke="currentColor" stroke-width="1.1" stroke-linejoin="round"/><path d="M8.5 1.5V4H11" stroke="currentColor" stroke-width="1.1" stroke-linejoin="round"/></svg>
            <span class="ts-sm font-semibold truncate">{{ item.title }}</span>
            <span
class="ts-xs tabular-nums shrink-0 px-1.5 py-0.5 rounded-full font-medium"
              style="background: var(--accent-light); color: var(--accent);">
              {{ item.snippets.length }}
            </span>
          </button>

          <!-- Snippets -->
          <div class="px-5 pb-3">
            <button
              v-for="(snippet, i) in item.snippets" :key="i"
              class="w-full text-left block px-3 py-2 rounded-lg ts-sm leading-relaxed transition-all mb-1"
              style="color: var(--text-secondary); border: 1px solid var(--border);"
              @click="emit('open-note', item.id, snippet.offset)"
              @mouseenter="e => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--accent)'; (e.currentTarget as HTMLElement).style.background = 'var(--bg-hover)' }"
              @mouseleave="e => { (e.currentTarget as HTMLElement).style.borderColor = 'var(--border)'; (e.currentTarget as HTMLElement).style.background = 'transparent' }"
            >
              <span v-html="snippet.html"></span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  query: string
  contentIndex: Map<string, string>
  noteTitles: Record<string, string>
  t: Record<string, any>
}>()

const emit = defineEmits<{
  'open-note': [id: string, charOffset: number]
}>()

/** Escape HTML special chars to prevent XSS in snippet rendering. */
function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

interface Snippet { html: string; offset: number }
interface SearchResult { id: string; title: string; snippets: Snippet[] }

const results = computed<SearchResult[]>(() => {
  const q = props.query.toLowerCase().trim()
  if (!q) return []
  const out: SearchResult[] = []
  const CONTEXT = 40 // chars before and after match
  const MAX_SNIPPETS = 5 // per note

  for (const [id, text] of props.contentIndex) {
    const lower = text.toLowerCase()
    const snippets: Snippet[] = []
    let pos = 0
    while (snippets.length < MAX_SNIPPETS) {
      const idx = lower.indexOf(q, pos)
      if (idx === -1) break
      const start = Math.max(0, idx - CONTEXT)
      const end = Math.min(text.length, idx + q.length + CONTEXT)
      const before = esc(text.slice(start, idx))
      const match = esc(text.slice(idx, idx + q.length))
      const after = esc(text.slice(idx + q.length, end))
      const prefix = start > 0 ? '…' : ''
      const suffix = end < text.length ? '…' : ''
      snippets.push({
        html: `${prefix}<span style="color:var(--text-muted)">${before}</span><mark style="background:rgba(234,179,8,0.25);color:var(--text-primary);font-weight:600;border-radius:2px;padding:0 1px">${match}</mark><span style="color:var(--text-muted)">${after}</span>${suffix}`,
        offset: idx
      })
      pos = idx + q.length
    }
    if (snippets.length > 0) {
      const title = props.noteTitles[id] || id
      out.push({ id, title, snippets })
    }
  }

  // Sort: title matches first, then by number of matches descending
  out.sort((a, b) => {
    const aTitle = a.title.toLowerCase().includes(q) ? 1 : 0
    const bTitle = b.title.toLowerCase().includes(q) ? 1 : 0
    if (aTitle !== bTitle) return bTitle - aTitle
    return b.snippets.length - a.snippets.length
  })

  return out
})

const noteCount = computed(() => results.value.length)
const matchCount = computed(() => results.value.reduce((sum, r) => sum + r.snippets.length, 0))
</script>
