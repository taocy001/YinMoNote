/**
 * 09 – Encryption mode round-trips
 *
 * Tests that notes survive different encryption lifecycle scenarios:
 *
 *   • Password mode basic: create, save, and reload a note
 *   • Password mode + serverEncrypt ON: note stored as ENC1, readable after reload
 *   • Full encryption round-trip: lock → re-unlock with same password → note decrypted correctly
 *   • Mode transition: keyless session can read plaintext notes from a password-mode session
 *     (both sessions use serverEncrypt=OFF, so data is always plaintext on server)
 *
 * These tests verify end-to-end cryptographic correctness: that the encrypt-on-save
 * and decrypt-on-load paths work correctly as a pair.
 *
 * NOTE on lock/re-unlock modal: once the library is initialized with a password, the
 * lock modal shows "Password Unlock" directly (no mode-tab selection UI).  The user
 * only fills the password input and clicks UNLOCK — no tab click is needed.
 */
import { test, expect } from '../fixtures'
import {
  clickNewNote,
  typeInEditor,
  manualSave,
  unlockKeyless,
  initWithPassword,
  unlockAsNewDevice,
  browserRestartUnlock,
  setServerEncrypt,
  createAndSaveNote,
  clearServerAuth,
} from '../helpers/app'

// ─── Password mode basic ──────────────────────────────────────────────────────

test.describe('Password mode basic functionality', () => {
  test('can create and read a note in password mode', async ({ passwordPage: page }) => {
    await clickNewNote(page)
    const title = `PWMode-${Date.now()}`
    await typeInEditor(page, `# ${title}\n\nBody text for password mode test.`)
    await manualSave(page)
    // Verify the note is visible in the editor immediately after saving
    await expect(page.locator('.ProseMirror')).toContainText(title, { timeout: 8_000 })
  })

  test('note persists across browser-restart simulation (session lost, localStorage intact)', async ({ passwordPage: page }) => {
    const title = await createAndSaveNote(page, 'PWReload')
    // Simulate a real browser restart: sessionStorage is cleared (session key lost)
    // but localStorage persists (PBKDF2 salt, LIBRARY_KEY_STORE).
    // The same password re-derives the same key → encrypted notes remain readable.
    // (unlockAsNewDevice is wrong here: it wipes ALL storage via freshPage, which
    // forces a new random salt → different key → decryption failure.)
    await browserRestartUnlock(page, 'E2EPass123')
    await expect(page.locator('[data-note-key]').filter({ hasText: title })).toBeVisible({ timeout: 15_000 })
  })
})

// ─── Encryption round-trip ────────────────────────────────────────────────────

test.describe('serverEncrypt ON: full encryption round-trip', () => {
  test('ENC1-encrypted note is readable after enabling serverEncrypt', async ({ passwordPage: page }) => {
    await clickNewNote(page)
    const content = `RoundTrip-${Date.now()}`
    await typeInEditor(page, `# ${content}`)
    await manualSave(page)

    // Enable server-side encryption — all notes become ENC1 on server
    await setServerEncrypt(page, true)

    // Verify the editor still shows the correct content (batch completed + loadNote called)
    await expect(page.locator('.ProseMirror')).toContainText(content, { timeout: 10_000 })

    // Cleanup
    await setServerEncrypt(page, false)
  })

  test('lock and re-unlock with same password restores ENC1-encrypted note', async ({ passwordPage: page }, testInfo) => {
    testInfo.setTimeout(60_000)
    await clickNewNote(page)
    const content = `LockUnlock-${Date.now()}`
    // Register the response waiter BEFORE typing so it captures the title-debounce
    // PUT /api/structure that fires 500 ms after the last keystroke.  clickNewNote
    // has already completed its own structure PUT, so this waiter only fires for
    // the title-debounce save triggered by typeInEditor below.
    const titleSaved = page.waitForResponse(
      r => r.url().includes('/api/structure') && r.request().method() === 'PUT',
      { timeout: 5_000 }
    )
    await typeInEditor(page, `# ${content}`)
    await manualSave(page)
    // Wait for the title-debounce saveStructure() PUT to complete so the
    // server-side structure has the correct title before locking.
    await titleSaved.catch(() => {})
    // Ensure all pending network activity has settled (e.g. late-firing debounce saves).
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})

    // Enable server-side encryption
    await setServerEncrypt(page, true)

    // Lock the library — this immediately shows the unlock modal
    await page.locator('[data-testid="sidebar-lock-btn"]').click()
    await page.locator('[data-testid="unlock-btn"]').waitFor({ state: 'visible', timeout: 8_000 })

    // Re-unlock: when the library is already initialized with a password, the modal
    // shows "Password Unlock" directly (no mode-tab selector).  Just fill + submit.
    await page.locator('[data-testid="unlock-password-input"]').fill('E2EPass123')
    await page.locator('[data-testid="unlock-btn"]').click()
    await page.locator('[data-testid="sidebar-lock-btn"]').waitFor({ state: 'visible', timeout: 20_000 })
    // Wait for loadNotesList to finish before checking the sidebar.
    await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})

    // Click the note in the sidebar to open it
    const noteItem = page.locator('[data-note-key]').filter({ hasText: content })
    await noteItem.waitFor({ state: 'visible', timeout: 15_000 })
    await noteItem.click()

    // The editor should decrypt the ENC1 content and show the original text
    await expect(page.locator('.ProseMirror')).toContainText(content, { timeout: 10_000 })

    // Cleanup: restore plaintext state
    await setServerEncrypt(page, false)
  })
})

// ─── Mode transition ──────────────────────────────────────────────────────────

test.describe('Encryption mode transitions', () => {
  test('keyless mode can read plaintext notes saved by a password-mode session', async ({ page }) => {
    // This test performs multiple full auth-mode transitions; give it extra time.
    test.setTimeout(90_000)
    // First: create a note in password mode (serverEncrypt=OFF → plaintext on server)
    await initWithPassword(page, 'E2EPass123')
    // Ensure serverEncrypt is OFF (a previous test may have left it ON if it failed)
    await setServerEncrypt(page, false)
    const title = await createAndSaveNote(page, 'ModeTransition')

    // Clear server auth before switching to keyless mode so the server accepts
    // API calls without a Bearer token (the token is still in sessionStorage here).
    await clearServerAuth(page)

    // Second: start a fresh keyless session on the same server
    await unlockKeyless(page)
    // The keyless session loads the structure and notes (plaintext from password session)
    await expect(page.locator('[data-note-key]').filter({ hasText: title })).toBeVisible({ timeout: 8_000 })
    // Open the note and verify it's readable
    await page.locator('[data-note-key]').filter({ hasText: title }).click()
    await expect(page.locator('.ProseMirror')).toContainText(title, { timeout: 8_000 })
  })
})
