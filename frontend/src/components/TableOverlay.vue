<template>
  <Teleport to="body">
    <!-- ── Row selector bars (thin pill, left of each row) ──────────────── -->
    <template v-if="tableRect">
      <div
        v-for="row in rowInfos"
        :key="`rb${row.index}`"
        data-table-overlay
        class="fixed z-[77] cursor-pointer rounded-sm transition-colors"
        :style="{
          top:       (row.rect.top    + 4) + 'px',
          left:      (tableRect.left  - 8) + 'px',
          width:     '5px',
          height:    Math.max(14, row.rect.height - 8) + 'px',
          transform: 'translateX(-100%)',
          background: activeRowIdx === row.index ? 'var(--accent)' : 'color-mix(in srgb, var(--accent) 35%, transparent)',
        }"
        @mouseenter="activeRowIdx = row.index; keepVisible()"
        @mouseleave="activeRowIdx = -1; scheduleHide()"
        @click.stop="onRowBarClick($event, row)"
      />
    </template>

    <!-- ── Column selector bars (thin pill, above each column) ─────────── -->
    <template v-if="tableRect">
      <div
        v-for="col in colInfos"
        :key="`cb${col.index}`"
        data-table-overlay
        class="fixed z-[77] cursor-pointer rounded-sm transition-colors"
        :style="{
          left:      (col.rect.left + 4) + 'px',
          top:       (tableRect.top - 8) + 'px',
          height:    '5px',
          width:     Math.max(14, col.rect.width - 8) + 'px',
          transform: 'translateY(-100%)',
          background: activeColIdx === col.index ? 'var(--accent)' : 'color-mix(in srgb, var(--accent) 35%, transparent)',
        }"
        @mouseenter="activeColIdx = col.index; keepVisible()"
        @mouseleave="activeColIdx = -1; scheduleHide()"
        @click.stop="onColBarClick($event, col)"
      />
    </template>

    <!-- ── Table corner button (⊞ top-left outside table) ──────────────── -->
    <div
      v-if="tableRect"
      data-table-overlay
      class="fixed z-[78] select-none"
      :style="{ top: (tableRect.top - 6) + 'px', left: (tableRect.left - 6) + 'px', transform: 'translate(-100%, -100%)' }"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <button
        class="w-5 h-5 flex items-center justify-center rounded transition-colors"
        :style="tableMenuVisible
          ? 'background: var(--accent-light); color: var(--accent); border: 1px solid var(--accent);'
          : 'background: var(--bg-editor); color: var(--text-muted); border: 1px solid var(--border);'"
        style="box-shadow: var(--shadow-sm);"
        @click.stop="toggleMenu('table', $event)"
        @mouseenter="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><TableProperties :size="11" /></button>
    </div>

    <!-- ── Table menu ──────────────────────────────────────────────────── -->
    <div
      v-if="tableMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="popupStyle(tableMenuPos)"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <template v-for="item in tableMenuItems" :key="item.key">
        <div v-if="item.separator" class="my-1 mx-2" style="height:1px; background: var(--border);"></div>
        <div
          v-else
          class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
          :style="item.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
          @click="item.action?.()"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >
          <component :is="item.icon" :size="13" class="shrink-0" />
          <span>{{ item.label }}</span>
        </div>
      </template>
    </div>

    <!-- ── Row menu ────────────────────────────────────────────────────── -->
    <div
      v-if="rowMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="popupStyle(rowMenuPos)"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <template v-for="item in rowMenuItems" :key="item.key">
        <div v-if="item.separator" class="my-1 mx-2" style="height:1px; background: var(--border);"></div>
        <!-- Inline alignment buttons row -->
        <div v-else-if="item.alignRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.alignLabel }}</span>
          <button
            v-for="a in alignOptions" :key="a.value"
            class="flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyAlignToRow(a.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="a.icon" :size="13" /></button>
        </div>
        <div
          v-else
          class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
          :style="item.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
          @click="item.action?.()"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >
          <component :is="item.icon" :size="13" class="shrink-0" />
          <span>{{ item.label }}</span>
        </div>
      </template>
    </div>

    <!-- ── Column menu ─────────────────────────────────────────────────── -->
    <div
      v-if="colMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-hidden rounded-xl anim-pop-in"
      :style="popupStyle(colMenuPos)"
      @mouseenter="keepVisible"
      @mouseleave="scheduleHide"
    >
      <template v-for="item in colMenuItems" :key="item.key">
        <div v-if="item.separator" class="my-1 mx-2" style="height:1px; background: var(--border);"></div>
        <div v-else-if="item.alignRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.alignLabel }}</span>
          <button
            v-for="a in alignOptions" :key="a.value"
            class="flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyAlignToCol(a.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="a.icon" :size="13" /></button>
        </div>
        <div
          v-else
          class="flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
          :style="item.danger ? 'color: var(--color-danger);' : 'color: var(--text-secondary);'"
          @click="item.action?.()"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
        >
          <component :is="item.icon" :size="13" class="shrink-0" />
          <span>{{ item.label }}</span>
        </div>
      </template>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import type { Component } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'
