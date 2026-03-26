/**
 * 10 – Coverage Supplement
 *
 * Fills gaps not covered by the earlier nine specs:
 *   • Empty search results (zero visible items)
 *   • Rapid note switching — UI settles on the last-clicked note
 *   • Multi-note persistence — all notes visible and switchable
 *   • Edit + save updates the sidebar title
 *   • Consecutive saves (save-status lifecycle)
 *   • Three notes survive a full page reload
 *   • Clearing the search filter restores all notes
 */
import { test, expect } from '../fixtures'
import { clickNewNote, typeInEditor, manualSave, createAndSaveNote } from '../helpers/app'

test.describe('Coverage – supplemental scenarios', () => {

  // ── Empty search results ───────────────────────────────────────────────────

  test('search with no match shows zero note items', async ({ unlockedPage: page }) => {
    const title = `Unique-${Date.now()}`
    await createAndSaveNote(page, title)

    const searchInput = page.locator('[data-testid="search-input"]')
    await searchInput.fill('__WILL_NEVER_MATCH_ANYTHING__')
    await page.waitForTimeout(400) // debounce

    // After debounce the sidebar should list zero notes
    const items = page.locator('[data-testid="note-item"]')
    await expect(items).toHaveCount(0, { timeout: 5_000 })
  })

  // ── Clearing search restores all notes ────────────────────────────────────

  test('clearing search restores all notes in the sidebar', async ({ unlockedPage: page }) => {
    const titleA = `ClearSearchA-${Date.now()}`
    const titleB = `ClearSearchB-${Date.now()}`
    await createAndSaveNote(page, titleA)
    await createAndSaveNote(page, titleB)

    const searchInput = page.locator('[data-testid="search-input"]')
    await searchInput.fill('ClearSearchA')
    await page.waitForTimeout(400)

    // Only A should be visible
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleB })
    ).toHaveCount(0, { timeout: 5_000 })

    // Clear the search
    await searchInput.fill('')
    await page.waitForTimeout(400)

    // Both notes should now appear
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleA })
    ).toBeVisible({ timeout: 5_000 })
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleB })
    ).toBeVisible({ timeout: 5_000 })
  })

  // ── Edit + save updates sidebar title ─────────────────────────────────────

  test('editing a note title and saving updates the sidebar entry', async ({ unlockedPage: page }) => {
    const original = `OriginalTitle-${Date.now()}`
    await createAndSaveNote(page, original)

    // Select all and replace with a new title
    await page.keyboard.press('Control+a')
    const updated = `UpdatedTitle-${Date.now()}`
    await typeInEditor(page, `# ${updated}`)
    await manualSave(page)
    // Wait for the title-debounce structure save
    await page.waitForResponse(
      r => r.url().includes('/api/structure') && r.request().method() === 'PUT',
      { timeout: 5_000 }
    ).catch(() => {})

    // Sidebar should show the new title
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: updated })
    ).toBeVisible({ timeout: 8_000 })
  })

  // ── Consecutive saves (status lifecycle) ──────────────────────────────────

  test('consecutive edits and saves cycle through status correctly', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# ConsecutiveSave-${Date.now()}`)

    const status = page.locator('[data-testid="save-status"]')
    await expect(status).toContainText('Unsaved', { timeout: 5_000 })

    // First save
    await status.click()
    await expect(status).not.toContainText('Unsaved', { timeout: 15_000 })

    // Type again → dirty again
    await typeInEditor(page, ' extra content')
    await expect(status).toContainText('Unsaved', { timeout: 5_000 })

    // Second save
    await status.click()
    await expect(status).not.toContainText('Unsaved', { timeout: 15_000 })
  })

  // ── Multi-note: two notes accessible and switchable ──────────────────────

  test('two notes are both visible in sidebar and can be switched', async ({ unlockedPage: page }) => {
    test.setTimeout(60_000)
    const titleA = `SwitchA-${Date.now()}`
    const titleB = `SwitchB-${Date.now()}`
    await createAndSaveNote(page, titleA)
    await createAndSaveNote(page, titleB)

    // Both should appear in the sidebar
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleA })
    ).toBeVisible({ timeout: 8_000 })
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleB })
    ).toBeVisible({ timeout: 8_000 })

    // Click A and verify it loads in the editor
    await page.locator('[data-testid="note-item"]').filter({ hasText: titleA }).click()
    await expect(page.locator('.ProseMirror')).toContainText(titleA, { timeout: 8_000 })
  })

  // ── Rapid note switching ───────────────────────────────────────────────────

  test('rapid note switching settles on the last-clicked note', async ({ unlockedPage: page }) => {
    test.setTimeout(60_000)
    const titleA = `RapidA-${Date.now()}`
    const titleB = `RapidB-${Date.now()}`
    await createAndSaveNote(page, titleA)
    await createAndSaveNote(page, titleB)

    const itemA = page.locator('[data-testid="note-item"]').filter({ hasText: titleA })
    const itemB = page.locator('[data-testid="note-item"]').filter({ hasText: titleB })

    // Click A then immediately B — UI should settle on B
    await itemA.click()
    await itemB.click()

    await expect(page.locator('.ProseMirror')).toContainText(titleB, { timeout: 10_000 })
  })

  // ── Page reload persistence ────────────────────────────────────────────────

  test('two notes survive a full page reload', async ({ unlockedPage: page }) => {
    test.setTimeout(60_000)
    const titleA = `ReloadA-${Date.now()}`
    const titleB = `ReloadB-${Date.now()}`
    await createAndSaveNote(page, titleA)
    await createAndSaveNote(page, titleB)

    // Reload without clearing storage (tests persistence, not first-visit flow)
    await page.reload({ waitUntil: 'domcontentloaded' })
    // Wait for the app to re-initialise (sidebar becomes visible)
    await page.locator('[data-testid="new-note-btn"]').waitFor({
      state: 'visible',
      timeout: 20_000,
    })
    await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})

    // Both notes should still be in the sidebar after reload
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleA })
    ).toBeVisible({ timeout: 10_000 })
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: titleB })
    ).toBeVisible({ timeout: 10_000 })
  })

})
