<template>
  <Teleport to="body">
    <!-- ── Row selector bars — slim, right-anchored, Feishu-style ────────────── -->
    <template v-if="tableRect">
      <div
        v-for="row in rowInfos"
        :key="`rb${row.index}`"
        data-table-overlay
        class="fixed z-[77] cursor-pointer select-none"
        :style="{
          top:          row.rect.top + 'px',
          right:        (vw - tableRect.left + 2) + 'px',
          width:        activeRowIdx === row.index ? '14px' : '5px',
          height:       row.rect.height + 'px',
          background:   'var(--accent)',
          borderRadius: '3px 0 0 3px',
          opacity:      activeRowIdx === row.index ? '0.9' : '0.2',
          pointerEvents: 'auto',
          transition:   'width 0.12s, opacity 0.12s',
        }"
        @mouseenter="activeRowIdx = row.index; keepVisible()"
        @mouseleave="activeRowIdx = -1; keepVisible()"
        @click.stop="onRowBarClick($event, row)"
      />
    </template>

    <!-- ── Column selector bars — slim, bottom-anchored, Feishu-style ──────────── -->
    <template v-if="tableRect">
      <div
        v-for="col in colInfos"
        :key="`cb${col.index}`"
        data-table-overlay
        class="fixed z-[77] cursor-pointer select-none"
        :style="{
          left:         col.rect.left + 'px',
          bottom:       (vh - tableRect.top + 2) + 'px',
          width:        col.rect.width + 'px',
          height:       activeColIdx === col.index ? '12px' : '4px',
          background:   'var(--accent)',
          borderRadius: '3px 3px 0 0',
          opacity:      activeColIdx === col.index ? '0.9' : '0.2',
          pointerEvents: 'auto',
          transition:   'height 0.12s, opacity 0.12s',
        }"
        @mouseenter="activeColIdx = col.index; keepVisible()"
        @mouseleave="activeColIdx = -1; keepVisible()"
        @click.stop="onColBarClick($event, col)"
      />
    </template>

    <!-- ── Corner group: [⠿ drag handle] + [⊞ table menu] ──────────────────── -->
    <div
      v-if="tableRect"
      data-table-overlay
      class="fixed z-[78] flex items-center gap-0.5 select-none"
      :style="{
        top:     Math.max(4, tableRect.top - 30) + 'px',
        left:    Math.max(4, tableRect.left - 4) + 'px',
        opacity: isCornerDragging ? '0' : '1',
        transition: 'opacity 0.1s',
      }"
      @mouseenter="keepVisible"
      @mouseleave="keepVisible"
    >
      <!-- Drag handle: drags the whole table block -->
      <div
        draggable="true"
        class="w-6 h-6 flex items-center justify-center rounded transition-colors"
        :class="isCornerDragging ? 'cursor-grabbing' : 'cursor-grab'"
        :style="{ color: 'var(--text-muted)', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-sm)' }"
        :title="t.dragHandle"
        @mousedown.prevent="onCornerMousedown"
        @dragstart="onCornerDragStart"
        @dragend="onCornerDragEnd"
        @mouseenter="(e: MouseEvent) => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
        @mouseleave="(e: MouseEvent) => { if (!isCornerDragging) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><GripVertical :size="12" /></div>
      <!-- Table menu button -->
      <button
        class="menu-item w-6 h-6 flex items-center justify-center rounded transition-colors"
        :style="tableMenuVisible
          ? 'background: var(--accent); color: #fff; border: 1px solid var(--accent);'
          : 'background: var(--bg-editor); color: var(--text-muted); border: 1px solid var(--border);'"
        style="box-shadow: var(--shadow-sm);"
        @click.stop="toggleMenu('table', $event)"
        @mouseenter="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-hover)' }"
        @mouseleave="e => { if (!tableMenuVisible) (e.currentTarget as HTMLElement).style.background='var(--bg-editor)' }"
      ><TableProperties :size="12" /></button>
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
          class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
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
        <!-- Alignment buttons row -->
        <div v-else-if="item.alignRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.alignLabel }}</span>
          <button
            v-for="a in alignOptions" :key="a.value"
            class="menu-item flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyAlignToRow(a.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="a.icon" :size="13" /></button>
        </div>
        <!-- Text format buttons row -->
        <div v-else-if="item.formatRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableFormatLabel }}</span>
          <button
            v-for="f in formatOptions" :key="f.mark"
            class="menu-item flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyMarkToRow(f.mark)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="f.icon" :size="13" /></button>
        </div>
        <!-- Background colour palette row -->
        <div v-else-if="item.bgRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableBgColor }}</span>
          <button
            v-for="c in bgColorOptions" :key="c.value ?? 'clear'"
            class="menu-item w-5 h-5 rounded flex items-center justify-center shrink-0 transition-opacity"
            :style="c.value ? `background:${c.value}; border:1px solid var(--border);` : 'border:1px solid var(--border); color:var(--text-muted);'"
            :title="c.value ? c.value : t.tableBgColorClear"
            @click="applyBgToRow(c.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.opacity='0.7'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.opacity='1'"
          >
            <X v-if="!c.value" :size="10" />
          </button>
        </div>
        <div
          v-else
          class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
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

    <!-- ── Mobile context menu (long-press on touch devices) ────────────── -->
    <div
      v-if="mobileMenuVisible"
      data-table-overlay
      class="fixed z-[79] overflow-y-auto rounded-xl anim-pop-in"
      :style="popupStyle(mobileMenuPos, true)"
      @touchstart.stop="keepVisible"
    >
      <!-- Row section -->
      <div class="px-3 pt-2 pb-0.5 text-xs font-semibold" style="color: var(--text-muted);">{{ t.tableRowOps }}</div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runRowCmd('addRowBefore')" @touchend.prevent="runRowCmd('addRowBefore')">
        <ArrowUpToLine :size="13" class="shrink-0" /><span>{{ t.tableInsertRowAbove }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runRowCmd('addRowAfter')" @touchend.prevent="runRowCmd('addRowAfter')">
        <ArrowDownToLine :size="13" class="shrink-0" /><span>{{ t.tableInsertRowBelow }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runRowCmd('toggleHeaderRow')" @touchend.prevent="runRowCmd('toggleHeaderRow')">
        <Rows2 :size="13" class="shrink-0" /><span>{{ t.tableToggleHeaderRow }}</span>
      </div>
      <!-- Row align -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.alignLabel }}</span>
        <button v-for="a in alignOptions" :key="a.value" class="menu-item flex-1 h-7 flex items-center justify-center rounded" style="color: var(--text-muted);" @click="applyAlignToRow(a.value)" @touchend.prevent="applyAlignToRow(a.value)"><component :is="a.icon" :size="14" /></button>
      </div>
      <!-- Row bg -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableBgColor }}</span>
        <button v-for="c in bgColorOptions" :key="c.value ?? 'clear'" class="menu-item w-6 h-6 rounded flex items-center justify-center shrink-0" :style="c.value ? `background:${c.value}; border:1px solid var(--border);` : 'border:1px solid var(--border); color:var(--text-muted);'" @click="applyBgToRow(c.value)" @touchend.prevent="applyBgToRow(c.value)"><X v-if="!c.value" :size="10" /></button>
      </div>
      <!-- Row format -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableFormatLabel }}</span>
        <button v-for="f in formatOptions" :key="f.mark" class="menu-item flex-1 h-7 flex items-center justify-center rounded" style="color: var(--text-muted);" @click="applyMarkToRow(f.mark)" @touchend.prevent="applyMarkToRow(f.mark)"><component :is="f.icon" :size="14" /></button>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--color-danger);" @click="runRowCmd('deleteRow')" @touchend.prevent="runRowCmd('deleteRow')">
        <Trash2 :size="13" class="shrink-0" /><span>{{ t.tableDeleteRow }}</span>
      </div>
      <!-- Col section -->
      <div class="mx-2 my-1" style="height:1px; background: var(--border);"></div>
      <div class="px-3 pt-1 pb-0.5 text-xs font-semibold" style="color: var(--text-muted);">{{ t.tableColOps }}</div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runColCmd('addColumnBefore')" @touchend.prevent="runColCmd('addColumnBefore')">
        <ArrowLeftToLine :size="13" class="shrink-0" /><span>{{ t.tableInsertColLeft }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runColCmd('addColumnAfter')" @touchend.prevent="runColCmd('addColumnAfter')">
        <ArrowRightToLine :size="13" class="shrink-0" /><span>{{ t.tableInsertColRight }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="runColCmd('toggleHeaderColumn')" @touchend.prevent="runColCmd('toggleHeaderColumn')">
        <Columns2 :size="13" class="shrink-0" /><span>{{ t.tableToggleHeaderCol }}</span>
      </div>
      <!-- Col align -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.alignLabel }}</span>
        <button v-for="a in alignOptions" :key="a.value" class="menu-item flex-1 h-7 flex items-center justify-center rounded" style="color: var(--text-muted);" @click="applyAlignToCol(a.value)" @touchend.prevent="applyAlignToCol(a.value)"><component :is="a.icon" :size="14" /></button>
      </div>
      <!-- Col bg -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableBgColor }}</span>
        <button v-for="c in bgColorOptions" :key="c.value ?? 'clear'" class="menu-item w-6 h-6 rounded flex items-center justify-center shrink-0" :style="c.value ? `background:${c.value}; border:1px solid var(--border);` : 'border:1px solid var(--border); color:var(--text-muted);'" @click="applyBgToCol(c.value)" @touchend.prevent="applyBgToCol(c.value)"><X v-if="!c.value" :size="10" /></button>
      </div>
      <!-- Col format -->
      <div class="flex items-center gap-1 px-3 py-1.5">
        <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableFormatLabel }}</span>
        <button v-for="f in formatOptions" :key="f.mark" class="menu-item flex-1 h-7 flex items-center justify-center rounded" style="color: var(--text-muted);" @click="applyMarkToCol(f.mark)" @touchend.prevent="applyMarkToCol(f.mark)"><component :is="f.icon" :size="14" /></button>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--color-danger);" @click="runColCmd('deleteColumn')" @touchend.prevent="runColCmd('deleteColumn')">
        <Trash2 :size="13" class="shrink-0" /><span>{{ t.tableDeleteCol }}</span>
      </div>
      <!-- Table section -->
      <div class="mx-2 my-1" style="height:1px; background: var(--border);"></div>
      <div class="px-3 pt-1 pb-0.5 text-xs font-semibold" style="color: var(--text-muted);">{{ t.tableTableOps }}</div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="copyTable" @touchend.prevent="copyTable">
        <Copy :size="13" class="shrink-0" /><span>{{ t.tableCopyTable }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="cutTable" @touchend.prevent="cutTable">
        <Scissors :size="13" class="shrink-0" /><span>{{ t.tableCutTable }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--text-secondary);" @click="distributeColsEvenly" @touchend.prevent="distributeColsEvenly">
        <LayoutList :size="13" class="shrink-0" /><span>{{ t.tableDistributeCols }}</span>
      </div>
      <div class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer" style="color: var(--color-danger);" @click="runCmd('deleteTable')" @touchend.prevent="runCmd('deleteTable')">
        <Trash2 :size="13" class="shrink-0" /><span>{{ t.tableDeleteTable }}</span>
      </div>
      <div class="pb-1"></div>
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
            class="menu-item flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyAlignToCol(a.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="a.icon" :size="13" /></button>
        </div>
        <div v-else-if="item.formatRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableFormatLabel }}</span>
          <button
            v-for="f in formatOptions" :key="f.mark"
            class="menu-item flex-1 h-6 flex items-center justify-center rounded transition-colors"
            style="color: var(--text-muted);"
            @click="applyMarkToCol(f.mark)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'"
          ><component :is="f.icon" :size="13" /></button>
        </div>
        <div v-else-if="item.bgRow" class="flex items-center gap-1 px-3 py-1.5">
          <span class="text-xs shrink-0 mr-1" style="color: var(--text-muted);">{{ t.tableBgColor }}</span>
          <button
            v-for="c in bgColorOptions" :key="c.value ?? 'clear'"
            class="menu-item w-5 h-5 rounded flex items-center justify-center shrink-0 transition-opacity"
            :style="c.value ? `background:${c.value}; border:1px solid var(--border);` : 'border:1px solid var(--border); color:var(--text-muted);'"
            :title="c.value ? c.value : t.tableBgColorClear"
            @click="applyBgToCol(c.value)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.opacity='0.7'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.opacity='1'"
          >
            <X v-if="!c.value" :size="10" />
          </button>
        </div>
        <div
          v-else
          class="menu-item flex items-center gap-2 px-3 py-2 text-sm cursor-pointer transition-colors"
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
import { DOMSerializer } from '@tiptap/pm/model'
import type { Slice } from '@tiptap/pm/model'
import { NodeSelection } from '@tiptap/pm/state'
import type { EditorView } from '@tiptap/pm/view'

/** ProseMirror EditorView 的内部 dragging 状态（非公开 API，局部声明） */
interface EditorViewWithDragging extends EditorView {
  dragging: { slice: Slice; move: boolean; node?: unknown } | null
}
/** editor.chain() 动态命令调用辅助类型 */
type ChainedAny = ReturnType<TiptapEditor['chain']> & Record<string, () => ReturnType<TiptapEditor['chain']>>
import {
  TableProperties, Trash2,
  ArrowUpToLine, ArrowDownToLine, ArrowLeftToLine, ArrowRightToLine,
  AlignLeft, AlignCenter, AlignRight,
  Rows2, Columns2, LayoutList,
  Copy, Scissors, MoveRight, MoveLeft,
  Bold, Italic, Underline, Strikethrough, Code,
  X, GripVertical,
} from 'lucide-vue-next'

const props = defineProps<{
  editor: TiptapEditor | undefined
  t: Record<string, any>
}>()

// ── Viewport helpers (reactive, used in template :style bindings) ─────────
const vw = ref(window.innerWidth)
const vh = ref(window.innerHeight)

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

// Drag state for the corner drag handle
const isCornerDragging = ref(false)

// ── Menu state ────────────────────────────────────────────────────────────
const tableMenuVisible  = ref(false)
const tableMenuPos      = ref({ top: 0, left: 0 })
const rowMenuVisible    = ref(false)
const rowMenuPos        = ref({ top: 0, left: 0 })
const rowMenuTargetRow  = ref<HTMLTableRowElement | null>(null)
const colMenuVisible    = ref(false)
const colMenuPos        = ref({ top: 0, left: 0 })
const colMenuTargetCell = ref<HTMLTableCellElement | null>(null)

// Mobile long-press context menu
const mobileMenuVisible = ref(false)
const mobileMenuPos     = ref({ top: 0, left: 0 })

let hideTimer:      ReturnType<typeof setTimeout> | null = null
let longPressTimer: ReturnType<typeof setTimeout> | null = null
let touchStartX = 0
let touchStartY = 0

// ── Option lists ──────────────────────────────────────────────────────────
const alignOptions = [
  { value: 'left',   icon: AlignLeft   },
  { value: 'center', icon: AlignCenter },
  { value: 'right',  icon: AlignRight  },
]

const formatOptions = [
  { mark: 'bold',          icon: Bold         },
  { mark: 'italic',        icon: Italic       },
  { mark: 'underline',     icon: Underline    },
  { mark: 'strike',        icon: Strikethrough },
  { mark: 'code',          icon: Code         },
]

// Preset cell background colours. null = clear.
const bgColorOptions: Array<{ value: string | null }> = [
  { value: null },
  { value: '#fef9c3' }, // yellow
  { value: '#dcfce7' }, // green
  { value: '#dbeafe' }, // blue
  { value: '#fce7f3' }, // pink
  { value: '#ede9fe' }, // purple
  { value: '#ffedd5' }, // orange
  { value: '#fee2e2' }, // red
  { value: '#f1f5f9' }, // gray
]

// ── Menu item definitions ─────────────────────────────────────────────────
type MenuItem = {
  key: string; label?: string; icon?: Component; danger?: boolean
  separator?: boolean; alignRow?: boolean; formatRow?: boolean; bgRow?: boolean
  action?: () => void
}

const tableMenuItems = computed<MenuItem[]>(() => [
  { key: 'copy',       label: props.t.tableCopyTable,         icon: Copy,       action: copyTable      },
  { key: 'cut',        label: props.t.tableCutTable,          icon: Scissors,   action: cutTable       },
  { key: 's1',  separator: true },
  { key: 'ind-in',     label: props.t.tableIndentIn,          icon: MoveRight,  action: () => indentTable(1)  },
  { key: 'ind-out',    label: props.t.tableIndentOut,         icon: MoveLeft,   action: () => indentTable(-1) },
  { key: 's2',  separator: true },
  { key: 'hrow',       label: props.t.tableToggleHeaderRow,   icon: Rows2,      action: () => runCmd('toggleHeaderRow')    },
  { key: 'hcol',       label: props.t.tableToggleHeaderCol,   icon: Columns2,   action: () => runCmd('toggleHeaderColumn') },
  { key: 'dist',       label: props.t.tableDistributeCols,    icon: LayoutList, action: distributeColsEvenly               },
  { key: 's3',  separator: true },
  { key: 'del',        label: props.t.tableDeleteTable,       icon: Trash2,     danger: true, action: () => runCmd('deleteTable') },
])

const rowMenuItems = computed<MenuItem[]>(() => [
  { key: 'ins-above', label: props.t.tableInsertRowAbove, icon: ArrowUpToLine,   action: () => runRowCmd('addRowBefore') },
  { key: 'ins-below', label: props.t.tableInsertRowBelow, icon: ArrowDownToLine, action: () => runRowCmd('addRowAfter')  },
  { key: 's1', separator: true },
  { key: 'hrow', label: props.t.tableToggleHeaderRow, icon: Rows2, action: () => runRowCmd('toggleHeaderRow') },
  { key: 's2', separator: true },
  { key: 'bg',     bgRow:     true },
  { key: 's3', separator: true },
  { key: 'align',  alignRow:  true },
  { key: 's4', separator: true },
  { key: 'format', formatRow: true },
  { key: 's5', separator: true },
  { key: 'del', label: props.t.tableDeleteRow, icon: Trash2, danger: true, action: () => runRowCmd('deleteRow') },
])

const colMenuItems = computed<MenuItem[]>(() => [
  { key: 'ins-left',  label: props.t.tableInsertColLeft,  icon: ArrowLeftToLine,  action: () => runColCmd('addColumnBefore') },
  { key: 'ins-right', label: props.t.tableInsertColRight, icon: ArrowRightToLine, action: () => runColCmd('addColumnAfter')  },
  { key: 's1', separator: true },
  { key: 'hcol', label: props.t.tableToggleHeaderCol, icon: Columns2, action: () => runColCmd('toggleHeaderColumn') },
  { key: 's2', separator: true },
  { key: 'bg',     bgRow:     true },
  { key: 's3', separator: true },
  { key: 'align',  alignRow:  true },
  { key: 's4', separator: true },
  { key: 'format', formatRow: true },
  { key: 's5', separator: true },
  { key: 'del', label: props.t.tableDeleteCol, icon: Trash2, danger: true, action: () => runColCmd('deleteColumn') },
])

// ── Popup positioning ─────────────────────────────────────────────────────
const popupStyle = (pos: { top: number; left: number }, mobile = false): Record<string, string> => ({
  top:        pos.top  + 'px',
  left:       pos.left + 'px',
  background: 'var(--bg-editor)',
  border:     '1px solid var(--border)',
  boxShadow:  'var(--shadow-lg)',
  minWidth:   '200px',
  maxWidth:   '280px',
  maxHeight:  mobile ? '70vh' : 'none',
  overflowY:  mobile ? 'auto' : 'visible',
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
    // posAtDOM(el, 0) returns the position just inside el's opening token
    // (i.e. the start of the cell's content).  +1 moves into the first child
    // node (usually a paragraph) so Tiptap's setTextSelection places the
    // cursor at the beginning of the cell text rather than on the cell node
    // itself, which is not a valid cursor position for all commands.
    // Wrapped in try-catch because posAtDOM throws when cellEl has been
    // detached from the editor DOM (e.g. table deleted while menu is open).
    const pos = props.editor.view.posAtDOM(cellEl, 0) + 1
    props.editor.chain().focus().setTextSelection(pos).run()
    return true
  } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] posAtDOM failed:', e); return false }
}

