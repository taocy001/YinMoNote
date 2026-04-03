<template>
  <div class="flex flex-col h-full overflow-hidden" style="background: var(--bg-editor); color: var(--text-primary);">
    <!-- Diff header bar -->
    <div class="shrink-0 flex items-center justify-between px-4 py-2 border-b" style="border-color: var(--border);">
      <div class="flex items-center gap-3">
        <span class="text-sm font-semibold" style="color: var(--text-primary);">{{ t.diffView }}</span>
        <span
v-if="!isComputing && totalChanges > 0" class="text-xs px-2 py-0.5 rounded-full"
          style="background: var(--accent-light); color: var(--accent);">
          {{ totalChanges }} {{ t.diffChanges }}
        </span>
        <span v-else-if="!isComputing" class="text-xs" style="color: var(--text-muted);">{{ t.diffNoChanges }}</span>
        <span v-else class="text-xs" style="color: var(--text-muted);">{{ t.diffLoading }}</span>
      </div>
      <div class="flex items-center gap-1">
        <template v-if="!isComputing && totalChanges > 0">
          <button
class="w-7 h-7 flex items-center justify-center rounded-lg transition-all text-sm" style="color: var(--text-muted);"
            :title="t.diffPrev"
            @click="navigate(-1)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">←</button>
          <span class="text-xs tabular-nums px-1" style="color: var(--text-muted);">
            {{ currentChangeIdx + 1 }}/{{ totalChanges }}
          </span>
          <button
class="w-7 h-7 flex items-center justify-center rounded-lg transition-all text-sm" style="color: var(--text-muted);"
            :title="t.diffNext"
            @click="navigate(1)"
            @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'"
            @mouseleave="e => (e.currentTarget as HTMLElement).style.background='transparent'">→</button>
        </template>
        <button
class="ml-1 px-3 py-1 text-xs rounded-lg font-medium transition-all active:scale-95"
          style="background: var(--bg-hover); color: var(--text-secondary);"
          @click="emit('exit')"
          @mouseenter="e => (e.currentTarget as HTMLElement).style.background='var(--border)'"
          @mouseleave="e => (e.currentTarget as HTMLElement).style.background='var(--bg-hover)'">
          {{ t.diffExitView }}
        </button>
      </div>
    </div>

    <!-- Computing indicator -->
    <div v-if="isComputing" class="flex-1 flex items-center justify-center">
      <div class="flex flex-col items-center gap-2">
        <div
class="w-5 h-5 border-2 border-t-transparent rounded-full animate-spin"
          style="border-color: var(--border); border-top-color: var(--accent);"></div>
        <span class="text-xs" style="color: var(--text-muted);">{{ t.diffLoading }}</span>
      </div>
    </div>

    <!-- Read-only rich-text diff — rendered by a real Tiptap editor with Decoration overlays -->
    <div v-show="!isComputing" ref="scrollEl" class="flex-1 overflow-y-auto">
      <EditorContent
        :editor="diffEditor"
        class="diff-editor-root px-4 md:px-8 pt-6 pb-16 focus:outline-none"
        :class="editorWidth === 'full' ? 'w-full' : 'max-w-3xl mx-auto'"
        :style="{ fontSize: fontSize + 'px' }"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * DiffView — 飞书风格富文本 diff 视图。
 *
 * 使用与主编辑器完全相同的 Tiptap 扩展集（相同 schema），将两个 Markdown 版本
 * 各自解析为 ProseMirror 文档，然后通过以下步骤生成 Decoration：
 *   1. 顶层节点 LCS 对齐（段落/标题/代码块/列表等）
 *   2. 相邻 delete+insert 对 → replace（字符级 diff-match-patch）
 *   3. 新增块  → Decoration.node（绿色左边框）
 *   4. 删除块  → Decoration.widget（红色删除占位块）
 *   5. 变更块  → Decoration.node（琥珀左边框）+ Decoration.inline（绿色高亮新增）
 *              + Decoration.widget（红色行内删除线）
 *
 * 最终在只读 Tiptap 编辑器中渲染，加粗/斜体/代码/图片等格式完整保留。
 */
