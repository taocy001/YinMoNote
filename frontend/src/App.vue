<template>
  <div :class="['flex h-screen overflow-hidden', isDark ? 'dark' : '']" style="background: var(--bg-app); color: var(--text-primary); font-family: inherit;">
    <!-- Left Sidebar: Note Navigation (always present on desktop; collapses to 44px icon bar) -->
    <aside v-if="isDesktop || showMobileSidebar" :class="sidebarClass" :style="sidebarStyle">
      <!-- Collapsed icon bar (desktop only) — click anywhere blank to expand -->
      <template v-if="isDesktop && !sidebarVisible">
        <div class="flex flex-col items-center gap-2 py-3 flex-1 cursor-pointer" :title="t.expandSidebar" @click="sidebarVisible = true">
          <!-- Expand toggle — first/top position -->
          <button class="sidebar-btn focus-ring w-8 h-8 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);" @click.stop="sidebarVisible = true">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M4.5 2.5L7.5 6L4.5 9.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
          <!-- New note icon (hidden when library is locked) -->
          <button v-if="!isLibraryLocked" class="sidebar-btn focus-ring w-8 h-8 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);" :title="t.newNote" @click.stop="createNewNote">
            <svg width="13" height="13" viewBox="0 0 14 14" fill="none"><path d="M7 2.5V11.5M2.5 7H11.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
          </button>
          <!-- Search icon (hidden when library is locked) -->
          <button v-if="!isLibraryLocked" class="sidebar-btn focus-ring w-8 h-8 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);" :title="t.searchPlaceholder" @click.stop="expandAndFocusSearch">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="1.3"/><path d="M9.5 9.5L12 12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
          </button>
          <!-- Spacer to push lock to bottom -->
          <div class="flex-1"></div>
          <!-- Lock (when unlocked) -->
          <button v-if="!isLibraryLocked && !isKeylessModeActive" class="sidebar-btn focus-ring w-8 h-8 flex items-center justify-center rounded-lg transition-all active:scale-[0.97] mb-1" style="color: var(--text-muted);" :title="t.lockLibrary" @click.stop="handleLockLibrary">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><rect x="2.5" y="6.5" width="9" height="6" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
          </button>
        </div>
      </template>

      <!-- Expanded sidebar content -->
      <template v-else>
        <!-- Header: logo (clickable on desktop) on left, collapse/close arrow on right -->
        <div class="px-3 pt-3 pb-2 flex items-center gap-2 shrink-0">
          <!-- Logo: clickable to collapse on desktop -->
          <button v-if="isDesktop" class="flex items-center gap-1.5 flex-1 min-w-0 px-1 py-1 rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-primary);" :title="t.collapseSidebar" @click="sidebarVisible = false">
            <div class="w-6 h-6 flex items-center justify-center rounded-md shrink-0" style="background: var(--accent-light); color: var(--accent);">
              <svg width="14" height="14" viewBox="0 0 18 18" fill="none"><path d="M9 2C9 2 3.5 7.5 3.5 11a5.5 5.5 0 0 0 11 0C14.5 7.5 9 2 9 2z" fill="currentColor" opacity="0.9"/><ellipse cx="11" cy="10" rx="1.5" ry="2" fill="white" opacity="0.35"/></svg>
            </div>
            <span class="font-bold ts-base tracking-[-0.02em] truncate">{{ t.logo }}</span>
          </button>
          <!-- Mobile: non-clickable logo -->
          <div v-if="!isDesktop" class="flex items-center gap-1.5 flex-1 min-w-0 px-1">
            <div class="w-6 h-6 flex items-center justify-center rounded-md shrink-0" style="background: var(--accent-light); color: var(--accent);">
              <svg width="14" height="14" viewBox="0 0 18 18" fill="none"><path d="M9 2C9 2 3.5 7.5 3.5 11a5.5 5.5 0 0 0 11 0C14.5 7.5 9 2 9 2z" fill="currentColor" opacity="0.9"/><ellipse cx="11" cy="10" rx="1.5" ry="2" fill="white" opacity="0.35"/></svg>
            </div>
            <span class="font-bold ts-base tracking-[-0.02em] truncate" style="color: var(--text-primary);">{{ t.logo }}</span>
          </div>
          <!-- Collapse button (desktop) — right side, where + was -->
          <button v-if="isDesktop" class="sidebar-btn focus-ring w-8 h-8 flex items-center justify-center rounded-lg transition-all active:scale-[0.97] shrink-0" style="color: var(--text-muted);" :title="t.collapseSidebar" @click="sidebarVisible = false">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M7.5 2.5L4.5 6L7.5 9.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
          <!-- Mobile close -->
          <button v-if="!isDesktop" class="sidebar-btn w-8 h-8 flex items-center justify-center rounded-lg transition-all shrink-0" style="color: var(--text-muted);" @click="showMobileSidebar = false">
            <svg width="12" height="12" viewBox="0 0 12 12" fill="none"><path d="M2 2L10 10M10 2L2 10" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
          </button>
        </div>
        <!-- Sidebar content hidden when library is locked to prevent title leakage -->
        <template v-if="!isLibraryLocked">
        <!-- New Note + Import buttons -->
        <div class="px-3 pb-2 shrink-0 flex gap-2">
          <button data-testid="new-note-btn" class="new-note-btn focus-ring flex-1 flex items-center gap-2 px-3 py-2 rounded-xl ts-sm font-medium transition-all active:scale-[0.97]" style="background: var(--bg-hover); color: var(--text-secondary); border: 1px solid var(--border);" @click="createNewNote">
            <svg width="12" height="12" viewBox="0 0 14 14" fill="none"><path d="M7 2.5V11.5M2.5 7H11.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
            <span>{{ t.newNote }}</span>
          </button>
          <div class="relative">
            <button class="h-full px-2.5 rounded-xl ts-sm font-medium transition-all active:scale-[0.97]" style="background: var(--bg-hover); color: var(--text-muted); border: 1px solid var(--border);" :title="t.importNotes" @click="showImportMenu = !showImportMenu">
              <svg width="13" height="13" viewBox="0 0 16 16" fill="none"><path d="M9 1H4.5A1.5 1.5 0 0 0 3 2.5v11A1.5 1.5 0 0 0 4.5 15h7a1.5 1.5 0 0 0 1.5-1.5V5L9 1z" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/><path d="M8 7v4M6 9l2 2 2-2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
          </div>
        </div>
        <!-- Hidden file inputs for import -->
        <input ref="fileInputRef" type="file" multiple accept=".md,.txt" class="hidden" @change="onImportFiles" />
        <input ref="folderInputRef" type="file" webkitdirectory class="hidden" @change="onImportFolder" />
        <input ref="zipInputRef" type="file" accept=".zip" class="hidden" @change="onImportZip" />

        <!-- Search box -->
        <div class="px-3 pb-2 shrink-0">
          <div class="relative">
            <div class="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" style="color: var(--text-muted);">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="6" cy="6" r="4" stroke="currentColor" stroke-width="1.3"/><path d="M9.5 9.5L12 12" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            </div>
            <input
ref="searchInputRef" v-model="searchQuery" data-testid="search-input" :placeholder="t.searchPlaceholder"
              class="search-input focus-ring w-full pl-9 pr-3 py-2 ts-sm rounded-xl outline-none transition-all"
              style="background: var(--bg-editor); border: 1px solid var(--border); color: var(--text-primary); font-family: inherit;"
            />
            <!-- Indexing pulse -->
            <div v-if="isIndexing" class="absolute right-3 top-1/2 -translate-y-1/2 w-1.5 h-1.5 rounded-full animate-pulse" style="background: var(--accent);"></div>
          </div>
          <!-- Tag chips -->
          <div v-if="allTags.length > 0" class="flex flex-wrap gap-1 mt-2">
            <button
:style="activeTagFilter === '' ? 'background:var(--accent);color:white;border-color:var(--accent)' : 'background:transparent;color:var(--text-muted);border-color:var(--border)'"
              class="px-2.5 py-0.5 rounded-full ts-xs font-medium border transition-all"
              @click="activeTagFilter = ''">{{ t.tagFilterAll }}</button>
            <button
v-for="tag in allTags" :key="tag" :style="activeTagFilter === tag ? 'background:var(--accent);color:white;border-color:var(--accent)' : 'background:transparent;color:var(--text-muted);border-color:var(--border)'"
              class="px-2.5 py-0.5 rounded-full ts-xs font-medium border transition-all"
              @click="activeTagFilter = activeTagFilter === tag ? '' : tag"># {{ tag }}</button>
          </div>
        </div>

        <!-- Note list -->
        <nav class="flex-1 overflow-y-auto px-2 pb-20" style="scrollbar-width: thin; scrollbar-color: var(--border) transparent; touch-action: pan-y; -webkit-overflow-scrolling: touch;" @dragover.prevent="onSidebarDragOver" @drop="onSidebarDrop" @scroll="handleSidebarScroll">
          <div
v-for="item in displayList" :key="item.key"
            data-testid="note-item" :data-note-key="item.key"
            :draggable="isDesktop" :style="{ paddingLeft: (item.level * 14 + 8) + 'px', opacity: draggedKey === item.key ? '0.2' : '1' }" class="group relative flex items-center gap-2 h-[36px] my-[1px] rounded-lg cursor-pointer transition-all duration-150 select-none ts-sm pr-2" :class="currentNote === item.key ? 'note-item-active' : 'note-item-default'" @dragstart="onNoteDragStart($event, item.key)"
            @dragover.prevent.stop="onNoteDragOver($event, item.key)"
            @drop.prevent.stop="onNoteDrop($event, item.key)"
            @dragend="onNoteDragEnd"
            @click="selectNote(item.key)" @dblclick="selectNotePinned(item.key)"
          >
            <!-- Drag-drop position indicators -->
            <div v-if="dropTarget === item.key && dropPosition === 'before'" class="absolute -top-[1px] left-0 right-0 h-[2.5px] z-30 pointer-events-none" style="background: var(--accent);"></div>
            <div v-if="dropTarget === item.key && dropPosition === 'after'" class="absolute -bottom-[1px] left-0 right-0 h-[2.5px] z-30 pointer-events-none" style="background: var(--accent);"></div>

            <!-- Left accent bar for active state -->
            <div v-if="currentNote === item.key" class="absolute left-0 top-[6px] bottom-[6px] w-[2.5px] rounded-full" style="background: var(--accent);"></div>

            <!-- Collapse toggle -->
            <button v-if="item.hasChildren" class="shrink-0 w-4 h-4 flex items-center justify-center transition-all" style="color: var(--text-muted);" @click.stop="toggleCollapse(item.key)">
              <svg v-if="item.isCollapsed" width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M4.5 2.5L7.5 6L4.5 9.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
              <svg v-else width="10" height="10" viewBox="0 0 12 12" fill="none"><path d="M2.5 4.5L6 7.5L9.5 4.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
            <div v-else class="shrink-0 w-4"></div>

            <!-- Title -->
            <span class="truncate flex-1 leading-none font-[450]" :style="currentNote === item.key ? 'color: var(--accent); font-weight: 600;' : 'color: var(--text-secondary);'">{{ item.label }}</span>

            <!-- Content match badge -->
            <span v-if="item.contentMatch" class="shrink-0 ts-xs px-1.5 py-0.5 rounded-full font-semibold" style="background: rgba(234,179,8,0.15); color: var(--color-warning);">{{ t.contentMatch }}</span>

            <!-- Pin indicator (always visible when pinned) -->
            <svg v-if="item.isPinned" class="shrink-0" width="10" height="10" viewBox="0 0 16 16" fill="none" style="color: var(--accent); opacity: 0.7;"><path d="M9.828 2.172a1.5 1.5 0 0 1 2.121 0l1.879 1.879a1.5 1.5 0 0 1 0 2.121L11 9l-.5 3.5L7 9l-3.5.5L6.5 5l-2.828-2.828z" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round" fill="currentColor" fill-opacity="0.2"/><path d="M3.5 12.5L7 9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>

            <!-- More menu button: hover-only on desktop, always visible on mobile -->
            <button @click.stop="openNoteMenu($event, item.key)" class="md:opacity-0 md:group-hover:opacity-100 sidebar-btn w-5 h-5 flex items-center justify-center rounded transition-all shrink-0" style="color: var(--text-muted);" data-testid="note-more-btn">
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="3" r="1.2" fill="currentColor"/><circle cx="8" cy="8" r="1.2" fill="currentColor"/><circle cx="8" cy="13" r="1.2" fill="currentColor"/></svg>
            </button>

            <!-- Inline tag editor popup -->
            <Teleport v-if="tagEditKey === item.key" to="body">
              <div class="fixed inset-0 z-[150]" @click.self="cancelTagEdit">
                <div