// ── Command helpers ───────────────────────────────────────────────────────
const runCmd = (cmd: string) => {
  if (!props.editor) return
  ;(props.editor.chain().focus() as ChainedAny)[cmd]().run()
  closeAllMenus()
  nextTick(refreshInfo)
}

const runRowCmd = (cmd: string) => {
  const row = rowMenuTargetRow.value
  if (!row || !props.editor) return
  const cell = row.querySelector('td, th') as HTMLElement | null
  if (!cell) return
  if (focusCell(cell)) {
    ;(props.editor.chain().focus() as ChainedAny)[cmd]().run()
  }
  closeAllMenus()
  nextTick(refreshInfo)
}

const runColCmd = (cmd: string) => {
  const cell = colMenuTargetCell.value
  if (!cell || !props.editor) return
  if (focusCell(cell)) {
    ;(props.editor.chain().focus() as ChainedAny)[cmd]().run()
  }
  closeAllMenus()
  nextTick(refreshInfo)
}

// Apply text alignment to all block children inside each cell via direct ProseMirror tr.
// setTextAlign does not work on CellSelection; we iterate paragraph/heading nodes instead.
const applyAlignToCells = (cells: HTMLElement[], align: string) => {
  if (!props.editor || cells.length === 0) return
  const { state, dispatch } = props.editor.view
  const tr = state.tr
  for (const cellEl of cells) {
    try {
      const cellContentStart = props.editor.view.posAtDOM(cellEl, 0)
      const $pos = state.doc.resolve(cellContentStart)
      const cellNode = $pos.parent
      cellNode.forEach((child, offset) => {
        if (child.isBlock) {
          // cellContentStart points inside the cell open token; children are at cellContentStart + offset
          tr.setNodeMarkup(cellContentStart + offset, undefined, { ...child.attrs, textAlign: align })
        }
      })
    } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] applyAlignToCells: cell skip:', e) }
  }
  dispatch(tr)
}

