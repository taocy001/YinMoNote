<template>
  <Teleport to="body">
    <!-- ── Table corner menu button (top-left of table) ──────────────────── -->
    <div
      v-if="tableRect"
      data-table-overlay
      class="fixed z-[78] select-none"
      :style="{ top: (tableRect.top - 4) + 'px', left: (tableRect.left - 4) + 'px', transform: 'translate(-100%, -100%)' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <button
        class="w-6 h-6 flex items-center justify-center rounded-md transition-colors"
        :style="tableMenuVisible
          ? 'background: var(--accent-light); color: var(--accent);'
          : 'background: var(--bg-editor); color: var(--text-muted); border: 1px solid var(--border);'"
        style="box-shadow: var(--shadow-sm);"
        @click.stop="toggleTableMenu($event)"
        @mouseenter="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><TableProperties :size="12" /></button>
    </div>

    <!-- ── Table menu dropdown ─────────────────────────────────────────────── -->
    <div
      v-if="tableMenuVisible && tableRect"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="tableMenuDropdownStyle"
      style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg); min-width: 160px;"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <div
        class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
        style="color: var(--color-danger);"
        @click="runCmd('deleteTable')"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      ><Trash2 :size="13" class="shrink-0" /><span>{{ t.tableDeleteTable }}</span></div>
    </div>

    <!-- ── Row handle (left of hovered row) ──────────────────────────────── -->
    <div
      v-if="tableRect && hoveredRowRect && !insertRowY"
      data-table-overlay
      class="fixed z-[78] select-none"
      :style="{ top: (hoveredRowRect.top + hoveredRowRect.height / 2) + 'px', left: (tableRect.left - 4) + 'px', transform: 'translate(-100%, -50%)' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <button
        class="w-6 h-6 flex items-center justify-center rounded-md transition-colors"
        :style="rowMenuVisible
          ? 'background: var(--accent-light); color: var(--accent);'
          : 'background: var(--bg-editor); color: var(--text-muted); border: 1px solid var(--border);'"
        style="box-shadow: var(--shadow-sm);"
        @click.stop="toggleRowMenu($event)"
        @mouseenter="e => { if (!rowMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (!rowMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><MoreHorizontal :size="12" /></button>
    </div>

    <!-- ── Row menu dropdown ───────────────────────────────────────────────── -->
    <div
      v-if="rowMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: rowMenuPos.top + 'px', left: rowMenuPos.left + 'px' }"
      style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg); min-width: 164px;"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <div
        v-for="item in rowMenuItems"
        :key="item.key"
        class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
        :style="item.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
        @click="item.action()"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      ><component :is="item.icon" :size="13" class="shrink-0" /><span>{{ item.label }}</span></div>
    </div>

    <!-- ── Insert-row handle (on row top border, left edge) ──────────────── -->
    <div
      v-if="tableRect && insertRowY !== null"
      data-table-overlay
      class="fixed z-[78] flex items-center cursor-pointer group select-none"
      :style="{ top: (insertRowY - 8) + 'px', left: (tableRect.left - 12) + 'px', height: '16px', width: (tableRect.width + 16) + 'px' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
      @click.stop="insertRowAtBorder"
    >
      <!-- Dot -->
      <div
        class="w-3 h-3 rounded-full shrink-0 flex items-center justify-center transition-all group-hover:scale-110"
        style="background: var(--accent);"
      ><Plus :size="8" style="color: white;" /></div>
      <!-- Dashed line -->
      <div class="flex-1 opacity-0 group-hover:opacity-100 transition-opacity" style="height: 2px; background: var(--accent); border-radius: 1px; margin-left: 2px;"></div>
    </div>

    <!-- ── Column handle (above hovered column) ──────────────────────────── -->
    <div
      v-if="tableRect && hoveredCellRect && !insertColX"
      data-table-overlay
      class="fixed z-[78] select-none"
      :style="{ top: (tableRect.top - 4) + 'px', left: (hoveredCellRect.left + hoveredCellRect.width / 2) + 'px', transform: 'translate(-50%, -100%)' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <button
        class="w-6 h-6 flex items-center justify-center rounded-md transition-colors"
        :style="colMenuVisible
          ? 'background: var(--accent-light); color: var(--accent);'
          : 'background: var(--bg-editor); color: var(--text-muted); border: 1px solid var(--border);'"
        style="box-shadow: var(--shadow-sm);"
        @click.stop="toggleColMenu($event)"
        @mouseenter="e => { if (!colMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (!colMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><MoreVertical :size="12" /></button>
    </div>

    <!-- ── Column menu dropdown ────────────────────────────────────────────── -->
    <div
      v-if="colMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="{ top: colMenuPos.top + 'px', left: colMenuPos.left + 'px' }"
      style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg); min-width: 164px;"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <div
        v-for="item in colMenuItems"
        :key="item.key"
        class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
        :style="item.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
        @click="item.action()"
        @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
      ><component :is="item.icon" :size="13" class="shrink-0" /><span>{{ item.label }}</span></div>
    </div>

    <!-- ── Insert-column handle (on column left border, top edge) ────────── -->
    <div
      v-if="tableRect && insertColX !== null"
      data-table-overlay
      class="fixed z-[78] flex flex-col items-center cursor-pointer group select-none"
      :style="{ left: (insertColX - 8) + 'px', top: (tableRect.top - 12) + 'px', width: '16px', height: (tableRect.height + 16) + 'px' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
      @click.stop="insertColAtBorder"
    >
      <!-- Dot -->
      <div
        class="w-3 h-3 rounded-full shrink-0 flex items-center justify-center transition-all group-hover:scale-110"
        style="background: var(--accent);"
      ><Plus :size="8" style="color: white;" /></div>
      <!-- Dashed line -->
      <div class="flex-1 opacity-0 group-hover:opacity-100 transition-opacity" style="width: 2px; background: var(--accent); border-radius: 1px; margin-top: 2px;"></div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { Component } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import {
  TableProperties, MoreHorizontal, MoreVertical, Trash2, Plus,
  ArrowUpToLine, ArrowDownToLine, ArrowLeftToLine, ArrowRightToLine,
} from 'lucide-vue-next'

