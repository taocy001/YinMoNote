/**
 * Client-side E2EE layer for YinMoNote using AES-256-GCM.
 *
 * Architecture invariants:
 * 1. The master key (_key) lives in module-private memory as a CryptoKey. On first
 *    derivation it is extractable:true (needed to export JWK for session wrapping);
 *    on session restore it is imported as extractable:false (non-exportable steady state).
 *    It is never written to localStorage or exposed through any public API.
 * 2. sessionStorage holds only an encrypted copy of the key (wrapped by sessionWrapKey).
 *    sessionWrapKey is derived from window.name and never persisted, so the wrapped key
 *    becomes permanently undecryptable the moment the browser tab is closed.
 * 3. All ciphertext is prefixed with 'ENC1:' to allow zero-cost encrypted-state detection
 *    without attempting decryption.
 * 4. PBKDF2 uses a per-user random salt stored in localStorage. New users get a fresh
 *    random salt on first initialisation; existing users (pre-v2 salt) retain the legacy
 *    fixed salt for backwards compatibility so their existing notes remain decryptable.
 */

const LIBRARY_KEY_STORE = 'note_enc_verify_v2'
const LEGACY_SALT = new TextEncoder().encode('yinmo-stable-salt-v1')
const PBKDF2_SALT_KEY = 'yinmo_pbkdf2_salt'
const HW_CRED_KEY = 'yinmo_hw_cred_id'
const HW_PRF_KEY = 'yinmo_hw_prf'
const KEYLESS_KEY = 'yinmo_keyless'
const SERVER_ENCRYPT_KEY = 'yinmo_server_encrypt'

/**
 * Returns the PBKDF2 salt for this user's library.
 * - Returning users (salt in localStorage): returns stored random salt.
 * - Existing users (library token present, no stored salt): returns the
 *   legacy fixed salt so their encrypted data remains decryptable.
 * - New users: generates a fresh random 32-byte salt and persists it.
 */
function getOrCreateSalt(): Uint8Array {
  const stored = localStorage.getItem(PBKDF2_SALT_KEY)
  if (stored) {
    return Uint8Array.from(atob(stored), c => c.charCodeAt(0))
  }
  if (localStorage.getItem(LIBRARY_KEY_STORE)) {
    // Existing user without a stored salt — use legacy salt for compatibility.
    return LEGACY_SALT
  }
  // New user — generate and persist a random salt.
  const salt = window.crypto.getRandomValues(new Uint8Array(32))
  localStorage.setItem(PBKDF2_SALT_KEY, btoa(String.fromCharCode(...salt)))
  return salt
}

/**
 * Import salt from server config if the local device has none.
 * This enables cross-device password unlock: the first device generates
 * the salt and uploads it; subsequent devices download it before key derivation.
 */
export function importSaltFromConfig(serverSaltB64: string): void {
  if (!serverSaltB64) return
  const local = localStorage.getItem(PBKDF2_SALT_KEY)
  // Always sync to server's salt — a local salt that differs from the server
  // means this device generated its own salt before the sync feature existed.
  // Using the wrong salt produces the wrong key and all notes fail to decrypt.
  if (local !== serverSaltB64) {
    localStorage.setItem(PBKDF2_SALT_KEY, serverSaltB64)
  }
}

/** Return the current salt as Base64 for syncing to the server. */
export function getSaltBase64(): string {
  const stored = localStorage.getItem(PBKDF2_SALT_KEY)
  if (stored) return stored
  // Generate one if not yet created
  getOrCreateSalt()
  return localStorage.getItem(PBKDF2_SALT_KEY) || ''
}

/**
 * SEC-018: Migrates legacy users (PBKDF2_SALT_KEY absent) to per-user random salt.
 *
 * Called once after a successful password unlock. If the user has LIBRARY_KEY_STORE
 * but no PBKDF2_SALT_KEY, they are a pre-B2-fix user with the fixed legacy salt.
 * This function:
 *   1. Generates a new random salt and derives a fresh key.
 *   2. Re-encrypts the verification token with the new key.
 *   3. Persists the new salt and token, replacing the legacy values.
 *
 * On failure it silently falls back — the user stays on the legacy salt and can
 * unlock normally on the next attempt.
 */