const applyAlignToRow = (align: string) => {
  const row = rowMenuTargetRow.value
  if (!row) return
  applyAlignToCells(Array.from(row.querySelectorAll('td, th')) as HTMLElement[], align)
  closeAllMenus()
}

const applyAlignToCol = (align: string) => {
  const targetCell = colMenuTargetCell.value
  if (!targetCell || !tableEl.value) return
  const colIdx = Array.from(targetCell.parentElement?.querySelectorAll('td, th') ?? []).indexOf(targetCell)
  if (colIdx < 0) return
  const rows = Array.from(tableEl.value.querySelectorAll('tr')) as HTMLTableRowElement[]
  const cells = rows.map(r => r.querySelectorAll('td, th')[colIdx] as HTMLElement).filter(Boolean)
  applyAlignToCells(cells, align)
  closeAllMenus()
}

// ── Apply mark (bold/italic/etc.) to a set of cells ───────────────────────
// Tiptap's setMark/toggleMark do not work on CellSelection.  We iterate each
// cell's content range and call tr.addMark / tr.removeMark directly.
const applyMarkToCells = (cells: HTMLElement[], markName: string) => {
  if (!props.editor || cells.length === 0) return
  const { state, dispatch } = props.editor.view
  const markType = state.schema.marks[markName]
  if (!markType) return

  // Check whether every text node in the target cells already has the mark.
  let allHaveMark = true
  const ranges: Array<{ from: number; to: number }> = []
  for (const cellEl of cells) {
    try {
      // posAtDOM(cellEl, 0) returns posAtStart of the cell (inside the open token).
      const cellContentStart = props.editor.view.posAtDOM(cellEl, 0)
      const $pos = state.doc.resolve(cellContentStart)
      const cellNode = $pos.parent  // the td/th node
      const from = cellContentStart
      const to = cellContentStart + cellNode.nodeSize - 2  // posAtEnd (before close token)
      ranges.push({ from, to })
      state.doc.nodesBetween(from, to, node => {
        if (node.isText && !markType.isInSet(node.marks)) allHaveMark = false
      })
    } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] applyMarkToCells: cell skip:', e) }
  }

  const tr = state.tr
  for (const { from, to } of ranges) {
    if (allHaveMark) tr.removeMark(from, to, markType)
    else             tr.addMark(from, to, markType.create())
  }
  dispatch(tr)
}

