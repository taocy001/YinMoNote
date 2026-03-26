<template>
  <node-view-wrapper
    ref="container"
    as="span"
    :class="[
      'relative inline-block max-w-full min-h-[100px] min-w-[100px] transition-all duration-500',
      selected ? 'ring-2 ring-blue-500 ring-offset-1 rounded' : '',
      !isIntersecting ? 'bg-gray-100 dark:bg-gray-800 animate-pulse rounded' : ''
    ]"
  >
    <!-- Placeholder while waiting for scroll into view -->
    <div v-if="!isIntersecting" class="flex items-center justify-center h-full text-gray-400">
       <span class="ts-xs font-bold uppercase tracking-widest">Lazy Loading...</span>
    </div>

    <!-- Image Display -->
    <template v-else>
      <div v-if="loading" class="flex items-center justify-center p-4 bg-gray-100 dark:bg-gray-800 rounded animate-pulse">
        <div class="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin"></div>
      </div>

      <img
        v-else
        ref="imgEl"
        :src="displaySrc"
        :alt="node.attrs.alt || ''"
        :style="imgStyle"
        class="inline-block max-w-full rounded select-none shadow-sm transition-opacity duration-300"
        :class="displaySrc ? 'opacity-100' : 'opacity-0'"
        draggable="false"
        @load="onImgLoad"
      />
    </template>

    <!-- Resize Handle -->
    <span
      v-if="selected && !loading && isIntersecting"
      class="absolute bottom-0 right-0 w-4 h-4 bg-blue-500 rounded-tl-md cursor-se-resize z-10 opacity-80 hover:opacity-100"
      @mousedown.prevent.stop="startResize"
    ></span>

    <span
      v-if="resizing"
      class="absolute top-1 left-1 text-xs bg-black/60 text-white rounded px-1.5 py-0.5 pointer-events-none"
    >
      {{ currentWidth }}px
    </span>
  </node-view-wrapper>
</template>

<script setup lang="ts">
/**
 * ImageView.vue (Optimized) - Features Lazy Decryption & Memory Safety.
 * Prevents UI lag when opening notes with many high-res encrypted images.
 */
import { ref, computed, onBeforeUnmount, onMounted, nextTick, watch, inject, type Ref } from 'vue'
import { NodeViewWrapper, nodeViewProps } from '@tiptap/vue-3'
import { useImageDecrypt } from '../composables/useImageDecrypt'

const props = defineProps(nodeViewProps)
const isLibraryLocked = inject<Ref<boolean>>('isLibraryLocked', ref(true))
const serverEncrypt = inject<Ref<boolean>>('serverEncrypt', ref(true))
const allowExternalImages = inject<Ref<boolean>>('allowExternalImages', ref(false))

const container = ref<HTMLElement>()
const imgEl = ref<HTMLImageElement>()
const resizing = ref(false)
const currentWidth = ref(0)
const isIntersecting = ref(false)
// Natural display width computed from naturalWidth/devicePixelRatio after load.
// Only used when the node has no explicit width attr (i.e. not yet manually resized).
const naturalDisplayWidth = ref<number | null>(null)

const { loading, displaySrc, fetchAndDecrypt, cleanup } = useImageDecrypt()

let startX = 0, startW = 0
let observer: IntersectionObserver | null = null

const imgStyle = computed(() => {
  const w = props.node.attrs.width ?? naturalDisplayWidth.value
  return w ? { width: w + 'px', maxWidth: '100%' } : { maxWidth: '100%' }
})

// Called when the <img> element fires its load event.
// Reads the true pixel dimensions and converts to CSS pixels via devicePixelRatio
// so that a Retina 2× screenshot doesn't appear at double its visual size.
// Also persists the computed width back to node attrs so that page refresh
// renders the image at the correct size immediately (no layout jump).
const onImgLoad = () => {
  if (!imgEl.value) return
  // Skip error/lock placeholder SVGs — they should not dictate image width.
  if (displaySrc.value.startsWith('data:image/svg+xml')) return
  const dpr = window.devicePixelRatio || 1
  const w = Math.round(imgEl.value.naturalWidth / dpr)
  naturalDisplayWidth.value = w
  // Persist to node attrs only when not yet manually resized, so the width
  // survives a page refresh without causing a layout jump.
  if (!props.node.attrs.width) {
    props.updateAttributes({ width: w })
  }
}

/** Triggers image fetch+decrypt. Only called when image has scrolled into view. */
const fetchAndDecryptImage = async () => {
  if (!isIntersecting.value) return
  naturalDisplayWidth.value = null
  await fetchAndDecrypt(props.node.attrs.src, isLibraryLocked.value, allowExternalImages.value)
}

onMounted(async () => {
  await nextTick()
  // In Vue 3, ref on a component returns the proxy, not a DOM element.
  // Use $el to get the actual underlying DOM element for IntersectionObserver.
  const el = (container.value as any)?.$el ?? container.value
  if (!el || !(el instanceof Element)) return

  observer = new IntersectionObserver((entries) => {
    if (entries[0].isIntersecting && !isIntersecting.value) {
      isIntersecting.value = true
      fetchAndDecryptImage()
      // Once loaded, we can stop observing this specific instance.
      observer?.disconnect()
    }
  }, { rootMargin: '400px' }) // Start loading 400px before it enters viewport

  observer.observe(el)
})

watch(() => props.node.attrs.src, () => {
  if (isIntersecting.value) fetchAndDecryptImage()
})

watch(isLibraryLocked, locked => {
  if (!locked && isIntersecting.value) fetchAndDecryptImage()
})

watch(serverEncrypt, () => {
  if (isIntersecting.value) fetchAndDecryptImage()
})

/**
 * Initiates the image resizing process.
 * @param {MouseEvent} e - The mouse down event.
 */
const startResize = (e: MouseEvent) => {
  resizing.value = true
  startX = e.clientX
  startW = imgEl.value?.offsetWidth ?? props.node.attrs.width ?? 400
  currentWidth.value = startW
  window.addEventListener('mousemove', onResize)
  window.addEventListener('mouseup', stopResize)
}

/**
 * Handles the mouse movement during resizing.
 * @param {MouseEvent} e - The mouse move event.
 */
const onResize = (e: MouseEvent) => {
  if (!resizing.value) return
  const w = Math.max(80, Math.round(startW + (e.clientX - startX)))
  currentWidth.value = w
  props.updateAttributes({ width: w })
}

/**
 * Finalizes the resizing process and cleans up event listeners.
 */
const stopResize = () => {
  resizing.value = false
  window.removeEventListener('mousemove', onResize)
  window.removeEventListener('mouseup', stopResize)
}

onBeforeUnmount(() => {
  observer?.disconnect()
  window.removeEventListener('mousemove', onResize)
  window.removeEventListener('mouseup', stopResize)
  cleanup()
})
</script>
