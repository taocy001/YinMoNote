/**
 * Shared helpers for YinMoNote E2E tests.
 *
 * Design decisions:
 *  - All tests use keyless (No Encryption) mode so no crypto setup is needed.
 *  - freshPage() resets both client (localStorage/sessionStorage/window.name) and
 *    navigates to the home URL, ensuring each test sees the first-visit unlock modal.
 *  - English is forced on every fresh page so test assertions can use English strings.
 */
import { type Page, expect } from '@playwright/test'

export const BASE_URL = process.env.APP_URL ?? 'http://localhost:8080'

// ─── Auth Reset ───────────────────────────────────────────────────────────────

/**
 * Clear the server-side SessionTokenHash so subsequent keyless tests can reach
 * the API without a Bearer token.
 *
 * Uses POST /api/test/reset-auth (available only when SYNC_COMMIT=1, i.e. the
 * E2E Docker environment).  Falls back to the token-authenticated
 * POST /api/auth/setup path when the test endpoint is absent.
 *
 * Best-effort — errors are swallowed so tests don't fail on teardown noise.
 */
export async function clearServerAuth(page: Page): Promise<void> {
  try {
    // Preferred: unconditional reset endpoint (only present in E2E builds).
    const res = await page.request.post(`${BASE_URL}/api/test/reset-auth`)
    if (res.ok()) return
  } catch (_) { /* endpoint absent or failed — try token-based fallback */ }

  try {
    const token = await page.evaluate(() => sessionStorage.getItem('yinmo_session_token'))
    if (!token) return
    await page.request.post(`${BASE_URL}/api/auth/setup`, {
      data: { token_hash: '' },
      headers: { 'Authorization': `Bearer ${token}` },
    })
  } catch (_) {
    // best-effort
  }
}

// ─── Navigation & State Reset ─────────────────────────────────────────────────

/**
 * Navigate to the app and wipe all browser-side state so the unlock modal
 * appears as if this were the very first visit.
 */
export async function freshPage(page: Page): Promise<void> {
  await page.goto('/')
  await page.evaluate(() => {
    localStorage.clear()
    sessionStorage.clear()
    // Reset window.name so the session-wrap key is regenerated on reload.
    window.name = ''
  })
  await page.reload({ waitUntil: 'domcontentloaded' })
}

// ─── Language ─────────────────────────────────────────────────────────────────

/**
 * Ensure the app is displaying in English by setting the localStorage lang key.
 * Must be called before or after freshPage — if called after, a reload is needed
 * for the change to take effect.
 */
export async function switchToEnglish(page: Page): Promise<void> {
  await page.evaluate(() => localStorage.setItem('lang', 'en'))
  await page.reload({ waitUntil: 'domcontentloaded' })
}

// ─── Unlock Flows ─────────────────────────────────────────────────────────────

/**
 * Perform a full keyless (No Encryption) unlock from a completely fresh page.
 * After this call the sidebar is visible and the app is ready for interaction.
 */
export async function unlockKeyless(page: Page): Promise<void> {
  await freshPage(page)
  await switchToEnglish(page)
  await page.locator('[data-testid="mode-tab-none"]').click()
  await page.locator('[data-testid="unlock-btn"]').click()
  // Sidebar New Note button signals the app is fully unlocked
  await page.locator('[data-testid="new-note-btn"]').waitFor({
    state: 'visible',
    timeout: 20_000,
  })
  // Wait for the initial loadNotesList / loadConfig API calls to finish so the
  // sidebar note list is fully populated before tests inspect it.
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
}

/**
 * Initialise the library with a password (first-time setup).
 * After this call the app is unlocked and the sidebar is visible.
 */
export async function initWithPassword(page: Page, password: string): Promise<void> {
  // Clear server auth BEFORE freshPage so the reload's automatic loadNotesList() call
  // doesn't get a 401 (server still has the old hash, but sessionStorage was just wiped).
  // A 401 from loadNotesList causes a "Failed to load notes" toast that covers the
  // unlock button and prevents the subsequent click from landing.
  await clearServerAuth(page)
  await freshPage(page)
  await switchToEnglish(page)
  await page.locator('[data-testid="mode-tab-password"]').click()
  await page.locator('[data-testid="unlock-password-input"]').fill(password)
  await page.locator('[data-testid="unlock-btn"]').click()
  // sidebar-lock-btn (v-if="!isLibraryLocked") is the reliable unlock signal
  // in password mode; new-note-btn has no v-if and is always in the DOM.
  await page.locator('[data-testid="sidebar-lock-btn"]').waitFor({
    state: 'visible',
    timeout: 20_000,
  })
  // Wait for the initial loadNotesList / loadConfig API calls to finish so the
  // sidebar note list is fully populated before tests inspect it.
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
  // seedReleaseNote sets currentNote reactively; the Editor mounts in the next
  // Vue render cycle (async). Give the DOM time to stabilise before callers
  // try to interact with editor toolbar buttons.
  await page.waitForTimeout(300)
}