const props = defineProps<{
  editor: TiptapEditor | undefined
  t: Record<string, any>
}>()

// ── Hover state ───────────────────────────────────────────────────────────
const tableRect        = ref<DOMRect | null>(null)
const hoveredRowEl     = ref<HTMLTableRowElement | null>(null)
const hoveredRowRect   = ref<DOMRect | null>(null)
const hoveredCellEl    = ref<HTMLTableCellElement | null>(null)
const hoveredCellRect  = ref<DOMRect | null>(null)

// Insert-between state
const insertRowY           = ref<number | null>(null)
const insertRowBeforeIdx   = ref(0)
const insertColX           = ref<number | null>(null)
const insertColTargetCell  = ref<HTMLTableCellElement | null>(null)
const insertColIsAfter     = ref(false)  // true when clicking right border of last column

// ── Menu state ────────────────────────────────────────────────────────────
const tableMenuVisible = ref(false)
const tableMenuPos     = ref({ top: 0, left: 0 })
const rowMenuVisible   = ref(false)
const rowMenuPos       = ref({ top: 0, left: 0 })
const rowMenuTargetRow = ref<HTMLTableRowElement | null>(null)
const colMenuVisible   = ref(false)
const colMenuPos       = ref({ top: 0, left: 0 })
const colMenuTargetCell = ref<HTMLTableCellElement | null>(null)

let hideTimer: ReturnType<typeof setTimeout> | null = null

// ── Menu items ────────────────────────────────────────────────────────────
interface MenuItem { key: string; label: string; icon: Component; danger?: boolean; action: () => void }

const rowMenuItems = computed<MenuItem[]>(() => [
  { key: 'row-above', label: props.t.tableInsertRowAbove, icon: ArrowUpToLine,   action: () => runRowCmd('addRowBefore') },
  { key: 'row-below', label: props.t.tableInsertRowBelow, icon: ArrowDownToLine, action: () => runRowCmd('addRowAfter')  },
  { key: 'row-del',   label: props.t.tableDeleteRow,      icon: Trash2,          danger: true, action: () => runRowCmd('deleteRow') },
])

const colMenuItems = computed<MenuItem[]>(() => [
  { key: 'col-left',  label: props.t.tableInsertColLeft,  icon: ArrowLeftToLine,  action: () => runColCmd('addColumnBefore') },
  { key: 'col-right', label: props.t.tableInsertColRight, icon: ArrowRightToLine, action: () => runColCmd('addColumnAfter')  },
  { key: 'col-del',   label: props.t.tableDeleteCol,      icon: Trash2,           danger: true, action: () => runColCmd('deleteColumn') },
])