class="absolute z-[160] p-3 rounded-xl w-56"
                  style="top: 50%; left: 50%; transform: translate(-50%, -50%); background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
                  <p class="text-xs font-bold mb-2" style="color: var(--text-muted);">{{ t.tagEdit }}</p>
                  <input
v-model="tagEditValue" :placeholder="t.tagPlaceholder" class="w-full px-2 py-1.5 text-sm rounded-lg border outline-none transition-all"
                    style="background: var(--bg-app); border-color: var(--border); color: var(--text-primary); font-family: inherit;"
                    autofocus
                    @keydown.enter="saveTagEdit"
                    @keydown.esc="cancelTagEdit" />
                  <div class="flex gap-2 mt-2">
                    <button class="flex-1 py-1 text-xs rounded-lg font-semibold transition-all" style="background: var(--accent); color: white;" @click="saveTagEdit">{{ t.tagSave }}</button>
                    <button class="flex-1 py-1 text-xs rounded-lg transition-all" style="background: var(--bg-hover); color: var(--text-secondary);" @click="cancelTagEdit">{{ t.close }}</button>
                  </div>
                </div>
              </div>
            </Teleport>
          </div>
          <!-- Load-more indicator -->
          <div v-if="displayList.length >= displayLimit" class="py-4 flex justify-center opacity-30">
            <div class="w-1.5 h-1.5 rounded-full bg-current animate-bounce mx-0.5"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-current animate-bounce mx-0.5 [animation-delay:0.2s]"></div>
            <div class="w-1.5 h-1.5 rounded-full bg-current animate-bounce mx-0.5 [animation-delay:0.4s]"></div>
          </div>
        </nav>
        </template>
        <!-- /locked guard -->

        <!-- Sidebar footer -->
        <div class="absolute bottom-0 left-0 right-0 px-3 py-2 shrink-0" style="background: var(--bg-sidebar); border-top: 1px solid var(--border); box-shadow: 0 -2px 8px rgba(0,0,0,0.04);">
          <div class="flex items-center justify-between">
            <!-- Trash button -->
            <button class="sidebar-btn flex items-center gap-2 px-2 py-1.5 rounded-lg ts-xs font-medium transition-all active:scale-[0.97]" :style="showTrash ? 'color:var(--accent)' : 'color:var(--text-muted)'" @click="showTrash = !showTrash">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none"><path d="M2.5 3.5h9M5 3.5V2.5a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1M3.5 3.5l.5 8a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
              <span>{{ t.trash }}</span>
              <span v-if="trashDisplayList.length > 0" class="ml-0.5 px-1 py-0 rounded-full ts-xs font-bold" style="background: var(--bg-hover);">{{ trashDisplayList.length }}</span>
            </button>
            <!-- Lock button -->
            <button v-if="!isLibraryLocked && !isKeylessModeActive" data-testid="sidebar-lock-btn" class="sidebar-btn flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg ts-sm font-medium transition-all active:scale-[0.97]" style="color: var(--text-secondary);" :title="t.lockLibrary" @click="handleLockLibrary">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none"><rect x="2.5" y="6.5" width="9" height="6" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
              <span>{{ t.lockLibrary }}</span>
            </button>
            <div v-else-if="isLibraryLocked" class="ts-xs font-medium" style="color: var(--text-muted);">{{ t.libLocked }}</div>
          </div>

          <!-- Trash panel (slides up from footer) -->
          <div v-if="showTrash" class="mt-2 rounded-xl overflow-hidden" style="background: var(--bg-editor); border: 1px solid var(--border);">
            <div v-if="trashDisplayList.length === 0" class="p-4 text-center ts-sm" style="color: var(--text-muted);">{{ t.trashEmpty }}</div>
            <div v-else>
              <div style="max-height: 200px; overflow-y: auto;">
                <div v-for="item in trashDisplayList" :key="item.key" class="trash-item flex items-center gap-2 px-3 py-2.5 group transition-colors" style="border-bottom: 1px solid var(--border);">
                  <svg class="shrink-0 opacity-40" width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><path d="M3 2h8v10a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2z" stroke="currentColor" stroke-width="1.2"/><path d="M3 2h8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/><path d="M5 5v4M7 5v4M9 5v4" stroke="currentColor" stroke-width="1" stroke-linecap="round" opacity="0.5"/></svg>
                  <div class="flex-1 min-w-0">
                    <div class="truncate ts-sm font-medium" style="color: var(--text-secondary);">{{ item.label }}</div>
                    <div class="ts-xs mt-0.5" style="color: var(--text-muted);">{{ t.trashDaysLeft(item.daysLeft) }}</div>
                  </div>
                  <button class="ts-xs px-2 py-1 rounded-md shrink-0 opacity-0 group-hover:opacity-100 transition-all" style="background: var(--accent-light); color: var(--accent);" @click="restoreNote(item.key)">{{ t.restoreNote }}</button>
                  <button class="ts-xs px-2 py-1 rounded-md shrink-0 opacity-0 group-hover:opacity-100 transition-all" style="background: rgba(239,68,68,0.08); color: var(--color-danger);" @click="permanentDeleteNote(item.key)">{{ t.permanentDelete }}</button>
                </div>
              </div>
              <button class="w-full py-2 ts-xs font-medium transition-all shrink-0" style="color: var(--color-danger); background: rgba(239,68,68,0.05); border-top: 1px solid var(--border);" @click="emptyTrash">{{ t.emptyTrash }}</button>
            </div>
          </div>
        </div>
      </template>

      <!-- Resize handle -->
      <div v-if="isDesktop && sidebarVisible" class="resize-handle absolute right-0 top-0 bottom-0 w-1 cursor-col-resize transition-colors" @mousedown="startDrag"></div>
    </aside>

    <main class="flex-1 flex flex-col min-w-0 relative">
      <!-- Mobile header -->
      <header class="md:hidden flex items-center gap-2 px-3 py-2 shrink-0" style="background: var(--bg-sidebar); border-bottom: 1px solid var(--border);">
        <!-- Hamburger -->
        <button class="w-9 h-9 flex items-center justify-center rounded-lg shrink-0 transition-all active:scale-[0.97]" style="color: var(--text-secondary);" @click="showMobileSidebar = true">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M2 4h12M2 8h12M2 12h12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </button>
        <!-- Note title — fills remaining space -->
        <span class="font-semibold text-sm truncate flex-1 min-w-0" style="color: var(--text-primary);">{{ currentNote ? (noteTitles[currentNote] || currentNote) : t.logo }}</span>
        <!-- Right action group -->
        <div class="flex items-center gap-0.5 shrink-0">
          <!-- Save-status pill: amber when dirty (tappable to save), pulsing dot when saving -->
          <button
v-if="editorRef?.saveStatus === 'dirty'" class="flex items-center gap-1 px-2 py-1 rounded-md transition-all active:scale-[0.97]"
            style="background: rgba(217,119,6,0.08);"
            @click="editorRef?.doSave()">
            <div class="w-1.5 h-1.5 rounded-full" style="background: var(--color-warning);"></div>
            <span class="ts-xs font-medium" style="color: var(--color-warning);">{{ t.unsaved }}</span>
          </button>
          <div v-else-if="editorRef?.saveStatus === 'saving'" class="w-2 h-2 rounded-full animate-pulse mx-2" style="background: var(--accent);"></div>
          <!-- More button — history / export (only when a note is open and unlocked) -->
          <button
v-if="currentNote && !isLibraryLocked" class="w-9 h-9 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]"
            :style="showMobileMore ? 'background:var(--accent-light);color:var(--accent)' : 'color:var(--text-secondary)'"
            @click="showMobileMore = !showMobileMore">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="3" r="1.2" fill="currentColor"/><circle cx="7" cy="7" r="1.2" fill="currentColor"/><circle cx="7" cy="11" r="1.2" fill="currentColor"/></svg>
          </button>
          <!-- Lock -->
          <button
v-if="!isLibraryLocked && !isKeylessModeActive" class="w-9 h-9 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]"
            style="color: var(--text-secondary);" @click="handleLockLibrary">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><rect x="2.5" y="6.5" width="9" height="6" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
          </button>
          <!-- Settings -->
          <button class="w-9 h-9 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-secondary);" @click="openSettings">
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="2" stroke="currentColor" stroke-width="1.3"/><path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.6 2.6l1.1 1.1M10.3 10.3l1.1 1.1M2.6 11.4l1.1-1.1M10.3 3.7l1.1-1.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
          </button>
        </div>
      </header>

      <!-- Import menu dropdown -->
      <Teleport to="body">
        <div v-if="showImportMenu" class="fixed inset-0 z-[140]" @click="showImportMenu = false">
          <div class="absolute z-[150] py-1 rounded-xl overflow-hidden anim-pop-in" style="top: 100px; left: 16px; background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg); min-width: 150px;">
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click="showImportMenu = false; fileInputRef?.click()">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><path d="M4 1h6l3 3v8a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1z" stroke="currentColor" stroke-width="1.2"/></svg>
              {{ t.importFiles }}
            </button>
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click="showImportMenu = false; folderInputRef?.click()">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><path d="M1.5 3.5V11a1 1 0 0 0 1 1h9a1 1 0 0 0 1-1V5.5a1 1 0 0 0-1-1H7l-1.5-1.5H2.5a1 1 0 0 0-1 1z" stroke="currentColor" stroke-width="1.2"/></svg>
              {{ t.importFolder }}
            </button>
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click="showImportMenu = false; zipInputRef?.click()">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><path d="M4 1h6l3 3v8a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2a1 1 0 0 1 1-1z" stroke="currentColor" stroke-width="1.2"/><path d="M6 4h2v1H6V4zm0 2h2v1H6V6zm0 2h2v2H6V8z" fill="currentColor" opacity="0.4"/></svg>
              {{ t.importZip }}
            </button>
          </div>
        </div>
      </Teleport>

      <!-- Import progress overlay -->
      <Teleport to="body">
        <div v-if="importProcessing" class="fixed inset-0 z-[400] flex items-center justify-center" style="background: rgba(255,255,255,0.8); backdrop-filter: blur(8px);">
          <div class="w-full max-w-xs p-6 rounded-2xl text-center space-y-4" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
            <div class="relative w-20 h-20 mx-auto">
              <svg class="w-full h-full" viewBox="0 0 36 36">
                <path style="color: var(--border);" stroke-width="3" stroke="currentColor" fill="none" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                <path style="color: var(--accent);" stroke-width="3" :stroke-dasharray="`${importProgress}, 100`" stroke-linecap="round" stroke="currentColor" fill="none" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
              </svg>
              <div class="absolute inset-0 flex items-center justify-center text-sm font-black" style="color: var(--text-primary);">{{ importProgress }}%</div>
            </div>
            <p class="font-bold text-sm" style="color: var(--text-secondary);">{{ t.importProgress }}</p>
            <p class="text-xs truncate" style="color: var(--text-muted);">{{ importStatus }}</p>
          </div>
        </div>
      </Teleport>

      <!-- Import results modal -->
      <Teleport to="body">
        <div v-if="showImportResults" class="fixed inset-0 z-[400] flex items-center justify-center" style="background: rgba(0,0,0,0.4);" @click.self="showImportResults = false">
          <div class="w-full max-w-sm p-5 rounded-2xl space-y-3" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
            <p class="font-bold text-sm" style="color: var(--text-primary);">{{ t.importDone }}</p>
            <div class="flex gap-3 text-xs font-medium">
              <span style="color: var(--color-success);">{{ importSuccessCount }} ✓</span>
              <span v-if="importSkipCount > 0" style="color: var(--color-warning);">{{ importSkipCount }} ⚠️</span>
              <span v-if="importFailCount > 0" style="color: var(--color-danger);">{{ importFailCount }} ✗</span>
            </div>
            <div v-if="importFailedList.length > 0" class="max-h-48 overflow-y-auto rounded-lg" style="border: 1px solid var(--border);">
              <div v-for="(r, i) in importFailedList" :key="i" class="flex items-center gap-2 px-3 py-1.5 ts-xs" style="border-bottom: 1px solid var(--border);">
                <span :style="r.status === 'skipped' ? 'color: var(--color-warning)' : 'color: var(--color-danger)'">{{ r.status === 'skipped' ? '⚠️' : '❌' }}</span>
                <span class="truncate flex-1" style="color: var(--text-secondary);">{{ r.fileName }}</span>
                <span class="shrink-0" style="color: var(--text-muted);">{{ importReasonText(r.reason) }}</span>
              </div>
            </div>
            <button class="w-full py-2 rounded-xl text-sm font-semibold transition-all" style="background: var(--accent); color: white;" @click="showImportResults = false">OK</button>
          </div>
        </div>
      </Teleport>

      <!-- Note context menu (⋯ dropdown) -->
      <Teleport to="body">
        <div v-if="noteMenuKey" class="fixed inset-0 z-[140]" @click="closeNoteMenu">
          <div