/**
 * Simulate a "new device" connecting to a server whose library was already
 * initialized on another device.
 *
 * Unlike initWithPassword, this does NOT call clearServerAuth first —
 * the server keeps its existing SessionTokenHash.  The app should detect the
 * initialized state via GET /api/auth/status and present the password-unlock
 * UI (not the init wizard) for the user to enter their existing password.
 */
export async function unlockAsNewDevice(page: Page, password: string): Promise<void> {
  // Wipe client-side state only; server auth remains intact.
  await freshPage(page)
  await switchToEnglish(page)
  // onMounted queries /api/auth/status; when initialized=true it shows the
  // UNLOCK UI with a password input.  Wait for that input to appear.
  const input = page.locator('[data-testid="unlock-password-input"]')
  await input.waitFor({ state: 'visible', timeout: 10_000 })
  await input.fill(password)
  await page.locator('[data-testid="unlock-btn"]').click()
  // sidebar-lock-btn (v-if="!isLibraryLocked") is the reliable unlock signal
  // in password mode; new-note-btn has no v-if and is always in the DOM.
  await page.locator('[data-testid="sidebar-lock-btn"]').waitFor({
    state: 'visible',
    timeout: 20_000,
  })
  // Wait for the initial loadNotesList / loadConfig API calls to finish so the
  // sidebar note list is fully populated before tests inspect it.
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
  // seedReleaseNote sets currentNote reactively; give Vue time to render the Editor.
  await page.waitForTimeout(300)
}

/**
 * Simulate a "browser restart" on the SAME device: the session key in
 * sessionStorage is lost (as it is on a real browser close), but localStorage
 * (PBKDF2 salt, LIBRARY_KEY_STORE, etc.) persists — exactly as in a real
 * browser restart.
 *
 * This is distinct from unlockAsNewDevice (which wipes ALL browser storage via
 * freshPage). Use this helper for tests that verify note persistence across a
 * session loss, NOT for multi-device scenarios.
 *
 * The PBKDF2 salt stays in localStorage, so the same password re-derives the
 * same library key and all encrypted notes remain decryptable.
 */
export async function browserRestartUnlock(page: Page, password: string): Promise<void> {
  // Simulate session expiry: clear only sessionStorage (session key lost) and
  // window.name (wrap key regenerated), but leave localStorage intact.
  await page.evaluate(() => {
    sessionStorage.clear()
    window.name = ''
  })
  await page.reload({ waitUntil: 'domcontentloaded' })
  await switchToEnglish(page)
  // Library key exists in localStorage → app shows the lock screen (not init
  // wizard and not new-device unlock).  Password input is visible directly.
  const input = page.locator('[data-testid="unlock-password-input"]')
  await input.waitFor({ state: 'visible', timeout: 10_000 })
  await input.fill(password)
  await page.locator('[data-testid="unlock-btn"]').click()
  await page.locator('[data-testid="sidebar-lock-btn"]').waitFor({
    state: 'visible',
    timeout: 20_000,
  })
  await page.waitForLoadState('networkidle', { timeout: 10_000 }).catch(() => {})
}

// ─── Note Operations ──────────────────────────────────────────────────────────

/**
 * Click the sidebar New Note button and wait for the editor's ProseMirror area
 * to become visible and focused.
 */
export async function clickNewNote(page: Page): Promise<void> {
  // createNewNote() calls saveStructure() which sends PUT /api/structure.
  // Waiting for this response ensures:
  //   1. currentNote has been set to the new note's ID (happens before the await)
  //   2. Vue has re-rendered (happens before the axios call starts, in the microtask queue)
  //   3. The new Editor is mounted with the correct noteFileName prop
  // This avoids the timing ambiguity where .ProseMirror.waitFor('visible') returns
  // immediately from an OLD editor that was open (from notes left by earlier specs),
  // causing subsequent history/type operations to target the wrong note.
  const structureSaved = page.waitForResponse(
    r => r.url().includes('/api/structure') && r.request().method() === 'PUT',
    { timeout: 10_000 }
  )
  await page.locator('[data-testid="new-note-btn"]').click()
  await structureSaved
  // Allow the async loadNote() call (which returns 404 for a pending note) to
  // complete and isLoading to reset before tests interact with the editor.
  await page.waitForTimeout(400)
}

/**
 * Type text into the Tiptap editor.  Clicks to focus first.
 * Use page.keyboard.press('Control+a') before calling this to replace any
 * existing content.
 */