export async function migrateLegacySaltIfNeeded(password: string): Promise<void> {
  // Only applicable to legacy users: LIBRARY_KEY_STORE exists, PBKDF2_SALT_KEY absent.
  if (localStorage.getItem(PBKDF2_SALT_KEY) || !localStorage.getItem(LIBRARY_KEY_STORE)) return
  try {
    const enc = new TextEncoder()
    const iterations = Number(import.meta.env.VITE_PBKDF2_ITERATIONS) || 100000
    // Generate a new random salt and derive a new key.
    const newSalt = window.crypto.getRandomValues(new Uint8Array(32))
    const baseKey = await window.crypto.subtle.importKey('raw', enc.encode(password), 'PBKDF2', false, ['deriveKey'])
    const newKey = await window.crypto.subtle.deriveKey(
      { name: 'PBKDF2', salt: newSalt, iterations, hash: 'SHA-256' },
      baseKey, { name: 'AES-GCM', length: 256 }, true, ['encrypt', 'decrypt']
    )
    // Re-encrypt the verification token with the new key.
    const iv = window.crypto.getRandomValues(new Uint8Array(12))
    const ciphertext = await window.crypto.subtle.encrypt(
      { name: 'AES-GCM', iv }, newKey, enc.encode('YINMO_VALID')
    )
    const newStore = {
      iv: btoa(String.fromCharCode(...iv)),
      data: btoa(String.fromCharCode(...new Uint8Array(ciphertext)))
    }
    // Commit: salt first, then token (atomic enough for localStorage).
    localStorage.setItem(PBKDF2_SALT_KEY, btoa(String.fromCharCode(...newSalt)))
    localStorage.setItem(LIBRARY_KEY_STORE, JSON.stringify(newStore))
    _key = newKey
    await saveKeyToSession(newKey)
  } catch (e) {
    console.warn('[YinMo] Salt migration failed, will retry on next unlock:', e)
  }
}

let _key: CryptoKey | null = null
// Keyless mode: the library is accessible without any encryption key.
// All encrypt/decrypt operations become identity pass-throughs.
let _keyless = false

/** Activates keyless mode. No key is needed; all crypto operations are no-ops. */
export function setKeylessMode(): void {
  _keyless = true
  _key = null
}

/** Returns true when keyless mode is active. */
export function isKeylessMode(): boolean {
  return _keyless
}

/** Returns true if a hardware (WebAuthn/Passkey) credential is registered on this device. */
export function hasHardwareKey(): boolean {
  return !!localStorage.getItem(HW_CRED_KEY)
}

/** Persists keyless mode flag so it survives page reloads. */
export function persistKeylessMode(): void {
  localStorage.setItem(KEYLESS_KEY, '1')
}

/** Returns true if keyless mode was persisted in a previous session. */
export function wasKeylessPersisted(): boolean {
  return localStorage.getItem(KEYLESS_KEY) === '1'
}

/** Clears the persisted keyless mode flag. */
export function clearKeylessPersisted(): void {
  localStorage.removeItem(KEYLESS_KEY)
}

/** Returns whether server-side encryption is enabled. */
export function isServerEncryptEnabled(): boolean {
  return localStorage.getItem(SERVER_ENCRYPT_KEY) === '1'
}

/**
 * Unified encryption decision point.
 * Returns true when content should be encrypted before sending to the server.
 * Requires both server encryption to be enabled AND a loaded key (library unlocked).
 */
export function shouldEncrypt(): boolean {
  return isServerEncryptEnabled() && !isLibraryLocked() && !_keyless
}

/** Persists the server-side encryption preference. */
export function setServerEncrypt(enabled: boolean): void {
  localStorage.setItem(SERVER_ENCRYPT_KEY, enabled ? '1' : '0')
}

/** Clears the hardware credential ID and PRF flag (used when switching to password mode). */
export function clearHardwareKey(): void {
  localStorage.removeItem(HW_CRED_KEY)
  localStorage.removeItem(HW_PRF_KEY)
}

/**
 * Attempts to restore the master key from the current browser session.
 *
 * On success, _key is set to a non-extractable CryptoKey so that subsequent
 * encrypt/decrypt calls work without requiring the user to re-authenticate.
 * On failure (wrong tab, closed and reopened, or corrupted storage), the
 * session entry is cleared and the caller should redirect to the unlock flow.
 *
 * @returns {Promise<boolean>} True if the key was successfully restored.
 */
export async function restoreKeyFromSession(): Promise<boolean> {
  const saved = sessionStorage.getItem('yinmo_session_key')
  if (!saved) return false
  try {
    const data = JSON.parse(atob(saved))
    const iv = Uint8Array.from(atob(data.iv), c => c.charCodeAt(0))
    const ciphertext = Uint8Array.from(atob(data.data), c => c.charCodeAt(0))

    // sessionWrapKey is derived from window.name, which is tab-scoped and not
    // persisted by the browser. If this tab was closed and reopened, window.name
    // will be empty/'default-session', causing decryption to fail intentionally.
    const wrapKey = await deriveSessionWrapKey()
    const decrypted = await window.crypto.subtle.decrypt({ name: 'AES-GCM', iv }, wrapKey, ciphertext)
    const jwk = JSON.parse(new TextDecoder().decode(decrypted))

    // Import as non-extractable so that XSS or console access cannot retrieve
    // the raw key bytes even after successful session restore.
    _key = await window.crypto.subtle.importKey(
      'jwk', jwk, { name: 'AES-GCM' }, false, ['encrypt', 'decrypt']
    )
    return true
  } catch (_e) {
    sessionStorage.removeItem('yinmo_session_key')
    return false
  }
}

