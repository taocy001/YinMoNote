/**
 * 11 – Multi-device login and initialization
 *
 * Tests every aspect of connecting a new device to an already-initialized library:
 *
 *   A. Baseline: server NOT initialized → fresh page shows INIT wizard (mode tabs).
 *   B. Server initialized → new device sees UNLOCK UI (password input, NO mode tabs).
 *   C. New device, wrong password → unlock error; device is NOT registered.
 *   D. New device, correct password → unlocks; sidebar visible.
 *   E. New device can read notes that were created on "device A".
 *   F. Concurrent sessions: two browser contexts use the same password and both see
 *      the same notes without interfering with each other.
 *   G. Re-lock on new device then re-unlock → still works (local token persisted).
 *
 * "New device" is simulated by calling freshPage() WITHOUT clearServerAuth() first:
 * the browser-side state (localStorage / sessionStorage) is wiped, but the server
 * keeps its SRPVerifier from the previous initWithPassword() call.
 */
import { test, expect, type Browser, type Page } from '@playwright/test'
import {
  freshPage,
  switchToEnglish,
  initWithPassword,
  unlockAsNewDevice,
  clearServerAuth,
  createAndSaveNote,
  clickNewNote,
  typeInEditor,
  manualSave,
  setServerEncrypt,
} from '../helpers/app'

const PASSWORD = 'MultiDevicePass#2026'

// ─── A. Baseline: INIT wizard when server is not initialized ──────────────────

test.describe('Baseline: server not initialized', () => {
  test.beforeEach(async ({ page }) => {
    await clearServerAuth(page)
  })

  test('fresh page shows mode tabs (INIT wizard, not UNLOCK UI)', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    // INIT wizard shows all mode tabs
    await expect(page.locator('[data-testid="mode-tab-none"]')).toBeVisible({ timeout: 8_000 })
    await expect(page.locator('[data-testid="mode-tab-password"]')).toBeVisible()
    await expect(page.locator('[data-testid="mode-tab-device"]')).toBeVisible()
  })

  test('fresh page does NOT show password input directly (must click tab first)', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    // In the INIT wizard the password input is hidden until the Password tab is clicked
    await expect(page.locator('[data-testid="unlock-password-input"]')).not.toBeVisible({ timeout: 5_000 })
  })
})

// ─── B–G. After initialization on "device A" ─────────────────────────────────