const applyMarkToRow = (markName: string) => {
  const row = rowMenuTargetRow.value
  if (!row) return
  applyMarkToCells(Array.from(row.querySelectorAll('td, th')) as HTMLElement[], markName)
  closeAllMenus()
}

const applyMarkToCol = (markName: string) => {
  const targetCell = colMenuTargetCell.value
  if (!targetCell || !tableEl.value) return
  // Determine the column index of the clicked cell within its row
  const colIdx = Array.from(targetCell.parentElement?.querySelectorAll('td, th') ?? []).indexOf(targetCell)
  if (colIdx < 0) return
  const rows = Array.from(tableEl.value.querySelectorAll('tr')) as HTMLTableRowElement[]
  const cells = rows.map(r => r.querySelectorAll('td, th')[colIdx] as HTMLElement).filter(Boolean)
  applyMarkToCells(cells, markName)
  closeAllMenus()
}

// ── Apply background colour to a set of cells ─────────────────────────────
const applyBgToCells = (cells: HTMLElement[], color: string | null) => {
  if (!props.editor || cells.length === 0) return
  const { state, dispatch } = props.editor.view
  const tr = state.tr
  for (const cellEl of cells) {
    try {
      const cellContentStart = props.editor.view.posAtDOM(cellEl, 0)
      const $pos = state.doc.resolve(cellContentStart)
      const cellNode = $pos.parent
      // The cell node position is one before its content start
      const cellPos = cellContentStart - 1
      if (cellNode.type.name === 'tableCell' || cellNode.type.name === 'tableHeader') {
        tr.setNodeMarkup(cellPos, undefined, { ...cellNode.attrs, backgroundColor: color })
      }
    } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] applyBgToCells: cell skip:', e) }
  }
  dispatch(tr)
}

