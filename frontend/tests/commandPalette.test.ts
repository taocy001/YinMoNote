/**
 * CommandPalette unit tests
 *
 * Tests the core logic of the command palette: mode switching,
 * search filtering, keyboard navigation, and state management.
 *
 * Note: CommandPalette uses <Teleport to="body">, so we query
 * document.body directly instead of the wrapper's DOM.
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, VueWrapper, DOMWrapper } from '@vue/test-utils'
import CommandPalette from '../src/components/CommandPalette.vue'

const mockT = {
  cmdPaletteSearch: 'Search notes or type a command…',
  cmdPaletteNoResults: 'No results',
  cmdPaletteRecent: 'Recent',
  cmdNewNote: 'New note',
  cmdSettings: 'Open settings',
  cmdLock: 'Lock library',
  cmdToggleTheme: 'Toggle theme',
  cmdTrash: 'Open trash',
}

const defaultProps = {
  modelValue: true,
  titles: {
    'note-1.md': 'Meeting Notes',
    'note-2.md': 'Shopping List',
    'note-3.md': 'Project Plan',
  } as Record<string, string>,
  noteKeys: ['note-1.md', 'note-2.md', 'note-3.md'],
  recentNotes: ['note-2.md', 'note-1.md'],
  t: mockT,
}

let wrapper: VueWrapper<any>

function mountPalette(overrides: Record<string, any> = {}) {
  wrapper = mount(CommandPalette, {
    props: { ...defaultProps, ...overrides },
    attachTo: document.body,
  })
  return wrapper
}

/** Query the teleported content in document.body */
function bodyFind(selector: string) {
  return document.body.querySelector(selector)
}
function bodyFindAll(selector: string) {
  return Array.from(document.body.querySelectorAll(selector))
}
async function setInputValue(value: string) {
  const input = bodyFind('input') as HTMLInputElement
  // Use native input event to trigger v-model
  input.value = value
  input.dispatchEvent(new Event('input', { bubbles: true }))
  await wrapper.vm.$nextTick()
}
async function triggerKeydown(key: string) {
  const input = bodyFind('input') as HTMLInputElement
  input.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
  await wrapper.vm.$nextTick()
}

afterEach(() => {
  wrapper?.unmount()
})

// ─── Mode switching ─────────────────────────────────────────────────────────

describe('command mode detection', () => {
  it('enters command mode when query starts with >', async () => {
    mountPalette()
    await setInputValue('>')
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(5) // 5 built-in commands
  })

  it('stays in note mode when query does not start with >', async () => {
    mountPalette()
    await setInputValue('meeting')
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(1)
    expect(options[0].textContent).toContain('Meeting Notes')
  })
})

// ─── Command filtering ──────────────────────────────────────────────────────

describe('command filtering', () => {
  it('shows all 5 commands when query is just ">"', async () => {
    mountPalette()
    await setInputValue('>')
    expect(bodyFindAll('[role="option"]').length).toBe(5)
  })

  it('filters commands by substring match', async () => {
    mountPalette()
    await setInputValue('>settings')
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(1)
    expect(options[0].textContent).toContain('Open settings')
  })

  it('returns empty when no command matches', async () => {
    mountPalette()
    await setInputValue('>xyznonexistent')
    expect(bodyFindAll('[role="option"]').length).toBe(0)
  })
})

// ─── Note search filtering ──────────────────────────────────────────────────

describe('note search filtering', () => {
  it('returns matching notes by title substring', async () => {
    mountPalette()
    await setInputValue('shop')
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(1)
    expect(options[0].textContent).toContain('Shopping List')
  })

  it('is case-insensitive', async () => {
    mountPalette()
    await setInputValue('MEETING')
    expect(bodyFindAll('[role="option"]').length).toBe(1)
  })

  it('falls back to key when title is missing', async () => {
    mountPalette({ titles: {}, noteKeys: ['orphan.md'], recentNotes: [] })
    await setInputValue('orphan')
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(1)
    expect(options[0].textContent).toContain('orphan.md')
  })

  it('limits results to 20', async () => {
    const keys = Array.from({ length: 50 }, (_, i) => `note-${i}.md`)
    const titles = Object.fromEntries(keys.map(k => [k, `Note ${k}`]))
    mountPalette({ noteKeys: keys, titles, recentNotes: [] })
    await setInputValue('Note')
    expect(bodyFindAll('[role="option"]').length).toBeLessThanOrEqual(20)
  })
})

// ─── Recent notes display ───────────────────────────────────────────────────