// ── Dropdown positions ────────────────────────────────────────────────────
const tableMenuDropdownStyle = computed(() => {
  if (!tableRect.value) return {}
  const left = tableRect.value.left - 4
  // Anchor below the corner button (which is at top - 4, shifted -100% up)
  return { top: (tableRect.value.top - 4 - 24 + 4) + 'px', left: (left - 4) + 'px', transform: 'translateX(-100%)' }
})

// ── Command helpers ───────────────────────────────────────────────────────
const focusCell = (cellEl: HTMLElement): boolean => {
  if (!props.editor) return false
  try {
    const pos = props.editor.view.posAtDOM(cellEl, 0) + 1
    props.editor.chain().focus().setTextSelection(pos).run()
    return true
  } catch (_) { return false }
}

const runCmd = (cmd: string) => {
  if (!props.editor) return
  ;(props.editor.chain().focus() as any)[cmd]().run()
  closeAllMenus()
}

const runRowCmd = (cmd: string) => {
  const row = rowMenuTargetRow.value
  if (!row) return
  const cell = row.querySelector('td, th') as HTMLElement | null
  if (cell && focusCell(cell)) {
    ;(props.editor?.chain().focus() as any)[cmd]().run()
  }
  closeAllMenus()
}

const runColCmd = (cmd: string) => {
  const cell = colMenuTargetCell.value
  if (!cell) return
  if (focusCell(cell)) {
    ;(props.editor?.chain().focus() as any)[cmd]().run()
  }
  closeAllMenus()
}

const insertRowAtBorder = () => {
  if (!props.editor) return
  const table = props.editor.view.dom.querySelector('table') as HTMLTableElement | null
  if (!table) return
  const rows = Array.from(table.querySelectorAll('tr')) as HTMLTableRowElement[]
  const idx = insertRowBeforeIdx.value

  const targetRow = rows[Math.min(idx, rows.length - 1)]
  const cell = targetRow?.querySelector('td, th') as HTMLElement | null
  if (!cell) return
  focusCell(cell)

  if (idx === 0) {
    props.editor.chain().focus().addRowBefore().run()
  } else if (idx >= rows.length) {
    const lastCell = rows[rows.length - 1].querySelector('td, th') as HTMLElement | null
    if (lastCell) { focusCell(lastCell); props.editor.chain().focus().addRowAfter().run() }
  } else {
    // Insert before row[idx] = insert after row[idx-1]
    const prevCell = rows[idx - 1]?.querySelector('td, th') as HTMLElement | null
    if (prevCell) { focusCell(prevCell); props.editor.chain().focus().addRowAfter().run() }
  }
  closeAllMenus()
}

const insertColAtBorder = () => {
  const cell = insertColTargetCell.value
  if (!cell || !props.editor) return
  focusCell(cell)
  if (insertColIsAfter.value) {
    props.editor.chain().focus().addColumnAfter().run()
  } else {
    props.editor.chain().focus().addColumnBefore().run()
  }
  closeAllMenus()
}

// ── Menu toggle helpers ───────────────────────────────────────────────────
const closeAllMenus = () => {
  tableMenuVisible.value = false
  rowMenuVisible.value   = false
  colMenuVisible.value   = false
}

const toggleTableMenu = (e: MouseEvent) => {
  if (tableMenuVisible.value) { tableMenuVisible.value = false; return }
  const btn = (e.currentTarget as HTMLElement).getBoundingClientRect()
  tableMenuPos.value = { top: btn.bottom + 4, left: btn.left }
  rowMenuVisible.value = colMenuVisible.value = false
  tableMenuVisible.value = true
}

const toggleRowMenu = (e: MouseEvent) => {
  if (rowMenuVisible.value) { rowMenuVisible.value = false; return }
  rowMenuTargetRow.value = hoveredRowEl.value
  const btn = (e.currentTarget as HTMLElement).getBoundingClientRect()
  rowMenuPos.value = { top: btn.bottom + 4, left: btn.left }
  tableMenuVisible.value = colMenuVisible.value = false
  rowMenuVisible.value = true
}

const toggleColMenu = (e: MouseEvent) => {
  if (colMenuVisible.value) { colMenuVisible.value = false; return }
  colMenuTargetCell.value = hoveredCellEl.value
  const btn = (e.currentTarget as HTMLElement).getBoundingClientRect()
  colMenuPos.value = { top: btn.bottom + 4, left: btn.left }
  tableMenuVisible.value = rowMenuVisible.value = false
  colMenuVisible.value = true
}

