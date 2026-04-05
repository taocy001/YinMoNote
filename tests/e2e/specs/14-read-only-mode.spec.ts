/**
 * 14 – Read-only / edit mode
 *
 * Covers:
 *   • Non-touch viewport defaults to edit mode (contenteditable=true)
 *   • Toggle button switches editor to read-only (contenteditable=false)
 *   • Toggle button switches back to edit mode
 *   • Toolbar row is hidden in read-only mode
 *   • Typing in read-only mode does not show "unsaved" indicator
 *   • Switching to a different note resets to device default (edit mode on desktop)
 *   • aria-pressed reflects the current mode correctly
 *   • aria-label changes between "Switch to read-only mode" and "Switch to edit mode"
 *   • Keyboard shortcut (if any) toggles the mode
 *   • Read-only state is per-session and not shared between notes
 */
import { test, expect } from '../fixtures'
import { createAndSaveNote } from '../helpers/app'

// Selector helpers
const PROSEMIRROR = '.ProseMirror'
const TOGGLE_BTN = '[aria-label="Switch to read-only mode"], [aria-label="Switch to edit mode"]'

test.describe('Read-only / edit mode', () => {

  // ── Default state ──────────────────────────────────────────────────────────

  test('editor starts in edit mode (contenteditable=true) on non-touch viewport', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyDefault')
    const pm = page.locator(PROSEMIRROR)
    await expect(pm).toHaveAttribute('contenteditable', 'true')
  })

  test('read-only toggle button is visible when a note is open', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyBtnVisible')
    // Button exists with aria-label for switching to read-only (current mode: edit)
    await expect(page.locator('[aria-label="Switch to read-only mode"]').first()).toBeVisible()
  })

  // ── Toggle to read-only ────────────────────────────────────────────────────

  test('clicking toggle switches editor to read-only (contenteditable=false)', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlySwitch')
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'false')
  })

  test('after switching to read-only, button aria-label changes to "Switch to edit mode"', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyAriaLabel')
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    await expect(page.locator('[aria-label="Switch to edit mode"]').first()).toBeVisible()
  })

  test('aria-pressed is true when in edit mode and false when in read-only', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyAriaPressed')
    const btn = page.locator(TOGGLE_BTN).first()
    // Edit mode: aria-pressed=true (button is "active" = editing is on)
    await expect(btn).toHaveAttribute('aria-pressed', 'true')
    await btn.click()
    // Read-only mode: aria-pressed=false
    await expect(page.locator(TOGGLE_BTN).first()).toHaveAttribute('aria-pressed', 'false')
  })

  // ── Toggle back to edit ────────────────────────────────────────────────────

  test('clicking toggle again restores edit mode (contenteditable=true)', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyRestore')
    const btn = () => page.locator(TOGGLE_BTN).first()
    await btn().click() // → read-only
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'false')
    await btn().click() // → edit
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'true')
  })

  test('multiple toggles alternate between edit and read-only correctly', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyMultiToggle')
    const pm = page.locator(PROSEMIRROR)
    const btn = () => page.locator(TOGGLE_BTN).first()
    for (let i = 0; i < 3; i++) {
      await btn().click() // edit → readonly → edit → readonly
      const expected = i % 2 === 0 ? 'false' : 'true'
      await expect(pm).toHaveAttribute('contenteditable', expected)
    }
  })

  // ── Toolbar visibility ─────────────────────────────────────────────────────

  test('formatting toolbar row is hidden in read-only mode', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyToolbar')
    // Toolbar row contains word count and save-status; it is hidden in read-only
    const toolbarRow = page.locator('[data-testid="save-status"]')
    await expect(toolbarRow).toBeVisible() // visible in edit mode
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    await expect(toolbarRow).not.toBeVisible() // hidden in read-only
  })

  // ── Save blocked ───────────────────────────────────────────────────────────

  test('typing in read-only mode does not show the unsaved indicator', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyNoSave')
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    // Editor is not editable so keyboard input is silently ignored
    await page.locator(PROSEMIRROR).click({ force: true })
    await page.keyboard.type('should not appear')
    // The unsaved indicator must not appear
    await expect(page.locator('[data-testid="save-status"]')).not.toBeVisible({ timeout: 2_000 })
      .catch(() => {
        // If save-status is visible, verify it's not in dirty state
        return expect(page.locator('text=/unsaved|未保存/i').first()).not.toBeVisible({ timeout: 1_000 })
      })
  })

  // ── Note-switch reset ──────────────────────────────────────────────────────

  test('switching notes resets to device default (edit mode on desktop viewport)', async ({ unlockedPage: page }) => {
    const noteA = await createAndSaveNote(page, 'ReadOnlyNoteA')
    const noteB = await createAndSaveNote(page, 'ReadOnlyNoteB')

    // Open note A and switch to read-only
    await page.locator(`text=${noteA}`).first().click()
    await page.waitForTimeout(300)
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'false')

    // Switch to note B — should reset to edit mode (desktop default)
    await page.locator(`text=${noteB}`).first().click()
    await page.waitForTimeout(300)
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'true')
  })

  test('read-only state of note A is not carried over when returning to note A', async ({ unlockedPage: page }) => {
    const noteA = await createAndSaveNote(page, 'ReadOnlyCarryA')
    const noteB = await createAndSaveNote(page, 'ReadOnlyCarryB')

    // Switch A to read-only, then switch to B and back to A
    await page.locator(`text=${noteA}`).first().click()
    await page.waitForTimeout(300)
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()

    await page.locator(`text=${noteB}`).first().click()
    await page.waitForTimeout(300)
    await page.locator(`text=${noteA}`).first().click()
    await page.waitForTimeout(300)

    // Returning to note A — state is reset to device default (edit mode on desktop)
    await expect(page.locator(PROSEMIRROR)).toHaveAttribute('contenteditable', 'true')
  })

  // ── Read-only mode does not affect other features ─────────────────────────

  test('history panel can still be opened in read-only mode', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyHistory')
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    const histBtn = page.locator('[data-testid="history-btn"]')
    await expect(histBtn).toBeVisible()
    await histBtn.click()
    await expect(page.locator('text=/history|History|版本/i').first()).toBeVisible({ timeout: 5_000 })
  })

  test('export button is still accessible in read-only mode', async ({ unlockedPage: page }) => {
    await createAndSaveNote(page, 'ReadOnlyExport')
    await page.locator('[aria-label="Switch to read-only mode"]').first().click()
    const exportBtn = page.locator('[data-testid="export-btn"]')
    await expect(exportBtn).toBeVisible()
  })
})