const applyBgToRow = (color: string | null) => {
  const row = rowMenuTargetRow.value
  if (!row) return
  applyBgToCells(Array.from(row.querySelectorAll('td, th')) as HTMLElement[], color)
  closeAllMenus()
}

const applyBgToCol = (color: string | null) => {
  const targetCell = colMenuTargetCell.value
  if (!targetCell || !tableEl.value) return
  const colIdx = Array.from(targetCell.parentElement?.querySelectorAll('td, th') ?? []).indexOf(targetCell)
  if (colIdx < 0) return
  const rows = Array.from(tableEl.value.querySelectorAll('tr')) as HTMLTableRowElement[]
  const cells = rows.map(r => r.querySelectorAll('td, th')[colIdx] as HTMLElement).filter(Boolean)
  applyBgToCells(cells, color)
  closeAllMenus()
}

// ── Distribute column widths evenly (current table only) ──────────────────
const distributeColsEvenly = () => {
  if (!props.editor || !tableEl.value) return
  const { state, dispatch } = props.editor.view

  // Locate the table node via its DOM element to avoid cursor-position dependency
  // (user's focus may have shifted when the floating menu button was clicked).
  // posAtDOM returns posAtStart (inside the table open token); subtract 1 to get
  // the position of the table node itself so nodeAt() returns the table node.
  // Wrap in try-catch: posAtDOM throws if the element has been detached from
  // the editor DOM (e.g. table deleted between menu open and click).
  let tablePos: number
  try {
    tablePos = props.editor.view.posAtDOM(tableEl.value, 0) - 1
  } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] posAtDOM failed:', e); return }
  const tableNode = state.doc.nodeAt(tablePos)
  if (!tableNode || tableNode.type.name !== 'table') return

  const tr = state.tr
  // descendants relPos is relative to tableNode content start.
  // absPos = tablePos + 1 + relPos gives the document-absolute position.
  tableNode.descendants((node, relPos) => {
    if (node.type.name === 'tableCell' || node.type.name === 'tableHeader') {
      const absPos = tablePos + 1 + relPos
      tr.setNodeMarkup(absPos, undefined, { ...node.attrs, colwidth: null })
    }
  })
  dispatch(tr)
  props.editor.view.focus()
  closeAllMenus()
  nextTick(refreshInfo)
}

