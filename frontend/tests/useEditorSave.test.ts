/**
 * TE-006: Unit tests for useEditorSave composable.
 *
 * Covers:
 * - Initial state: saveStatus='idle', lastSaveError='', lastSavedTime=null
 * - statusText computed: returns correct label for each SaveStatus value
 * - saveStatusStyle / saveDotStyle computed: returns correct CSS for each status
 * - doSave: sets saveStatus 'saving' → 'saved' on success
 * - doSave: sets saveStatus 'error' and lastSaveError on axios failure
 * - doSave: skips save when isContentEmpty is true
 * - doSave: skips save when noteFileName is empty
 * - doSave: skips save when editor is undefined
 * - doSave: encrypts content when serverEncrypt is true
 * - doSave: calls indexNote and scheduleOrphanCleanup on success
 * - doSave: removes note from pendingNotes on first save
 * - doSave: serialises concurrent calls (second call waits for first)
 * - onContentChanged: sets saveStatus to 'dirty'
 * - onContentChanged: triggers doSave after autoSaveDebounceMs
 * - onContentChanged: cancels previous debounce timer on second call
 * - onContentChanged: force-saves after autoSaveIntervalMs during continuous editing
 * - clearTimers / resetForLoad: resets status to 'idle' and clears timers
 * - saveIfDirty: calls doSave only when status is 'dirty'
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'
import axios from 'axios'

// ── Global stubs needed before module-level BroadcastChannel is constructed ───
vi.stubGlobal('BroadcastChannel', class {
  postMessage = vi.fn()
  close = vi.fn()
})

// ── Module-level mocks ────────────────────────────────────────────────────────

vi.mock('axios')
vi.mock('../src/crypto', () => ({
  encryptText: vi.fn(async (s: string) => `ENC1:${s}`),
  getSessionToken: vi.fn(() => null),
}))
vi.mock('../src/composables/useLibrary', () => ({
  pendingNotes: {
    has: vi.fn(() => true),
    add: vi.fn(),
    delete: vi.fn(),
  },
  generateId: vi.fn(),
}))

import { useEditorSave } from '../src/composables/useEditorSave'
import { encryptText } from '../src/crypto'
import { pendingNotes } from '../src/composables/useLibrary'
import { config } from '../src/config'

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Minimal mock TiptapEditor that returns a fixed markdown string. */
function mockEditor(markdown = '# Hello') {
  return {
    storage: {
      markdown: {
        getMarkdown: () => markdown,
      },
    },
  } as any
}

/** Build minimal deps for useEditorSave. */
function makeDeps(overrides: Partial<Parameters<typeof useEditorSave>[0]> = {}) {
  const editor = ref<any>(mockEditor())
  const noteFileName = ref('20240101abcdefghijklmnop.md')
  const isContentEmpty = ref(false)
  const serverEncrypt = ref(false)
  const indexNote = vi.fn()
  const scheduleOrphanCleanup = vi.fn()
  const t = ref<Record<string, string>>({
    unsaved: 'Unsaved',
    saving: 'Saving…',
    saved: 'Saved',
    saveError: 'Save error',
  })
  return {
    editor,
    noteFileName,
    isContentEmpty,
    serverEncrypt,
    indexNote,
    scheduleOrphanCleanup,
    t,
    ...overrides,
  }
}

// ── Setup / teardown ──────────────────────────────────────────────────────────

beforeEach(() => {
  vi.useFakeTimers()
  vi.mocked(axios.put).mockResolvedValue({ data: {} })
  vi.mocked(encryptText).mockImplementation(async (s: string) => `ENC1:${s}`)
  vi.mocked(pendingNotes.delete).mockClear()
  vi.mocked(pendingNotes.has).mockReturnValue(true)
})

afterEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
})

// ── Initial state ─────────────────────────────────────────────────────────────