class="absolute z-[150] py-1 rounded-xl overflow-hidden anim-pop-in"
            :style="{ top: noteMenuPos.top + 'px', left: noteMenuPos.left + 'px', background: 'var(--bg-editor)', border: '1px solid var(--border)', boxShadow: 'var(--shadow-lg)', minWidth: '150px' }">
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click.stop="noteMenuAction(k => togglePin(k))">
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" style="color: var(--text-muted);"><path d="M9.828 2.172a1.5 1.5 0 0 1 2.121 0l1.879 1.879a1.5 1.5 0 0 1 0 2.121L11 9l-.5 3.5L7 9l-3.5.5L6.5 5l-2.828-2.828z" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/><path d="M3.5 12.5L7 9" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/></svg>
              {{ structure.pinned?.includes(noteMenuKey!) ? t.unpinNote : t.pinNote }}
            </button>
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click.stop="noteMenuAction(k => openTagEdit(k, noteTags[k] || []))">
              <svg width="12" height="12" viewBox="0 0 12 12" fill="none" style="color: var(--text-muted);"><path d="M1.5 1.5h4.2l4.8 4.8-4.2 4.2L1.5 5.7V1.5z" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/><circle cx="4" cy="4" r="0.8" fill="currentColor"/></svg>
              {{ t.tagEdit }}
            </button>
            <button class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--text-secondary);" @click.stop="noteMenuAction(k => createSubNote(k))">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--text-muted);"><path d="M7 2.5V11.5M2.5 7H11.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
              {{ t.createSubNote }}
            </button>
            <div style="border-top: 1px solid var(--border); margin: 2px 8px;"></div>
            <button data-testid="note-delete-btn" class="note-menu-item flex items-center gap-3 w-full px-3 py-2 ts-sm text-left transition-colors" style="color: var(--color-danger);" @click.stop="noteMenuAction(k => confirmDeleteNote(k))">
              <svg width="12" height="12" viewBox="0 0 14 14" fill="none" style="color: var(--color-danger);"><path d="M2.5 3.5h9M5 3.5V2.5a1 1 0 0 1 1-1h2a1 1 0 0 1 1 1v1M3.5 3.5l.5 8a1 1 0 0 0 1 1h4a1 1 0 0 0 1-1l.5-8" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/></svg>
              {{ t.delete }}
            </button>
          </div>
        </div>
      </Teleport>

      <!-- Mobile more menu (history + export) -->
      <Teleport to="body">
        <div v-if="showMobileMore" class="fixed inset-0 z-[90]" @click="showMobileMore = false">
          <div
class="absolute right-3 top-14 rounded-xl overflow-hidden anim-pop-in"
            style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg); min-width: 160px;">
            <button
class="flex items-center gap-3 w-full px-4 py-3 text-sm text-left transition-all active:opacity-70"
              style="color: var(--text-secondary);"
              @click.stop="editorRef?.toggleHistory(); showMobileMore = false">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="5" stroke="currentColor" stroke-width="1.3"/><path d="M7 4v3.5l2 1.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
              {{ t.historyBtn }}
            </button>
            <div style="border-top: 1px solid var(--border);">
              <button
class="flex items-center gap-3 w-full px-4 py-3 text-sm text-left transition-all active:opacity-70"
                style="color: var(--text-secondary);"
                @click.stop="editorRef?.exportHTML(); showMobileMore = false">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M7 1v8M4 6l3 3 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/><path d="M2 10v2h10v-2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
                {{ t.exportHTML }}
              </button>
              <button
class="flex items-center gap-3 w-full px-4 py-3 text-sm text-left transition-all active:opacity-70"
                style="color: var(--text-secondary); border-top: 1px solid var(--border);"
                @click.stop="editorRef?.exportPDF(); showMobileMore = false">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><path d="M7 1v8M4 6l3 3 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/><path d="M2 10v2h10v-2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
                {{ t.exportPDF }}
              </button>
              <button
class="flex items-center gap-3 w-full px-4 py-3 text-sm text-left transition-all active:opacity-70"
                style="color: var(--text-secondary); border-top: 1px solid var(--border);"
                @click.stop="editorRef?.exportMarkdown(); showMobileMore = false">
                <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><rect x="1.5" y="2" width="11" height="10" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4 7l2 2 4-4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round"/></svg>
                {{ t.exportMarkdown }}
              </button>
            </div>
          </div>
        </div>
      </Teleport>

      <div class="flex-1 overflow-hidden flex flex-col">
        <!-- Tab bar — desktop only, above editor -->
        <TabBar
v-if="!isLibraryLocked && !isSearchResultsVisible"
          :tabs="openTabs" :current-note="currentNote" :titles="noteTitles" :preview-tab="previewTab"
          @select="selectNote" @close="closeTab" @pin="pinPreviewTab" @reorder="reorderTabs"
        />

        <!-- Search results panel — replaces editor while a search query is active -->
        <SearchResults
v-if="isSearchResultsVisible"
          :query="debouncedSearch"
          :content-index="contentIndex"
          :note-titles="noteTitles"
          :t="t"
          @open-note="openFromSearchResult"
        />

        <Editor v-else-if="currentNote && !isLibraryLocked" :key="currentNote + (searchHighlight || '')" ref="editorRef" :note-file-name="currentNote" :is-dark="isDark" :search-highlight="searchHighlight" :search-offset="searchOffset" :commit-labels="structure.commitLabels || {}" @title-changed="onTitleChanged" @open-settings="openSettings" @set-label="setCommitLabel" />

        <!-- Empty state -->
        <div v-else-if="!currentNote" class="h-full flex flex-col" style="background: var(--bg-editor);">
          <!-- Toolbar row — desktop only, matches Editor.vue header style -->
          <div class="hidden md:flex items-center justify-end px-4 py-2 shrink-0" style="border-bottom: 1px solid var(--border);">
            <button data-testid="empty-state-settings-btn" class="w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);" :title="t.settings" @click="openSettings">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="2" stroke="currentColor" stroke-width="1.3"/><path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.6 2.6l1.1 1.1M10.3 10.3l1.1 1.1M2.6 11.4l1.1-1.1M10.3 3.7l1.1-1.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            </button>
          </div>
          <div class="flex-1 flex items-center justify-center flex-col gap-5 empty-state-bg">
            <div class="w-20 h-20 rounded-2xl flex items-center justify-center" style="background: var(--accent-light); color: var(--accent);">
              <svg width="36" height="36" viewBox="0 0 18 18" fill="none"><path d="M9 2C9 2 3.5 7.5 3.5 11a5.5 5.5 0 0 0 11 0C14.5 7.5 9 2 9 2z" fill="currentColor" opacity="0.85"/><ellipse cx="11" cy="10" rx="1.5" ry="2" fill="white" opacity="0.3"/></svg>
            </div>
            <p class="text-sm font-medium" style="color: var(--text-muted);">{{ t.notSelected }}</p>
            <button data-testid="empty-state-new-note-btn" class="px-5 py-3 rounded-xl text-sm font-semibold transition-all active:scale-[0.97]" style="background: var(--accent); color: white; box-shadow: 0 2px 8px rgba(79,70,229,0.3);" @click="createNewNote">{{ t.newNote }}</button>
          </div>
        </div>

        <!-- Locked state -->
        <div v-else-if="isLibraryLocked" class="h-full flex flex-col" style="background: var(--bg-editor);">
          <!-- Toolbar row — desktop only, matches Editor.vue header style -->
          <div class="hidden md:flex items-center justify-end px-4 py-2 shrink-0" style="border-bottom: 1px solid var(--border);">
            <button class="w-7 h-7 flex items-center justify-center rounded-lg transition-all active:scale-[0.97]" style="color: var(--text-muted);" :title="t.settings" @click="openSettings">
              <svg width="14" height="14" viewBox="0 0 14 14" fill="none"><circle cx="7" cy="7" r="2" stroke="currentColor" stroke-width="1.3"/><path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.6 2.6l1.1 1.1M10.3 10.3l1.1 1.1M2.6 11.4l1.1-1.1M10.3 3.7l1.1-1.1" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            </button>
          </div>
          <div class="flex-1 flex items-center justify-center flex-col gap-4">
            <div class="w-16 h-16 rounded-2xl flex items-center justify-center" style="background: var(--accent-light); color: var(--accent);">
              <svg width="28" height="28" viewBox="0 0 14 14" fill="none"><rect x="2.5" y="6.5" width="9" height="6" rx="1.5" stroke="currentColor" stroke-width="1.3"/><path d="M4.5 6.5V4.5a2.5 2.5 0 0 1 5 0v2" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>
            </div>
            <p class="font-semibold text-sm" style="color: var(--text-secondary);">{{ t.libLocked }}</p>
            <button class="px-6 py-3 rounded-xl font-semibold text-sm transition-all active:scale-[0.97]" style="background: var(--accent); color: white; box-shadow: 0 4px 16px rgba(79,70,229,0.35);" @click="showUnlockModal = true">{{ t.libUnlockBtn }}</button>
          </div>
        </div>
      </div>

      <!-- Batch processing overlay -->
      <Teleport to="body">
        <div v-if="batchProcessing" class="fixed inset-0 z-[400] flex items-center justify-center" style="background: rgba(255,255,255,0.8); backdrop-filter: blur(8px);">
          <div class="w-full max-w-xs p-6 rounded-2xl text-center space-y-4" style="background: var(--bg-editor); border: 1px solid var(--border); box-shadow: var(--shadow-lg);">
            <div class="relative w-20 h-20 mx-auto">
              <svg class="w-full h-full" viewBox="0 0 36 36">
                <path style="color: var(--border);" stroke-width="3" stroke="currentColor" fill="none" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                <path style="color: var(--accent);" stroke-width="3" :stroke-dasharray="`${batchProgress}, 100`" stroke-linecap="round" stroke="currentColor" fill="none" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
              </svg>
              <div class="absolute inset-0 flex items-center justify-center text-sm font-black" style="color: var(--text-primary);">{{ batchProgress }}%</div>
            </div>
            <p class="font-bold text-sm" style="color: var(--text-secondary);">{{ t.convertingEncryption }}</p>
          </div>
        </div>
      </Teleport>

      <!-- Settings panel -->
      <SettingsPanel
        ref="settingsPanelRef"
        v-model="showSettings"
        :t="t"
        :draft-settings="draftSettings"
        :batch-processing="batchProcessing"
        :show-settings-close-confirm="showSettingsCloseConfirm"
        :export-key-status="exportKeyStatus"
        :exported-key-text="exportedKeyText"
        :batch-result-msg="batchResultMsg"
        :settings-save-error="settingsSaveError"
        :reset-is-hardware="resetIsHardware"
        :is-keyless-mode-active="isKeylessModeActive"
        :settings-tab="settingsTab"
        :mcp-token-value="mcpTokenValue"
        :mcp-token-set="mcpTokenSet"
        :mcp-token-loading="mcpTokenLoading"
        :mcp-token-error="mcpTokenError"
        :mcp-ca-fingerprint="mcpCaFingerprint"
        :mcp-ca-expiry="mcpCaExpiry"
        :webdav-token-value="webdavTokenValue"
        :webdav-token-set="webdavTokenSet"
        :webdav-token-loading="webdavTokenLoading"
        :webdav-token-error="webdavTokenError"
        @close="closeSettings"
        @apply="applySettings(true)"
        @force-close="showSettings = false; showSettingsCloseConfirm = false; mcpTokenValue = ''; webdavTokenValue = ''"
        @open-reset-modal="openResetModal"
        @export-key="handleExportMasterKey"
        @change-password="handleChangePassword"
        @mcp-generate-token="handleMCPGenerateToken"
        @mcp-revoke-token="handleMCPRevokeToken"
        @webdav-generate-token="handleWebDAVGenerateToken"
        @webdav-revoke-token="handleWebDAVRevokeToken"
        @update:draft-settings="v => Object.assign(draftSettings, v)"
        @update:show-settings-close-confirm="v => showSettingsCloseConfirm = v"
        @update:settings-tab="v => settingsTab = v as 'appearance' | 'editor' | 'security' | 'ai'"
      />

      <!-- Command palette (Cmd+K / Ctrl+K) — hidden when library is locked -->
      <CommandPalette
        v-if="!isLibraryLocked"
        v-model="showCommandPalette"
        :titles="noteTitles"
        :note-keys="structure.order || []"
        :recent-notes="recentNoteKeys"
        :t="t"
        @select-note="selectNote"
        @new-note="createNewNote"
        @open-settings="openSettings"
        @lock="handleLockLibrary"
        @toggle-theme="cycleTheme"
        @toggle-trash="showTrash = !showTrash"
      />

      <!-- Unlock modal -->
      <UnlockModal
        v-model="showUnlockModal"
        :is-dark="isDark"
        :has-library-key="hasLibraryKey"
        :has-server-notes="hasServerNotes"
        :unlock-mode="unlockMode"
        :unlock-password="unlockPassword"
        :import-key-text="importKeyText"
        :is-unlocking="isUnlocking"
        :unlock-error="unlockError"
        :unlock-error-msg="unlockErrorMsg"
        :show-password="showPassword"
        :t="t"
        @unlock="handleUnlock"
        @update:unlock-mode="v => unlockMode = v as typeof unlockMode"
        @update:unlock-password="v => unlockPassword = v"
        @update:import-key-text="v => importKeyText = v"
        @update:show-password="v => showPassword = v"
      />

      <!-- Delete note confirmation modal -->
      <Teleport to="body">
        <Transition name="fade">
          <div v-if="deleteTargetKey" class="fixed inset-0 z-[400] flex items-center justify-center p-4" style="background: rgba(0,0,0,0.45); backdrop-filter: blur(3px);">
            <div class="w-full max-w-xs rounded-2xl overflow-hidden" style="background: var(--bg-editor); box-shadow: var(--shadow-lg); border: 1px solid var(--border);">
              <div class="px-6 pt-6 pb-4">
                <p class="font-bold text-sm mb-1" style="color: var(--text-primary);">{{ t.deleteModalTitle }}</p>
                <p class="ts-sm leading-relaxed" style="color: var(--text-muted);">{{ deleteModalBodyText }}</p>
              </div>
              <div class="px-6 pb-6 flex gap-3">
                <button data-testid="delete-cancel-btn" class="flex-1 py-2 rounded-xl text-sm font-semibold transition-all" style="background: var(--bg-hover); color: var(--text-secondary);" @click="cancelDelete">{{ t.libResetModalCancel }}</button>
                <button data-testid="delete-confirm-btn" class="flex-1 py-2 rounded-xl text-sm font-bold transition-all" style="background: var(--color-danger); color: white;" @click="executeDelete">{{ t.delete }}</button>
              </div>
            </div>
          </div>
        </Transition>
      </Teleport>

      <!-- Reset library confirmation modal -->
      <ResetModal
        v-model="showResetModal"
        :lang="lang"
        :t="t"
        :reset-countdown-total="config.resetCountdownSeconds"
        :is-keyless-mode-active="isKeylessModeActive"
        :reset-is-hardware="resetIsHardware"
        :reset-countdown="resetCountdown"
        :reset-executing="resetExecuting"
        :reset-error="resetError"
        :reset-password="resetPassword"
        :show-reset-password="showResetPassword"
        @close="closeResetModal"
        @confirm="handleResetLibrary"
        @update:reset-password="v => resetPassword = v"
        @update:show-reset-password="v => showResetPassword = v"
      />

      <!-- Notes list load failure notification -->
      <Teleport to="body">
        <Transition name="fade">
          <div v-if="notesLoadError" class="fixed bottom-6 left-1/2 -translate-x-1/2 z-[500] px-5 py-3 rounded-2xl text-sm font-medium shadow-xl" style="background: var(--color-danger-light); color: var(--color-danger); border: 1px solid var(--color-danger-light); max-width: 90vw; text-align: center;">
            {{ t.notesLoadFailed ?? (lang === 'zh' ? '笔记列表加载失败，请检查网络后刷新页面。' : 'Failed to load notes. Please check your connection and refresh.') }}
          </div>
        </Transition>
      </Teleport>

      <!-- Server encryption state change notification -->
      <Teleport to="body">
        <Transition name="fade">
          <div v-if="encryptionChangedByServer" class="fixed bottom-6 left-1/2 -translate-x-1/2 z-[500] px-5 py-3 rounded-2xl text-sm font-medium shadow-xl" style="background: var(--color-warning-light); color: var(--color-warning); border: 1px solid var(--color-warning-light); max-width: 90vw; text-align: center;">
            {{ t.encryptionStateChangedByServer ?? (lang === 'zh' ? '服务器已更改加密设置，当前加密状态已同步。' : 'Encryption setting was overridden by the server.') }}
          </div>
        </Transition>
      </Teleport>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, provide, reactive, computed, nextTick, type Ref } from 'vue'