// ── Copy / cut table ──────────────────────────────────────────────────────
const getTableHtml = (): string | null => {
  if (!props.editor || !tableEl.value) return null
  const { state } = props.editor.view
  const tableContentStart = props.editor.view.posAtDOM(tableEl.value, 0)
  const tablePos = tableContentStart - 1
  const tableNode = state.doc.nodeAt(tablePos)
  if (!tableNode) return null
  const serializer = DOMSerializer.fromSchema(state.schema)
  const dom = serializer.serializeNode(tableNode)
  const wrapper = document.createElement('div')
  wrapper.appendChild(dom)
  return wrapper.innerHTML
}

const copyTableToClipboard = async (html: string): Promise<boolean> => {
  // Use the modern async Clipboard API only.
  // The legacy execCommand('copy') fallback is intentionally omitted: it writes
  // raw HTML as plain text into the system clipboard, which is readable by any
  // local process and could expose decrypted note content.  If ClipboardItem is
  // unavailable (non-HTTPS, old Firefox) the copy silently fails rather than
  // leaking content through an insecure channel.
  if (typeof ClipboardItem === 'undefined' || !navigator.clipboard?.write) return false
  try {
    // text/plain carries a stripped version of the HTML so paste targets that
    // only accept plain text get readable content instead of raw markup.
    const parser = new DOMParser()
    const doc = parser.parseFromString(html, 'text/html')
    const plainText = doc.body.innerText ?? doc.body.textContent ?? ''
    await navigator.clipboard.write([
      new ClipboardItem({
        'text/html':  new Blob([html],      { type: 'text/html'  }),
        'text/plain': new Blob([plainText], { type: 'text/plain' }),
      }),
    ])
    return true
  } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] copyTable failed:', e); return false }
}

