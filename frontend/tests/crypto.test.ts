/**
 * Tests for the E2EE crypto module.
 *
 * Covers the full key lifecycle (derivation → init → unlock → lock → reset),
 * all encryption/decryption edge cases, key export/import, and keyless mode.
 * Each describe block is independent — beforeEach resets all crypto state.
 */
import { describe, it, expect, beforeEach } from 'vitest'
import * as crypto from '../src/crypto'

beforeEach(async () => {
  localStorage.clear()
  sessionStorage.clear()
  await crypto.resetLibrary()
})

// ─── Key derivation ───────────────────────────────────────────────────────────

describe('deriveKeyFromPassword', () => {
  it('produces consistent keys from the same password (deterministic PBKDF2)', async () => {
    const k1 = await crypto.deriveKeyFromPassword('test-pass')
    const k2 = await crypto.deriveKeyFromPassword('test-pass')
    const r1 = await window.crypto.subtle.exportKey('raw', k1)
    const r2 = await window.crypto.subtle.exportKey('raw', k2)
    expect(new Uint8Array(r1)).toEqual(new Uint8Array(r2))
  })

  it('produces different keys from different passwords', async () => {
    const k1 = await crypto.deriveKeyFromPassword('password-a')
    const k2 = await crypto.deriveKeyFromPassword('password-b')
    const r1 = await window.crypto.subtle.exportKey('raw', k1)
    const r2 = await window.crypto.subtle.exportKey('raw', k2)
    expect(new Uint8Array(r1)).not.toEqual(new Uint8Array(r2))
  })

  it('produces a 256-bit (32-byte) AES-GCM key', async () => {
    const key = await crypto.deriveKeyFromPassword('any-pass')
    const raw = await window.crypto.subtle.exportKey('raw', key)
    expect(new Uint8Array(raw).length).toBe(32)
  })
})

// ─── Library lifecycle ────────────────────────────────────────────────────────

describe('hasLibrary / initLibrary / verifyAndUnlockLibrary', () => {
  it('hasLibrary returns false before any init', () => {
    expect(crypto.hasLibrary()).toBe(false)
  })

  it('hasLibrary returns true after initLibrary', async () => {
    const key = await crypto.deriveKeyFromPassword('secret')
    await crypto.initLibrary(key)
    expect(crypto.hasLibrary()).toBe(true)
  })

  it('verifyAndUnlockLibrary succeeds with the correct password', async () => {
    const key = await crypto.deriveKeyFromPassword('correct')
    await crypto.initLibrary(key)
    const sameKey = await crypto.deriveKeyFromPassword('correct')
    expect(await crypto.verifyAndUnlockLibrary(sameKey)).toBe(true)
    expect(crypto.isLibraryLocked()).toBe(false)
  })

  it('verifyAndUnlockLibrary fails with a wrong password and stays locked', async () => {
    const key = await crypto.deriveKeyFromPassword('correct')
    await crypto.initLibrary(key)
    const wrongKey = await crypto.deriveKeyFromPassword('wrong')
    expect(await crypto.verifyAndUnlockLibrary(wrongKey)).toBe(false)
    expect(crypto.isLibraryLocked()).toBe(true)
  })
})

// ─── lockLibrary / isLibraryLocked ───────────────────────────────────────────

describe('lockLibrary / isLibraryLocked', () => {
  it('isLibraryLocked is true after resetLibrary', () => {
    expect(crypto.isLibraryLocked()).toBe(true)
  })

  it('isLibraryLocked becomes false after successful unlock', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(crypto.isLibraryLocked()).toBe(false)
  })

  it('lockLibrary transitions from unlocked to locked', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(crypto.isLibraryLocked()).toBe(false)
    crypto.lockLibrary()
    expect(crypto.isLibraryLocked()).toBe(true)
  })
})

// ─── resetLibrary ─────────────────────────────────────────────────────────────

describe('resetLibrary', () => {
  it('clears hasLibrary and leaves the library locked', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(crypto.hasLibrary()).toBe(true)
    await crypto.resetLibrary()
    expect(crypto.hasLibrary()).toBe(false)
    expect(crypto.isLibraryLocked()).toBe(true)
  })

  it('clears keyless mode', () => {
    crypto.setKeylessMode()
    expect(crypto.isKeylessMode()).toBe(true)
    crypto.resetLibrary()
    expect(crypto.isKeylessMode()).toBe(false)
    expect(crypto.isLibraryLocked()).toBe(true)
  })
})

// ─── encryptText / decryptText ────────────────────────────────────────────────

