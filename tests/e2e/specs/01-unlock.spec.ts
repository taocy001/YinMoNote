/**
 * 01 – Unlock / Initialization flow
 *
 * Tests every UI element visible before the user authenticates:
 *   • Unlock modal presence
 *   • Mode tabs (device / password / no-encryption)
 *   • Keyless "Skip Encryption" flow → app becomes usable
 *   • Password mode: UI elements present and accepting input
 */
import { test, expect } from '@playwright/test'
import { freshPage, switchToEnglish } from '../helpers/app'

test.describe('Unlock / Init flow', () => {
  // ── Modal presence ────────────────────────────────────────────────────────

  test('unlock modal is shown on first visit (no localStorage key)', async ({ page }) => {
    await freshPage(page)
    // The action button is present inside the unlock modal
    await expect(page.locator('[data-testid="unlock-btn"]')).toBeVisible()
  })

  test('unlock modal covers the full viewport (full-screen overlay)', async ({ page }) => {
    await freshPage(page)
    // Verify the unlock modal is visible — it has z-[300] and covers the entire screen
    await expect(page.locator('[data-testid="unlock-btn"]')).toBeVisible()
    // The fixed-inset-0 wrapper should exist in the DOM
    const backdrop = page.locator('.fixed.inset-0').first()
    await expect(backdrop).toBeAttached()
  })

  // ── Mode tabs (init flow) ─────────────────────────────────────────────────

  test('all three mode tabs are visible: Device, Password, No Encryption', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await expect(page.locator('[data-testid="mode-tab-device"]')).toBeVisible()
    await expect(page.locator('[data-testid="mode-tab-password"]')).toBeVisible()
    await expect(page.locator('[data-testid="mode-tab-none"]')).toBeVisible()
  })

  test('clicking Password tab reveals the password input field', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-password"]').click()
    await expect(page.locator('[data-testid="unlock-password-input"]')).toBeVisible()
  })

  test('clicking No Encryption tab hides the password input', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    // First switch to password to show the input
    await page.locator('[data-testid="mode-tab-password"]').click()
    await expect(page.locator('[data-testid="unlock-password-input"]')).toBeVisible()
    // Switch back to No Encryption → input should be gone
    await page.locator('[data-testid="mode-tab-none"]').click()
    await expect(page.locator('[data-testid="unlock-password-input"]')).not.toBeVisible()
  })

  // ── Keyless unlock ───────────────────────────────────────────────────────

  test('No Encryption → Skip Encryption → sidebar and New Note button visible', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-none"]').click()
    await page.locator('[data-testid="unlock-btn"]').click()
    // Unlock modal must disappear
    await expect(page.locator('[data-testid="unlock-btn"]')).not.toBeVisible({ timeout: 15_000 })
    // Sidebar must be usable
    await expect(page.locator('[data-testid="new-note-btn"]')).toBeVisible()
  })

  test('after keyless unlock the search box is visible in the sidebar', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-none"]').click()
    await page.locator('[data-testid="unlock-btn"]').click()
    await page.locator('[data-testid="new-note-btn"]').waitFor({ state: 'visible', timeout: 15_000 })
    await expect(page.locator('[data-testid="search-input"]')).toBeVisible()
  })

  // ── Password mode UI ─────────────────────────────────────────────────────

  test('password input accepts typed characters', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-password"]').click()
    const input = page.locator('[data-testid="unlock-password-input"]')
    await input.fill('My$ecretP@ss123')
    await expect(input).toHaveValue('My$ecretP@ss123')
  })

  test('password field type is password (characters are masked)', async ({ page }) => {
    await freshPage(page)
    await switchToEnglish(page)
    await page.locator('[data-testid="mode-tab-password"]').click()
    const input = page.locator('[data-testid="unlock-password-input"]')
    await expect(input).toHaveAttribute('type', 'password')
  })
})