import axios from 'axios'
import Editor from './components/Editor.vue'
import SearchResults from './components/SearchResults.vue'
import UnlockModal from './components/UnlockModal.vue'
import ResetModal from './components/ResetModal.vue'
import SettingsPanel from './components/SettingsPanel.vue'
import CommandPalette from './components/CommandPalette.vue'
import * as crypto from './crypto'
import { useI18n, type Lang } from './i18n'
import { useLibrary } from './composables/useLibrary'
import { useBatchEncryption } from './composables/useBatchEncryption'
import { useDragDrop } from './composables/useDragDrop'
import { syncChannel } from './composables/useEditorSave'
import TabBar from './components/TabBar.vue'
import { useBatchImport, filesFromDrop } from './composables/useBatchImport'
import { config } from './config'
import { clearCache as clearIndexCache } from './indexCache'

// Initialise window.name with a cryptographically random 32-char hex value on the
// first visit to this tab. The session wrap key (used to protect the copy of the
// master key stored in sessionStorage) is derived from window.name via PBKDF2.
// Without this initialisation window.name defaults to '' which causes the code to
// fall back to the known constant 'default-session', making the wrap key trivially
// recoverable from sessionStorage alone.
// window.name persists across same-tab navigations (so restore-from-session keeps
// working) but is cleared when the tab is closed (session expiry by design).
if (!window.name || !/^[0-9a-f]{32}$/.test(window.name)) {
  const arr = window.crypto.getRandomValues(new Uint8Array(16))
  window.name = Array.from(arr).map(b => b.toString(16).padStart(2, '0')).join('')
}
// SEC-013: Clear window.name before the tab navigates away to a different origin.
// Modern browsers restrict cross-origin reads of window.name, but clearing it on
// unload eliminates the residual risk for older browsers and browsing-context reuse.
// Registered in onMounted / removed in onUnmounted to avoid leaking on HMR.
const clearWindowName = () => { window.name = '' }

const { lang, t, setLang } = useI18n()
const {
  structure, noteTitles, noteTags, currentNote, isLibraryLocked,
  searchQuery, debouncedSearch, displayList, displayLimit,
  hasServerNotes, notesLoadError,
  toggleCollapse,
  activeTagFilter, allTags, isIndexing, contentIndex,
  loadNotesList, saveStructure, createNewNote: _createNewNote, createSubNote: _createSubNote, deleteNote, silentDeleteNote, moveNote, setNoteTags, buildContentIndex, clearContentIndex, indexNote, scheduleOrphanCleanup, pendingNotes,
  togglePin, restoreNote, permanentDeleteNote, emptyTrash, setCommitLabel
} = useLibrary()

// Prune the current note if the user never typed anything into it, then switch / create.
// Only pending notes are candidates — if a note has been uploaded it has content on the server.
// We prune unless the editor is mounted and explicitly confirms content exists, which handles
// the case where the editor is still mounting (null ref) or has already shown the note is empty.
const pruneEmptyCurrentNote = () => {
  if (!currentNote.value) return
  if (!pendingNotes.has(currentNote.value)) return
  const hasContent = editorRef.value && !editorRef.value.isContentEmpty.value
  if (!hasContent) {
    const id = currentNote.value
    silentDeleteNote(id)
    const tidx = openTabs.value.indexOf(id)
    if (tidx >= 0) openTabs.value.splice(tidx, 1)
    currentNote.value = null
  }
}

let isCreatingNote = false

/** Creates a new untitled note, selects it, and hides the mobile sidebar. */
const createNewNote = async () => {
  if (isLibraryLocked.value) return // guard: locked library
  if (isCreatingNote) return
  isCreatingNote = true
  try {
    // If the current note is pending and still empty, reuse it (just focus the editor).
    // Treat a null editorRef (still mounting) as empty — a mounting editor has no user input.
    if (currentNote.value && pendingNotes.has(currentNote.value)) {
      const hasContent = editorRef.value && !editorRef.value.isContentEmpty.value
      if (!hasContent) {
        nextTick(() => (document.querySelector('.ProseMirror') as HTMLElement)?.focus())
        return
      }
    }
    pruneEmptyCurrentNote()
    await _createNewNote(() => (document.querySelector('.ProseMirror') as HTMLElement)?.focus())
    if (currentNote.value) { addTab(currentNote.value); pinPreviewTab() }
  } finally {
    isCreatingNote = false
  }
}

const selectNote = (k: string) => {
  if (isLibraryLocked.value) return // guard: locked library
  pruneEmptyCurrentNote()
  openAsPreview(k)
  currentNote.value = k
  showMobileSidebar.value = false
  // Track recent notes for command palette
  recentNoteKeys.value = [k, ...recentNoteKeys.value.filter(x => x !== k)].slice(0, 12)
}
const selectNotePinned = (k: string) => {
  pruneEmptyCurrentNote()
  pinPreviewTab(k)
  addTab(k)
  currentNote.value = k
  showMobileSidebar.value = false
}

const API_BASE = '/api'

// UI State
const showUnlockModal = ref(false); const hasLibraryKey = ref(crypto.hasLibrary())
const isKeylessModeActive = ref(crypto.wasKeylessPersisted())
// serverInitialized: true when GET /api/auth/status returns {initialized:true} on a device
// that has no local library token yet. Controls the "new-device unlock" code path.
const serverInitialized = ref(false)
const unlockMode = ref<'password' | 'device' | 'import' | 'none'>(
  crypto.hasLibrary()
    ? (crypto.wasKeylessPersisted() ? 'none'
      : crypto.hasHardwareKey() ? 'device'
      : 'password')
    : 'none'
); const unlockPassword = ref(''); const importKeyText = ref('')
const isUnlocking = ref(false); const unlockError = ref(false); const unlockErrorMsg = ref(''); const showPassword = ref(false); const idleTimeout = ref(Number(localStorage.getItem('yinmo_idle_timeout') || '0'))
// Theme: 'auto' follows system, 'light'/'dark' forced
const themeMode = ref<'auto' | 'light' | 'dark'>(
  (localStorage.getItem('themeMode') as 'auto' | 'light' | 'dark') || 'auto'
)
const systemPrefersDark = ref(window.matchMedia('(prefers-color-scheme: dark)').matches)
const isDark = computed(() =>
  themeMode.value === 'auto' ? systemPrefersDark.value : themeMode.value === 'dark'
)
// Listen for system theme changes
const _mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
const _onSystemThemeChange = (e: MediaQueryListEvent) => { systemPrefersDark.value = e.matches }
_mediaQuery.addEventListener('change', _onSystemThemeChange)