const copyTable = async () => {
  const html = getTableHtml()
  if (html) await copyTableToClipboard(html)
  closeAllMenus()
}

const cutTable = async () => {
  const html = getTableHtml()
  if (html) {
    await copyTableToClipboard(html)
    props.editor?.chain().focus().deleteTable().run()
  }
  closeAllMenus()
}

// ── Table indent ──────────────────────────────────────────────────────────
const indentTable = (delta: number) => {
  if (!props.editor || !tableEl.value) return
  const { state, dispatch } = props.editor.view
  const tableContentStart = props.editor.view.posAtDOM(tableEl.value, 0)
  const tablePos = tableContentStart - 1
  const tableNode = state.doc.nodeAt(tablePos)
  if (!tableNode || tableNode.type.name !== 'table') return
  const current = (tableNode.attrs.indent as number) || 0
  const next = Math.max(0, Math.min(10, current + delta))
  if (next === current) return
  dispatch(state.tr.setNodeMarkup(tablePos, undefined, { ...tableNode.attrs, indent: next }))
  closeAllMenus()
}

// ── Menu open/close ───────────────────────────────────────────────────────
const closeAllMenus = () => {
  tableMenuVisible.value  = false
  rowMenuVisible.value    = false
  colMenuVisible.value    = false
  mobileMenuVisible.value = false
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
  const barEl = e.currentTarget as HTMLElement
  const rect  = barEl.getBoundingClientRect()
  rowMenuPos.value = { top: rect.top, left: rect.right + 6 }
  closeAllMenus()
  rowMenuVisible.value = true
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
  if (hideTimer) clearTimeout(hideTimer)
  // 300 ms: long enough for the mouse to travel from a table cell to the overlay
  // bar or menu without the overlay disappearing, but short enough to feel
  // responsive when the user genuinely leaves the table.
  // The re-check inside the callback guards against a race where the user opens
  // a menu between the time scheduleHide() is called and the timer fires.
  hideTimer = setTimeout(() => {
    if (tableMenuVisible.value || rowMenuVisible.value || colMenuVisible.value || mobileMenuVisible.value) return
    tableEl.value      = null
    tableRect.value    = null
    rowInfos.value     = []
    colInfos.value     = []
    activeRowIdx.value = -1
    activeColIdx.value = -1
  }, 300)
}

// ── Corner drag handlers (drag the entire table block) ────────────────────

const onCornerMousedown = (_e: MouseEvent) => {
  // Close any open menus so they don't interfere with the drag
  closeAllMenus()
}

const onCornerDragStart = (e: DragEvent) => {
  if (!props.editor || !tableEl.value) return
  const { state, view } = props.editor

  let tablePos: number
  try {
    tablePos = view.posAtDOM(tableEl.value, 0) - 1
  } catch (e) { if (import.meta.env.DEV) console.warn('[YinMo] posAtDOM failed:', e); return }

  const tableNode = state.doc.nodeAt(tablePos)
  if (!tableNode || tableNode.type.name !== 'table') return

  isCornerDragging.value = true

  const slice = state.doc.slice(tablePos, tablePos + tableNode.nodeSize)

  // Set NodeSelection so PM knows what to delete after the move
  try {
    const nodeSel = NodeSelection.create(state.doc, tablePos)
    view.dispatch(state.tr.setSelection(nodeSel))
    ;(view as EditorViewWithDragging).dragging = { slice, move: true, node: nodeSel }
  } catch (e) {
    if (import.meta.env.DEV) console.warn('[YinMo] NodeSelection failed, falling back:', e)
    ;(view as EditorViewWithDragging).dragging = { slice, move: true }
  }

  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', tableNode.textContent)
    // Ghost image: clone the table element
    const ghost = tableEl.value.cloneNode(true) as HTMLElement
    ghost.style.cssText = 'position:fixed;top:-9999px;left:-9999px;opacity:0.6;' +
      'pointer-events:none;max-width:420px;max-height:200px;overflow:hidden;' +
      'background:var(--bg-editor);padding:4px;border-radius:8px;font-size:14px;'
    document.body.appendChild(ghost)
    e.dataTransfer.setDragImage(ghost, 0, 16)
    setTimeout(() => ghost.remove(), 0)
  }
}

const onCornerDragEnd = () => {
  isCornerDragging.value = false
  if (props.editor && (props.editor.view as EditorViewWithDragging).dragging) {
    ;(props.editor.view as EditorViewWithDragging).dragging = null
  }
}

// ── Mouse tracking ────────────────────────────────────────────────────────
// Update which row/col bar is visible based on mouse position
const updateActiveByMouse = (clientX: number, clientY: number) => {
  const hitRow = rowInfos.value.find(r => clientY >= r.rect.top && clientY <= r.rect.bottom)
  activeRowIdx.value = hitRow ? hitRow.index : -1
  const hitCol = colInfos.value.find(c => clientX >= c.rect.left && clientX <= c.rect.right)
  activeColIdx.value = hitCol ? hitCol.index : -1
}