export async function typeInEditor(page: Page, text: string): Promise<void> {
  const editor = page.locator('.ProseMirror')
  // Click near the top-left corner so the cursor lands inside the first line (heading),
  // not in empty space below it (which would create a second paragraph and leave the
  // first heading empty, causing doc.firstChild.textContent to always be "").
  await editor.click({ position: { x: 4, y: 4 } })
  await page.keyboard.type(text)
}

/**
 * Click the save-status pill to trigger an immediate (manual) save, then wait
 * until the "Unsaved" label is gone (status is "Saving…" → "Saved").
 */
export async function manualSave(page: Page): Promise<void> {
  const status = page.locator('[data-testid="save-status"]')
  // The pill is only clickable when saveStatus === 'dirty'
  await status.click()
  await expect(status).not.toContainText(/Unsaved/i, { timeout: 15_000 })
}

// ─── Settings Helpers ─────────────────────────────────────────────────────────

/**
 * Open the settings panel via the editor toolbar button and navigate to the
 * Security tab.  Assumes an editor is currently open.
 */
export async function openSecuritySettings(page: Page): Promise<void> {
  // Wait for the app to stabilise — post-login reactive updates (loadConfig,
  // loadNotesList) can cause the empty-state panel to detach and re-attach
  // while the DOM is still settling, making the button un-clickable.
  await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})

  // The settings button exists in the editor toolbar (when a note is open) and
  // in the empty-state panel (when no note is selected).
  const editorBtn = page.locator('[data-testid="settings-btn"]')
  const emptyBtn  = page.locator('[data-testid="empty-state-settings-btn"]')

  // Prefer the editor toolbar button: it lives inside the mounted Editor and is
  // stable.  The empty-state button can detach/re-attach while Vue processes the
  // seedReleaseNote render cycle after initWithPassword.  Give the editor up to
  // 2 s to mount (covers the async Vue render after currentNote is set), then
  // fall back to the empty-state button if no note is selected.
  const editorBtnVisible = await editorBtn.waitFor({ state: 'visible', timeout: 2_000 })
    .then(() => true).catch(() => false)

  if (editorBtnVisible) {
    await editorBtn.click()
  } else {
    await emptyBtn.waitFor({ state: 'visible', timeout: 8_000 })
    await emptyBtn.click()
  }
  await page.locator('[data-testid="tab-security"]').click()
}

/**
 * Toggle the serverEncrypt switch to the desired state, then click Apply and
 * wait for any batch re-encryption overlay to finish.
 *
 * @param targetState - true to enable server-side encryption, false to disable
 */
export async function setServerEncrypt(page: Page, targetState: boolean): Promise<void> {
  await openSecuritySettings(page)
  const toggle = page.locator('[data-testid="server-encrypt-toggle"]')
  // The toggle style contains "var(--accent)" when enabled, "var(--border-strong)" when disabled.
  const style = (await toggle.getAttribute('style')) ?? ''
  const isCurrentlyOn = style.includes('var(--accent)')
  if (isCurrentlyOn !== targetState) {
    await toggle.click()
  }
  await page.locator('[data-testid="settings-apply-btn"]').click()
  // Settings panel closes before the batch starts; wait for it to close first.
  await expect(page.locator('[data-testid="settings-panel"]')).not.toBeVisible({ timeout: 8_000 })
  // Wait for the batch re-encryption overlay to disappear (or never appear).
  // The overlay text is "Converting document encryption..." / "正在转换文档加密状态…"
  await page.locator('text=/Converting document encryption|正在转换/').waitFor({
    state: 'hidden',
    timeout: 60_000,
  }).catch(() => {
    // Element never appeared (batch was instant or not triggered) — that's fine.
  })
}

/**
 * Create a note and save it.  Returns the unique title used.
 */
export async function createAndSaveNote(page: Page, prefix = 'E2ENote'): Promise<string> {
  const title = `${prefix}-${Date.now()}`
  await clickNewNote(page)
  // Set up the structure-save waiter BEFORE typing so it captures the
  // title-debounce PUT /api/structure that fires 500 ms after the last
  // keystroke.  clickNewNote has already completed its own structure PUT, so
  // this waiter only fires for the title-debounce save triggered by typeInEditor.
  const titleSaved = page.waitForResponse(
    r => r.url().includes('/api/structure') && r.request().method() === 'PUT',
    { timeout: 5_000 }
  )
  await typeInEditor(page, `# ${title}\n\nBody content for ${title}.`)
  await manualSave(page)
  // Ensure the title-debounce saveStructure() PUT has completed so the server
  // has the correct title before any subsequent reload or page navigation.
  await titleSaved.catch(() => {})
  // Belt-and-suspenders: wait for all network activity (including any late-firing
  // debounce structure PUTs) to settle before returning.
  await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => {})
  return title
}
