<template>
  <node-view-wrapper as="span" class="inline align-baseline">
    <span
      ref="el"
      :class="[
        'math-inline cursor-default select-none rounded px-0.5 transition-shadow',
        selected ? 'ring-2 ring-blue-400' : '',
        error ? 'text-red-500 font-mono text-sm' : '',
      ]"
    />
  </node-view-wrapper>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'

const props = defineProps(nodeViewProps)
const el = ref<HTMLElement>()
const error = ref(false)

/**
 * Renders the LaTeX formula into the DOM span using KaTeX.
 * Falls back to raw `$formula$` text when the formula is invalid,
 * and sets `error` so the template can apply an error style.
 */
const render = async () => {
  if (!el.value) return
  const katex = (await import('katex')).default
  try {
    katex.render(props.node.attrs.formula || '', el.value, {
      throwOnError: true,
      displayMode: false,
      output: 'html',
    })
    error.value = false
  } catch {
    el.value.textContent = `$${props.node.attrs.formula}$`
    error.value = true
  }
}

onMounted(render)
watch(() => props.node.attrs.formula, render)
</script>