import {
  TableProperties, Trash2,
  ArrowUpToLine, ArrowDownToLine, ArrowLeftToLine, ArrowRightToLine,
  AlignLeft, AlignCenter, AlignRight,
  Rows2, Columns2, LayoutList,
} from 'lucide-vue-next'

const props = defineProps<{
  editor: TiptapEditor | undefined
  t: Record<string, any>
}>()

// ── DOM state ─────────────────────────────────────────────────────────────
interface RowInfo { el: HTMLTableRowElement; rect: DOMRect; index: number }
interface ColInfo { el: HTMLTableCellElement; rect: DOMRect; index: number }

const tableEl    = ref<HTMLTableElement | null>(null)
const tableRect  = ref<DOMRect | null>(null)
const rowInfos   = ref<RowInfo[]>([])
const colInfos   = ref<ColInfo[]>([])

// Hover highlight indices
const activeRowIdx = ref(-1)
const activeColIdx = ref(-1)

// ── Menu state ────────────────────────────────────────────────────────────
const tableMenuVisible  = ref(false)
const tableMenuPos      = ref({ top: 0, left: 0 })
const rowMenuVisible    = ref(false)
const rowMenuPos        = ref({ top: 0, left: 0 })
const rowMenuTargetRow  = ref<HTMLTableRowElement | null>(null)
const colMenuVisible    = ref(false)
const colMenuPos        = ref({ top: 0, left: 0 })
const colMenuTargetCell = ref<HTMLTableCellElement | null>(null)

let hideTimer: ReturnType<typeof setTimeout> | null = null

// ── Alignment options ─────────────────────────────────────────────────────
const alignOptions = [
  { value: 'left',   icon: AlignLeft   },
  { value: 'center', icon: AlignCenter },
  { value: 'right',  icon: AlignRight  },
]

// ── Menu item definitions ─────────────────────────────────────────────────
type MenuItem = {
  key: string; label?: string; icon?: Component; danger?: boolean
  separator?: boolean; alignRow?: boolean; action?: () => void
}

const tableMenuItems = computed<MenuItem[]>(() => [
  { key: 'hrow', label: props.t.tableToggleHeaderRow, icon: Rows2,    action: () => runCmd('toggleHeaderRow') },
  { key: 'hcol', label: props.t.tableToggleHeaderCol, icon: Columns2, action: () => runCmd('toggleHeaderColumn') },
  { key: 'dist', label: props.t.tableDistributeCols,  icon: LayoutList, action: distributeColsEvenly },
  { key: 's1',   separator: true },
  { key: 'del',  label: props.t.tableDeleteTable,     icon: Trash2,   danger: true, action: () => runCmd('deleteTable') },
])

const rowMenuItems = computed<MenuItem[]>(() => [
  { key: 'ins-above', label: props.t.tableInsertRowAbove, icon: ArrowUpToLine,   action: () => runRowCmd('addRowBefore') },
  { key: 'ins-below', label: props.t.tableInsertRowBelow, icon: ArrowDownToLine, action: () => runRowCmd('addRowAfter')  },
  { key: 's1', separator: true },
  { key: 'hrow', label: props.t.tableToggleHeaderRow,    icon: Rows2,    action: () => runRowCmd('toggleHeaderRow') },
  { key: 's2', separator: true },
  { key: 'align', alignRow: true },
  { key: 's3', separator: true },
  { key: 'del', label: props.t.tableDeleteRow, icon: Trash2, danger: true, action: () => runRowCmd('deleteRow') },
])