/**
 * Wraps the master key with sessionWrapKey and stores the result in sessionStorage.
 *
 * The JWK export is a necessary intermediate step for wrapping; the plaintext JWK
 * exists only in local function scope and is not retained after encryption completes.
 * The wrapped key in sessionStorage is useless without sessionWrapKey, which is never
 * persisted and can only be reconstructed while the same browser tab remains open.
 */
async function saveKeyToSession(key: CryptoKey) {
  try {
    const jwk = await window.crypto.subtle.exportKey('jwk', key)
    const wrapKey = await deriveSessionWrapKey()
    const iv = window.crypto.getRandomValues(new Uint8Array(12))
    const encoded = new TextEncoder().encode(JSON.stringify(jwk))
    const ciphertext = await window.crypto.subtle.encrypt({ name: 'AES-GCM', iv }, wrapKey, encoded)

    const store = {
      iv: btoa(String.fromCharCode(...iv)),
      data: btoa(bufToString(ciphertext))
    }
    sessionStorage.setItem('yinmo_session_key', btoa(JSON.stringify(store)))
  } catch (e) {
    console.error('Failed to save key to session', e)
  }
}

/**
 * Derives a transient wrapping key bound to the current browser tab.
 *
 * window.name persists across navigations within the same tab but is cleared when
 * the tab is closed. By deriving the wrap key from window.name, we get a key that
 * is effectively tab-scoped: navigating away and back still works, but closing and
 * reopening the tab destroys the ability to unwrap the session key. This provides
 * automatic session expiry without an explicit timeout mechanism.
 *
 * The 10,000-iteration count is intentionally low — this key protects against an
 * offline attacker who steals sessionStorage but not window.name (which is not
 * accessible outside the tab). The primary entropy comes from window.name being
 * set to a random value by the app at session start.
 *
 * Security note: Under an XSS attack, both window.name and sessionStorage
 * are readable, making this PBKDF2 iteration count irrelevant. The real last-defence
 * is the non-extractable _key CryptoKey — even under XSS the raw key bytes cannot be
 * exfiltrated via the WebCrypto API.
 */
async function deriveSessionWrapKey(): Promise<CryptoKey> {
  const salt = new TextEncoder().encode('yinmo-session-wrap-v1')
  const baseKey = await window.crypto.subtle.importKey(
    'raw', new TextEncoder().encode(window.name || 'default-session'), 'PBKDF2', false, ['deriveKey']
  )
  return await window.crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt, iterations: 10000, hash: 'SHA-256' },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  )
}

/**
 * Derives the master AES-256-GCM key from a user-provided password via PBKDF2.
 *
 * The key is set extractable:true so that saveKeyToSession can export the JWK
 * once for wrapping. This is the unavoidable tradeoff: Web Crypto does not
 * support wrapping a non-extractable key directly. The JWK plaintext is
 * immediately discarded inside saveKeyToSession after encryption.
 *
 * 100,000 PBKDF2 iterations aligns with OWASP 2023 guidance for SHA-256.
 *
 * @param {string} password - The user's passphrase.
 * @returns {Promise<CryptoKey>} The derived AES-GCM key (also set as the active _key).
 */
export async function deriveKeyFromPassword(password: string): Promise<CryptoKey> {
  const enc = new TextEncoder()
  const baseKey = await window.crypto.subtle.importKey(
    'raw', enc.encode(password), 'PBKDF2', false, ['deriveKey']
  )
  // Allow the iteration count to be overridden at Vite build time via
  // VITE_PBKDF2_ITERATIONS.  Production builds default to 100 000 (OWASP 2023).
  // E2E test images set VITE_PBKDF2_ITERATIONS=1000 so the Alpine Chromium
  // PBKDF2 completes in milliseconds instead of tens of seconds.
  const iterations = Number(import.meta.env.VITE_PBKDF2_ITERATIONS) || 100000
  const key = await window.crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: getOrCreateSalt(), iterations, hash: 'SHA-256' },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    true,
    ['encrypt', 'decrypt']
  )
  _key = key
  await saveKeyToSession(key)
  return key
}