describe('encryptText / decryptText', () => {
  it('produces an ENC1-prefixed ciphertext when unlocked', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(await crypto.encryptText('Hello')).toMatch(/^ENC1:/)
  })

  it('roundtrips ASCII plaintext correctly', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const plain = 'Hello, World!'
    expect(await crypto.decryptText(await crypto.encryptText(plain))).toBe(plain)
  })

  it('roundtrips CJK and multi-byte content correctly', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const plain = '隐墨笔记 — 加密笔记本 🔐'
    expect(await crypto.decryptText(await crypto.encryptText(plain))).toBe(plain)
  })

  it('uses a fresh random IV per call (no ciphertext reuse)', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const c1 = await crypto.encryptText('same text')
    const c2 = await crypto.encryptText('same text')
    expect(c1).not.toBe(c2)
  })

  it('returns plaintext unchanged when the library is locked', async () => {
    const plain = 'not encrypted'
    expect(await crypto.encryptText(plain)).toBe(plain)
  })

  it('passes non-ENC1 strings through decryptText without modification', async () => {
    expect(await crypto.decryptText('plain note content')).toBe('plain note content')
    expect(await crypto.decryptText('')).toBe('')
  })

  it('returns [Locked] when decrypting ENC1 content with no key loaded', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const encrypted = await crypto.encryptText('secret')
    crypto.lockLibrary()
    expect(await crypto.decryptText(encrypted)).toBe('[Locked]')
  })

  it('returns [Decryption Error] for a tampered or invalid ENC1 ciphertext', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(await crypto.decryptText('ENC1:invalidiv:invaliddata')).toBe('[Decryption Error]')
  })

  it('returns [Decryption Error] for ciphertext encrypted with a different key', async () => {
    const k1 = await crypto.deriveKeyFromPassword('key-one')
    await crypto.initLibrary(k1)
    const encrypted = await crypto.encryptText('data')

    // Switch to a different key
    const k2 = await crypto.deriveKeyFromPassword('key-two')
    await crypto.initLibrary(k2)
    expect(await crypto.decryptText(encrypted)).toBe('[Decryption Error]')
  })
})

// ─── encryptObject / decryptObject ───────────────────────────────────────────

describe('encryptObject / decryptObject', () => {
  it('roundtrips a plain object correctly', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const obj = { titles: { 'a.md': '笔记一' }, order: ['a.md'], tags: {} }
    const encrypted = await crypto.encryptObject(obj)
    expect(encrypted).toMatch(/^ENC1:/)
    expect(await crypto.decryptObject(encrypted)).toEqual(obj)
  })

  it('returns empty string from encryptObject when locked', async () => {
    expect(await crypto.encryptObject({ foo: 'bar' })).toBe('')
  })

  it('returns null from decryptObject when locked', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const encrypted = await crypto.encryptObject({ x: 1 })
    crypto.lockLibrary()
    expect(await crypto.decryptObject(encrypted)).toBeNull()
  })

  it('returns null for null/undefined input', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(await crypto.decryptObject(null as any)).toBeNull()
    expect(await crypto.decryptObject(undefined as any)).toBeNull()
  })

  it('returns null for the [Locked] sentinel string', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(await crypto.decryptObject('[Locked]')).toBeNull()
  })

  it('returns null for invalid (non-JSON) ciphertext', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    expect(await crypto.decryptObject('ENC1:garbage:garbage')).toBeNull()
  })
})

// ─── exportRawKey / importRawKey ─────────────────────────────────────────────

describe('exportRawKey / importRawKey', () => {
  it('exports a non-empty Base64 string', async () => {
    const key = await crypto.deriveKeyFromPassword('export-test')
    await crypto.initLibrary(key)
    const exported = await crypto.exportRawKey()
    expect(typeof exported).toBe('string')
    expect(exported.length).toBeGreaterThan(20)
  })

  it('re-imported key can decrypt data encrypted with the original', async () => {
    const key = await crypto.deriveKeyFromPassword('export-test')
    await crypto.initLibrary(key)
    const plain = 'data to protect'
    const encrypted = await crypto.encryptText(plain)

    const exported = await crypto.exportRawKey()
    crypto.lockLibrary()

    const imported = await crypto.importRawKey(exported)
    // verifyAndUnlockLibrary sets _key to the imported key
    await crypto.verifyAndUnlockLibrary(imported)
    expect(await crypto.decryptText(encrypted)).toBe(plain)
  })

  it('two exports of the same key produce the same Base64', async () => {
    const key = await crypto.deriveKeyFromPassword('stable-key')
    await crypto.initLibrary(key)
    const e1 = await crypto.exportRawKey()
    const e2 = await crypto.exportRawKey()
    expect(e1).toBe(e2)
  })
})

// ─── Keyless mode ─────────────────────────────────────────────────────────────

