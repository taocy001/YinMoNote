<template>
  <button
    :data-testid="testId"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="label"
    @click="emit('update:modelValue', !modelValue)"
    class="w-11 h-6 rounded-full transition-colors relative shrink-0 focus-ring"
    :style="modelValue ? 'background: var(--accent);' : 'background: var(--border-strong);'"
  >
    <div
      class="absolute top-1 w-4 h-4 bg-white rounded-full transition-all shadow-sm"
      :style="modelValue ? 'left: calc(100% - 20px);' : 'left: 4px;'"
    ></div>
  </button>
</template>

<script setup lang="ts">
defineProps<{
  modelValue: boolean
  testId?: string
  label?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
}>()
</script>