/**
 * Derives the master key using a WebAuthn platform authenticator (TouchID, FaceID, PIN).
 *
 * Security model (two tiers):
 *
 * Tier 1 — PRF-capable authenticators (Chrome 120+, macOS Sonoma+):
 *   The WebAuthn PRF extension (`prf`) is used to obtain a deterministic,
 *   hardware-bound secret from the authenticator. This secret is combined with
 *   the credential rawId to form the PBKDF2 password input. The PRF output
 *   never leaves the authenticator in cleartext — even with full localStorage
 *   access, the master key cannot be derived without the hardware.
 *
 * Tier 2 — Legacy fallback (no PRF support):
 *   Falls back to rawId-only derivation. This provides only UI-level gating
 *   (biometric prompt) — the rawId stored in localStorage is sufficient to
 *   derive the key. Users on this tier see a console warning.
 *
 * The localStorage flag HW_PRF_KEY tracks which tier was used during registration
 * so that subsequent unlocks use the matching derivation path.
 *
 * @returns {Promise<CryptoKey>} The derived AES-GCM key (also set as the active _key).
 */
export async function deriveKeyFromHardware(): Promise<CryptoKey> {
  // WebAuthn requires a valid domain as RP ID — IP addresses are not allowed
  const host = window.location.hostname
  if (/^\d+\.\d+\.\d+\.\d+$/.test(host) || host === 'localhost') {
    throw new Error('WebAuthn requires a domain name. IP addresses and localhost are not supported.')
  }

  const credentialId = localStorage.getItem(HW_CRED_KEY)
  const prfSalt = new TextEncoder().encode('yinmo-prf-salt-v1')
  let key: CryptoKey

  if (!credentialId) {
    // ── Registration ──────────────────────────────────────────────────────
    const registrationOptions: any = {
      publicKey: {
        challenge: window.crypto.getRandomValues(new Uint8Array(32)),
        rp: { name: "YinMoNote", id: window.location.hostname },
        user: { id: new Uint8Array(16), name: "user", displayName: "User" },
        pubKeyCredParams: [{ alg: -7, type: "public-key" }],
        authenticatorSelection: { authenticatorAttachment: "platform", userVerification: "required" },
        extensions: { prf: { eval: { first: prfSalt } } },
        timeout: 60000
      }
    }
    const cred: any = await navigator.credentials.create(registrationOptions)
    const idB64 = btoa(String.fromCharCode(...new Uint8Array(cred.rawId)))
    localStorage.setItem(HW_CRED_KEY, idB64)

    const prfResult = cred.getClientExtensionResults?.()?.prf
    if (prfResult?.results?.first) {
      // Tier 1: combine hardware-bound PRF secret with rawId
      const prfSecret = new Uint8Array(prfResult.results.first)
      const combined = new Uint8Array(prfSecret.length + cred.rawId.byteLength)
      combined.set(prfSecret, 0)
      combined.set(new Uint8Array(cred.rawId), prfSecret.length)
      const combinedB64 = btoa(String.fromCharCode(...combined))
      localStorage.setItem(HW_PRF_KEY, '1')
      key = await deriveKeyFromPassword(combinedB64)
    } else {
      // Tier 2: legacy rawId-only (UI gating only)
      console.warn('[YinMo] WebAuthn PRF not supported — hardware key provides UI-level gating only')
      localStorage.removeItem(HW_PRF_KEY)
      key = await deriveKeyFromPassword(idB64)
    }
  } else {
    // ── Assertion (unlock) ────────────────────────────────────────────────
    const hasPrf = localStorage.getItem(HW_PRF_KEY) === '1'
    const rawId = Uint8Array.from(atob(credentialId), c => c.charCodeAt(0))
    const assertionOptions: any = {
      publicKey: {
        challenge: window.crypto.getRandomValues(new Uint8Array(32)),
        allowCredentials: [{ type: 'public-key', id: rawId }],
        userVerification: 'required',
        extensions: hasPrf ? { prf: { eval: { first: prfSalt } } } : {},
        timeout: 60000
      }
    }
    const assertion: any = await navigator.credentials.get(assertionOptions)

    if (hasPrf) {
      // Tier 1: derive from PRF secret + rawId
      const prfResult = assertion.getClientExtensionResults?.()?.prf
      if (!prfResult?.results?.first) {
        throw new Error('WebAuthn PRF extension did not return a result — hardware key may have changed')
      }
      const prfSecret = new Uint8Array(prfResult.results.first)
      const combined = new Uint8Array(prfSecret.length + rawId.length)
      combined.set(prfSecret, 0)
      combined.set(rawId, prfSecret.length)
      const combinedB64 = btoa(String.fromCharCode(...combined))
      key = await deriveKeyFromPassword(combinedB64)
    } else {
      // Tier 2: legacy rawId-only
      key = await deriveKeyFromPassword(credentialId)
    }
  }

  _key = key
  await saveKeyToSession(key)
  return key
}

