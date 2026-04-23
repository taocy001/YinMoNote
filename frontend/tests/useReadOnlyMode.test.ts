/**
 * Unit tests for useReadOnlyMode composable.
 *
 * Covers:
 * - isTouchDevice: true when matchMedia('pointer: coarse').matches is true
 * - isTouchDevice: false when matchMedia('pointer: coarse').matches is false
 * - isReadOnly initial value equals isTouchDevice
 * - toggleReadOnly: flips isReadOnly from false to true
 * - toggleReadOnly: flips isReadOnly from true to false
 * - toggleReadOnly: multiple toggles alternate correctly
 * - resetToDeviceDefault: resets isReadOnly to isTouchDevice after manual toggle
 * - resetToDeviceDefault: is idempotent when isReadOnly already matches isTouchDevice
 * - Non-touch device: isReadOnly starts false; toggle makes it true; reset makes it false
 * - Touch device: isReadOnly starts true; toggle makes it false; reset makes it true
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// ── Helpers ────────────────────────────────────────────────────────────────────

function mockMatchMedia(matches: boolean) {
  vi.stubGlobal('matchMedia', vi.fn((query: string) => ({
    matches,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })))
}

// ── Teardown ───────────────────────────────────────────────────────────────────

afterEach(() => {
  vi.unstubAllGlobals()
  vi.resetModules()
})

// ── isTouchDevice detection ────────────────────────────────────────────────────

describe('isTouchDevice detection', () => {
  it('is true when pointer:coarse matches', async () => {
    mockMatchMedia(true)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isTouchDevice } = useReadOnlyMode()
    expect(isTouchDevice).toBe(true)
  })

  it('is false when pointer:coarse does not match', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isTouchDevice } = useReadOnlyMode()
    expect(isTouchDevice).toBe(false)
  })

  it('passes the correct media query string to matchMedia', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    useReadOnlyMode()
    expect(vi.mocked(matchMedia)).toHaveBeenCalledWith('(pointer: coarse)')
  })
})

// ── isReadOnly initial value ───────────────────────────────────────────────────

describe('isReadOnly initial value', () => {
  it('starts true on touch device', async () => {
    mockMatchMedia(true)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly } = useReadOnlyMode()
    expect(isReadOnly.value).toBe(true)
  })

  it('starts false on non-touch device', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly } = useReadOnlyMode()
    expect(isReadOnly.value).toBe(false)
  })

  it('isReadOnly is a Vue Ref (has .value)', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly } = useReadOnlyMode()
    // Should be writable via .value
    isReadOnly.value = true
    expect(isReadOnly.value).toBe(true)
  })
})

// ── toggleReadOnly ─────────────────────────────────────────────────────────────

describe('toggleReadOnly', () => {
  it('flips isReadOnly from false to true (non-touch)', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly } = useReadOnlyMode()
    expect(isReadOnly.value).toBe(false)
    toggleReadOnly()
    expect(isReadOnly.value).toBe(true)
  })

  it('flips isReadOnly from true to false (touch)', async () => {
    mockMatchMedia(true)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly } = useReadOnlyMode()
    expect(isReadOnly.value).toBe(true)
    toggleReadOnly()
    expect(isReadOnly.value).toBe(false)
  })

  it('multiple toggles alternate the value', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly } = useReadOnlyMode()
    toggleReadOnly() // false → true
    expect(isReadOnly.value).toBe(true)
    toggleReadOnly() // true → false
    expect(isReadOnly.value).toBe(false)
    toggleReadOnly() // false → true
    expect(isReadOnly.value).toBe(true)
  })

  it('is a function', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { toggleReadOnly } = useReadOnlyMode()
    expect(typeof toggleReadOnly).toBe('function')
  })
})

// ── resetToDeviceDefault ──────────────────────────────────────────────────────

describe('resetToDeviceDefault', () => {
  it('resets to false after toggling on non-touch device', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    toggleReadOnly() // false → true
    expect(isReadOnly.value).toBe(true)
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(false)
  })

  it('resets to true after toggling on touch device', async () => {
    mockMatchMedia(true)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    toggleReadOnly() // true → false
    expect(isReadOnly.value).toBe(false)
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(true)
  })

  it('is idempotent: calling twice keeps value at device default', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    resetToDeviceDefault()
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(false)
  })

  it('is idempotent after multiple toggles', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, toggleReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    toggleReadOnly()
    toggleReadOnly()
    toggleReadOnly() // ends at true (odd toggles)
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(false) // non-touch default
  })

  it('is a function', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { resetToDeviceDefault } = useReadOnlyMode()
    expect(typeof resetToDeviceDefault).toBe('function')
  })
})

// ── Full lifecycle ────────────────────────────────────────────────────────────

describe('full lifecycle', () => {
  it('non-touch: init→false, toggle→true, reset→false', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, isTouchDevice, toggleReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    expect(isTouchDevice).toBe(false)
    expect(isReadOnly.value).toBe(false)
    toggleReadOnly()
    expect(isReadOnly.value).toBe(true)
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(false)
  })

  it('touch: init→true, toggle→false, reset→true', async () => {
    mockMatchMedia(true)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const { isReadOnly, isTouchDevice, toggleReadOnly, resetToDeviceDefault } = useReadOnlyMode()
    expect(isTouchDevice).toBe(true)
    expect(isReadOnly.value).toBe(true)
    toggleReadOnly()
    expect(isReadOnly.value).toBe(false)
    resetToDeviceDefault()
    expect(isReadOnly.value).toBe(true)
  })

  it('each call to useReadOnlyMode returns independent state', async () => {
    mockMatchMedia(false)
    const { useReadOnlyMode } = await import('../src/composables/useReadOnlyMode')
    const a = useReadOnlyMode()
    const b = useReadOnlyMode()
    a.toggleReadOnly() // only toggles a
    expect(a.isReadOnly.value).toBe(true)
    expect(b.isReadOnly.value).toBe(false) // b is unaffected
  })
})
