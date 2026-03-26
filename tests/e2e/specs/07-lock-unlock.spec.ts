/**
 * 07 – Library lock / unlock (password mode)
 *
 * Tests the full password-based lock/unlock lifecycle:
 *   • Library can be initialized with a password (first-time setup)
 *   • Lock button is visible after init (not shown in keyless mode)
 *   • Clicking lock shows the "Library Locked" state
 *   • Clicking the Unlock button in the locked-state area shows the unlock modal
 *   • Wrong password shows an error message
 *   • Correct password unlocks successfully
 *   • Notes are still accessible after unlock
 *
 * These tests share a single page across the serial group to preserve the
 * password-initialized state between steps.
 */
import { test, expect, type Browser, type Page } from '@playwright/test'
import { freshPage, switchToEnglish, clearServerAuth } from '../helpers/app'

const PASSWORD = 'E2eTestPass#2026'

// Use serial mode so the lock/unlock sequence runs in order on the same session
test.describe.serial('Library lock / unlock', () => {
  let browser: Browser
  let page: Page

  test.beforeAll(async ({ browser: b }, testInfo) => {
    // PBKDF2 in Alpine Docker's headless Chromium can be slow under CPU pressure.
    // E2E builds set VITE_PBKDF2_ITERATIONS=1000 so this is normally fast, but
    // keep a generous deadline as a safety net.
    testInfo.setTimeout(60_000)
    browser = b
    page = await browser.newPage()

    // Capture JS errors and console.error for diagnostics
    const pageErrors: string[] = []
    page.on('pageerror', err => pageErrors.push(err.message))
    page.on('console', msg => { if (msg.type() === 'error') pageErrors.push(msg.text()) })

    // ── Step 1: Initialize with password ──────────────────────────────────
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-password"]').click()
    await page.locator('[data-testid="unlock-password-input"]').fill(PASSWORD)
    await page.locator('[data-testid="unlock-btn"]').click()

    // Wait for either success signal or error indicator.
    // sidebar-lock-btn is only rendered when !isLibraryLocked && !isKeylessModeActive.
    try {
      await page.locator('[data-testid="sidebar-lock-btn"]').waitFor({ state: 'visible', timeout: 45_000 })
    } catch (e) {
      const unlockErrorVisible = await page.locator('[data-testid="unlock-error"]').isVisible()
      const modalStillOpen = await page.locator('[data-testid="unlock-btn"]').isVisible()
      const newNoteVisible = await page.locator('[data-testid="new-note-btn"]').isVisible()
      throw new Error(
        `Password init beforeAll timed out.\n` +
        `  unlockError shown: ${unlockErrorVisible}\n` +
        `  modal still open: ${modalStillOpen}\n` +
        `  new-note-btn visible: ${newNoteVisible}\n` +
        `  JS errors: ${JSON.stringify(pageErrors)}`
      )
    }
  })

  test.afterAll(async () => {
    // Reset server auth so subsequent keyless tests can reach the API without a token.
    await clearServerAuth(page)
    await page.close()
  })

  // ── Lock button visibility ────────────────────────────────────────────────

  test('lock button is visible in the sidebar after password init', async () => {
    // In password mode (not keyless), the lock button is shown
    await expect(page.locator('[data-testid="sidebar-lock-btn"]')).toBeVisible()
  })

  // ── Lock ──────────────────────────────────────────────────────────────────

  test('clicking the lock button locks the library', async () => {
    await page.locator('[data-testid="sidebar-lock-btn"]').click()
    // The lock button itself disappears once the library is locked
    await expect(page.locator('[data-testid="sidebar-lock-btn"]')).not.toBeVisible({ timeout: 5_000 })
  })

  test('locked state shows "Library Locked" message in main area', async () => {
    await expect(page.locator('text=/Library Locked|笔记库已锁定/i').first()).toBeVisible({ timeout: 5_000 })
  })

  test('locked state shows an "Unlock" button to open the modal', async () => {
    await expect(page.getByRole('button', { name: /Unlock|解锁/i }).first()).toBeVisible()
  })

  // ── Unlock modal in locked state ──────────────────────────────────────────

  test('locking library immediately shows unlock modal', async () => {
    // handleLockLibrary() sets showUnlockModal=true at the same time as
    // isLibraryLocked=true, so the unlock modal is already visible after the
    // lock button click in the previous test — no additional click is needed.
    await expect(page.locator('[data-testid="unlock-btn"]')).toBeVisible({ timeout: 5_000 })
  })

  // ── Wrong password ────────────────────────────────────────────────────────

  test('entering wrong password shows an error message', async () => {
    const input = page.locator('[data-testid="unlock-password-input"]')
    await input.fill('wrongpassword!!!')
    await page.locator('[data-testid="unlock-btn"]').click()
    await expect(page.locator('[data-testid="unlock-error"]')).toBeVisible({ timeout: 8_000 })
  })

  // ── Correct password ──────────────────────────────────────────────────────

  test('entering correct password clears the error and unlocks', async () => {
    const input = page.locator('[data-testid="unlock-password-input"]')
    await input.fill(PASSWORD)
    await page.locator('[data-testid="unlock-btn"]').click()
    // Error must disappear
    await expect(page.locator('[data-testid="unlock-error"]')).not.toBeVisible({ timeout: 5_000 })
    // Sidebar comes back
    await expect(page.locator('[data-testid="new-note-btn"]')).toBeVisible({ timeout: 20_000 })
  })

  test('after unlock the full sidebar is accessible', async () => {
    await expect(page.locator('[data-testid="search-input"]')).toBeVisible()
    await expect(page.locator('[data-testid="sidebar-lock-btn"]')).toBeVisible()
  })
})