import { ref, inject, nextTick, onMounted, onBeforeUnmount, watch, type Ref } from 'vue'
import { useEditor, EditorContent, VueNodeViewRenderer } from '@tiptap/vue-3'
import { Extension } from '@tiptap/core'
import { Plugin, PluginKey } from 'prosemirror-state'
import { Decoration, DecorationSet } from 'prosemirror-view'
import type { Node as PmNode } from 'prosemirror-model'
import StarterKit from '@tiptap/starter-kit'
import Paragraph from '@tiptap/extension-paragraph'
import Image from '@tiptap/extension-image'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import Heading from '@tiptap/extension-heading'
import { Markdown } from 'tiptap-markdown'
import Link from '@tiptap/extension-link'
import Underline from '@tiptap/extension-underline'
import Highlight from '@tiptap/extension-highlight'
import Subscript from '@tiptap/extension-subscript'
import Superscript from '@tiptap/extension-superscript'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import { Table } from '@tiptap/extension-table'
import { TableRow } from '@tiptap/extension-table-row'
import { TableHeader } from '@tiptap/extension-table-header'
import { TableCell } from '@tiptap/extension-table-cell'
import Typography from '@tiptap/extension-typography'
import Gapcursor from '@tiptap/extension-gapcursor'
import { all, createLowlight } from 'lowlight'
// @ts-expect-error — diff-match-patch has no bundled type declarations
import DiffMatchPatch from 'diff-match-patch'
import { InlineMath } from './InlineMath'
import { Callout } from './Callout'
import { ToggleBlock } from './ToggleBlock'
import CalloutView from './CalloutView.vue'
import ToggleBlockView from './ToggleBlockView.vue'
import CodeBlockView from './CodeBlockView.vue'
import { useI18n } from '../i18n'

// ─── Constants ───────────────────────────────────────────────────────────────
const lowlight = createLowlight(all)
const dmp = new DiffMatchPatch()

// ─── Diff Decoration Plugin ───────────────────────────────────────────────────
const DIFF_KEY = new PluginKey<DecorationSet>('diffHighlight')

const DiffPlugin = Extension.create({
  name: 'diffPlugin',
  addProseMirrorPlugins() {
    return [
      new Plugin({
        key: DIFF_KEY,
        state: {
          init: () => DecorationSet.empty,
          apply: (tr, set) => {
            const meta = tr.getMeta(DIFF_KEY)
            if (meta !== undefined) return meta as DecorationSet
            return set.map(tr.mapping, tr.doc)
          },
        },
        props: {
          decorations: state => DIFF_KEY.getState(state) ?? DecorationSet.empty,
        },
      }),
    ]
  },
})

// ─── Props / emits ───────────────────────────────────────────────────────────
const props = defineProps<{ oldContent: string; newContent: string }>()
const emit = defineEmits<{ exit: [] }>()

const { t } = useI18n()
const editorWidth = inject<Ref<string>>('editorWidth', ref('standard') as Ref<string>)
const fontSize = inject<Ref<number>>('fontSize', ref(16) as Ref<number>)

const scrollEl = ref<HTMLElement | null>(null)
const currentChangeIdx = ref(0)
const isComputing = ref(true)
const totalChanges = ref(0)

// ─── Read-only Tiptap editor (same schema as the main editor) ────────────────
const diffEditor = useEditor({
  editable: false,
  content: '',
  extensions: [
    StarterKit.configure({ codeBlock: false, heading: false, paragraph: false }),
    // Match main editor: keep empty paragraphs as <p></p>
    Paragraph,
    Typography,
    Gapcursor,
    Heading.configure({ levels: [1, 2, 3, 4, 5, 6] }),
    CodeBlockLowlight
      .extend({ addNodeView() { return VueNodeViewRenderer(CodeBlockView) } })
      .configure({ lowlight }),
    Image,
    Markdown.configure({ html: true, transformPastedText: false, transformCopiedText: false }),
    Underline,
    Highlight.configure({ multicolor: false }),
    Subscript,
    Superscript,
    TaskList,
    TaskItem.configure({ nested: true }),
    InlineMath,
    Callout.extend({ addNodeView() { return VueNodeViewRenderer(CalloutView) } }),
    ToggleBlock.extend({ addNodeView() { return VueNodeViewRenderer(ToggleBlockView) } }),
    Link.configure({ openOnClick: false, autolink: true }),
    Table.configure({ resizable: false }),
    TableRow,
    TableHeader,
    TableCell,
    DiffPlugin,
  ],
  editorProps: { attributes: { class: 'focus:outline-none' } },
})

// ─── Top-level node LCS diff ─────────────────────────────────────────────────
type RawOp = { op: 'equal' | 'delete' | 'insert'; node: PmNode }
type DiffOp =
  | { op: 'equal'; node: PmNode }
  | { op: 'insert'; node: PmNode }
  | { op: 'delete'; node: PmNode }
  | { op: 'replace'; oldNode: PmNode; newNode: PmNode }

