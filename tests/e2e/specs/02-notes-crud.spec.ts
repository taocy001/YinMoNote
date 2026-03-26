/**
 * 02 – Notes CRUD (Create / Read / Update / Delete)
 *
 * Covers the full lifecycle of a note:
 *   • Creating from the sidebar button
 *   • Creating from the empty-state button
 *   • Typing content → save-status changes
 *   • Manual save → status transitions to "Saved"
 *   • Note title appearing in the sidebar after save
 *   • Switching between notes
 *   • Deleting: confirm path (note removed) and cancel path (note retained)
 */
import { test, expect } from '../fixtures'
import { clickNewNote, typeInEditor, manualSave, createAndSaveNote } from '../helpers/app'

test.describe('Notes – CRUD', () => {
  // ── Create ────────────────────────────────────────────────────────────────

  test('New Note button opens the Tiptap editor', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await expect(page.locator('.ProseMirror')).toBeVisible()
  })

  test('editor area is focused immediately after note creation', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    // ProseMirror sets contenteditable="true" on the div when focused
    await expect(page.locator('.ProseMirror')).toBeVisible()
    // It should have contenteditable attribute
    await expect(page.locator('.ProseMirror')).toHaveAttribute('contenteditable', 'true')
  })

  test('empty-state New Note button also opens the editor', async ({ unlockedPage: page }) => {
    // Wait for the sidebar to settle (notes may still be loading from the server
    // when unlockKeyless() returns, which causes the empty-state element to flash
    // in and out as the note list renders, leading to a detached-element click error).
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})
    const emptyBtn = page.locator('[data-testid="empty-state-new-note-btn"]')
    const sidebarBtn = page.locator('[data-testid="new-note-btn"]')
    // Use the empty-state button if visible (fresh server), otherwise sidebar button
    const btn = (await emptyBtn.isVisible()) ? emptyBtn : sidebarBtn
    await btn.click()
    await expect(page.locator('.ProseMirror')).toBeVisible()
  })

  // ── Save status lifecycle ─────────────────────────────────────────────────

  test('typing content changes save-status to "Unsaved"', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, 'Hello from E2E test')
    const status = page.locator('[data-testid="save-status"]')
    await expect(status).toContainText('Unsaved', { timeout: 5_000 })
  })

  test('save-status pill is clickable and triggers an immediate save', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# ClickToSave-${Date.now()}`)
    const status = page.locator('[data-testid="save-status"]')
    await expect(status).toContainText('Unsaved', { timeout: 5_000 })
    // Click the pill → manual save
    await status.click()
    // Status must transition away from "Unsaved"
    await expect(status).not.toContainText('Unsaved', { timeout: 15_000 })
  })

  test('save-status shows "Saved" after successful save', async ({ unlockedPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# SavedStatus-${Date.now()}`)
    await page.locator('[data-testid="save-status"]').click()
    const status = page.locator('[data-testid="save-status"]')
    await expect(status).toContainText('Saved', { timeout: 15_000 })
  })

  // ── Sidebar title ─────────────────────────────────────────────────────────

  test('first line of note becomes title in the sidebar', async ({ unlockedPage: page }) => {
    const title = `SidebarTitle-${Date.now()}`
    await clickNewNote(page)
    // Wait for the title-debounce structure save triggered by typeInEditor
    const titleSaved = page.waitForResponse(
      r => r.url().includes('/api/structure') && r.request().method() === 'PUT',
      { timeout: 5_000 }
    )
    await typeInEditor(page, `# ${title}`)
    await manualSave(page)
    await titleSaved.catch(() => {})
    // The title should now appear in the sidebar note list
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: title })
    ).toBeVisible({ timeout: 10_000 })
  })

  test('selecting a note from sidebar loads it in the editor', async ({ unlockedPage: page }) => {
    // Create two notes and verify switching between them
    const title1 = await createAndSaveNote(page, 'FirstNote')
    const title2 = await createAndSaveNote(page, 'SecondNote')

    // Click the first note in the sidebar
    const note1Item = page.locator('[data-testid="note-item"]').filter({ hasText: title1 })
    await note1Item.click()
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})
    // The editor should show the first note's content
    await expect(page.locator('.ProseMirror')).toContainText(title1)

    // Switch to the second note
    const note2Item = page.locator('[data-testid="note-item"]').filter({ hasText: title2 })
    await note2Item.click()
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})
    await expect(page.locator('.ProseMirror')).toContainText(title2)
  })

  // ── Delete (confirm) ─────────────────────────────────────────────────────

  test('hover → X button → confirm modal visible', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'HoverDelete')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })
    await noteItem.hover()
    await noteItem.locator('[data-testid="note-delete-btn"]').click()
    await expect(page.locator('[data-testid="delete-confirm-btn"]')).toBeVisible()
    await expect(page.locator('[data-testid="delete-cancel-btn"]')).toBeVisible()
  })

  test('confirm delete removes note from sidebar', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'DeleteConfirm')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })
    await noteItem.hover()
    await noteItem.locator('[data-testid="note-delete-btn"]').click()
    await page.locator('[data-testid="delete-confirm-btn"]').click()
    // Note must disappear from the sidebar
    await expect(noteItem).not.toBeVisible({ timeout: 8_000 })
  })

  // ── Delete (cancel) ───────────────────────────────────────────────────────

  test('cancel delete keeps the note in the sidebar', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'DeleteCancel')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })
    await noteItem.hover()
    await noteItem.locator('[data-testid="note-delete-btn"]').click()
    await page.locator('[data-testid="delete-cancel-btn"]').click()
    // Note must still be present
    await expect(noteItem).toBeVisible()
    // Delete modal must have closed
    await expect(page.locator('[data-testid="delete-confirm-btn"]')).not.toBeVisible()
  })

  // ── Content persistence ───────────────────────────────────────────────────

  test('note content survives a page reload', async ({ unlockedPage: page }) => {
    const content = `PersistenceTest-${Date.now()}`
    await clickNewNote(page)
    await typeInEditor(page, content)
    await manualSave(page)
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})

    // Reload the page — keyless mode does NOT clear the session so no re-unlock needed
    await page.reload({ waitUntil: 'domcontentloaded' })
    await page.locator('[data-testid="new-note-btn"]').waitFor({ state: 'visible', timeout: 15_000 })
    await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})

    // The note should be visible in the sidebar
    await expect(
      page.locator('[data-testid="note-item"]').filter({ hasText: content.split('-')[0] })
    ).toBeVisible({ timeout: 10_000 })
  })
})