const colMenuItems = computed<MenuItem[]>(() => [
  { key: 'ins-left',  label: props.t.tableInsertColLeft,  icon: ArrowLeftToLine,  action: () => runColCmd('addColumnBefore') },
  { key: 'ins-right', label: props.t.tableInsertColRight, icon: ArrowRightToLine, action: () => runColCmd('addColumnAfter')  },
  { key: 's1', separator: true },
  { key: 'hcol', label: props.t.tableToggleHeaderCol, icon: Columns2, action: () => runColCmd('toggleHeaderColumn') },
  { key: 's2', separator: true },
  { key: 'align', alignRow: true },
  { key: 's3', separator: true },
  { key: 'del', label: props.t.tableDeleteCol, icon: Trash2, danger: true, action: () => runColCmd('deleteColumn') },
])

// ── Popup positioning ─────────────────────────────────────────────────────
const popupStyle = (pos: { top: number; left: number }) => ({
  top:        pos.top  + 'px',
  left:       pos.left + 'px',
  background: 'var(--bg-editor)',
  border:     '1px solid var(--border)',
  boxShadow:  'var(--shadow-lg)',
  minWidth:   '168px',
})

// ── DOM info refresh ──────────────────────────────────────────────────────
const refreshInfo = () => {
  const table = tableEl.value
  if (!table) { rowInfos.value = []; colInfos.value = []; tableRect.value = null; return }
  tableRect.value = table.getBoundingClientRect()
  const rows = Array.from(table.querySelectorAll('tr')) as HTMLTableRowElement[]
  rowInfos.value  = rows.map((el, index) => ({ el, rect: el.getBoundingClientRect(), index }))
  const firstRow  = rows[0]
  const cells     = firstRow ? Array.from(firstRow.querySelectorAll('td, th')) as HTMLTableCellElement[] : []
  colInfos.value  = cells.map((el, index) => ({ el, rect: el.getBoundingClientRect(), index }))
}

// ── Cell focus helper ─────────────────────────────────────────────────────
const focusCell = (cellEl: HTMLElement): boolean => {
  if (!props.editor) return false
  try {
    const pos = props.editor.view.posAtDOM(cellEl, 0) + 1
    props.editor.chain().focus().setTextSelection(pos).run()
    return true
  } catch (_) { return false }
}

// ── Command helpers ───────────────────────────────────────────────────────
const runCmd = (cmd: string) => {
  if (!props.editor) return
  ;(props.editor.chain().focus() as any)[cmd]().run()
  closeAllMenus()
  nextTick(refreshInfo)
}

const runRowCmd = (cmd: string) => {
  const row = rowMenuTargetRow.value
  if (!row || !props.editor) return
  const cell = row.querySelector('td, th') as HTMLElement | null
  if (!cell) return
  if (focusCell(cell)) {
    // Select the entire row so alignment / format commands affect all cells
    ;(props.editor.chain().focus() as any).selectRow()[cmd]().run()
  }
  closeAllMenus()
  nextTick(refreshInfo)
}

const runColCmd = (cmd: string) => {
  const cell = colMenuTargetCell.value
  if (!cell || !props.editor) return
  if (focusCell(cell)) {
    ;(props.editor.chain().focus() as any).selectColumn()[cmd]().run()
  }
  closeAllMenus()
  nextTick(refreshInfo)
}

const applyAlignToRow = (align: string) => {
  const row = rowMenuTargetRow.value
  if (!row || !props.editor) return
  const cell = row.querySelector('td, th') as HTMLElement | null
  if (!cell) return
  if (focusCell(cell)) {
    ;(props.editor.chain().focus() as any).selectRow().setTextAlign(align).run()
  }
  closeAllMenus()
}

const applyAlignToCol = (align: string) => {
  const cell = colMenuTargetCell.value
  if (!cell || !props.editor) return
  if (focusCell(cell)) {
    ;(props.editor.chain().focus() as any).selectColumn().setTextAlign(align).run()
  }
  closeAllMenus()
}