describe('recent notes display', () => {
  it('shows recent notes when query is empty', () => {
    mountPalette()
    const options = bodyFindAll('[role="option"]')
    expect(options.length).toBe(2) // recentNotes has 2 items
  })

  it('limits recent notes to 8', () => {
    const recent = Array.from({ length: 15 }, (_, i) => `note-${i}.md`)
    const titles = Object.fromEntries(recent.map(k => [k, `Note ${k}`]))
    mountPalette({ recentNotes: recent, noteKeys: recent, titles })
    expect(bodyFindAll('[role="option"]').length).toBe(8)
  })
})

// ─── Keyboard navigation ────────────────────────────────────────────────────

describe('keyboard navigation', () => {
  it('moves selection down with arrow key', async () => {
    mountPalette()
    await triggerKeydown('ArrowDown')
    const options = bodyFindAll('[role="option"]')
    expect(options[1].getAttribute('aria-selected')).toBe('true')
  })

  it('wraps around when reaching the end', async () => {
    mountPalette()
    await triggerKeydown('ArrowDown')
    await triggerKeydown('ArrowDown')
    const options = bodyFindAll('[role="option"]')
    expect(options[0].getAttribute('aria-selected')).toBe('true')
  })

  it('wraps around when pressing up from first item', async () => {
    mountPalette()
    await triggerKeydown('ArrowUp')
    const options = bodyFindAll('[role="option"]')
    expect(options[1].getAttribute('aria-selected')).toBe('true')
  })

  it('does not crash when list is empty', async () => {
    mountPalette({ noteKeys: [], recentNotes: [] })
    await triggerKeydown('ArrowDown')
    await triggerKeydown('ArrowUp')
    expect(bodyFindAll('[role="option"]').length).toBe(0)
  })
})

// ─── Emit events ────────────────────────────────────────────────────────────

describe('event emissions', () => {
  it('emits select-note when Enter is pressed on a note', async () => {
    mountPalette()
    await triggerKeydown('Enter')
    expect(wrapper.emitted('select-note')).toBeTruthy()
    expect(wrapper.emitted('select-note')![0]).toEqual(['note-2.md'])
  })

  it('emits update:modelValue false when Esc is pressed', async () => {
    mountPalette()
    await triggerKeydown('Escape')
    expect(wrapper.emitted('update:modelValue')).toBeTruthy()
    expect(wrapper.emitted('update:modelValue')![0]).toEqual([false])
  })

  it('emits command action when Enter is pressed in command mode', async () => {
    mountPalette()
    await setInputValue('>new')
    await triggerKeydown('Enter')
    expect(wrapper.emitted('new-note')).toBeTruthy()
  })

  it('clicking a note item emits select-note', async () => {
    mountPalette()
    const option = bodyFind('[role="option"]') as HTMLElement
    option.click()
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('select-note')).toBeTruthy()
  })
})

// ─── State reset ────────────────────────────────────────────────────────────

describe('state management', () => {
  it('resets query when reopened', async () => {
    mountPalette()
    await setInputValue('some query')
    await wrapper.setProps({ modelValue: false })
    await wrapper.setProps({ modelValue: true })
    await wrapper.vm.$nextTick()
    const input = bodyFind('input') as HTMLInputElement
    expect(input?.value ?? '').toBe('')
  })

  it('resets selection when query changes', async () => {
    mountPalette()
    await triggerKeydown('ArrowDown')
    await setInputValue('meeting')
    const options = bodyFindAll('[role="option"]')
    if (options.length > 0) {
      expect(options[0].getAttribute('aria-selected')).toBe('true')
    }
  })
})

// ─── ARIA attributes ────────────────────────────────────────────────────────

describe('accessibility', () => {
  it('has role=dialog on the panel', () => {
    mountPalette()
    expect(bodyFind('[role="dialog"]')).not.toBeNull()
  })

  it('has role=combobox on the input', () => {
    mountPalette()
    expect(bodyFind('input')?.getAttribute('role')).toBe('combobox')
  })

  it('has role=listbox on the results container', () => {
    mountPalette()
    expect(bodyFind('[role="listbox"]')).not.toBeNull()
  })

  it('has role=option on each result item', () => {
    mountPalette()
    expect(bodyFindAll('[role="option"]').length).toBeGreaterThan(0)
  })

  it('sets aria-activedescendant to the selected item id', () => {
    mountPalette()
    const input = bodyFind('input')
    expect(input?.getAttribute('aria-activedescendant')).toBe('cmd-palette-item-0')
  })

  it('marks the selected option with aria-selected=true', () => {
    mountPalette()
    const options = bodyFindAll('[role="option"]')
    expect(options[0].getAttribute('aria-selected')).toBe('true')
    expect(options[1].getAttribute('aria-selected')).toBe('false')
  })
})