// ── Show / hide timer ─────────────────────────────────────────────────────
const keepVisible = () => { if (hideTimer) clearTimeout(hideTimer) }

const scheduleHide = () => {
  hideTimer = setTimeout(() => {
    if (!tableMenuVisible.value && !rowMenuVisible.value && !colMenuVisible.value) {
      tableRect.value       = null
      hoveredRowEl.value    = null
      hoveredRowRect.value  = null
      hoveredCellEl.value   = null
      hoveredCellRect.value = null
      insertRowY.value      = null
      insertColX.value      = null
    }
  }, 200)
}

// ── Mouse tracking ────────────────────────────────────────────────────────
const SNAP_PX = 7 // px distance to row/col border to show insert handle

const onMouseMove = (e: MouseEvent) => {
  const target = e.target as HTMLElement

  // Mouse is over one of our overlay controls — keep everything visible
  if (target.closest('[data-table-overlay]')) {
    keepVisible()
    return
  }

  const table = target.closest<HTMLTableElement>('table')
  if (!table) {
    scheduleHide()
    return
  }

  keepVisible()
  tableRect.value = table.getBoundingClientRect()

  const mouseY = e.clientY
  const mouseX = e.clientX

  // ── Row border detection ──────────────────────────────────────────────
  const rows = Array.from(table.querySelectorAll('tr')) as HTMLTableRowElement[]
  let nearRow: { y: number; idx: number } | null = null

  for (let i = 0; i < rows.length; i++) {
    const r = rows[i].getBoundingClientRect()
    if (Math.abs(mouseY - r.top) <= SNAP_PX) {
      nearRow = { y: r.top, idx: i }; break
    }
    if (i === rows.length - 1 && Math.abs(mouseY - r.bottom) <= SNAP_PX) {
      nearRow = { y: r.bottom, idx: rows.length }; break
    }
  }

  if (nearRow) {
    insertRowY.value       = nearRow.y
    insertRowBeforeIdx.value = nearRow.idx
    hoveredRowEl.value     = null
    hoveredRowRect.value   = null
  } else {
    insertRowY.value = null
    const row = target.closest<HTMLTableRowElement>('tr')
    hoveredRowEl.value   = row
    hoveredRowRect.value = row ? row.getBoundingClientRect() : null
  }

  // ── Column border detection ───────────────────────────────────────────
  const firstRow = rows[0]
  const cells = firstRow
    ? (Array.from(firstRow.querySelectorAll('td, th')) as HTMLTableCellElement[])
    : []
  let nearCol: { x: number; cell: HTMLTableCellElement; isAfter: boolean } | null = null

  for (let j = 0; j < cells.length; j++) {
    const r = cells[j].getBoundingClientRect()
    if (Math.abs(mouseX - r.left) <= SNAP_PX) {
      nearCol = { x: r.left, cell: cells[j], isAfter: false }; break
    }
    if (j === cells.length - 1 && Math.abs(mouseX - r.right) <= SNAP_PX) {
      nearCol = { x: r.right, cell: cells[j], isAfter: true }; break
    }
  }

  if (nearCol) {
    insertColX.value          = nearCol.x
    insertColTargetCell.value = nearCol.cell
    insertColIsAfter.value    = nearCol.isAfter
    hoveredCellEl.value       = null
    hoveredCellRect.value     = null
  } else {
    insertColX.value = null
    const cell = target.closest<HTMLTableCellElement>('td, th')
    // For column handle, use the matching cell in the first row (same column index)
    if (cell && firstRow) {
      const rowCells = Array.from(cell.closest('tr')!.querySelectorAll('td, th'))
      const colIdx   = rowCells.indexOf(cell)
      hoveredCellEl.value   = (colIdx >= 0 ? cells[colIdx] : cell) ?? cell
      hoveredCellRect.value = hoveredCellEl.value.getBoundingClientRect()
    } else {
      hoveredCellEl.value   = null
      hoveredCellRect.value = null
    }
  }
}

const onDocumentClick = (e: MouseEvent) => {
  if (!(e.target as HTMLElement).closest('[data-table-overlay]')) closeAllMenus()
}

onMounted(() => {
  document.addEventListener('mousemove', onMouseMove, { passive: true })
  document.addEventListener('click', onDocumentClick)
})
onUnmounted(() => {
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('click', onDocumentClick)
  if (hideTimer) clearTimeout(hideTimer)
})
</script>
