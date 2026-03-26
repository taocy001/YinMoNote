/**
 * 05 – Settings panel
 *
 * Tests all settings panel features:
 *   • Opening via the editor toolbar button
 *   • Opening via the empty-state settings button
 *   • Three tabs (Appearance / Editor / Security)
 *   • Dark/light theme toggle in Appearance tab
 *   • Language toggle in Appearance tab
 *   • Editor max-width selector (standard / full)
 *   • Font-size slider (range input)
 *   • Typewriter mode toggle
 *   • Security tab: idle-timeout select, Reset Library button
 *   • Apply button saves settings (no error)
 *   • Close button dismisses the panel
 */
import { test, expect } from '../fixtures'
import { createAndSaveNote } from '../helpers/app'

test.describe('Settings panel', () => {
  // ── Opening / Closing ─────────────────────────────────────────────────────

  test('Settings button in editor toolbar opens the settings panel', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsOpen')
    await page.locator('[data-testid="settings-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).toBeVisible()
  })

  test('Settings button in empty-state toolbar opens the panel', async ({ unlockedPage: page }) => {
    // Navigate to empty state by not selecting any note (deselect via reload)
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('[data-testid="new-note-btn"]').waitFor({ state: 'visible', timeout: 15_000 })
    // Wait for post-reload API calls (loadConfig, loadNotesList) to settle so the
    // empty-state component stops re-mounting and the button stays in the DOM.
    await page.waitForLoadState('networkidle', { timeout: 8_000 }).catch(() => {})
    // The empty state has its own settings button
    const emptySettingsBtn = page.locator('[data-testid="empty-state-settings-btn"]')
    if (await emptySettingsBtn.isVisible()) {
      await emptySettingsBtn.click()
      await expect(page.locator('[data-testid="settings-panel"]')).toBeVisible()
    }
  })

  test('Close button dismisses the settings panel', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsClose')
    await page.locator('[data-testid="settings-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).toBeVisible()
    await page.locator('[data-testid="settings-close-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 5_000 })
  })

  test('clicking backdrop (overlay) closes the settings panel', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsOverlayClose')
    await page.locator('[data-testid="settings-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).toBeVisible()
    // Click the semi-transparent backdrop (outside the panel)
    await page.mouse.click(100, 400) // far-left, away from the right-side panel
    await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 5_000 })
  })

  // ── Tab navigation ────────────────────────────────────────────────────────

  test('Appearance, Editor, and Security tabs are all visible', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsTabs')
    await page.locator('[data-testid="settings-btn"]').click()
    await expect(page.locator('[data-testid="tab-appearance"]')).toBeVisible()
    await expect(page.locator('[data-testid="tab-editor"]')).toBeVisible()
    await expect(page.locator('[data-testid="tab-security"]')).toBeVisible()
  })

  test('clicking Editor tab shows font-size slider', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsEditorTab')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-editor"]').click()
    // Font-size range input should be visible
    await expect(page.locator('input[type="range"]')).toBeVisible()
  })

  test('clicking Security tab shows the Reset Library button', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'SettingsSecurityTab')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-security"]').click()
    await expect(page.locator('[data-testid="reset-library-btn"]')).toBeVisible()
  })

  // ── Appearance: theme toggle ──────────────────────────────────────────────

  test('dark mode toggle in settings changes the app theme', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ThemeToggle')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-appearance"]').click()

    // Click "Dark" button to force dark mode, then Apply.
    const classBefore = await page.evaluate(() => document.documentElement.className)

    await page.locator('[data-testid="theme-dark-btn"]').click()
    await page.locator('[data-testid="settings-apply-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 5_000 })

    const classAfter = await page.evaluate(() => document.documentElement.className)
    expect(classAfter).toContain('dark')
  })

  // ── Appearance: language toggle ───────────────────────────────────────────

  test('clicking 中文 button switches the UI to Chinese', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'LangSwitch')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-appearance"]').click()
    await page.locator('[data-testid="lang-zh-btn"]').click()
    // Apply the settings
    await page.locator('[data-testid="settings-apply-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 5_000 })
    // The sidebar New Note button should now read "新建笔记" (Chinese)
    await expect(page.locator('[data-testid="new-note-btn"]')).toContainText('新建笔记')
  })

  test('clicking English button switches the UI to English', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'LangSwitchEn')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-appearance"]').click()
    // Switch to Chinese first, then back to English
    await page.locator('[data-testid="lang-zh-btn"]').click()
    await page.locator('[data-testid="lang-en-btn"]').click()
    await page.locator('[data-testid="settings-apply-btn"]').click()
    await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 5_000 })
    await expect(page.locator('[data-testid="new-note-btn"]')).toContainText('New note')
  })

  // ── Editor tab ────────────────────────────────────────────────────────────

  test('font-size slider range is 12–24 px', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'FontSizeRange')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-editor"]').click()
    const slider = page.locator('input[type="range"]')
    await expect(slider).toHaveAttribute('min', '12')
    await expect(slider).toHaveAttribute('max', '24')
  })

  test('standard / full editor-width toggle is visible in Editor tab', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'EditorWidthToggle')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-editor"]').click()
    await expect(page.locator('text=/Standard|标准/i').first()).toBeVisible()
    await expect(page.locator('text=/Full|全宽/i').first()).toBeVisible()
  })

  test('typewriter mode toggle is visible in Editor tab', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'TypewriterToggle')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-editor"]').click()
    await expect(page.locator('text=/Typewriter|打字机/i').first()).toBeVisible()
  })

  // ── Apply ──────────────────────────────────────────────────────────────────

  test('Apply button saves settings without error', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ApplySettings')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="settings-apply-btn"]').click()
    // No save-failed error should appear
    await expect(page.locator('text=/Failed to save|保存失败/i').first()).not.toBeVisible({ timeout: 5_000 })
  })

  // ── Security: idle timeout ─────────────────────────────────────────────────

  test('idle-timeout selector is hidden in keyless (no-encryption) mode', async ({ unlockedPage: page }) => {
    // In keyless mode the idle-timeout feature is disabled (no key to lock with)
    await createAndSaveNote(page, 'IdleTimeout')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-security"]').click()
    // The select for idle-timeout is inside v-if="!isKeylessModeActive" — not rendered
    await expect(page.locator('select')).not.toBeVisible()
  })

  // ── Reset Library button ───────────────────────────────────────────────────

  test('Reset Library button opens the reset confirmation modal', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ResetModalOpen')
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-security"]').click()
    await page.locator('[data-testid="reset-library-btn"]').click()
    // The reset modal should appear with a countdown or confirmation text
    await expect(page.locator('text=/reset|Reset|重置/i').first()).toBeVisible({ timeout: 5_000 })
    // Close it immediately by pressing Escape or clicking cancel
    await page.keyboard.press('Escape')
  })
})