const onMouseMove = (e: MouseEvent) => {
  const target = e.target as HTMLElement
  if (target.closest('[data-table-overlay]')) { keepVisible(); return }

  const pmDom = props.editor?.view.dom
  const table = target.closest<HTMLTableElement>('table')
  if (!table || !pmDom?.contains(table)) { scheduleHide(); return }

  keepVisible()

  if (tableEl.value !== table) {
    tableEl.value = table
    nextTick(() => { refreshInfo(); updateActiveByMouse(e.clientX, e.clientY) })
  } else {
    tableRect.value = table.getBoundingClientRect()
    updateActiveByMouse(e.clientX, e.clientY)
  }
}

const onDocumentClick = (e: MouseEvent) => {
  if (!(e.target as HTMLElement).closest('[data-table-overlay]')) closeAllMenus()
}

// ── Touch event handling ──────────────────────────────────────────────────
const onTouchStart = (e: TouchEvent) => {
  const touch = e.touches[0]
  touchStartX = touch.clientX
  touchStartY = touch.clientY

  // Taps on overlay elements (menus, bars) → keep visible
  if ((e.target as HTMLElement).closest('[data-table-overlay]')) {
    keepVisible()
    return
  }

  // Close menus on tap outside
  if (mobileMenuVisible.value || tableMenuVisible.value || rowMenuVisible.value || colMenuVisible.value) {
    closeAllMenus()
    return
  }

  // Find the table being touched
  const target = document.elementFromPoint(touch.clientX, touch.clientY) as HTMLElement | null
  const table  = target?.closest<HTMLTableElement>('table')
  const pmDom  = props.editor?.view.dom
  if (!table || !pmDom?.contains(table)) { scheduleHide(); return }

  keepVisible()
  if (tableEl.value !== table) {
    tableEl.value = table
    nextTick(refreshInfo)
  }

  // Identify the target cell for long-press context menu
  const cellEl = target?.closest<HTMLTableCellElement>('td, th')
  if (!cellEl) return

  // 500 ms matches the Android long-press convention and feels natural on iOS.
  // The timer is cancelled in onTouchMove if the finger drifts more than 10 px
  // (the 10 px threshold absorbs natural touch jitter without requiring DPI
  // normalisation; typical jitter is 2–4 px on modern phones).
  longPressTimer = setTimeout(() => {
    longPressTimer = null
    // Set row / col targets for menu commands
    rowMenuTargetRow.value  = cellEl.closest('tr') as HTMLTableRowElement
    colMenuTargetCell.value = cellEl

    // Focus the cell so Tiptap commands know the context
    focusCell(cellEl)

    // Position menu above the touch point, shifted left to avoid covering finger
    const vw = window.innerWidth
    const menuW = Math.min(280, vw - 16)
    let left = Math.max(8, touch.clientX - menuW / 2)
    if (left + menuW > vw - 8) left = vw - menuW - 8
    const top = Math.max(8, touch.clientY - 50)
    mobileMenuPos.value = { top, left }
    mobileMenuVisible.value = true
  }, 500)
}

const onTouchMove = (e: TouchEvent) => {
  // Cancel long-press if finger moves more than 10px (user is scrolling)
  if (longPressTimer) {
    const dx = e.touches[0].clientX - touchStartX
    const dy = e.touches[0].clientY - touchStartY
    if (Math.abs(dx) > 10 || Math.abs(dy) > 10) {
      clearTimeout(longPressTimer)
      longPressTimer = null
    }
  }
  if (tableEl.value) refreshInfo()
}

const onTouchEnd = () => {
  if (longPressTimer) { clearTimeout(longPressTimer); longPressTimer = null }
  // After a long-press the mobile menu is open; cancel any pending hide timer so
  // the overlay does not vanish 300 ms after the finger lifts.
  if (mobileMenuVisible.value && hideTimer) { clearTimeout(hideTimer); hideTimer = null }
}

const onScroll = () => { if (tableEl.value) refreshInfo() }

const onResize = () => { vw.value = window.innerWidth; vh.value = window.innerHeight; refreshInfo() }

onMounted(() => {
  document.addEventListener('mousemove',  onMouseMove,   { passive: true })
  document.addEventListener('click',      onDocumentClick)
  document.addEventListener('touchstart', onTouchStart,  { passive: true })
  document.addEventListener('touchmove',  onTouchMove,   { passive: true })
  document.addEventListener('touchend',   onTouchEnd,    { passive: true })
  window.addEventListener('scroll',       onScroll,      { passive: true, capture: true })
  window.addEventListener('resize',       onResize)
})
onUnmounted(() => {
  document.removeEventListener('mousemove',  onMouseMove)
  document.removeEventListener('click',      onDocumentClick)
  document.removeEventListener('touchstart', onTouchStart)
  document.removeEventListener('touchmove',  onTouchMove)
  document.removeEventListener('touchend',   onTouchEnd)
  // @ts-expect-error — passive is ignored by browsers on removeEventListener but valid at runtime; absent from TS EventListenerOptions type
  window.removeEventListener('scroll',       onScroll,  { passive: true, capture: true })
  window.removeEventListener('resize',       onResize)
  if (hideTimer)      clearTimeout(hideTimer)
  if (longPressTimer) clearTimeout(longPressTimer)
})
</script>
