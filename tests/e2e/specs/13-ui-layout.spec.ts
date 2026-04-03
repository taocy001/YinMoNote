/**
 * 13 – UI Layout Invariants
 *
 * Guards against structural regressions where action buttons that belong inside
 * the "…" context menu leak into the note-list row directly.
 *
 * Rules enforced:
 *   TC-LAYOUT-01  No "Edit Tags" button directly on a note-item row
 *   TC-LAYOUT-02  No "Create Sub-note" button directly on a note-item row
 *   TC-LAYOUT-04  note-more-btn is hidden by default (not hover state)
 *   TC-LAYOUT-05  note-more-btn becomes visible after hovering the row
 *   TC-LAYOUT-06  note-delete-btn is hidden until "…" menu is opened
 *   TC-LAYOUT-07  "…" menu contains all expected actions after click
 *
 * Intentionally omitted:
 *   TC-LAYOUT-03 (button count) — fragile; count changes legitimately when new
 *                                  quick-actions are added. Rules 01/02 cover intent.
 *   TC-VR-*      (screenshot)   — not in CI gate; environment-dependent pixel output.
 */
import { test, expect } from '../fixtures'
import { createAndSaveNote } from '../helpers/app'

test.describe('UI Layout – note list row invariants', () => {
  // ── TC-LAYOUT-01 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-01: no "Edit Tags" button directly on note-item row', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout01')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })

    // Do NOT hover — we want default state
    // Any button with title="Edit Tags" / "编辑标签" that is a direct descendant of
    // the note-item row (not inside the dropdown portal) must not exist.
    const tagBtnOnRow = noteItem.locator('button[title="Edit Tags"], button[title="编辑标签"]')
    await expect(tagBtnOnRow).toHaveCount(0)
  })

  // ── TC-LAYOUT-02 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-02: no "Create Sub-note" button directly on note-item row', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout02')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })

    const subNoteBtnOnRow = noteItem.locator('button[title="New Sub-note"], button[title="新建子笔记"]')
    await expect(subNoteBtnOnRow).toHaveCount(0)
  })

  // ── TC-LAYOUT-04 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-04: note-more-btn is hidden before hover', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout04')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })
    const moreBtn = noteItem.locator('[data-testid="note-more-btn"]')

    // Move mouse away from the note list so no accidental hover
    await page.mouse.move(0, 0)

    // toBeHidden() accounts for opacity:0, visibility:hidden, display:none and
    // pointer-events:none — more robust than reading computed opacity directly.
    await expect(moreBtn).toBeHidden()
  })

  // ── TC-LAYOUT-05 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-05: note-more-btn becomes visible on row hover', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout05')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })
    const moreBtn = noteItem.locator('[data-testid="note-more-btn"]')

    await noteItem.hover()
    await expect(moreBtn).toBeVisible()
  })

  // ── TC-LAYOUT-06 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-06: note-delete-btn is hidden until "…" menu is opened', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout06')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })

    // Default state: delete button not visible anywhere
    await page.mouse.move(0, 0)
    await expect(page.locator('[data-testid="note-delete-btn"]')).toBeHidden()

    // After hover but before opening menu: still hidden
    await noteItem.hover()
    await expect(page.locator('[data-testid="note-delete-btn"]')).toBeHidden()
  })

  // ── TC-LAYOUT-07 ─────────────────────────────────────────────────────────────
  test('TC-LAYOUT-07: "…" menu contains all expected actions', async ({ unlockedPage: page }) => {
    const title = await createAndSaveNote(page, 'Layout07')
    const noteItem = page.locator('[data-testid="note-item"]').filter({ hasText: title })

    await noteItem.hover()
    await noteItem.locator('[data-testid="note-more-btn"]').click()

    // Delete must appear inside the dropdown (not on the row)
    await expect(page.locator('[data-testid="note-delete-btn"]')).toBeVisible()

    // The menu must have exactly 4 items: pin/unpin, edit tags, new sub-note, delete
    // We check by counting .note-menu-item buttons inside the visible dropdown container.
    // The dropdown is rendered in a fixed-position div controlled by noteMenuKey.
    const menuContainer = page.locator('.fixed.z-\\[140\\]').last()
    const menuItems = menuContainer.locator('button.note-menu-item')
    await expect(menuItems).toHaveCount(4)
  })
})