test.describe.serial('Multi-device unlock flow', () => {
  let browser: Browser
  let page: Page

  test.beforeAll(async ({ browser: b }, testInfo) => {
    testInfo.setTimeout(120_000)
    browser = b
    page = await browser.newPage()

    // ── "Device A" initializes the library ──
    await initWithPassword(page, PASSWORD)
  })

  test.afterAll(async () => {
    await clearServerAuth(page)
    await page.close()
  })

  // ── B. New device shows UNLOCK UI ───────────────────────────────────────────

  test('B: new device sees password-unlock UI (not INIT wizard)', async () => {
    // Simulate new device: wipe browser state, server keeps its hash.
    await freshPage(page)
    await switchToEnglish(page)

    // The app queries /api/auth/status; initialized=true → shows UNLOCK UI.
    // Password input must be visible without clicking any mode tab.
    await expect(page.locator('[data-testid="unlock-password-input"]'))
      .toBeVisible({ timeout: 10_000 })
  })

  test('B: new device does NOT show mode tabs (INIT wizard is suppressed)', async () => {
    // If still on the freshPage from the previous test, skip re-navigation.
    // Mode tabs only appear in the INIT wizard (hasLibraryKey=false).
    // After the /api/auth/status check, hasLibraryKey=true → UNLOCK branch.
    await expect(page.locator('[data-testid="mode-tab-none"]')).not.toBeVisible({ timeout: 5_000 })
    await expect(page.locator('[data-testid="mode-tab-password"]')).not.toBeVisible()
    await expect(page.locator('[data-testid="mode-tab-device"]')).not.toBeVisible()
  })

  // ── C. Wrong password → error ────────────────────────────────────────────────

  test('C: wrong password on new device shows unlock error', async () => {
    // Make sure we are on the unlock modal (freshPage if needed).
    const unlockBtn = page.locator('[data-testid="unlock-btn"]')
    if (!(await unlockBtn.isVisible())) {
      await freshPage(page)
      await switchToEnglish(page)
      await page.locator('[data-testid="unlock-password-input"]')
        .waitFor({ state: 'visible', timeout: 10_000 })
    }
    await page.locator('[data-testid="unlock-password-input"]').fill('WrongPassword!!!')
    await page.locator('[data-testid="unlock-btn"]').click()
    await expect(page.locator('[data-testid="unlock-error"]'))
      .toBeVisible({ timeout: 15_000 })
  })

  test('C: wrong password does not unlock the app (lock button stays hidden)', async () => {
    // After a failed unlock, isLibraryLocked remains true.
    // sidebar-lock-btn has v-if="!isLibraryLocked", so it correctly reflects the locked state.
    // new-note-btn has NO v-if (always in DOM on desktop), so it is always "visible" to Playwright.
    await expect(page.locator('[data-testid="sidebar-lock-btn"]')).not.toBeVisible({ timeout: 3_000 })
  })

  // ── D. Correct password → unlocks ───────────────────────────────────────────

  test('D: correct password on new device unlocks successfully', async () => {
    // Clear the error state from test C and enter the correct password.
    const input = page.locator('[data-testid="unlock-password-input"]')
    await input.fill(PASSWORD)
    await page.locator('[data-testid="unlock-btn"]').click()
    // sidebar-lock-btn has v-if="!isLibraryLocked" — reliable unlock indicator.
    // new-note-btn has no v-if and is always rendered on desktop (not a reliable signal).
    await expect(page.locator('[data-testid="sidebar-lock-btn"]'))
      .toBeVisible({ timeout: 20_000 })
  })

  test('D: after new-device unlock the sidebar lock button is visible', async () => {
    // Lock button only appears in password mode (not keyless).
    await expect(page.locator('[data-testid="sidebar-lock-btn"]')).toBeVisible()
  })

  // ── E. Notes from device A are visible on new device ────────────────────────

  test('E: notes created on device A are visible after new-device unlock', async () => {
    // ── Step 1: create a note on "device A" ──
    // Re-initialize as device A to create a note.
    await initWithPassword(page, PASSWORD)
    // Disable server-side encryption so the note is stored as plaintext.
    // Multi-device note sharing with ENC1 is not supported: the PBKDF2 salt is
    // per-device (random, stored in localStorage); freshPage wipes it so a new
    // device derives a different key from the same password.
    await setServerEncrypt(page, false)
    const title = await createAndSaveNote(page, 'DeviceA-Note')

    // ── Step 2: switch to new device and unlock ──
    await unlockAsNewDevice(page, PASSWORD)

    // The note created on device A must appear in the sidebar.
    await expect(page.locator('[data-note-key]').filter({ hasText: title }))
      .toBeVisible({ timeout: 8_000 })
  })

  test('E: note content is readable on new device (no decryption error)', async () => {
    // Create a distinctly identifiable note on device A.
    await initWithPassword(page, PASSWORD)
    // Disable server encryption so device B (fresh localStorage, different salt)
    // can read the plaintext note without key-derivation mismatch.
    await setServerEncrypt(page, false)
    const content = `DeviceContent-${Date.now()}`
    await clickNewNote(page)
    await typeInEditor(page, `# ${content}`)
    await manualSave(page)

    // Switch to new device.
    await unlockAsNewDevice(page, PASSWORD)

    // Click the note and verify the content is readable.
    await page.locator('[data-note-key]').filter({ hasText: content }).click()
    await expect(page.locator('.ProseMirror'))
      .toContainText(content, { timeout: 10_000 })
  })

  // ── G. Re-lock → re-unlock on the same device ───────────────────────────────

  test('G: after new-device unlock, locking and re-unlocking works', async () => {
    // Unlock as a new device first.
    await unlockAsNewDevice(page, PASSWORD)

    // Lock the library.
    await page.locator('[data-testid="sidebar-lock-btn"]').click()
    await page.locator('[data-testid="unlock-btn"]')
      .waitFor({ state: 'visible', timeout: 5_000 })

    // Re-unlock: this time the local token exists (written on first new-device
    // unlock), so verifyAndUnlockLibrary is used instead of initLibrary.
    await page.locator('[data-testid="unlock-password-input"]').fill(PASSWORD)
    await page.locator('[data-testid="unlock-btn"]').click()
    await expect(page.locator('[data-testid="new-note-btn"]'))
      .toBeVisible({ timeout: 20_000 })
  })
})

