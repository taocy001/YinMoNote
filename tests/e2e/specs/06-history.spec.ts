/**
 * 06 – Version history & rollback
 *
 * Tests the Git-backed version-history panel:
 *   • Opening the history panel on a freshly created note
 *   • History entries appear after each save
 *   • Commit hash (short) is shown for each entry
 *   • Diff view: clicking "Diff" shows the before/after comparison
 *   • Revert: clicking "Revert to" → inline confirm → "Confirm" → content reverted
 *   • Revert cancel: content unchanged after clicking "Cancel"
 */
import { test, expect } from '../fixtures'
import { clickNewNote, typeInEditor, manualSave } from '../helpers/app'

test.describe('Version history', () => {
  // ── Panel presence ────────────────────────────────────────────────────────

  test('History button is visible in editor toolbar', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await expect(page.locator('[data-testid="history-btn"]')).toBeVisible()
  })

  test('opening history panel on a new note shows "no history" message', async ({ unlockedPage: page }) => {
    // clickNewNote() waits for the PUT /api/structure response before returning,
    // which guarantees the new Editor is mounted with the correct noteFileName.
    await clickNewNote(page)
    await page.locator('[data-testid="history-btn"]').click()
    // A brand-new note that has never been saved has no commits yet.
    // The backend returns [] (not null) for files with no history.
    await expect(page.locator('text=/No history|暂无历史/i').first()).toBeVisible({ timeout: 8_000 })
  })

  // ── Entries after save ─────────────────────────────────────────────────────

  test('saving a note creates a history entry', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# HistoryEntry-${Date.now()}`)
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    // History panel should show at least one commit row (contains a short hash)
    await expect(
      page.locator('.font-mono').first()
    ).toBeVisible({ timeout: 8_000 })
  })

  test('history entry shows a short commit hash (7 characters)', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# HashCheck-${Date.now()}`)
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    // The hash is rendered in a monospace span; should be 7 hex chars
    const hashEl = page.locator('.font-mono').first()
    await hashEl.waitFor({ state: 'visible', timeout: 8_000 })
    const hashText = await hashEl.textContent()
    expect(hashText?.trim()).toMatch(/^[0-9a-f]{7}$/)
  })

  test('multiple saves create multiple history entries', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# MultiSave-${Date.now()}`)
    await manualSave(page)
    await page.waitForTimeout(500)

    // Make a second save
    await page.locator('.ProseMirror').click()
    await page.keyboard.type(' (edit 2)')
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    // There should be at least 2 history rows
    const rows = page.locator('.font-mono')
    await expect(rows.first()).toBeVisible({ timeout: 8_000 })
    const count = await rows.count()
    expect(count).toBeGreaterThanOrEqual(2)
  })

  // ── Diff view ─────────────────────────────────────────────────────────────

  test('clicking "Diff" shows the diff view with +/- line counts', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# DiffTest-${Date.now()}`)
    await manualSave(page)
    await page.waitForTimeout(500)

    // Make a change and save again so there are 2 versions to diff
    await page.locator('.ProseMirror').click()
    await page.keyboard.type('\nAdded line for diff')
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    // Hover over the first history entry to show the Diff button
    const historyRow = page.locator('.group').filter({ has: page.locator('.font-mono') }).first()
    await historyRow.hover()
    await historyRow.locator('.history-diff-btn').first().click()

    // Diff view should show + and - counts
    await expect(page.locator('text=/\\+\\d+/').first()).toBeVisible({ timeout: 8_000 })
  })

  // ── Revert ────────────────────────────────────────────────────────────────

  test('clicking "Revert to" shows inline confirm UI', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# RevertConfirm-${Date.now()}`)
    await manualSave(page)
    await page.waitForTimeout(500)

    await page.locator('.ProseMirror').click()
    await page.keyboard.type('\nExtra line before revert')
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    const historyRow = page.locator('.group').filter({ has: page.locator('.font-mono') }).first()
    await historyRow.hover()
    await historyRow.locator('.history-revert-btn').first().click()

    // Inline confirm bar should appear
    await expect(page.locator('text=/Confirm|确认/i').first()).toBeVisible({ timeout: 5_000 })
  })

  test('confirming revert restores the note to the selected version', async ({ unlockedPage: page }) => {
    const v1 = `V1Content-${Date.now()}`
    await clickNewNote(page)
    await typeInEditor(page, v1)
    await manualSave(page)
    await page.waitForTimeout(500)

    // Edit the note (v2)
    await page.locator('.ProseMirror').click()
    await page.keyboard.press('Control+a')
    await page.keyboard.type('V2 replacement content')
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    // The oldest (bottom) entry is v1 — we want to revert to it
    const historyRows = page.locator('.group').filter({ has: page.locator('.font-mono') })
    const lastRow = historyRows.last()
    await lastRow.hover()
    await lastRow.locator('.history-revert-btn').first().click()
    // Confirm the revert
    await page.locator('.history-revert-btn').last().click() // inner "Confirm" button
    await page.waitForTimeout(2_000)

    // The editor should now show v1 content
    await expect(page.locator('.ProseMirror')).toContainText(v1.replace('# ', ''), { timeout: 8_000 })
  })

  test('cancelling revert leaves the note content unchanged', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    const original = `CancelRevert-${Date.now()}`
    await typeInEditor(page, original)
    await manualSave(page)
    await page.waitForTimeout(500)

    await page.locator('.ProseMirror').click()
    await page.keyboard.type('\nExtra content')
    await manualSave(page)
    await page.waitForTimeout(1_000)

    await page.locator('[data-testid="history-btn"]').click()
    const historyRow = page.locator('.group').filter({ has: page.locator('.font-mono') }).first()
    await historyRow.hover()
    await historyRow.locator('.history-revert-btn').first().click()

    // Click Cancel in the inline confirm bar
    await page.locator('text=/Cancel|取消/i').last().click()
    await page.waitForTimeout(500)

    // Content should still contain the most-recent text
    await expect(page.locator('.ProseMirror')).toContainText('Extra content')
  })
})
