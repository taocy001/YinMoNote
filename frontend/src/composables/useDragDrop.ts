/**
 * Sidebar drag-to-resize logic.
 */
import { ref } from 'vue'

export function useDragDrop(minWidth = 160) {
  const isDragging = ref(false)
  const sidebarWidth = ref(220)

  const onDrag = (e: MouseEvent) => {
    if (isDragging.value) sidebarWidth.value = Math.max(minWidth, e.clientX)
  }
  const startDrag = () => { isDragging.value = true }
  const stopDrag = () => { isDragging.value = false }

  return { isDragging, sidebarWidth, onDrag, startDrag, stopDrag }
}