/**
 * Returns true if a library verification token exists in localStorage.
 * Used to distinguish "first-time setup" from "locked but initialized" states.
 */
export function hasLibrary(): boolean {
  return !!localStorage.getItem(LIBRARY_KEY_STORE)
}

/**
 * Writes the library verification token to localStorage.
 *
 * The token is a known plaintext ("YINMO_VALID") encrypted with the provided key.
 * On subsequent unlocks, verifyAndUnlockLibrary decrypts the token and checks the
 * plaintext to confirm the key is correct — equivalent to a MAC over the known value.
 * The token reveals nothing about note contents; it only confirms key correctness.
 */
export async function initLibrary(key: CryptoKey): Promise<void> {
  _key = key
  await saveKeyToSession(key)
  const iv = window.crypto.getRandomValues(new Uint8Array(12))
  const enc = new TextEncoder().encode("YINMO_VALID")
  const ciphertext = await window.crypto.subtle.encrypt({ name: 'AES-GCM', iv }, _key, enc)
  const store = {
    iv: btoa(String.fromCharCode(...iv)),
    data: btoa(String.fromCharCode(...new Uint8Array(ciphertext)))
  }
  localStorage.setItem(LIBRARY_KEY_STORE, JSON.stringify(store))
}

/**
 * Verifies that the provided key can decrypt the library verification token.
 *
 * AES-GCM authentication tag failure throws a DOMException, which is caught here
 * and treated as a wrong-key signal. A successful decrypt confirms the key is correct
 * without needing to compare key bytes directly.
 *
 * @returns {Promise<boolean>} True if the key matches what was used during initLibrary.
 */
export async function verifyAndUnlockLibrary(key: CryptoKey): Promise<boolean> {
  const raw = localStorage.getItem(LIBRARY_KEY_STORE)
  if (!raw) return false
  try {
    const store = JSON.parse(raw)
    const iv = Uint8Array.from(atob(store.iv), c => c.charCodeAt(0))
    const data = Uint8Array.from(atob(store.data), c => c.charCodeAt(0))
    const decrypted = await window.crypto.subtle.decrypt({ name: 'AES-GCM', iv }, key, data)
    if (new TextDecoder().decode(decrypted) === "YINMO_VALID") {
      _key = key
      await saveKeyToSession(key)
      return true
    }
  } catch (_e) {
    // Decryption failure (wrong key or tampered token) — fall through to lockLibrary.
  }
  lockLibrary()
  return false
}

/**
 * Zeroes out the active key reference and clears the session-wrapped copy.
 * After this call, all encrypt/decrypt operations return locked-state values
 * until a new key is derived or restored.
 */
export function lockLibrary(): void {
  if (_keyless) return   // Keyless libraries cannot be locked.
  _key = null
  sessionStorage.removeItem('yinmo_session_key')
  sessionStorage.removeItem('yinmo_session_token')
}

/** Returns true when no master key is loaded and keyless mode is not active. */
export function isLibraryLocked(): boolean {
  return _key === null && !_keyless
}

/**
 * Encrypts plaintext with AES-256-GCM using the current master key.
 * A fresh random 96-bit IV is generated per call, ensuring ciphertext uniqueness.
 * Returns the plaintext unchanged if the library is locked (caller decides how to handle).
 *
 * Output format: `ENC1:<iv_base64>:<ciphertext_base64>`
 */
export async function encryptText(text: string): Promise<string> {
  if (!_key || _keyless) return text
  const iv = window.crypto.getRandomValues(new Uint8Array(12))
  const encoded = new TextEncoder().encode(text)
  const ciphertext = await window.crypto.subtle.encrypt({ name: 'AES-GCM', iv }, _key, encoded)
  // Use a loop instead of spread (...) to avoid RangeError on large notes.
  // String.fromCharCode(...largeArray) exceeds V8's maximum argument count (~65535)
  // for notes approaching the 512KB limit. The loop is equivalent and always safe.
  return `ENC1:${btoa(String.fromCharCode(...iv))}:${btoa(bufToString(ciphertext))}`
}

/** Converts an ArrayBuffer to a binary string without the argument-count limit of spread.
 * Uses array + join instead of string concatenation to avoid O(n²) allocations. */
function bufToString(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf)
  const chars = new Array<string>(bytes.length)
  for (let i = 0; i < bytes.length; i++) chars[i] = String.fromCharCode(bytes[i])
  return chars.join('')
}

