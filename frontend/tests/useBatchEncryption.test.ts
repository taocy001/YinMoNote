/**
 * TEST-P1-5: Unit tests for useBatchEncryption composable.
 *
 * Covers:
 * - locked library aborts immediately
 * - successful encryption of plaintext notes
 * - partial failure reported in batchResultMsg
 * - axios failure counted as failed, does not corrupt other notes
 * - mixed plain/encrypted state: only processes notes needing conversion
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'

vi.mock('axios', () => ({
  default: {
    get: vi.fn(),
    put: vi.fn(),
    interceptors: { request: { use: vi.fn() } },
  },
}))

vi.mock('../src/crypto', async () => {
  const actual = await vi.importActual('../src/crypto')
  return {
    ...actual,
    isLibraryLocked: vi.fn(() => false),
    encryptText: vi.fn(async (text: string) => `ENC1:${text}`),
    decryptText: vi.fn(async (text: string) => {
      if (text.startsWith('ENC1:')) return text.slice(5)
      return text
    }),
  }
})

import axios from 'axios'
import * as crypto from '../src/crypto'
import { useBatchEncryption } from '../src/composables/useBatchEncryption'

const ax = axios as any
const cr = crypto as any

beforeEach(() => {
  vi.clearAllMocks()
  cr.isLibraryLocked.mockReturnValue(false)
  ax.put.mockResolvedValue({ data: {} })
})

describe('useBatchEncryption', () => {
  it('aborts immediately when library is locked', async () => {
    cr.isLibraryLocked.mockReturnValue(true)
    const batchResultMsg = ref('')
    const { batchUpdateEncryption, batchProcessing } = useBatchEncryption(
      batchResultMsg, ref('LOCKED'), ref(() => '')
    )

    await batchUpdateEncryption(true)
    expect(batchResultMsg.value).toBe('LOCKED')
    expect(ax.get).not.toHaveBeenCalled()
    expect(batchProcessing.value).toBe(false)
  })

  it('encrypts plaintext notes and calls PUT /notes/:name', async () => {
    ax.get.mockImplementation((url: string) => {
      if (url.includes('/notes') && !url.includes('/notes/')) return Promise.resolve({ data: { notes: [{ name: 'a.md' }] } })
      if (url.includes('/notes/a.md')) return Promise.resolve({ data: { content: 'hello world' } })
      if (url.includes('/assets')) return Promise.resolve({ data: { assets: [] } })
      return Promise.reject(new Error('unexpected'))
    })

    const batchResultMsg = ref('')
    const { batchUpdateEncryption } = useBatchEncryption(
      batchResultMsg, ref(''), ref(() => '')
    )

    await batchUpdateEncryption(true)
    expect(ax.put).toHaveBeenCalledWith(
      expect.stringContaining('/notes/a.md'),
      { content: 'ENC1:hello world' }
    )
    expect(batchResultMsg.value).toBe('')  // No failures
  })

  it('does not re-encrypt already-encrypted notes', async () => {
    ax.get.mockImplementation((url: string) => {
      if (url.includes('/notes') && !url.includes('/notes/')) return Promise.resolve({ data: { notes: [{ name: 'a.md' }] } })
      if (url.includes('/notes/a.md')) return Promise.resolve({ data: { content: 'ENC1:already-encrypted' } })
      if (url.includes('/assets')) return Promise.resolve({ data: { assets: [] } })
      return Promise.reject(new Error('unexpected'))
    })
    // decryptText returns the plaintext for ENC1 content
    cr.decryptText.mockResolvedValue('already-encrypted')
    cr.encryptText.mockResolvedValue('ENC1:already-encrypted')  // Same after re-encrypt

    const batchResultMsg = ref('')
    const { batchUpdateEncryption } = useBatchEncryption(
      batchResultMsg, ref(''), ref(() => '')
    )

    await batchUpdateEncryption(true)
    // PUT should NOT be called because next === raw
    expect(ax.put).not.toHaveBeenCalled()
  })

  it('counts axios failures as failed and reports partial failure', async () => {
    ax.get.mockImplementation((url: string) => {
      if (url.includes('/notes') && !url.includes('/notes/')) return Promise.resolve({ data: { notes: [{ name: 'a.md' }, { name: 'b.md' }] } })
      if (url.includes('/notes/a.md')) return Promise.reject(new Error('network error'))
      if (url.includes('/notes/b.md')) return Promise.resolve({ data: { content: 'hello' } })
      if (url.includes('/assets')) return Promise.resolve({ data: { assets: [] } })
      return Promise.reject(new Error('unexpected'))
    })

    let reportedFailed = -1
    const batchResultMsg = ref('')
    const { batchUpdateEncryption } = useBatchEncryption(
      batchResultMsg,
      ref(''),
      ref((n: number) => { reportedFailed = n; return `${n} failed` })
    )

    await batchUpdateEncryption(true)
    expect(reportedFailed).toBe(1)
    expect(batchResultMsg.value).toBe('1 failed')
  })

  it('calls onComplete callback after processing', async () => {
    ax.get.mockImplementation((url: string) => {
      if (url.includes('/notes') && !url.includes('/notes/')) return Promise.resolve({ data: { notes: [] } })
      if (url.includes('/assets')) return Promise.resolve({ data: { assets: [] } })
      return Promise.reject(new Error('unexpected'))
    })

    const onComplete = vi.fn()
    const { batchUpdateEncryption } = useBatchEncryption(ref(''), ref(''), ref(() => ''), onComplete)
    await batchUpdateEncryption(true)
    expect(onComplete).toHaveBeenCalledOnce()
  })
})