describe('initial state', () => {
  it('saveStatus is idle', () => {
    const { saveStatus } = useEditorSave(makeDeps())
    expect(saveStatus.value).toBe('idle')
  })

  it('lastSaveError is empty string', () => {
    const { lastSaveError } = useEditorSave(makeDeps())
    expect(lastSaveError.value).toBe('')
  })

  it('lastSavedTime is null', () => {
    const { lastSavedTime } = useEditorSave(makeDeps())
    expect(lastSavedTime.value).toBeNull()
  })
})

// ── Computed display ──────────────────────────────────────────────────────────

describe('statusText', () => {
  it('returns empty string when idle', () => {
    const { statusText } = useEditorSave(makeDeps())
    expect(statusText.value).toBe('')
  })

  it('returns unsaved label when dirty', async () => {
    const { saveStatus, statusText } = useEditorSave(makeDeps())
    saveStatus.value = 'dirty'
    expect(statusText.value).toBe('Unsaved')
  })

  it('returns saving label when saving', async () => {
    const { saveStatus, statusText } = useEditorSave(makeDeps())
    saveStatus.value = 'saving'
    expect(statusText.value).toBe('Saving…')
  })

  it('returns saved label when saved', async () => {
    const { saveStatus, statusText } = useEditorSave(makeDeps())
    saveStatus.value = 'saved'
    expect(statusText.value).toBe('Saved')
  })

  it('returns error label when error', async () => {
    const { saveStatus, statusText } = useEditorSave(makeDeps())
    saveStatus.value = 'error'
    expect(statusText.value).toBe('Save error')
  })
})

describe('saveStatusStyle', () => {
  it('idle → muted color', () => {
    const { saveStatusStyle } = useEditorSave(makeDeps())
    expect(saveStatusStyle.value).toContain('--text-muted')
  })

  it('dirty → warning color', () => {
    const { saveStatus, saveStatusStyle } = useEditorSave(makeDeps())
    saveStatus.value = 'dirty'
    expect(saveStatusStyle.value).toContain('--color-warning')
  })

  it('saving → accent color', () => {
    const { saveStatus, saveStatusStyle } = useEditorSave(makeDeps())
    saveStatus.value = 'saving'
    expect(saveStatusStyle.value).toContain('--accent')
  })

  it('saved → success color', () => {
    const { saveStatus, saveStatusStyle } = useEditorSave(makeDeps())
    saveStatus.value = 'saved'
    expect(saveStatusStyle.value).toContain('--color-success')
  })

  it('error → danger color', () => {
    const { saveStatus, saveStatusStyle } = useEditorSave(makeDeps())
    saveStatus.value = 'error'
    expect(saveStatusStyle.value).toContain('--color-danger')
  })
})

describe('saveDotStyle', () => {
  it('idle → transparent background', () => {
    const { saveDotStyle } = useEditorSave(makeDeps())
    expect(saveDotStyle.value).toContain('transparent')
  })

  it('dirty → warning background', () => {
    const { saveStatus, saveDotStyle } = useEditorSave(makeDeps())
    saveStatus.value = 'dirty'
    expect(saveDotStyle.value).toContain('--color-warning')
  })
})

// ── doSave ────────────────────────────────────────────────────────────────────