/**
 * Decrypts an ENC1-prefixed ciphertext string.
 * Non-ENC1 strings are returned as-is, allowing callers to handle plaintext notes
 * transparently without checking the prefix themselves.
 *
 * @returns The decrypted plaintext, '[Locked]' if no key is loaded, or
 *          '[Decryption Error]' if the ciphertext is invalid or the key is wrong.
 */
export async function decryptText(encrypted: string): Promise<string> {
  if (!encrypted.startsWith('ENC1:')) return encrypted
  if (!_key) return '[Locked]'
  try {
    const [, ivB64, cipherB64] = encrypted.split(':')
    const iv = Uint8Array.from(atob(ivB64), c => c.charCodeAt(0))
    const data = Uint8Array.from(atob(cipherB64), c => c.charCodeAt(0))
    const decrypted = await window.crypto.subtle.decrypt({ name: 'AES-GCM', iv }, _key, data)
    return new TextDecoder().decode(decrypted)
  } catch (_e) {
    return '[Decryption Error]'
  }
}

/**
 * Removes all library credentials and key material.
 * Intended for "sign out" / "reset" flows where the user wants to start fresh.
 */
export async function resetLibrary(): Promise<void> {
  localStorage.removeItem(LIBRARY_KEY_STORE)
  localStorage.removeItem(PBKDF2_SALT_KEY)
  localStorage.removeItem(HW_CRED_KEY)
  localStorage.removeItem(HW_PRF_KEY)
  localStorage.removeItem(KEYLESS_KEY)
  localStorage.removeItem(SERVER_ENCRYPT_KEY)
  sessionStorage.removeItem('yinmo_session_key')
  sessionStorage.removeItem('yinmo_session_token')
  localStorage.removeItem('yinmo_note_titles_v2')
  localStorage.removeItem('yinmo_structure_backup_v2')
  _key = null
  _keyless = false
}

/**
 * Exports the raw master key bytes as a Base64 string for backup purposes.
 * The caller is responsible for securing the exported value.
 */
export async function exportRawKey(): Promise<string> {
  if (!_key) throw new Error('No key loaded')
  const exported = await window.crypto.subtle.exportKey('raw', _key)
  return btoa(String.fromCharCode(...new Uint8Array(exported)))
}

/**
 * Imports a raw key from a Base64 backup string.
 * Imported as extractable:true to allow the caller to re-wrap it via saveKeyToSession.
 */
export async function importRawKey(b64: string): Promise<CryptoKey> {
  const raw = Uint8Array.from(atob(b64), c => c.charCodeAt(0))
  return await window.crypto.subtle.importKey(
    'raw', raw, { name: 'AES-GCM' }, true, ['encrypt', 'decrypt']
  )
}

/**
 * Serialises an object to JSON, then encrypts it as a single ENC1 blob.
 * Used for the structure metadata and local backup, keeping them opaque to the server.
 * In keyless mode, returns the plain JSON (consistent with decryptObject which JSON.parses it).
 * Returns '' only if a key should exist but is not loaded — callers treat '' as a skip signal.
 */
export async function encryptObject(obj: any): Promise<string> {
  if (_keyless) return JSON.stringify(obj)
  if (!_key) return ''
  const text = JSON.stringify(obj)
  return await encryptText(text)
}

/**
 * Decrypts an ENC1 blob and parses it back to a JavaScript object.
 * Returns null rather than throwing so callers can treat decryption failure as
 * a missing-value signal and fall back to defaults or a re-fetch.
 */