// Sync dark class on document whenever isDark changes (auto/manual)
watch(isDark, (dark) => { document.documentElement.classList.toggle('dark', dark) }, { immediate: true })
const cycleTheme = () => {
  themeMode.value = themeMode.value === 'auto' ? 'dark' : themeMode.value === 'dark' ? 'light' : 'auto'
  localStorage.setItem('themeMode', themeMode.value)
}

const sidebarVisible = ref(true)
// Sidebar drag-to-resize (see useDragDrop composable)
const { isDragging, sidebarWidth, onDrag, startDrag, stopDrag } = useDragDrop()
const isDesktop = ref(window.innerWidth >= 768); const showMobileSidebar = ref(false)
const showSettings = ref(false)
const showCommandPalette = ref(false)
const recentNoteKeys = ref<string[]>([])
const settingsTab = ref<'appearance' | 'editor' | 'security' | 'ai'>('appearance')
// MCP token & CA state (fetched/managed separately from draftSettings)
const mcpTokenValue = ref('')       // raw token shown once after generation
const mcpTokenSet = ref(false)      // whether a token hash exists on the server
const webdavTokenValue = ref('')    // raw WebDAV token shown once after generation
const webdavTokenSet = ref(false)   // whether a WebDAV token hash exists on the server
const mcpCaFingerprint = ref('')    // colon-separated SHA-256 from /api/mcp/ca-fingerprint
const mcpCaExpiry = ref('')
const mcpTokenLoading = ref(false)  // prevents double-click generating orphaned tokens
const mcpTokenError = ref('')       // surfaced to user when generate/revoke fails
const webdavTokenLoading = ref(false)
const webdavTokenError = ref('')
const editorRef = ref<{ saveStatus: string; doSave: () => Promise<void>; loadNote: () => Promise<void>; isContentEmpty: { value: boolean }; toggleHistory: () => void; exportHTML: () => Promise<void>; exportPDF: () => void; exportMarkdown: () => Promise<void> } | null>(null)
const showMobileMore = ref(false)
const searchInputRef = ref<HTMLInputElement | null>(null)
// When user clicks a search result, we open the note and pass the search term
// so the editor can highlight matches. Cleared when search query is emptied.
const searchHighlight = ref('')
const searchOffset = ref(-1)
const isSearchResultsVisible = computed(() => !!debouncedSearch.value.trim() && !isLibraryLocked.value)

// ── Trash UI state ──────────────────────────────────────────────────────
const showTrash = ref(false)
const trashDisplayList = computed(() => {
  return (structure.value.trash || []).map(e => {
    const daysLeft = Math.max(0, 30 - Math.floor((Date.now() - e.deletedAt) / 86400000))
    return { key: e.id, label: noteTitles[e.id] || e.id, daysLeft }
  })
})

// ── Tab bar state ────────────────────────────────────────────────────────
const TABS_KEY = 'yinmo_open_tabs'
let _initTabs: string[] = []
try { _initTabs = JSON.parse(localStorage.getItem(TABS_KEY) || '[]') } catch { /* corrupted */ }
const openTabs = ref<string[]>(Array.isArray(_initTabs) ? _initTabs : [])
watch(openTabs, (v) => localStorage.setItem(TABS_KEY, JSON.stringify(v)), { deep: true })

// After notes list loads, prune tabs that reference deleted/trashed notes.
// Watch the entire structure (deep) so changes to parents/childOrder/trash also trigger cleanup.
watch(() => [structure.value.order, structure.value.parents, structure.value.trash], () => {
  const liveIds = new Set([
    ...(structure.value.order || []),
    ...Object.keys(structure.value.parents || {}),
    ...(Object.values(structure.value.childOrder || {}).flat() as string[]),
  ])
  openTabs.value = openTabs.value.filter(id => liveIds.has(id))
})

const previewTab = ref<string | null>(null)

const addTab = (id: string) => {
  if (!openTabs.value.includes(id)) openTabs.value.push(id)
}
/** Open as preview: replaces the current preview tab instead of adding a new one. */
const openAsPreview = (id: string) => {
  if (openTabs.value.includes(id)) return // already a pinned tab
  // Remove old preview tab from the list
  if (previewTab.value && previewTab.value !== id) {
    const oldIdx = openTabs.value.indexOf(previewTab.value)
    if (oldIdx >= 0) openTabs.value.splice(oldIdx, 1)
  }
  if (!openTabs.value.includes(id)) openTabs.value.push(id)
  previewTab.value = id
}
/** Promote the preview tab to a pinned (permanent) tab. */
const pinPreviewTab = (id?: string) => {
  const target = id || previewTab.value
  if (target && previewTab.value === target) previewTab.value = null
}
const closeTab = (id: string) => {
  const idx = openTabs.value.indexOf(id)
  if (idx < 0) return
  openTabs.value.splice(idx, 1)
  if (currentNote.value === id) {
    // Switch to adjacent tab or clear
    currentNote.value = openTabs.value[Math.min(idx, openTabs.value.length - 1)] ?? null
  }
}
const reorderTabs = (fromIdx: number, toIdx: number) => {
  const arr = [...openTabs.value]
  const [moved] = arr.splice(fromIdx, 1)
  arr.splice(toIdx > fromIdx ? toIdx - 1 : toIdx, 0, moved)
  openTabs.value = arr
}

const openFromSearchResult = (id: string, charOffset: number) => {
  searchHighlight.value = debouncedSearch.value.trim()
  searchOffset.value = charOffset
  addTab(id); pinPreviewTab()
  currentNote.value = id
  searchQuery.value = ''
}

/** Expand sidebar and focus search input (used by collapsed search icon click). */
const expandAndFocusSearch = () => {
  sidebarVisible.value = true
  nextTick(() => searchInputRef.value?.focus())
}

// ─── Note context menu (⋯ button) ────────────────────────────────────────────
const noteMenuKey = ref<string | null>(null)
const noteMenuPos = ref({ top: 0, left: 0 })
const openNoteMenu = (e: MouseEvent, key: string) => {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  noteMenuPos.value = { top: rect.bottom + 4, left: Math.min(rect.left, window.innerWidth - 180) }
  noteMenuKey.value = key
}
const closeNoteMenu = () => { noteMenuKey.value = null }
const noteMenuAction = (action: (key: string) => void) => {
  const key = noteMenuKey.value
  if (!key) return
  closeNoteMenu()
  action(key)
}

// ─── Delete confirmation modal ────────────────────────────────────────────────
const deleteTargetKey = ref<string | null>(null)
const confirmDeleteNote = (k: string) => { deleteTargetKey.value = k }
const cancelDelete = () => { deleteTargetKey.value = null }
const deleteModalBodyText = computed(() => {
  const key = deleteTargetKey.value
  if (!key) return ''
  const title = noteTitles[key] || key
  const fn = t.value.deleteModalBodyTrash
  return typeof fn === 'function' ? (fn as (s: string) => string)(title) : String(fn)
})
const executeDelete = async () => {
  if (!deleteTargetKey.value) return
  const k = deleteTargetKey.value
  deleteTargetKey.value = null
  await deleteNote(k)
  // Remove from open tabs
  const tidx = openTabs.value.indexOf(k)
  if (tidx >= 0) openTabs.value.splice(tidx, 1)
  scheduleOrphanCleanup()
}

/**
 * Handles the unlock/init form submission. Derives the encryption key from the
 * selected mode (hardware, password, import, or keyless), initialises or verifies
 * the library, and transitions to the unlocked state.
 */
const handleUnlock = async () => {
  isUnlocking.value = true; unlockError.value = false
  try {
    // Keyless mode: no key derivation, no encryption — just mark library accessible.
    if (unlockMode.value === 'none') {
      crypto.setKeylessMode()
      crypto.persistKeylessMode()
      crypto.setServerEncrypt(false)
      serverEncrypt.value = false
      hasLibraryKey.value = true
      isKeylessModeActive.value = true
      isLibraryLocked.value = false; showUnlockModal.value = false
      await loadNotesList()
      return
    }
    let key: CryptoKey
    if (unlockMode.value === 'import') key = await crypto.importRawKey(importKeyText.value.trim())
    else key = unlockMode.value === 'device' ? await crypto.deriveKeyFromHardware() : await crypto.deriveKeyFromPassword(unlockPassword.value)

    // Detect "new-device unlock": the server already has an initialized library but this
    // device has no local verification token yet (serverInitialized was set in onMounted).
    // We cannot call verifyAndUnlockLibrary (localStorage is empty) and must NOT call
    // initLibrary before confirming the password — otherwise a wrong password would silently
    // register this device with the wrong key, causing [Decryption Error] on all notes.
    const isNewDeviceUnlock = serverInitialized.value && !crypto.hasLibrary()

    if (isNewDeviceUnlock) {
      // New device connecting to an already-initialized library.
      // Perform SRP handshake to prove password knowledge; if it succeeds,
      // the server issues a Bearer token which confirms the password is correct.
      const tokenInput = unlockMode.value === 'device'
        ? (crypto.getHardwareCredentialId() ?? '')
        : unlockPassword.value
      if (!tokenInput) throw new Error('Invalid')
      try {
        const token = await crypto.deriveSessionToken(tokenInput, API_BASE)
        // SRP handshake succeeded — password confirmed. Persist token and init library.
        crypto.storeSessionToken(token)
      } catch {
        crypto.lockLibrary()   // clears _key and any partial session state
        throw new Error('Invalid')
      }
      await crypto.initLibrary(key)
      hasLibraryKey.value = true
      serverInitialized.value = false
    } else if (!hasLibraryKey.value || unlockMode.value === 'import') {
      await crypto.initLibrary(key); hasLibraryKey.value = true
      // Import mode: clear any stale hardware credential only after init succeeds.
      if (unlockMode.value === 'import') { crypto.clearHardwareKey(); resetIsHardware.value = false }
    } else if (!await crypto.verifyAndUnlockLibrary(key)) {
      // Local verification failed. If the server has SRP configured, prove the
      // password via SRP handshake — if it passes, the local LIBRARY_KEY_STORE
      // was created with a stale salt. Re-init with the correct key.
      if (serverInitialized.value || (unlockMode.value === 'password' && unlockPassword.value)) {
        try {
          const tokenInput = unlockMode.value === 'device'
            ? (crypto.getHardwareCredentialId() ?? '') : unlockPassword.value
          const token = await crypto.deriveSessionToken(tokenInput, API_BASE)
          crypto.storeSessionToken(token)
          await crypto.initLibrary(key)
          hasLibraryKey.value = true
        } catch {
          throw new Error('Invalid')
        }
      } else {
        throw new Error('Invalid')
      }
    } else if (unlockMode.value === 'password') {
      // Password mode does not use a hardware credential. Clear any stale hw_cred_id so
      // the settings page does not mistakenly show the hardware-only "Export Key" button.
      // Only cleared AFTER successful verification to avoid losing the credential on typo.
      crypto.clearHardwareKey(); resetIsHardware.value = false
      // SEC-018: migrate legacy fixed-salt users to per-user random salt after successful unlock.
      await crypto.migrateLegacySaltIfNeeded(unlockPassword.value)
    }
    // Obtain a server-side Bearer token via SRP BEFORE unlocking the UI.
    // If this runs after isLibraryLocked=false, Vue flushes the DOM during the
    // SRP handshake and the Editor mounts with no Bearer token → 401.
    // Skip for: keyless (handled above), import mode (no password), and new-device
    // unlock (already obtained a token above).
    if (!isNewDeviceUnlock && (unlockMode.value === 'password' || unlockMode.value === 'device')) {
      try {
        const tokenInput = unlockMode.value === 'password'
          ? unlockPassword.value
          : (crypto.getHardwareCredentialId() ?? '')
        if (tokenInput) {
          if (!serverInitialized.value) {
            // First-time password setup: register SRP verifier with the server.
            // After this, subsequent logins use SRP handshake only (no re-setup needed).
            // If setup returns 401 it means the server already has a verifier (race: the
            // onMounted auth/status call was delayed and hadn't set serverInitialized yet).
            // Treat 401 as "already initialized" and fall through to the SRP handshake.
            try {
              const srpSaltBytes = window.crypto.getRandomValues(new Uint8Array(16))
              const srpSaltB64 = btoa(String.fromCharCode(...srpSaltBytes))
              const verifierHex = await crypto.srpComputeVerifier(srpSaltBytes, tokenInput)
              await axios.post(`${API_BASE}/auth/setup`, {
                srpSalt: srpSaltB64,
                srpVerifier: verifierHex,
              })
            } catch (setupErr: any) {
              if (setupErr?.response?.status !== 401) throw setupErr
              // 401 → server verifier already set; proceed to SRP handshake below.
            }
            serverInitialized.value = true
          }
          const token = await crypto.deriveSessionToken(tokenInput, API_BASE)
          crypto.storeSessionToken(token)
        }
      } catch (srpErr: any) {
        // srp_m2_mismatch means the server's proof of the shared key is wrong —
        // this is a real authentication failure (possible MITM), not a keyless
        // server. Re-throw so the unlock flow is aborted.
        if (srpErr?.message === 'srp_m2_mismatch') throw srpErr
        // Other errors (network, keyless server, etc.) are non-fatal.
      }
    }

    // Sync PBKDF2 salt to server so other devices can derive the same key
    if (unlockMode.value === 'password') {
      try {
        const saltB64 = crypto.getSaltBase64()
        if (saltB64) {
          const currentCfg = (await axios.get(`${API_BASE}/config`)).data
          if (!currentCfg.pbkdf2Salt) {
            await axios.put(`${API_BASE}/config`, { ...currentCfg, pbkdf2Salt: saltB64 })
          }
        }
      } catch { /* non-fatal */ }
    }

    // Default server-side encryption ON for password/device modes when the user
    // has never configured it (localStorage key absent = new library or first use).
    // Returning users who explicitly turned it off have '0' in localStorage, so
    // their preference is preserved.  Set BEFORE isLibraryLocked=false so this
    // reactive update is batched with the unlock render, avoiding a second render
    // cycle after loadNotesList that would detach sidebar DOM nodes.
    if ((unlockMode.value === 'password' || unlockMode.value === 'device') &&
        localStorage.getItem('yinmo_server_encrypt') === null) {
      serverEncrypt.value = true
      crypto.setServerEncrypt(true)
    }
    isLibraryLocked.value = false; showUnlockModal.value = false
    unlockPassword.value = ''; importKeyText.value = ''
    await loadNotesList(); startBackgroundScanner()
  } catch (e: any) {
    unlockError.value = true
    // Surface WebAuthn domain requirement to the user
    if (e?.message?.includes('domain name')) unlockErrorMsg.value = e.message
    else unlockErrorMsg.value = ''
  } finally { isUnlocking.value = false }
}