const distributeColsEvenly = () => {
  if (!props.editor) return
  const { state, dispatch } = props.editor.view
  const tr = state.tr
  state.doc.descendants((node, pos) => {
    if (node.type.name === 'tableCell' || node.type.name === 'tableHeader') {
      tr.setNodeMarkup(pos, undefined, { ...node.attrs, colwidth: null })
    }
  })
  dispatch(tr)
  props.editor.view.focus()
  closeAllMenus()
  nextTick(refreshInfo)
}

// ── Menu open/close ───────────────────────────────────────────────────────
const closeAllMenus = () => {
  tableMenuVisible.value = false
  rowMenuVisible.value   = false
  colMenuVisible.value   = false
}

const toggleMenu = (which: 'table' | 'row' | 'col', e: MouseEvent) => {
  const btn = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const pos = { top: btn.bottom + 4, left: btn.left }
  if (which === 'table') {
    if (tableMenuVisible.value) { tableMenuVisible.value = false; return }
    tableMenuPos.value = pos; closeAllMenus(); tableMenuVisible.value = true
  } else if (which === 'row') {
    if (rowMenuVisible.value) { rowMenuVisible.value = false; return }
    rowMenuPos.value = pos; closeAllMenus(); rowMenuVisible.value = true
  } else {
    if (colMenuVisible.value) { colMenuVisible.value = false; return }
    colMenuPos.value = pos; closeAllMenus(); colMenuVisible.value = true
  }
}

// ── Bar click handlers ────────────────────────────────────────────────────
const onRowBarClick = (e: MouseEvent, row: RowInfo) => {
  rowMenuTargetRow.value = row.el
  // Position menu to the right of the bar
  const barEl = e.currentTarget as HTMLElement
  const rect  = barEl.getBoundingClientRect()
  rowMenuPos.value = { top: rect.top, left: rect.right + 6 }
  closeAllMenus()
  rowMenuVisible.value = true
  // Focus first cell of the row so commands know which row to target
  const cell = row.el.querySelector('td, th') as HTMLElement | null
  if (cell) focusCell(cell)
}

const onColBarClick = (e: MouseEvent, col: ColInfo) => {
  colMenuTargetCell.value = col.el
  const barEl = e.currentTarget as HTMLElement
  const rect  = barEl.getBoundingClientRect()
  colMenuPos.value = { top: rect.bottom + 6, left: rect.left }
  closeAllMenus()
  colMenuVisible.value = true
  focusCell(col.el)
}

// ── Show / hide ───────────────────────────────────────────────────────────
const keepVisible = () => { if (hideTimer) clearTimeout(hideTimer) }

const scheduleHide = () => {
  hideTimer = setTimeout(() => {
    if (tableMenuVisible.value || rowMenuVisible.value || colMenuVisible.value) return
    tableEl.value   = null
    tableRect.value = null
    rowInfos.value  = []
    colInfos.value  = []
    activeRowIdx.value = -1
    activeColIdx.value = -1
  }, 200)
}

// ── Mouse tracking ────────────────────────────────────────────────────────
const onMouseMove = (e: MouseEvent) => {
  const target = e.target as HTMLElement
  if (target.closest('[data-table-overlay]')) { keepVisible(); return }

  const pmDom = props.editor?.view.dom
  const table = target.closest<HTMLTableElement>('table')
  if (!table || !pmDom?.contains(table)) { scheduleHide(); return }

  keepVisible()

  if (tableEl.value !== table) {
    tableEl.value = table
    nextTick(refreshInfo)
  } else {
    // Lightweight rect update on every move so bars track scroll precisely
    tableRect.value = table.getBoundingClientRect()
  }
}

const onDocumentClick = (e: MouseEvent) => {
  if (!(e.target as HTMLElement).closest('[data-table-overlay]')) closeAllMenus()
}

const onScroll = () => { if (tableEl.value) refreshInfo() }

onMounted(() => {
  document.addEventListener('mousemove', onMouseMove, { passive: true })
  document.addEventListener('click',     onDocumentClick)
  window.addEventListener('scroll',      onScroll,          { passive: true, capture: true })
  window.addEventListener('resize',      refreshInfo)
})
onUnmounted(() => {
  document.removeEventListener('mousemove', onMouseMove)
  document.removeEventListener('click',     onDocumentClick)
  window.removeEventListener('scroll',      onScroll,  { capture: true })
  window.removeEventListener('resize',      refreshInfo)
  if (hideTimer) clearTimeout(hideTimer)
})
</script>