describe('doSave – success path', () => {
  it('sets saveStatus to saved after successful PUT', async () => {
    const deps = makeDeps()
    const { doSave, saveStatus } = useEditorSave(deps)
    await doSave()
    expect(saveStatus.value).toBe('saved')
  })

  it('PUTs to /api/notes/<filename>', async () => {
    const deps = makeDeps()
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).toHaveBeenCalledWith(
      '/api/notes/20240101abcdefghijklmnop.md',
      expect.objectContaining({ content: expect.any(String) }),
    )
  })

  it('sets lastSavedTime to a Date on success', async () => {
    const deps = makeDeps()
    const { doSave, lastSavedTime } = useEditorSave(deps)
    await doSave()
    expect(lastSavedTime.value).toBeInstanceOf(Date)
  })

  it('calls indexNote with filename and plain text', async () => {
    const deps = makeDeps()
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(deps.indexNote).toHaveBeenCalledWith(
      '20240101abcdefghijklmnop.md',
      '# Hello',
    )
  })

  it('calls scheduleOrphanCleanup on success', async () => {
    const deps = makeDeps()
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(deps.scheduleOrphanCleanup).toHaveBeenCalledOnce()
  })

  it('removes note from pendingNotes on first save', async () => {
    const deps = makeDeps()
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(pendingNotes.delete).toHaveBeenCalledWith('20240101abcdefghijklmnop.md')
  })

  it('transitions saving → saved (status passes through saving)', async () => {
    const deps = makeDeps()
    let statusDuringSave: string | undefined
    vi.mocked(axios.put).mockImplementation(async () => {
      // Capture mid-save status synchronously
      statusDuringSave = composable.saveStatus.value
      return { data: {} }
    })
    const composable = useEditorSave(deps)
    await composable.doSave()
    expect(statusDuringSave).toBe('saving')
    expect(composable.saveStatus.value).toBe('saved')
  })
})

describe('doSave – skip conditions', () => {
  it('skips save when editor is undefined', async () => {
    const deps = makeDeps({ editor: ref(undefined) })
    const { doSave, saveStatus } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
    expect(saveStatus.value).toBe('idle')
  })

  it('skips save when noteFileName is empty', async () => {
    const deps = makeDeps({ noteFileName: ref('') })
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
  })

  it('skips save when isContentEmpty is true', async () => {
    const deps = makeDeps({ isContentEmpty: ref(true) })
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
  })
})

describe('doSave – encryption', () => {
  it('sends encrypted content when serverEncrypt is true', async () => {
    const deps = makeDeps({ serverEncrypt: ref(true) })
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).toHaveBeenCalledWith(
      expect.any(String),
      { content: 'ENC1:# Hello' },
    )
  })

  it('sends plain content when serverEncrypt is false', async () => {
    const deps = makeDeps({ serverEncrypt: ref(false) })
    const { doSave } = useEditorSave(deps)
    await doSave()
    expect(vi.mocked(axios.put)).toHaveBeenCalledWith(
      expect.any(String),
      { content: '# Hello' },
    )
  })
})

describe('doSave – error path', () => {
  it('sets saveStatus to error on axios failure', async () => {
    vi.mocked(axios.put).mockRejectedValue(new Error('Network error'))
    const deps = makeDeps()
    const { doSave, saveStatus } = useEditorSave(deps)
    await doSave()
    expect(saveStatus.value).toBe('error')
  })

  it('sets lastSaveError from axios response error message', async () => {
    vi.mocked(axios.put).mockRejectedValue({
      response: { status: 413, data: { error: 'limit_note_size' } },
    })
    const deps = makeDeps()
    const { doSave, lastSaveError } = useEditorSave(deps)
    await doSave()
    expect(lastSaveError.value).toContain('limit_note_size')
    expect(lastSaveError.value).toContain('413')
  })

  it('falls back to err.message when no response body', async () => {
    vi.mocked(axios.put).mockRejectedValue({ message: 'timeout' })
    const deps = makeDeps()
    const { doSave, lastSaveError } = useEditorSave(deps)
    await doSave()
    expect(lastSaveError.value).toBe('timeout')
  })
})

describe('doSave – serialisation', () => {
  it('second doSave waits for first to complete', async () => {
    const order: number[] = []
    let resolveFirst!: () => void
    vi.mocked(axios.put)
      .mockImplementationOnce(
        () => new Promise<any>((res) => { resolveFirst = () => res({ data: {} }); order.push(1) }),
      )
      .mockImplementationOnce(async () => { order.push(2); return { data: {} } })

    const deps = makeDeps()
    const { doSave } = useEditorSave(deps)
    const p1 = doSave()
    const p2 = doSave()
    // Flush microtasks so the first _doSaveOnce call starts and assigns resolveFirst
    await Promise.resolve()
    await Promise.resolve()
    resolveFirst()
    await Promise.all([p1, p2])
    expect(order).toEqual([1, 2]) // second save ran after first completed
  })
})