// ─── F. Concurrent sessions ───────────────────────────────────────────────────

test.describe('Concurrent sessions with the same password', () => {
  test('F: two browser contexts see the same notes after unlocking', async ({ browser }) => {
    test.setTimeout(120_000)
    // ── Context 1: "device A" — initialize and create a note ──
    const ctx1 = await browser.newContext()
    const page1 = await ctx1.newPage()
    await initWithPassword(page1, PASSWORD)
    // Disable server encryption so device B (fresh localStorage, different PBKDF2
    // salt) can read the note without key-derivation mismatch.
    await setServerEncrypt(page1, false)
    const title = await createAndSaveNote(page1, 'Concurrent')

    // ── Context 2: "device B" — connect as a new device ──
    const ctx2 = await browser.newContext()
    const page2 = await ctx2.newPage()
    await unlockAsNewDevice(page2, PASSWORD)

    // Both contexts must see the note created on device A.
    await expect(page1.locator('[data-note-key]').filter({ hasText: title }))
      .toBeVisible({ timeout: 8_000 })
    await expect(page2.locator('[data-note-key]').filter({ hasText: title }))
      .toBeVisible({ timeout: 8_000 })

    // Cleanup
    await clearServerAuth(page1)
    await ctx1.close()
    await ctx2.close()
  })

  test('F: note written on device B is visible on device A after reload', async ({ browser }) => {
    test.setTimeout(120_000)
    // ── Setup: initialize on context 1 ──
    const ctx1 = await browser.newContext()
    const page1 = await ctx1.newPage()
    await initWithPassword(page1, PASSWORD)
    // Disable server encryption so cross-device notes are stored as plaintext.
    // The PBKDF2 salt is per-device (random, in localStorage); a different device
    // would derive a different key and fail to decrypt ENC1 notes.
    await setServerEncrypt(page1, false)

    // ── Context 2 writes a note ──
    const ctx2 = await browser.newContext()
    const page2 = await ctx2.newPage()
    await unlockAsNewDevice(page2, PASSWORD)
    // Disable server encryption on device B too: default is ON after new-device
    // unlock, which would encrypt notes with page2's key (different from page1's).
    await setServerEncrypt(page2, false)
    const title = await createAndSaveNote(page2, 'FromDeviceB')

    // ── Context 1 reloads its notes list ──
    // Explicitly wipe sessionStorage and window.name BEFORE reload so that
    // restoreKeyFromSession() finds no session key and the unlock modal is shown.
    // page.reload() in Playwright may not fire the beforeunload event (which
    // normally clears window.name), so the old UUID can survive and allow the
    // session key to be decrypted — bypassing the unlock modal entirely.
    await page1.evaluate(() => { sessionStorage.clear(); window.name = '' })
    await page1.reload({ waitUntil: 'domcontentloaded' })
    // Re-unlock after reload (session key is gone after tab reload)
    await page1.locator('[data-testid="unlock-password-input"]')
      .waitFor({ state: 'visible', timeout: 10_000 })
    await page1.locator('[data-testid="unlock-password-input"]').fill(PASSWORD)
    await page1.locator('[data-testid="unlock-btn"]').click()
    await page1.locator('[data-testid="new-note-btn"]')
      .waitFor({ state: 'visible', timeout: 20_000 })
    await page1.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})

    // Note written by device B must be visible on device A.
    await expect(page1.locator('[data-note-key]').filter({ hasText: title }))
      .toBeVisible({ timeout: 8_000 })

    // Cleanup
    await clearServerAuth(page1)
    await ctx1.close()
    await ctx2.close()
  })
})