/** Deletes all notes and assets from the server. Called during library reset. */
const deleteAllServerData = async () => {
  const notes: { name: string }[] = (await axios.get(`${API_BASE}/notes`)).data.notes || []
  await Promise.all(notes.map(n => axios.delete(`${API_BASE}/notes/${n.name}`).catch(() => {})))
  let assets: string[] = []
  try { assets = (await axios.get(`${API_BASE}/assets`)).data.assets || [] } catch (_) { /* assets endpoint may not exist */ }
  await Promise.all(assets.map((a: string) => axios.delete(`${API_BASE}/uploads/${a}`).catch(() => {})))
  // Structure integrity check: empty order is only rejected when notes exist on disk.
  // After deleting all notes, this PUT succeeds.
  await axios.put(`${API_BASE}/structure`, { order: [], parents: {}, childOrder: {} }).catch(() => {})
}

// ─── Reset library modal ───────────────────────────────────────────────────────
const showResetModal = ref(false)
const resetCountdown = ref(config.resetCountdownSeconds)
let resetCountdownTimer: ReturnType<typeof setInterval> | null = null
const resetExecuting = ref(false)
const resetError = ref(false)
const resetPassword = ref(''); const showResetPassword = ref(false)
const resetIsHardware = ref(crypto.hasHardwareKey())

const showSettingsCloseConfirm = ref(false)
const settingsSaveError = ref(false)
const encryptionChangedByServer = ref(false)
const batchResultMsg = ref('')
// Batch encryption composable (initialised below after t is available)
const exportKeyStatus = ref<'idle' | 'success' | 'error' | 'fallback'>('idle')

const openResetModal = () => {
  resetCountdown.value = config.resetCountdownSeconds
  resetExecuting.value = false
  resetError.value = false
  resetPassword.value = ''; showResetPassword.value = false
  resetIsHardware.value = crypto.hasHardwareKey()
  showResetModal.value = true
  resetCountdownTimer = setInterval(() => {
    if (resetCountdown.value > 0) resetCountdown.value--
    else { clearInterval(resetCountdownTimer!); resetCountdownTimer = null }
  }, 1000)
}

const closeResetModal = () => {
  showResetModal.value = false
  if (resetCountdownTimer) { clearInterval(resetCountdownTimer); resetCountdownTimer = null }
}

/**
 * Executes library reset: verifies identity (if applicable), deletes all server data,
 * clears local credentials, and reloads the page to return to the init screen.
 */
const handleResetLibrary = async () => {
  if (resetCountdown.value > 0 || resetExecuting.value) return
  resetExecuting.value = true
  resetError.value = false
  try {
    if (!isKeylessModeActive.value) {
      const isHardware = crypto.hasHardwareKey()
      const key = isHardware ? await crypto.deriveKeyFromHardware() : await crypto.deriveKeyFromPassword(resetPassword.value)
      if (!await crypto.verifyAndUnlockLibrary(key)) { resetError.value = true; resetExecuting.value = false; return }
    }
    await deleteAllServerData()
    // Clear server-side SRPVerifier so /api/auth/status returns
    // initialized=false after reload, showing the init wizard instead of
    // the password-unlock UI.
    const token = crypto.getSessionToken()
    if (token) {
      await axios.post(`${API_BASE}/auth/setup`, { srpSalt: '', srpVerifier: '' }, {
        headers: { 'Authorization': `Bearer ${token}` },
      }).catch(() => {})
    }
    await crypto.resetLibrary()
    await clearIndexCache().catch(() => {})
    location.reload()
  } catch (_e) { resetError.value = true; resetExecuting.value = false }
}

// Batch
// Batch encryption composable — initialized here where t is in scope
const { batchProcessing, batchProgress, batchUpdateEncryption } = useBatchEncryption(
  batchResultMsg,
  computed(() => t.value.libUnlockTitle as string),
  computed(() => t.value.conversionPartialFail as (n: number) => string),
  () => {
    // Re-save _structure.json so its encryption state matches the newly converted notes.
    saveStructure()
    if (currentNote.value && editorRef.value) editorRef.value.loadNote()
    // Server content format changed (ENC1 ↔ plain) — cached data is now stale.
    clearIndexCache().catch(() => {})
  }
)

// ── Batch import ──────────────────────────────────────────────────────────
const { importing: importProcessing, importProgress, importStatus, importResults, showImportResults, importNotes } = useBatchImport({
  structure, noteTitles, saveStructure, indexNote,
})
const fileInputRef = ref<HTMLInputElement | null>(null)
const folderInputRef = ref<HTMLInputElement | null>(null)
const zipInputRef = ref<HTMLInputElement | null>(null)
const showImportMenu = ref(false)

const onImportFiles = (e: Event) => {
  const files = Array.from((e.target as HTMLInputElement).files || [])
  if (files.length) importNotes(files)
  ;(e.target as HTMLInputElement).value = ''
}
const onImportFolder = (e: Event) => {
  const files = Array.from((e.target as HTMLInputElement).files || [])
  if (files.length) importNotes(files)
  ;(e.target as HTMLInputElement).value = ''
}
const onImportZip = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) importNotes(await file.arrayBuffer())
  ;(e.target as HTMLInputElement).value = ''
}
const handleFileDrop = async (e: DragEvent) => {
  if (!e.dataTransfer) return
  const files = await filesFromDrop(e.dataTransfer)
  const zipFile = files.find(f => f.name.endsWith('.zip'))
  if (zipFile) {
    importNotes(await zipFile.arrayBuffer())
  } else if (files.length > 0) {
    importNotes(files)
  }
}
const importSuccessCount = computed(() => importResults.value.filter(r => r.status === 'success').length)
const importSkipCount = computed(() => importResults.value.filter(r => r.status === 'skipped').length)
const importFailCount = computed(() => importResults.value.filter(r => r.status === 'failed').length)
const importFailedList = computed(() => importResults.value.filter(r => r.status !== 'success'))
const importReasonText = (reason?: string) => {
  if (reason === 'too_large') return t.value.importReasonTooLarge
  if (reason === 'quota_full') return t.value.importReasonQuotaFull
  return t.value.importReasonServerError
}

const startBackgroundScanner = async () => {
  if (isLibraryLocked.value) return
  // Build in-memory full-text search index in the background after unlock
  buildContentIndex()
}

// Settings & Sidebar
const openSettings = async () => {
  draftSettings.themeMode = themeMode.value; draftSettings.lang = lang.value; draftSettings.fontSize = fontSize.value; draftSettings.editorWidth = editorWidth.value
  draftSettings.typewriterMode = typewriterMode.value; draftSettings.serverEncrypt = serverEncrypt.value; draftSettings.idleTimeout = idleTimeout.value; draftSettings.allowExternalImages = allowExternalImages.value
  // Load MCP policy from server config
  try {
    const cfg = (await axios.get(`${API_BASE}/config`)).data
    mcpTokenSet.value = !!cfg.mcpTokenSet
    webdavTokenSet.value = !!cfg.webdavTokenSet
    const policy = cfg.mcpPolicy || {}
    draftSettings.mcpEnabled = !!policy.enabled
    draftSettings.mcpDefaultAccess = policy.default_access || 'read'
    draftSettings.mcpRules = (policy.rules || []).map((r: { tag?: string; note_id?: string; title_glob?: string; subtree_of?: string; access: string }) => ({
      condition: r.tag ? 'tag' : r.note_id ? 'note_id' : r.title_glob ? 'title_glob' : 'subtree_of',
      value: r.tag || r.note_id || r.title_glob || r.subtree_of || '',
      access: r.access,
    }))
  } catch (_) { /* non-fatal — MCP section will show defaults */ }
  // Fetch CA fingerprint (only present when TLS_SELF=1)
  try {
    const fp = (await axios.get(`${API_BASE}/mcp/ca-fingerprint`)).data
    mcpCaFingerprint.value = fp.fingerprint || ''
    mcpCaExpiry.value = fp.expiry || ''
  } catch (_) { mcpCaFingerprint.value = ''; mcpCaExpiry.value = '' }
  originalSettingsJson = JSON.stringify(draftSettings); settingsTab.value = 'appearance'; showSettings.value = true
}
const applySettings = async (close: boolean) => {
  if (batchProcessing.value) return  // Prevent concurrent batch runs
  const encChanged = draftSettings.serverEncrypt !== serverEncrypt.value
  if (encChanged && isLibraryLocked.value) { settingsTab.value = 'security'; batchResultMsg.value = t.value.libUnlockTitle; return }

  themeMode.value = draftSettings.themeMode as 'auto' | 'light' | 'dark'
  localStorage.setItem('themeMode', themeMode.value)
  setLang(draftSettings.lang as Lang); fontSize.value = draftSettings.fontSize; editorWidth.value = draftSettings.editorWidth
  typewriterMode.value = draftSettings.typewriterMode; serverEncrypt.value = draftSettings.serverEncrypt
  // Update localStorage immediately so saveStructure reads the new value.
  crypto.setServerEncrypt(draftSettings.serverEncrypt)
  idleTimeout.value = draftSettings.idleTimeout; allowExternalImages.value = draftSettings.allowExternalImages

  // Persist to backend config.json
  try {
    const currentConfig = (await axios.get(`${API_BASE}/config`)).data
    // Build MCPPolicy from draft rules
    const mcpRules = draftSettings.mcpRules
      .filter(r => r.value.trim())
      .map(r => ({ [r.condition]: r.value.trim(), access: r.access }))
    await axios.put(`${API_BASE}/config`, { ...currentConfig,
      lang: draftSettings.lang, themeMode: draftSettings.themeMode,
      editorWidth: draftSettings.editorWidth, fontSize: draftSettings.fontSize,
      typewriterMode: draftSettings.typewriterMode, serverEncrypt: draftSettings.serverEncrypt,
      idleTimeout: draftSettings.idleTimeout, allowExternalImages: draftSettings.allowExternalImages,
      mcpPolicy: { enabled: draftSettings.mcpEnabled, default_access: draftSettings.mcpDefaultAccess, rules: mcpRules },
    })
  } catch(e) {
    console.error('[YinMo] Failed to save settings to server:', e)
    // Surface the error in the settings panel so the user knows the save failed.
    settingsSaveError.value = true
    setTimeout(() => { settingsSaveError.value = false }, 4000)
  }

  // Close modal before long-running batch so it doesn't linger under the overlay.
  if (close) { showSettings.value = false; showSettingsCloseConfirm.value = false; mcpTokenValue.value = ''; webdavTokenValue.value = '' }

  if (encChanged) {
    // Flush editor and structure before batch so everything is saved in current state.
    if (editorRef.value) await editorRef.value.doSave()
    await saveStructure()
    await batchUpdateEncryption(draftSettings.serverEncrypt)
  } else {
    saveStructure()
  }
}
const closeSettings = () => {
  if (JSON.stringify(draftSettings) !== originalSettingsJson) { showSettingsCloseConfirm.value = true; return }
  showSettings.value = false
  showSettingsCloseConfirm.value = false
  mcpTokenValue.value = ''
  webdavTokenValue.value = ''
}