// ── onContentChanged ──────────────────────────────────────────────────────────

describe('onContentChanged', () => {
  it('immediately sets saveStatus to dirty', () => {
    const deps = makeDeps()
    const { onContentChanged, saveStatus } = useEditorSave(deps)
    onContentChanged()
    expect(saveStatus.value).toBe('dirty')
  })

  it('triggers doSave after autoSaveDebounceMs', async () => {
    const deps = makeDeps()
    const { onContentChanged } = useEditorSave(deps)
    onContentChanged()
    vi.advanceTimersByTime(config.autoSaveDebounceMs)
    await Promise.resolve() // flush microtask queue
    expect(vi.mocked(axios.put)).toHaveBeenCalledOnce()
  })

  it('cancels previous debounce timer on repeated calls', async () => {
    const deps = makeDeps()
    const { onContentChanged } = useEditorSave(deps)
    onContentChanged()
    vi.advanceTimersByTime(config.autoSaveDebounceMs - 100)
    onContentChanged() // reset timer
    vi.advanceTimersByTime(config.autoSaveDebounceMs - 100)
    // Not enough time elapsed since second call — should NOT have saved yet
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
    vi.advanceTimersByTime(200) // now crosses the debounce threshold
    await Promise.resolve()
    expect(vi.mocked(axios.put)).toHaveBeenCalledOnce()
  })

  it('force-saves after autoSaveIntervalMs during continuous editing', async () => {
    const deps = makeDeps()
    const { onContentChanged, saveStatus } = useEditorSave(deps)
    onContentChanged()
    saveStatus.value = 'dirty' // keep dirty to allow interval save
    // Advance to just before interval fires
    vi.advanceTimersByTime(config.autoSaveIntervalMs)
    await Promise.resolve()
    expect(vi.mocked(axios.put)).toHaveBeenCalledOnce()
  })
})

// ── clearTimers / resetForLoad ────────────────────────────────────────────────

describe('resetForLoad', () => {
  it('resets saveStatus to idle', () => {
    const deps = makeDeps()
    const { saveStatus, resetForLoad } = useEditorSave(deps)
    saveStatus.value = 'dirty'
    resetForLoad()
    expect(saveStatus.value).toBe('idle')
  })

  it('clears lastSavedTime', async () => {
    const deps = makeDeps()
    const { doSave, lastSavedTime, resetForLoad } = useEditorSave(deps)
    await doSave()
    expect(lastSavedTime.value).not.toBeNull()
    resetForLoad()
    expect(lastSavedTime.value).toBeNull()
  })

  it('cancels pending debounce timer after resetForLoad', async () => {
    const deps = makeDeps()
    const { onContentChanged, resetForLoad } = useEditorSave(deps)
    onContentChanged()
    resetForLoad()
    vi.advanceTimersByTime(config.autoSaveDebounceMs + 100)
    await Promise.resolve()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
  })
})

// ── saveIfDirty ───────────────────────────────────────────────────────────────

describe('saveIfDirty', () => {
  it('calls doSave when status is dirty', async () => {
    const deps = makeDeps()
    const { saveStatus, saveIfDirty } = useEditorSave(deps)
    saveStatus.value = 'dirty'
    await saveIfDirty()
    expect(vi.mocked(axios.put)).toHaveBeenCalledOnce()
  })

  it('does not call doSave when status is idle', async () => {
    const deps = makeDeps()
    const { saveIfDirty } = useEditorSave(deps)
    await saveIfDirty()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
  })

  it('does not call doSave when status is saved', async () => {
    const deps = makeDeps()
    const { saveStatus, saveIfDirty } = useEditorSave(deps)
    saveStatus.value = 'saved'
    await saveIfDirty()
    expect(vi.mocked(axios.put)).not.toHaveBeenCalled()
  })
})
