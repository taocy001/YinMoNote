<template>
  <!-- Mobile format toolbar — shown only on mobile (<768px).
       Provides formatting shortcuts in a persistent scrollable bar above
       the virtual keyboard. @mousedown.prevent keeps the editor focused. -->
  <div
    v-show="!isReadOnly"
    class="md:hidden shrink-0 flex items-center overflow-x-auto"
    style="background:var(--bg-sidebar);border-top:1px solid var(--border);min-height:44px;scrollbar-width:none;-webkit-overflow-scrolling:touch">
    <div class="flex items-center px-1 gap-0.5 py-1">
      <!-- Bold -->
      <button
class="mobile-fmt-btn text-sm font-bold"
        :style="editor?.isActive('bold') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleBold().run()">B</button>
      <!-- Italic -->
      <button
class="mobile-fmt-btn text-sm italic"
        :style="editor?.isActive('italic') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleItalic().run()">I</button>
      <!-- Strikethrough -->
      <button
class="mobile-fmt-btn text-sm line-through"
        :style="editor?.isActive('strike') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleStrike().run()">S</button>
      <!-- Inline code -->
      <button
class="mobile-fmt-btn ts-xs font-mono"
        :style="editor?.isActive('code') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleCode().run()">{ }</button>
      <!-- Separator -->
      <div class="w-px h-5 mx-1 shrink-0" style="background:var(--border)"></div>
      <!-- H1 -->
      <button
class="mobile-fmt-btn ts-sm font-bold"
        :style="editor?.isActive('heading',{level:1}) ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleHeading({level:1}).run()">H1</button>
      <!-- H2 -->
      <button
class="mobile-fmt-btn ts-sm font-bold"
        :style="editor?.isActive('heading',{level:2}) ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleHeading({level:2}).run()">H2</button>
      <!-- H3 -->
      <button
class="mobile-fmt-btn ts-sm font-bold"
        :style="editor?.isActive('heading',{level:3}) ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleHeading({level:3}).run()">H3</button>
      <!-- Separator -->
      <div class="w-px h-5 mx-1 shrink-0" style="background:var(--border)"></div>
      <!-- Bullet list -->
      <button
class="mobile-fmt-btn"
        :style="editor?.isActive('bulletList') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        :title="t.cmdUL"
        @mousedown.prevent="editor?.chain().focus().toggleBulletList().run()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><circle cx="2.5" cy="4.5" r="1.2" fill="currentColor"/><path d="M5.5 4.5h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><circle cx="2.5" cy="8" r="1.2" fill="currentColor"/><path d="M5.5 8h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><circle cx="2.5" cy="11.5" r="1.2" fill="currentColor"/><path d="M5.5 11.5h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
      </button>
      <!-- Ordered list -->
      <button
class="mobile-fmt-btn"
        :style="editor?.isActive('orderedList') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        :title="t.cmdOL"
        @mousedown.prevent="editor?.chain().focus().toggleOrderedList().run()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M2 3.5v3M1 6.5h2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/><path d="M1 9.5h2c0 0 0-1.5-1-1.5s-1 1.2-1 1.2 1 1.3 2 1.3" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/><path d="M5.5 4.5h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><path d="M5.5 8h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><path d="M5.5 11.5h9" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
      </button>
      <!-- Task list -->
      <button
class="mobile-fmt-btn"
        :style="editor?.isActive('taskList') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        :title="t.cmdTodo"
        @mousedown.prevent="editor?.chain().focus().toggleTaskList().run()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="1.5" y="3" width="4.5" height="4.5" rx="1" stroke="currentColor" stroke-width="1.3"/><path d="M2.5 5.5l1.2 1.2 2-2.5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/><path d="M8.5 5.5h6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/><rect x="1.5" y="9" width="4.5" height="4.5" rx="1" stroke="currentColor" stroke-width="1.3"/><path d="M8.5 11.5h6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>
      </button>
      <!-- Blockquote -->
      <button
class="mobile-fmt-btn"
        :style="editor?.isActive('blockquote') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        :title="t.cmdQuote"
        @mousedown.prevent="editor?.chain().focus().toggleBlockquote().run()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M2 4h3.5v4c0 1.5-1 2.5-3.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/><path d="M8 4h3.5v4c0 1.5-1 2.5-3.5 3.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
      <!-- Code block -->
      <button
class="mobile-fmt-btn ts-xs font-mono"
        :style="editor?.isActive('codeBlock') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        :title="t.cmdCode"
        @mousedown.prevent="editor?.chain().focus().toggleCodeBlock().run()">&lt;/&gt;</button>
      <!-- Separator -->
      <div class="w-px h-5 mx-1 shrink-0" style="background:var(--border)"></div>
      <!-- Highlight -->
      <button
class="mobile-fmt-btn"
        :style="editor?.isActive('highlight') ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
        @mousedown.prevent="editor?.chain().focus().toggleHighlight().run()">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M11 3L5 9l-1 3 3-1 6-6-2-2z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M2 14h4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
      </button>
      <!-- Horizontal rule -->
      <button
class="mobile-fmt-btn text-base"
        style="color:var(--text-secondary)"
        :title="t.cmdHR" @mousedown.prevent="editor?.chain().focus().setHorizontalRule().run()">—</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { inject, ref, type Ref } from 'vue'
import type { Editor as TiptapEditor } from '@tiptap/core'

defineProps<{
  /** TipTap editor instance for executing formatting commands */
  editor: TiptapEditor | undefined
  /** i18n translation object */
  t: Record<string, any>
}>()

const isReadOnly = inject<Ref<boolean>>('isReadOnly', ref(false))
</script>