export async function decryptObject(cipherText: string): Promise<any> {
  if (_keyless) { try { return JSON.parse(cipherText) } catch { return null } }
  if (!_key || !cipherText || cipherText === '[Locked]') return null
  const text = await decryptText(cipherText)
  if (text === '[Decryption Error]') return null
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

// ─── SRP-6a Client (server-side auth) ─────────────────────────────────────────
//
// Implements RFC 5054 / RFC 2945 SRP-6a using the 2048-bit group from RFC 5054
// Appendix A.  All BigInt arithmetic uses native BigInt + modPow (square-and-multiply).
// No new npm packages are used.
//
// SRP username is fixed: "yinmonote" (same constant as backend srpUsername).

const SRP_USERNAME = 'yinmonote'

// RFC 5054 Appendix A 2048-bit group parameters (N and g=2).
const SRP_N = BigInt(
  '0x' +
  'FFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD1' +
  '29024E088A67CC74020BBEA63B139B22514A08798E3404DD' +
  'EF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245' +
  'E485B576625E7EC6F44C42E9A637ED6B0BFF5CB6F406B7ED' +
  'EE386BFB5A899FA5AE9F24117C4B1FE649286651ECE45B3D' +
  'C2007CB8A163BF0598DA48361C55D39A69163FA8FD24CF5F' +
  '83655D23DCA3AD961C62F356208552BB9ED529077096966D' +
  '670C354E4ABC9804F1746C08CA18217C32905E462E36CE3B' +
  'E39E772C180E86039B2783A2EC07A28FB5C55DF06F4C52C9' +
  'DE2BCBF6955817183995497CEA956AE515D2261898FA0510' +
  '15728E5A8AACAA68FFFFFFFFFFFFFFFF'
)
const SRP_G = 2n

/**
 * Modular exponentiation using square-and-multiply algorithm.
 * Required because BigInt ** n mod m is not available in JS.
 */
function modPow(base: bigint, exp: bigint, mod: bigint): bigint {
  let result = 1n
  base = base % mod
  while (exp > 0n) {
    if (exp % 2n === 1n) result = result * base % mod
    exp = exp >> 1n
    base = base * base % mod
  }
  return result
}

/**
 * Converts a bigint to a 0x-prefixed hex string padded to exactly 256 bytes
 * (512 hex characters). Required for SRP hash inputs.
 */
function bigIntToHex256(n: bigint): string {
  return n.toString(16).padStart(512, '0')
}

/**
 * Converts a hex string to a Uint8Array.
 */
function hexToBytes(hex: string): Uint8Array {
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < hex.length; i += 2) {
    bytes[i / 2] = parseInt(hex.slice(i, i + 2), 16)
  }
  return bytes
}

/**
 * Converts a Uint8Array to a lowercase hex string.
 */
function bytesToHex(bytes: Uint8Array): string {
  return Array.from(bytes).map(b => b.toString(16).padStart(2, '0')).join('')
}

/**
 * Converts a base64 string to a Uint8Array.
 */
function base64ToBytes(b64: string): Uint8Array {
  return Uint8Array.from(atob(b64), c => c.charCodeAt(0))
}

/**
 * SHA-256 of concatenated byte sequences.
 */
async function sha256(...parts: Uint8Array[]): Promise<Uint8Array> {
  const total = parts.reduce((acc, p) => acc + p.byteLength, 0)
  const buf = new Uint8Array(total)
  let off = 0
  for (const p of parts) { buf.set(p, off); off += p.byteLength }
  const digest = await window.crypto.subtle.digest('SHA-256', buf)
  return new Uint8Array(digest)
}

/**
 * Generates 32 random bytes and returns them as a BigInt.
 * Used for the client ephemeral private key a.
 */
function randomBigInt256(): bigint {
  const bytes = window.crypto.getRandomValues(new Uint8Array(32))
  let n = 0n
  for (const b of bytes) n = (n << 8n) | BigInt(b)
  return n
}

/**
 * Computes x = SHA256(salt || SHA256("yinmonote:password")).
 * Matches the server-side srpComputeX implementation.
 */
async function srpComputeX(saltBytes: Uint8Array, password: string): Promise<bigint> {
  const inner = await sha256(new TextEncoder().encode(SRP_USERNAME + ':' + password))
  const xBytes = await sha256(saltBytes, inner)
  let x = 0n
  for (const b of xBytes) x = (x << 8n) | BigInt(b)
  return x
}

/**
 * Computes the SRP verifier v = g^x mod N for use in POST /api/auth/setup.
 * Returns the verifier as a 512-char hex string.
 */
export async function srpComputeVerifier(srpSaltBytes: Uint8Array, password: string): Promise<string> {
  const x = await srpComputeX(srpSaltBytes, password)
  const v = modPow(SRP_G, x, SRP_N)
  return bigIntToHex256(v)
}

/**
 * Performs the SRP-6a authentication handshake and returns a Bearer token.
 *
 * Flow:
 *  1. Generate ephemeral key pair (a, A = g^a mod N).
 *  2. POST /api/auth/srp/init with A → receive B and srpSalt from server.
 *  3. Compute M1 = SHA256(pad256(A) || pad256(B) || pad256(S)).
 *  4. POST /api/auth/srp/verify with A + M1 → receive token + M2.
 *  5. Verify M2 to confirm server knows the verifier.
 *  6. Store and return the Bearer token.
 *
 * @param password  The user's password.
 * @param apiBase   API base URL (e.g. "/api").
 * @returns         The Bearer token, or throws on authentication failure.
 */