const handleMCPGenerateToken = async () => {
  if (mcpTokenLoading.value) return
  mcpTokenLoading.value = true
  mcpTokenError.value = ''
  try {
    const res = (await axios.post(`${API_BASE}/mcp/token`)).data
    mcpTokenValue.value = res.token || ''
    mcpTokenSet.value = true
  } catch (e) {
    console.error('[YinMo] MCP token generation failed:', e)
    mcpTokenError.value = 'generate_failed'
  } finally {
    mcpTokenLoading.value = false
  }
}
const handleMCPRevokeToken = async () => {
  if (mcpTokenLoading.value) return
  mcpTokenLoading.value = true
  mcpTokenError.value = ''
  try {
    await axios.delete(`${API_BASE}/mcp/token`)
    mcpTokenValue.value = ''
    mcpTokenSet.value = false
  } catch (e) {
    console.error('[YinMo] MCP token revocation failed:', e)
    mcpTokenError.value = 'revoke_failed'
  } finally {
    mcpTokenLoading.value = false
  }
}
const handleWebDAVGenerateToken = async () => {
  if (webdavTokenLoading.value) return
  webdavTokenLoading.value = true
  webdavTokenError.value = ''
  try {
    const res = (await axios.post(`${API_BASE}/webdav/token`)).data
    webdavTokenValue.value = res.token || ''
    webdavTokenSet.value = true
  } catch (e) {
    console.error('[YinMo] WebDAV token generation failed:', e)
    webdavTokenError.value = 'generate_failed'
  } finally {
    webdavTokenLoading.value = false
  }
}
const handleWebDAVRevokeToken = async () => {
  if (webdavTokenLoading.value) return
  webdavTokenLoading.value = true
  webdavTokenError.value = ''
  try {
    await axios.delete(`${API_BASE}/webdav/token`)
    webdavTokenValue.value = ''
    webdavTokenSet.value = false
  } catch (e) {
    console.error('[YinMo] WebDAV token revocation failed:', e)
    webdavTokenError.value = 'revoke_failed'
  } finally {
    webdavTokenLoading.value = false
  }
}
const draftSettings = reactive({ themeMode: 'auto' as string, lang: 'zh', fontSize: 16, editorWidth: 'full', typewriterMode: false, serverEncrypt: false, idleTimeout: 0, allowExternalImages: false, mcpEnabled: false, mcpDefaultAccess: 'read', mcpRules: [] as Array<{ condition: string; value: string; access: string }> })
let originalSettingsJson = ''

// Tag editing state
const tagEditKey = ref<string | null>(null)
const tagEditValue = ref('')
const openTagEdit = (key: string, currentTags: string[]) => {
  tagEditKey.value = key; tagEditValue.value = currentTags.join(', ')
}
const saveTagEdit = async () => {
  if (!tagEditKey.value) return
  const tags = tagEditValue.value.split(',').map(s => s.trim()).filter(Boolean)
  await setNoteTags(tagEditKey.value, tags)
  tagEditKey.value = null; tagEditValue.value = ''
}
const cancelTagEdit = () => { tagEditKey.value = null; tagEditValue.value = '' }

const draggedKey = ref<string | null>(null); const dropTarget = ref<string | null>(null); const dropPosition = ref<'before' | 'after' | 'inside' | null>(null)
const onNoteDragStart = (e: DragEvent, key: string) => { draggedKey.value = key; e.dataTransfer?.setData('text/plain', key) }
const onNoteDragOver = (e: DragEvent, key: string) => {
  if (draggedKey.value === key) return
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const relY = e.clientY - rect.top
  const threshold = rect.height * 0.3

  if (relY < threshold) dropPosition.value = 'before'
  else if (relY > rect.height - threshold) dropPosition.value = 'after'
  else dropPosition.value = 'inside'

  dropTarget.value = key
}
const onNoteDrop = async (_e: DragEvent, targetKey: string) => {
  const src = draggedKey.value; const pos = dropPosition.value
  draggedKey.value = null; dropTarget.value = null; dropPosition.value = null
  if (!src || src === targetKey) return
  await moveNote(src, targetKey, pos || 'after')
}
const onNoteDragEnd = () => { draggedKey.value = null; dropTarget.value = null; dropPosition.value = null }
// Do NOT clear dropTarget here. The 2px gaps between list items belong to the nav container,
// so clearing on every nav-level dragover would lose the last known drop position.
const onSidebarDragOver = () => { /* only .prevent is needed; do not reset dropTarget */ }
const onSidebarDrop = async (e: DragEvent) => {
  // If files are being dropped from the OS (not internal note drag), handle as import
  if (e.dataTransfer?.files?.length && !draggedKey.value) {
    e.preventDefault()
    handleFileDrop(e)
    return
  }
  const src = draggedKey.value; const target = dropTarget.value; const pos = dropPosition.value
  draggedKey.value = null; dropTarget.value = null; dropPosition.value = null
  if (!src) return
  // Reuse the last hovered item's position when dropping in a gap; fall back to root otherwise.
  if (target && pos) await moveNote(src, target, pos)
  else await moveNote(src, null, 'root')
}
const createSubNote = async (pid: string) => {
  if (isCreatingNote) return
  isCreatingNote = true
  try {
    // If there is already an empty pending sub-note under this parent, reuse it.
    // Use .find() (not [0]) to locate any pending child of pid regardless of its
    // position — drag-drop reordering can move a non-pending note to index 0.
    const existingPending = structure.value.childOrder[pid]?.find((id: string) => pendingNotes.has(id))
    if (existingPending) {
      const hasContent = editorRef.value && !editorRef.value.isContentEmpty.value
      if (!hasContent) {
        currentNote.value = existingPending
        nextTick(() => (document.querySelector('.ProseMirror') as HTMLElement)?.focus())
        return
      }
    }
    pruneEmptyCurrentNote()
    if (editorRef.value && !editorRef.value.isContentEmpty.value) await editorRef.value.doSave()

    // Delegate structure mutation to useLibrary to respect its encapsulation (D1 fix).
    await _createSubNote(pid, () => (document.querySelector('.ProseMirror') as HTMLElement)?.focus())
  } finally {
    isCreatingNote = false
  }
}
const exportedKeyText = ref('')
const settingsPanelRef = ref<InstanceType<typeof SettingsPanel>>()

/**
 * Change password flow:
 * 1. Verify current password
 * 2. Derive new key from new password
 * 3. Re-init library verification token with new key
 * 4. Update session token on server
 * 5. Trigger batch re-encryption of all notes
 */
const handleChangePassword = async (currentPwd: string, newPwd: string) => {
  try {
    // Step 1: verify current password
    const oldKey = await crypto.deriveKeyFromPassword(currentPwd)
    if (!await crypto.verifyAndUnlockLibrary(oldKey)) {
      settingsPanelRef.value?.onPasswordChangeResult(false, t.value.currentPasswordWrong as string)
      return
    }
    // Step 2: derive new key
    const newKey = await crypto.deriveKeyFromPassword(newPwd)
    // Step 3: re-init library token with new key
    await crypto.initLibrary(newKey)
    // Step 4: update SRP verifier on server with new password, then obtain a fresh token.
    try {
      // Generate new SRP salt and verifier for the new password.
      const newSRPSaltBytes = window.crypto.getRandomValues(new Uint8Array(16))
      const newSRPSaltB64 = btoa(String.fromCharCode(...newSRPSaltBytes))
      const newVerifierHex = await crypto.srpComputeVerifier(newSRPSaltBytes, newPwd)
      // POST /api/auth/setup requires current Bearer token (already in sessionStorage).
      await axios.post(`${API_BASE}/auth/setup`, {
        srpSalt: newSRPSaltB64,
        srpVerifier: newVerifierHex,
      })
      // Now authenticate with the new password via SRP to get a fresh token.
      const newToken = await crypto.deriveSessionToken(newPwd, API_BASE)
      crypto.storeSessionToken(newToken)
    } catch { /* non-fatal */ }
    settingsPanelRef.value?.onPasswordChangeResult(true, t.value.passwordChangeSuccess as string)
    // Step 5: trigger batch re-encryption if server encryption is enabled
    if (serverEncrypt.value) {
      batchUpdateEncryption(true)
    }
  } catch {
    settingsPanelRef.value?.onPasswordChangeResult(false)
  }
}

/** Exports the master key as Base64 and copies it to the clipboard (with textarea fallback). */
const handleExportMasterKey = async () => {
  try {
    const b64 = await crypto.exportRawKey()
    try {
      await navigator.clipboard.writeText(b64)
      exportKeyStatus.value = 'success'
    } catch {
      // Clipboard API unavailable (non-HTTPS or blocked by browser). Fall back to
      // displaying the key in a read-only textarea so the user can copy manually.
      exportedKeyText.value = b64
      exportKeyStatus.value = 'fallback'
    }
  } catch { exportKeyStatus.value = 'error' }
  if (exportKeyStatus.value !== 'fallback') {
    setTimeout(() => { exportKeyStatus.value = 'idle' }, config.exportKeyFadeDurationMs)
  }
}

const handleLockLibrary = () => {
  crypto.lockLibrary()
  clearContentIndex()  // Prevent stale decrypted content from leaking into next unlock session.
  isLibraryLocked.value = true
  showUnlockModal.value = true
}

// key is the noteFileName the Editor was displaying when it emitted.
// Using the emitted key (instead of currentNote.value) prevents the race where
// a previous note's in-flight loadNote resolves after currentNote has advanced,
// which would otherwise overwrite the new note's title with stale data.
let _titleDebounceTimer: ReturnType<typeof setTimeout> | null = null
const onTitleChanged = (key: string, title: string) => {
  if (!key) return
  // Guard: only update if the note still exists in the structure.
  const inOrder = structure.value.order?.includes(key) ?? false
  const inParents = key in (structure.value.parents ?? {})
  if (!inOrder && !inParents) return
  const tr = title.trim() || (lang.value === 'zh' ? '无标题文档' : 'Untitled');
  if (noteTitles[key] !== tr) {
    noteTitles[key] = tr;
    // Promote preview tab only on actual content change (not initial load).
    if (previewTab.value === key) pinPreviewTab(key)
    // Debounce 500 ms to avoid a PUT /structure on every keystroke.
    if (_titleDebounceTimer !== null) clearTimeout(_titleDebounceTimer)
    _titleDebounceTimer = setTimeout(() => { _titleDebounceTimer = null; saveStructure() }, 500)
  }
}