function lcsNodes(oldNodes: PmNode[], newNodes: PmNode[]): DiffOp[] {
  const n = oldNodes.length, m = newNodes.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--)
    for (let j = m - 1; j >= 0; j--)
      dp[i][j] = oldNodes[i].textContent === newNodes[j].textContent
        ? dp[i + 1][j + 1] + 1
        : Math.max(dp[i + 1][j], dp[i][j + 1])

  const raw: RawOp[] = []
  let i = 0, j = 0
  while (i < n || j < m) {
    if (i >= n) { raw.push({ op: 'insert', node: newNodes[j++] }) }
    else if (j >= m) { raw.push({ op: 'delete', node: oldNodes[i++] }) }
    else if (oldNodes[i].textContent === newNodes[j].textContent) {
      raw.push({ op: 'equal', node: newNodes[j] }); i++; j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      raw.push({ op: 'delete', node: oldNodes[i++] })
    } else {
      raw.push({ op: 'insert', node: newNodes[j++] })
    }
  }

  // Adjacent delete+insert → replace (block-level char-diff)
  const ops: DiffOp[] = []
  for (let k = 0; k < raw.length; k++) {
    if (raw[k].op === 'delete' && k + 1 < raw.length && raw[k + 1].op === 'insert') {
      ops.push({ op: 'replace', oldNode: raw[k].node, newNode: raw[k + 1].node }); k++
    } else {
      ops.push(raw[k] as DiffOp)
    }
  }
  return ops
}

// ─── Text offset → document position mapping ────────────────────────────────
/**
 * 为节点内的每个字符建立 textOffset → docPos 映射。
 * childPos 是文本节点相对于 node 内容起始的偏移量（来自 descendants 回调）。
 * 文档位置 = nodeStart（节点开标记）+ 1（进入内容）+ childPos + i（字符序号）。
 */
function buildTextMap(node: PmNode, nodeStart: number): number[] {
  const map: number[] = []
  let textOffset = 0
  node.descendants((child, childPos) => {
    if (child.isText) {
      for (let i = 0; i < child.text!.length; i++) {
        map[textOffset + i] = nodeStart + 1 + childPos + i
      }
      textOffset += child.text!.length
    }
    return true
  })
  // Sentinel: textOffset === textContent.length maps to end of node content
  map[textOffset] = nodeStart + 1 + node.content.size
  return map
}

// ─── Widget creation helpers ─────────────────────────────────────────────────
function makeDeletedBlockWidget(node: PmNode, changeIdx: number): HTMLElement {
  const el = document.createElement('div')
  el.className = `diff-del-widget-block diff-change-${changeIdx}`
  el.setAttribute('data-diff-change', String(changeIdx))
  // Show deleted block text with strikethrough; complex formatting simplified to plain text
  const inner = document.createElement('span')
  inner.textContent = node.textContent || ' '
  el.appendChild(inner)
  return el
}

function makeDeletedInlineWidget(text: string): HTMLElement {
  const el = document.createElement('del')
  el.className = 'diff-del-widget-inline'
  el.textContent = text
  return el
}

// ─── Core: build Decoration list ─────────────────────────────────────────────
function buildDecorations(oldDoc: PmNode, newDoc: PmNode): { decos: Decoration[]; count: number } {
  const oldNodes: PmNode[] = []
  oldDoc.forEach(n => oldNodes.push(n))
  const newNodes: PmNode[] = []
  newDoc.forEach(n => newNodes.push(n))

  const ops = lcsNodes(oldNodes, newNodes)
  const decos: Decoration[] = []
  let count = 0
  let pos = 1 // Start from the opening tag of newDoc's first child node

  for (const op of ops) {
    if (op.op === 'equal') {
      pos += op.node.nodeSize
      continue
    }

    if (op.op === 'insert') {
      const ci = count++
      decos.push(Decoration.node(pos, pos + op.node.nodeSize, {
        class: `diff-added-node diff-change-${ci}`,
        'data-diff-change': String(ci),
      }))
      pos += op.node.nodeSize
      continue
    }

    if (op.op === 'delete') {
      const ci = count++
      decos.push(Decoration.widget(pos, () => makeDeletedBlockWidget(op.node, ci), {
        side: -1,
        key: `del-block-${ci}`,
      }))
      // Don't advance pos (deleted node not in newDoc)
      continue
    }

    // replace — block-level character diff
    {
      const ci = count++
      const oldText = op.oldNode.textContent
      const newText = op.newNode.textContent
      const charDiffs: [number, string][] = dmp.diff_main(oldText, newText)
      dmp.diff_cleanupSemantic(charDiffs)

      const textMap = buildTextMap(op.newNode, pos)
      let textPos = 0 // Current offset within newText

      for (const [diffOp, text] of charDiffs) {
        const len = text.length
        if (diffOp === 0) {
          textPos += len
        } else if (diffOp === 1) {
          // Inserted text → Decoration.inline (green highlight)
          const from = textMap[textPos] ?? (pos + 1)
          const to = textMap[textPos + len] ?? from + len
          if (from < to && from >= pos + 1 && to <= pos + op.newNode.nodeSize) {
            decos.push(Decoration.inline(from, to, { class: 'diff-ins-text' }))
          }
          textPos += len
        } else {
          // Deleted text → Decoration.widget (red strikethrough)
          const docPos = textMap[textPos] ?? (pos + 1)
          if (docPos >= pos + 1 && docPos <= pos + op.newNode.nodeSize) {
            decos.push(Decoration.widget(docPos, () => makeDeletedInlineWidget(text), {
              side: -1,
              key: `del-inline-${ci}-${docPos}`,
            }))
          }
        }
      }

      // Block-level change border
      decos.push(Decoration.node(pos, pos + op.newNode.nodeSize, {
        class: `diff-changed-node diff-change-${ci}`,
        'data-diff-change': String(ci),
      }))
      pos += op.newNode.nodeSize
    }
  }

  return { decos, count }
}