describe('keyless mode', () => {
  it('isKeylessMode returns false by default', () => {
    expect(crypto.isKeylessMode()).toBe(false)
  })

  it('setKeylessMode makes isKeylessMode return true', () => {
    crypto.setKeylessMode()
    expect(crypto.isKeylessMode()).toBe(true)
  })

  it('isLibraryLocked returns false in keyless mode (no unlock step needed)', () => {
    crypto.setKeylessMode()
    expect(crypto.isLibraryLocked()).toBe(false)
  })

  it('encryptText is a transparent pass-through in keyless mode', async () => {
    crypto.setKeylessMode()
    const plain = 'plain note — should not be encrypted'
    expect(await crypto.encryptText(plain)).toBe(plain)
  })

  it('decryptText is a transparent pass-through for plain text in keyless mode', async () => {
    crypto.setKeylessMode()
    expect(await crypto.decryptText('regular note content')).toBe('regular note content')
  })

  it('lockLibrary is a no-op in keyless mode', () => {
    crypto.setKeylessMode()
    crypto.lockLibrary()
    expect(crypto.isLibraryLocked()).toBe(false)
    expect(crypto.isKeylessMode()).toBe(true)
  })

  it('resetLibrary clears keyless mode and restores locked state', async () => {
    crypto.setKeylessMode()
    await crypto.resetLibrary()
    expect(crypto.isKeylessMode()).toBe(false)
    expect(crypto.isLibraryLocked()).toBe(true)
  })
})

// ─── Large note encryption (btoa spread safety) ───────────────────────────────

describe('encryptText — large payload', () => {
  it('encrypts and decrypts a 100KB payload without RangeError', async () => {
    // String.fromCharCode(...largeArray) throws RangeError when the ciphertext
    // exceeds V8's max argument count (~65535). This test catches regressions.
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const large = 'x'.repeat(100 * 1024) // 100KB
    const encrypted = await crypto.encryptText(large)
    expect(encrypted.startsWith('ENC1:')).toBe(true)
    const decrypted = await crypto.decryptText(encrypted)
    expect(decrypted).toBe(large)
  })

  it('encrypts and decrypts a 300KB payload without RangeError', async () => {
    const key = await crypto.deriveKeyFromPassword('pass')
    await crypto.initLibrary(key)
    const large = 'y'.repeat(300 * 1024) // 300KB
    const encrypted = await crypto.encryptText(large)
    const decrypted = await crypto.decryptText(encrypted)
    expect(decrypted).toBe(large)
  })
})

// ─── B2: per-user random PBKDF2 salt invariants ───────────────────────────────

const PBKDF2_SALT_KEY = 'yinmo_pbkdf2_salt'
const LIBRARY_KEY_STORE = 'note_enc_verify_v2'

describe('getOrCreateSalt — B2 per-user random salt', () => {
  it('new user: persists a random salt to localStorage on first derivation', async () => {
    expect(localStorage.getItem(PBKDF2_SALT_KEY)).toBeNull()
    await crypto.deriveKeyFromPassword('pass')
    expect(localStorage.getItem(PBKDF2_SALT_KEY)).not.toBeNull()
  })

  it('new user: successive derivations in the same session use the same salt', async () => {
    const k1 = await crypto.deriveKeyFromPassword('pass')
    const k2 = await crypto.deriveKeyFromPassword('pass')
    const r1 = await window.crypto.subtle.exportKey('raw', k1)
    const r2 = await window.crypto.subtle.exportKey('raw', k2)
    expect(new Uint8Array(r1)).toEqual(new Uint8Array(r2))
  })

  it('different sessions produce different salts (random per-user)', async () => {
    await crypto.deriveKeyFromPassword('pass')
    const salt1 = localStorage.getItem(PBKDF2_SALT_KEY)
    await crypto.resetLibrary()
    await crypto.deriveKeyFromPassword('pass')
    const salt2 = localStorage.getItem(PBKDF2_SALT_KEY)
    // Two independent random 32-byte salts should not match (probability 2^-256)
    expect(salt1).not.toBe(salt2)
  })

  it('legacy user: LIBRARY_KEY_STORE present without salt uses legacy path and does not persist a new salt', async () => {
    // Simulate an existing user whose library predates the random-salt migration
    localStorage.setItem(LIBRARY_KEY_STORE, 'legacy-verify-token')
    await crypto.deriveKeyFromPassword('pass')
    expect(localStorage.getItem(PBKDF2_SALT_KEY)).toBeNull()
  })

  it('legacy user: two derivations with same password produce the same key (legacy salt is stable)', async () => {
    localStorage.setItem(LIBRARY_KEY_STORE, 'legacy-verify-token')
    const k1 = await crypto.deriveKeyFromPassword('pass')
    const k2 = await crypto.deriveKeyFromPassword('pass')
    const r1 = await window.crypto.subtle.exportKey('raw', k1)
    const r2 = await window.crypto.subtle.exportKey('raw', k2)
    expect(new Uint8Array(r1)).toEqual(new Uint8Array(r2))
  })

  it('resetLibrary removes the persisted salt so the next session gets a fresh random salt', async () => {
    await crypto.deriveKeyFromPassword('pass')
    expect(localStorage.getItem(PBKDF2_SALT_KEY)).not.toBeNull()
    await crypto.resetLibrary()
    expect(localStorage.getItem(PBKDF2_SALT_KEY)).toBeNull()
  })
})