export async function srpAuthenticate(password: string, apiBase: string): Promise<string> {
  // Step 1: Generate client ephemeral key pair.
  const a = randomBigInt256()
  const A = modPow(SRP_G, a, SRP_N)
  const aHex = bigIntToHex256(A)

  // Step 2: Init handshake.
  const initRes = await fetch(`${apiBase}/auth/srp/init`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ A: aHex }),
  })
  if (!initRes.ok) throw new Error('srp_init_failed')
  const { salt: saltB64, B: bHex } = await initRes.json() as { salt: string; B: string }

  const saltBytes = base64ToBytes(saltB64)
  const BBytes = hexToBytes(bHex)
  let B = 0n
  for (const byte of BBytes) B = (B << 8n) | BigInt(byte)

  // Security check: B must not be 0 mod N (analogous to server's A=0 check).
  // If B = 0 mod N, the session key S = 0 and authentication is insecure.
  if (B % SRP_N === 0n) throw new Error('srp_invalid_B')

  // Compute k = SHA256(pad256(N) || pad256(g))
  const kBytes = await sha256(
    hexToBytes(bigIntToHex256(SRP_N)),
    hexToBytes(bigIntToHex256(SRP_G)),
  )
  let k = 0n
  for (const byte of kBytes) k = (k << 8n) | BigInt(byte)

  // Compute u = SHA256(pad256(A) || pad256(B))
  const uBytes = await sha256(
    hexToBytes(bigIntToHex256(A)),
    hexToBytes(bigIntToHex256(B)),
  )
  let u = 0n
  for (const byte of uBytes) u = (u << 8n) | BigInt(byte)

  // Compute x
  const x = await srpComputeX(saltBytes, password)

  // Compute S = (B - k*g^x) ^ (a + u*x) mod N
  const gx = modPow(SRP_G, x, SRP_N)
  const kgx = (k * gx) % SRP_N
  const base = ((B - kgx) % SRP_N + SRP_N) % SRP_N
  const exp = a + u * x
  const S = modPow(base, exp, SRP_N)

  // Step 3: Compute M1 = SHA256(pad256(A) || pad256(B) || pad256(S))
  const m1Bytes = await sha256(
    hexToBytes(bigIntToHex256(A)),
    hexToBytes(bigIntToHex256(B)),
    hexToBytes(bigIntToHex256(S)),
  )
  const m1Hex = bytesToHex(m1Bytes)

  // Step 4: Verify handshake.
  const verifyRes = await fetch(`${apiBase}/auth/srp/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ A: aHex, M1: m1Hex }),
  })
  if (!verifyRes.ok) throw new Error('srp_verify_failed')
  const { token, M2: m2Hex } = await verifyRes.json() as { token: string; M2: string }

  // Step 5: Verify M2 = SHA256(pad256(A) || M1_bytes || pad256(S))
  const expectedM2Bytes = await sha256(
    hexToBytes(bigIntToHex256(A)),
    m1Bytes,
    hexToBytes(bigIntToHex256(S)),
  )
  const expectedM2 = bytesToHex(expectedM2Bytes)
  if (expectedM2 !== m2Hex) throw new Error('srp_m2_mismatch')

  return token
}

/**
 * deriveSessionToken is now a thin wrapper around srpAuthenticate.
 * It performs the full SRP-6a handshake and returns the server-issued Bearer token.
 * The parameter name "input" is kept for API compatibility — it is the password.
 *
 * Note: In device mode (hardware key), the credential ID is passed as input.
 * For SRP, we use the credential ID as the "password" — this maintains the same
 * binding: the server verifier was computed from the credential ID string.
 *
 * @param input    The user's password (password mode) or hardware credential ID.
 * @param apiBase  API base URL; defaults to "/api" for production use.
 * @returns        A Bearer token string.
 */
export async function deriveSessionToken(input: string, apiBase = '/api'): Promise<string> {
  return srpAuthenticate(input, apiBase)
}

/**
 * Computes the SHA-256 hex digest of a token string.
 * Retained for API compatibility; no longer used for server auth (SRP replaced it).
 * May be used for other purposes (e.g. local token hashing) if needed.
 */
export async function hashToken(token: string): Promise<string> {
  const data = new TextEncoder().encode(token)
  const buf = await window.crypto.subtle.digest('SHA-256', data)
  return Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
}

/** Stores the session token in sessionStorage for the current tab. */
export function storeSessionToken(token: string): void {
  sessionStorage.setItem('yinmo_session_token', token)
}

/** Returns the current session token, or null if not set / locked. */
export function getSessionToken(): string | null {
  return sessionStorage.getItem('yinmo_session_token')
}

/** Returns the stored hardware credential ID (base64 rawId), or null if not registered. */
export function getHardwareCredentialId(): string | null {
  return localStorage.getItem(HW_CRED_KEY)
}
