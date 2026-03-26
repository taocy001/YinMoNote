/**
 * TE-004: Unit tests for useDragDrop composable (sidebar resize logic).
 *
 * Covers:
 * - Initial state
 * - startDrag / stopDrag toggle isDragging
 * - onDrag updates sidebarWidth to clientX when dragging
 * - onDrag does not update when not dragging
 * - sidebarWidth is clamped to minWidth
 * - Custom minWidth constructor argument is respected
 */
import { describe, it, expect } from 'vitest'
import { useDragDrop } from '../src/composables/useDragDrop'

function mouseEvent(clientX: number): MouseEvent {
  return { clientX } as MouseEvent
}

describe('useDragDrop', () => {
  it('initializes with isDragging=false and sidebarWidth=220', () => {
    const { isDragging, sidebarWidth } = useDragDrop()
    expect(isDragging.value).toBe(false)
    expect(sidebarWidth.value).toBe(220)
  })

  it('startDrag sets isDragging to true', () => {
    const { isDragging, startDrag } = useDragDrop()
    startDrag()
    expect(isDragging.value).toBe(true)
  })

  it('stopDrag sets isDragging to false', () => {
    const { isDragging, startDrag, stopDrag } = useDragDrop()
    startDrag()
    stopDrag()
    expect(isDragging.value).toBe(false)
  })

  it('onDrag updates sidebarWidth to clientX while dragging', () => {
    const { sidebarWidth, startDrag, onDrag } = useDragDrop()
    startDrag()
    onDrag(mouseEvent(350))
    expect(sidebarWidth.value).toBe(350)
  })

  it('onDrag does nothing when not dragging', () => {
    const { sidebarWidth, onDrag } = useDragDrop()
    onDrag(mouseEvent(350))
    expect(sidebarWidth.value).toBe(220) // unchanged
  })

  it('sidebarWidth is clamped to default minWidth (160)', () => {
    const { sidebarWidth, startDrag, onDrag } = useDragDrop()
    startDrag()
    onDrag(mouseEvent(50)) // below 160
    expect(sidebarWidth.value).toBe(160)
  })

  it('sidebarWidth is clamped to custom minWidth', () => {
    const { sidebarWidth, startDrag, onDrag } = useDragDrop(200)
    startDrag()
    onDrag(mouseEvent(100)) // below 200
    expect(sidebarWidth.value).toBe(200)
  })

  it('sidebarWidth at exactly minWidth is accepted', () => {
    const { sidebarWidth, startDrag, onDrag } = useDragDrop()
    startDrag()
    onDrag(mouseEvent(160))
    expect(sidebarWidth.value).toBe(160)
  })

  it('multiple drag events update correctly', () => {
    const { sidebarWidth, startDrag, onDrag } = useDragDrop()
    startDrag()
    onDrag(mouseEvent(300))
    expect(sidebarWidth.value).toBe(300)
    onDrag(mouseEvent(400))
    expect(sidebarWidth.value).toBe(400)
    onDrag(mouseEvent(180))
    expect(sidebarWidth.value).toBe(180)
  })
})