// ─── Diff computation entry point ────────────────────────────────────────────
async function computeDiff() {
  const editor = diffEditor.value
  if (!editor) return
  isComputing.value = true

  // Sync setContent: load old version to get oldDoc, then new version for newDoc
  editor.commands.setContent(props.oldContent)
  const oldDoc = editor.state.doc.copy(editor.state.doc.content)

  editor.commands.setContent(props.newContent)
  const newDoc = editor.state.doc

  const { decos, count } = buildDecorations(oldDoc, newDoc)
  totalChanges.value = count
  currentChangeIdx.value = 0

  const decoSet = DecorationSet.create(newDoc, decos)
  editor.view.dispatch(editor.state.tr.setMeta(DIFF_KEY, decoSet))

  isComputing.value = false

  if (count > 0) {
    await nextTick()
    scrollToChange(0)
  }
}

// ─── Navigation ─────────────────────────────────────────────────────────────
function scrollToChange(idx: number) {
  nextTick(() => {
    const el = scrollEl.value?.querySelector(`.diff-change-${idx}`) as HTMLElement | null
    el?.scrollIntoView({ behavior: 'smooth', block: 'center' })
  })
}

function navigate(dir: 1 | -1) {
  if (totalChanges.value === 0) return
  currentChangeIdx.value = (currentChangeIdx.value + dir + totalChanges.value) % totalChanges.value
  scrollToChange(currentChangeIdx.value)
}

// ─── Lifecycle ──────────────────────────────────────────────────────────────
onMounted(() => {
  // Wait for Tiptap editor ready (useEditor resolves after onMounted)
  const timer = setInterval(() => {
    if (diffEditor.value) { clearInterval(timer); computeDiff() }
  }, 30)
})

watch(() => [props.oldContent, props.newContent], computeDiff)

onBeforeUnmount(() => { diffEditor.value?.destroy() })
</script>

<!-- ───────────────────────────────────────────────────────────────────────────
  scoped：Tiptap 编辑器容器 + Decoration class 样式（通过 :deep() 穿透）
──────────────────────────────────────────────────────────────────────────── -->
<style scoped>
/* Tiptap 编辑器内的块级 Decoration —— 节点本身的类 */
:deep(.diff-added-node) {
  border-left: 3px solid #22c55e !important;
  background: rgba(34, 197, 94, 0.10) !important;
  padding-left: 10px;
  border-radius: 0 4px 4px 0;
  margin: 2px 0;
}
:deep(.diff-changed-node) {
  border-left: 3px solid #f59e0b !important;
  background: rgba(245, 158, 11, 0.08) !important;
  padding-left: 10px;
  border-radius: 0 4px 4px 0;
  margin: 2px 0;
}

/* 字符级新增文本 */
:deep(.diff-ins-text) {
  background: rgba(34, 197, 94, 0.30);
  border-radius: 2px;
  padding: 0 1px;
}
</style>

<!-- ───────────────────────────────────────────────────────────────────────────
  全局（非 scoped）：Decoration.widget 动态创建的 DOM 元素样式
  使用 diff-dv- 前缀避免全局污染
──────────────────────────────────────────────────────────────────────────── -->
<style>
/* 删除块 widget（取代删除的整个段落/标题/列表等） */
.diff-del-widget-block {
  border-left: 3px solid #ef4444;
  background: rgba(239, 68, 68, 0.10);
  padding: 4px 0 4px 10px;
  margin: 2px 0;
  border-radius: 0 4px 4px 0;
  display: block;
}
.diff-del-widget-block span {
  text-decoration: line-through;
  color: #b91c1c;
  opacity: 0.85;
}

/* 删除文本 widget（行内删除线） */
.diff-del-widget-inline {
  text-decoration: line-through;
  background: rgba(239, 68, 68, 0.18);
  color: #b91c1c;
  border-radius: 2px;
  padding: 0 1px;
}
</style>
