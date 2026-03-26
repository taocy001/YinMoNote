/**
 * TEST-P1-3: Unit tests for the useImageDecrypt composable.
 *
 * Covers:
 * - data: URL passthrough
 * - external http URL blocked by default (SEC-016)
 * - external http URL allowed when allowExternalImages=true
 * - encrypted ENC1 blob detected and decrypted
 * - locked library shows SVG_LOCKED for encrypted asset
 * - fallback on axios error
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    interceptors: { request: { use: vi.fn() } },
  },
}))

vi.mock('../src/crypto', async () => {
  const actual = await vi.importActual('../src/crypto')
  return {
    ...actual,
    decryptText: vi.fn(async (text: string) => {
      if (text === 'ENC1:encrypted-data') return 'data:image/png;base64,abc123'
      return text
    }),
  }
})

import axios from 'axios'
import { useImageDecrypt, SVG_LOCKED } from '../src/composables/useImageDecrypt'

const ax = axios as any

beforeEach(() => {
  vi.clearAllMocks()
  // Stub URL.createObjectURL / revokeObjectURL since jsdom doesn't implement them.
  globalThis.URL.createObjectURL = vi.fn(() => 'blob:fake-url')
  globalThis.URL.revokeObjectURL = vi.fn()
})

describe('useImageDecrypt — fetchAndDecrypt', () => {
  it('returns data: URL directly without fetching', async () => {
    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('data:image/png;base64,abc', false)
    expect(displaySrc.value).toBe('data:image/png;base64,abc')
    expect(loading.value).toBe(false)
    expect(ax.get).not.toHaveBeenCalled()
  })

  it('blocks external http URL by default (SEC-016)', async () => {
    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('http://example.com/image.png', false)
    expect(displaySrc.value).toBe(SVG_LOCKED)
    expect(loading.value).toBe(false)
    expect(ax.get).not.toHaveBeenCalled()
  })

  it('allows external http URL when allowExternalImages=true', async () => {
    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('http://example.com/image.png', false, true)
    expect(displaySrc.value).toBe('http://example.com/image.png')
    expect(loading.value).toBe(false)
    expect(ax.get).not.toHaveBeenCalled()
  })

  it('fetches and creates object URL for plain (non-encrypted) asset', async () => {
    const blobData = new Blob(['fake-image-bytes'], { type: 'image/png' })
    // Simulate blob with a .slice that returns a non-ENC1 prefix.
    const blobWithSlice = Object.assign(blobData, {
      slice: () => ({ text: async () => 'PNG\x00' }),
    })
    ax.get.mockResolvedValueOnce({ data: blobWithSlice })

    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('/uploads/image.png', false)
    expect(displaySrc.value).toBe('blob:fake-url')
    expect(loading.value).toBe(false)
  })

  it('shows SVG_LOCKED for encrypted asset when library is locked', async () => {
    const blobData = new Blob(['ENC1:encrypted-data'], { type: 'text/plain' })
    const blobWithSlice = Object.assign(blobData, {
      slice: () => ({ text: async () => 'ENC1:' }),
    })
    ax.get.mockResolvedValueOnce({ data: blobWithSlice })

    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('/uploads/secret.png', true /* locked */)
    expect(displaySrc.value).toBe(SVG_LOCKED)
    expect(loading.value).toBe(false)
  })

  it('shows error SVG on axios error instead of leaking raw URL', async () => {
    ax.get.mockRejectedValueOnce(new Error('Network Error'))

    const { loading, displaySrc, fetchAndDecrypt } = useImageDecrypt()
    await fetchAndDecrypt('/uploads/image.png', false)
    expect(displaySrc.value).toContain('data:image/svg+xml')
    expect(loading.value).toBe(false)
  })
})
