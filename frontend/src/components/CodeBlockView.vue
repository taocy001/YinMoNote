<template>
  <node-view-wrapper
    :class="['code-block not-prose rounded-xl overflow-hidden border my-4',
             isDark ? 'bg-gray-900 border-gray-700' : 'bg-gray-50 border-gray-200']">
    <!-- Header bar -->
    <div :class="['flex items-center justify-between px-3 py-1.5 border-b text-xs',
                  isDark ? 'bg-gray-800 border-gray-700' : 'bg-gray-100 border-gray-200']">
      <select
        :value="node.attrs.language || ''"
        @change="setLanguage"
        :class="['bg-transparent outline-none cursor-pointer font-mono',
                 isDark ? 'text-gray-400' : 'text-gray-500']"
      >
        <option value="">{{ t.codeAutoDetect }}</option>
        <option v-for="lang in LANGUAGES" :key="lang" :value="lang">{{ lang }}</option>
      </select>
      <div class="flex items-center gap-2">
        <!-- Toggle code/diagram for mermaid -->
        <button
          v-if="language === 'mermaid'"
          @click="showMermaidCode = !showMermaidCode"
          :class="['px-2 py-0.5 rounded transition-colors',
                   isDark ? 'text-gray-500 hover:text-gray-300' : 'text-gray-400 hover:text-gray-600']"
        >
          {{ showMermaidCode ? t.codePreview : t.codeSource }}
        </button>
        <button
          @click="copyCode"
          :class="['px-2 py-0.5 rounded transition-colors',
                   copied
                     ? (isDark ? 'text-green-400' : 'text-green-600')
                     : (isDark ? 'text-gray-500 hover:text-gray-300' : 'text-gray-400 hover:text-gray-600')]"
        >
          {{ copied ? t.codeCopied : t.codeCopy }}
        </button>
      </div>
    </div>

    <!-- Math rendered output (block KaTeX) -->
    <div
      v-if="language === 'math'"
      :class="['px-6 py-4 overflow-x-auto text-center border-b',
               isDark ? 'bg-gray-900/50 border-gray-700' : 'bg-white border-gray-200']"
    >
      <span v-if="mathError" class="text-red-500 text-sm font-mono">{{ mathError }}</span>
      <span v-else ref="mathEl" class="katex-display-wrap" />
    </div>

    <!-- Mermaid rendered diagram -->
    <div
      v-if="language === 'mermaid' && !showMermaidCode"
      :class="['px-4 py-4 overflow-x-auto flex justify-center',
               isDark ? 'bg-gray-900/50' : 'bg-white']"
    >
      <div v-if="mermaidError" class="text-red-500 text-sm font-mono whitespace-pre">{{ mermaidError }}</div>
      <div v-else ref="mermaidEl" />
    </div>

    <!-- Code content (always shown for math, hidden for mermaid when in preview mode) -->
    <pre
      v-show="language !== 'mermaid' || showMermaidCode"
      :class="['!m-0 !rounded-none !border-0 p-4 overflow-x-auto text-sm leading-relaxed',
               isDark ? '!text-gray-300 !bg-[#1e293b]/50' : '!text-gray-800 !bg-transparent']"
    ><node-view-content as="code" :class="`language-${language || 'plaintext'}`" /></pre>
  </node-view-wrapper>
</template>

<script setup lang="ts">
/**
 * CodeBlockView.vue - Custom Tiptap node view for code blocks.
 * Supports: syntax highlighting, one-click copy, KaTeX math rendering, Mermaid diagrams.
 */
import { ref, inject, watch, onMounted, type Ref } from 'vue'
import { NodeViewWrapper, NodeViewContent, nodeViewProps } from '@tiptap/vue-3'
import { useI18n } from '../i18n'

const props = defineProps(nodeViewProps)
const isDark = inject<Ref<boolean>>('isDark', ref(false))
const { t } = useI18n()

const LANGUAGES = [
  'bash', 'c', 'cpp', 'css', 'diff', 'dockerfile',
  'go', 'html', 'java', 'javascript', 'json',
  'kotlin', 'markdown', 'math', 'mermaid', 'nginx',
  'python', 'rust', 'shell', 'sql', 'swift', 'typescript', 'xml', 'yaml',
]

const language = ref<string>(props.node.attrs.language ?? '')
const copied = ref(false)
const showMermaidCode = ref(false)

// Math
const mathEl = ref<HTMLElement>()
const mathError = ref('')

// Mermaid
const mermaidEl = ref<HTMLElement>()
const mermaidError = ref('')
/** Generates a unique element ID for each mermaid render call. */
const newMermaidId = () => `mermaid-${Math.random().toString(36).slice(2, 9)}`

const setLanguage = (e: Event) => {
  const val = (e.target as HTMLSelectElement).value
  language.value = val
  props.updateAttributes({ language: val || null })
}

const copyCode = async () => {
  try {
    await navigator.clipboard.writeText(props.node.textContent)
    copied.value = true
    setTimeout(() => { copied.value = false }, 2000)
  } catch {}
}

// ── KaTeX block math ──────────────────────────────────────────────────────────
const renderMath = async () => {
  if (language.value !== 'math' || !mathEl.value) return
  const formula = props.node.textContent.trim()
  const katex = (await import('katex')).default
  try {
    katex.render(formula, mathEl.value, { throwOnError: true, displayMode: true, output: 'html' })
    mathError.value = ''
  } catch (e) {
    mathError.value = e instanceof Error ? e.message : 'Invalid LaTeX'
  }
}

// ── Mermaid diagram ───────────────────────────────────────────────────────────
const renderMermaid = async () => {
  if (language.value !== 'mermaid' || showMermaidCode.value || !mermaidEl.value) return
  const code = props.node.textContent.trim()
  try {
    const mermaid = (await import('mermaid')).default
    // 'strict' mode renders via a sandboxed iframe and runs DOMPurify on the
    // output SVG before injecting it into the page. This allows removing
    // 'unsafe-eval' from the Content-Security-Policy.
    mermaid.initialize({
      startOnLoad: false,
      theme: isDark.value ? 'dark' : 'default',
      securityLevel: 'strict',
    })
    const id = newMermaidId()
    const { svg } = await mermaid.render(id, code)
    if (mermaidEl.value) {
      // Secondary sanitization: strip script/foreignObject/use elements from the
      // SVG before insertion to defend against Mermaid DOMPurify bypass vulnerabilities.
      const tmp = document.createElement('div')
      tmp.innerHTML = svg
      tmp.querySelectorAll('script, foreignObject, use').forEach(el => el.remove())
      Array.from(tmp.querySelectorAll('*')).forEach(el => {
        Array.from(el.attributes).forEach(attr => { if (/^on/i.test(attr.name)) el.removeAttribute(attr.name) })
      })
      mermaidEl.value.innerHTML = tmp.innerHTML
      mermaidError.value = ''
    }
  } catch (e) {
    mermaidError.value = e instanceof Error ? e.message : 'Invalid Mermaid syntax'
    if (mermaidEl.value) mermaidEl.value.innerHTML = ''
  }
}

onMounted(() => {
  renderMath()
  renderMermaid()
})

watch(() => props.node.textContent, () => {
  renderMath()
  renderMermaid()
})

watch(() => props.node.attrs.language, val => {
  language.value = val ?? ''
  renderMath()
  renderMermaid()
})

watch(isDark, () => { renderMermaid() })
watch(showMermaidCode, (v) => { if (!v) renderMermaid() })
</script>