const serverEncrypt = ref(crypto.isServerEncryptEnabled())
const editorWidth = ref(localStorage.getItem('yinmo_editor_width') || 'full')
const fontSize = ref(Number(localStorage.getItem('yinmo_font_size') || '16'))
const typewriterMode = ref(localStorage.getItem('yinmo_typewriter_mode') === '1')
const allowExternalImages = ref(false)

provide<Ref<boolean>>('serverEncrypt', serverEncrypt); provide<Ref<string>>('editorWidth', editorWidth); provide<Ref<number>>('fontSize', fontSize); provide<Ref<boolean>>('typewriterMode', typewriterMode); provide<Ref<boolean>>('isDark', isDark); provide<Ref<boolean>>('isLibraryLocked', isLibraryLocked); provide<(id: string, plainText: string) => void>('indexNote', indexNote); provide<() => void>('scheduleOrphanCleanup', scheduleOrphanCleanup); provide<Ref<boolean>>('allowExternalImages', allowExternalImages)

// Reset import mode if server turns out to have no notes (fresh library).
watch(hasServerNotes, (v) => { if (!v && unlockMode.value === 'import') unlockMode.value = 'device' })

// TD-10: These UI prefs are owned by server config (AppConfig). The localStorage values
// below serve only as initial fallback until onMounted fetches the server config.
// We no longer write back to localStorage on change to avoid a multi-tab race where
// onMounted overwrites a sibling tab's localStorage write.
watch(serverEncrypt, (v) => crypto.setServerEncrypt(v))

// onDrag / startDrag / stopDrag come from useDragDrop (above)
const onResize = () => isDesktop.value = window.innerWidth >= 768
const handleSidebarScroll = (e: Event) => {
  const el = e.target as HTMLElement
  // Trigger next page when 100px from bottom
  if (el.scrollHeight - el.scrollTop - el.clientHeight < config.sidebarScrollThreshold) {
    if (displayList.value.length >= displayLimit.value) {
      displayLimit.value += config.sidebarDisplayLimit
    }
  }
}

let exitTime = 0
const onBlur = () => { exitTime = Date.now() }

// Save the current note if it has unsaved changes and the library is accessible.
// Called from all page-hide / unload paths so content is never lost on close/refresh.
// The returned promise is intentionally not awaited by event-driven callers (beforeunload,
// pagehide) because browsers do not honour async event handlers — the save is best-effort.
const saveIfDirty = (): void => {
  if (!isLibraryLocked.value && editorRef.value?.saveStatus === 'dirty') {
    void editorRef.value.doSave()
  }
}

const onVisibilityChange = () => {
  if (document.visibilityState === 'hidden') {
    exitTime = Date.now()
    saveIfDirty() // tab switch, minimize, lock screen, lid close
  } else if (idleTimeout.value > 0 && !isLibraryLocked.value && !crypto.isKeylessMode() && (Date.now() - exitTime) / 60000 >= idleTimeout.value) { crypto.lockLibrary(); clearContentIndex(); isLibraryLocked.value = true; showUnlockModal.value = true }
}

// beforeunload: save on refresh/close/navigate-away and show native confirm
const onBeforeUnload = (e: BeforeUnloadEvent) => {
  if (!isLibraryLocked.value && editorRef.value?.saveStatus === 'dirty') {
    saveIfDirty() // fire-and-forget; if user clicks "stay" the save may complete
    e.preventDefault()
    e.returnValue = '' // Chrome requires setting returnValue to show the dialog
  }
}

onMounted(async () => {
  // Sync preferences from server config.json with extreme caution
  try {
    const res = await axios.get(`${API_BASE}/config`)
    if (res && res.data) {
      const cfg = res.data
      if (cfg.lang) setLang(cfg.lang)
      // Theme: prefer themeMode, fallback to legacy isDark for migration
      if (cfg.themeMode) { themeMode.value = cfg.themeMode; localStorage.setItem('themeMode', cfg.themeMode) }
      else if (cfg.isDark !== undefined) { themeMode.value = cfg.isDark ? 'dark' : 'light'; localStorage.setItem('themeMode', themeMode.value) }
      // pbkdf2Salt is synced via /api/auth/status on lock screen (see onMounted)
      if (cfg.fontSize) fontSize.value = cfg.fontSize
      if (cfg.editorWidth) editorWidth.value = cfg.editorWidth
      if (cfg.typewriterMode !== undefined) typewriterMode.value = cfg.typewriterMode
      if (cfg.serverEncrypt !== undefined && cfg.serverEncrypt !== serverEncrypt.value) {
        const prev = serverEncrypt.value
        serverEncrypt.value = cfg.serverEncrypt
        // Notify the user when the server overrides the local encryption preference so
        // they are not silently surprised by a changed security posture on next save.
        console.warn(`[YinMo] Server config changed encryption: ${prev} → ${cfg.serverEncrypt}`)
        encryptionChangedByServer.value = true
        setTimeout(() => { encryptionChangedByServer.value = false }, 6000)
      } else if (cfg.serverEncrypt !== undefined) {
        serverEncrypt.value = cfg.serverEncrypt
      }
      if (cfg.idleTimeout !== undefined) idleTimeout.value = cfg.idleTimeout
      if (cfg.allowExternalImages !== undefined) allowExternalImages.value = cfg.allowExternalImages
    }
  } catch(e) {
    console.warn('Failed to fetch server config, falling back to local defaults', e)
  }

  // Keyless mode persists across page reloads — no key needed, just restore the flag.
  if (crypto.wasKeylessPersisted()) {
    crypto.setKeylessMode()
    isLibraryLocked.value = false; hasLibraryKey.value = true
    // showUnlockModal stays false
  } else {
    try {
      const r = await crypto.restoreKeyFromSession()
      if (r) { isLibraryLocked.value = false; startBackgroundScanner() }
      // Guard: handleUnlock may have already completed while restoreKeyFromSession was
      // running (e.g. during a PBKDF2 key derivation). Only show the modal if the
      // library has not been unlocked by the time we land here (isLibraryLocked still true).
      else if (isLibraryLocked.value) {
        // Always sync the PBKDF2 salt from the server — this is the ONLY way to
        // ensure all devices derive the same key from the same password. Must
        // happen BEFORE any key derivation, regardless of local library state.
        try {
          // GET /api/auth/status is unauthenticated; it returns both the
          // initialized flag and pbkdf2Salt so new devices can import the
          // salt before key derivation without requiring a Bearer token.
          const statusRes = await axios.get(`${API_BASE}/auth/status`)
          if (statusRes.data.pbkdf2Salt) crypto.importSaltFromConfig(statusRes.data.pbkdf2Salt)
          if (statusRes.data.initialized) {
            serverInitialized.value = true
            if (!crypto.hasLibrary()) {
              unlockMode.value = 'password'
              hasLibraryKey.value = true   // show UNLOCK UI, not INIT wizard
            }
          }
        } catch (_) { /* crypto restoration failure is non-fatal — show unlock UI */ }
        showUnlockModal.value = true
      }
    } catch(e) {
      console.error('Crypto restoration failed', e)
      showUnlockModal.value = true
    }
  }

  // Only load notes if the library is already unlocked (keyless mode or session key restored).
  // In password mode the library is still locked here; loadNotesList will be called by
  // handleUnlock after the user authenticates. Calling it while locked would produce a
  // 401 (SessionTokenHash is set but no Bearer token is available yet) and show a
  // spurious "notes load failed" error on every page load.
  if (!isLibraryLocked.value) loadNotesList()
  // Cross-tab sync: reload note list when another tab saves
  syncChannel?.addEventListener('message', (e: MessageEvent) => {
    if (e.data?.type === 'note-saved' && e.data.note && e.data.note !== currentNote.value) {
      // Another tab saved a note — refresh the note list to show updated titles
      loadNotesList()
    } else if (e.data?.type === 'structure-saved') {
      loadNotesList()
    }
  })
  window.addEventListener('resize', onResize); window.addEventListener('mousemove', onDrag); window.addEventListener('mouseup', stopDrag)
  document.addEventListener('visibilitychange', onVisibilityChange); window.addEventListener('blur', onBlur); window.addEventListener('focus', onVisibilityChange)
  window.addEventListener('pagehide', saveIfDirty)   // mobile Safari / PWA: Home key, app switch
  window.addEventListener('beforeunload', onBeforeUnload) // refresh, close tab, navigate away
  window.addEventListener('unload', clearWindowName) // SEC-013: clear after save completes
  window.addEventListener('keydown', onGlobalKeydown)
})

const onGlobalKeydown = (e: KeyboardEvent) => {
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault()
    if (isLibraryLocked.value) return // guard: locked library
    showCommandPalette.value = !showCommandPalette.value
  }
}
onUnmounted(() => { window.removeEventListener('resize', onResize); window.removeEventListener('mousemove', onDrag); window.removeEventListener('mouseup', stopDrag); document.removeEventListener('visibilitychange', onVisibilityChange); window.removeEventListener('blur', onBlur); window.removeEventListener('focus', onVisibilityChange); window.removeEventListener('pagehide', saveIfDirty); window.removeEventListener('beforeunload', onBeforeUnload); window.removeEventListener('unload', clearWindowName); window.removeEventListener('keydown', onGlobalKeydown); _mediaQuery.removeEventListener('change', _onSystemThemeChange) })

const sidebarClass = computed(() => 'flex flex-col z-50 relative')
const sidebarStyle = computed(() => ({
  width: isDesktop.value ? (sidebarVisible.value ? sidebarWidth.value + 'px' : '44px') : '100%',
  position: (isDesktop.value ? 'relative' : 'fixed') as any,
  // Mobile: fixed overlay must be pinned to viewport top-left and fill full height so
  // the inner flex-1 nav gets a constrained height and overflow-y-auto can scroll.
  // Use 100dvh (dynamic viewport height) so the sidebar doesn't extend under the
  // iOS browser chrome (address bar).
  ...(isDesktop.value ? {} : { top: 0, left: 0, height: '100dvh' }),
  background: 'var(--bg-sidebar)',
  borderRight: '1px solid var(--border)',
  transform: !isDesktop.value && !showMobileSidebar.value ? 'translateX(-100%)' : 'none',
  transition: isDragging.value ? 'none' : `transform var(--duration-expressive) var(--ease-expressive), width var(--duration-expressive) var(--ease-expressive)`
}))
</script>

<style>
.mt-safe { margin-top: env(safe-area-inset-top, 0); }

/* Trash item hover */
.trash-item:hover { background: var(--bg-hover); }

/* Note context menu item hover */
.note-menu-item:hover { background: var(--bg-hover); }

/* Note list item states */
.note-item-active {
  background: var(--bg-active);
}
.dark .note-item-active {
  box-shadow: inset 0 0 0 1px var(--accent-light), 0 0 8px var(--accent-light);
}
.note-item-default {
  background: transparent;
}
.note-item-default:hover {
  background: var(--bg-hover);
}

/* Sidebar button hover */
.sidebar-btn:hover {
  background: var(--bg-hover);
}

/* Sidebar resize handle */
.resize-handle:hover { background: var(--accent); }

/* Settings panel slide-in transition */
.settings-overlay-enter-active, .settings-overlay-leave-active { transition: opacity var(--duration-normal) var(--ease-micro); }
.settings-overlay-enter-from, .settings-overlay-leave-to { opacity: 0; }
.settings-panel-enter-active, .settings-panel-leave-active { transition: transform var(--duration-expressive) var(--ease-expressive); }
.settings-panel-enter-from, .settings-panel-leave-to { transform: translateX(100%); }

/* Fade transition for unlock modal */
.fade-enter-active, .fade-leave-active { transition: opacity var(--duration-normal) var(--ease-micro); }
.fade-enter-from, .fade-leave-to { opacity: 0; }

/* Thin scrollbar using CSS vars */
.custom-scrollbar::-webkit-scrollbar { width: 4px; }
.custom-scrollbar::-webkit-scrollbar-track { background: transparent; }
.custom-scrollbar::-webkit-scrollbar-thumb { background: var(--border); border-radius: 10px; }

/* Empty state subtle radial gradient background */
.empty-state-bg {
  background: radial-gradient(ellipse at 50% 40%, var(--bg-active) 0%, transparent 70%);
}
</style>
