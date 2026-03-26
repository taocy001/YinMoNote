/**
 * 08 – Server-side encryption (serverEncrypt) toggle
 *
 * Tests the serverEncrypt setting in the Security tab:
 *   • Toggle visibility: visible in password mode, hidden in keyless mode
 *   • Default state is ON for password/device mode (OFF for keyless)
 *   • With serverEncrypt OFF: note content sent to server is plaintext
 *   • With serverEncrypt ON: note content sent to server starts with "ENC1:"
 *   • Data integrity: note remains readable in editor after toggling ON
 *   • Data integrity: note remains readable in editor after toggling back OFF
 *   • Cloud-encryption badge appears in the sidebar when serverEncrypt is ON
 *   • idle-timeout selector is visible in password mode (hidden in keyless mode)
 *
 * All tests that mutate serverEncrypt restore it to OFF at the end to leave
 * the server in plaintext state for subsequent specs.
 */
import { test, expect } from '../fixtures'
import {
  clickNewNote,
  typeInEditor,
  manualSave,
  unlockKeyless,
  openSecuritySettings,
  setServerEncrypt,
} from '../helpers/app'

// ─── Visibility ───────────────────────────────────────────────────────────────

test.describe('serverEncrypt toggle visibility', () => {
  test('toggle is visible in Security tab when using password mode', async ({ passwordPage: page }) => {
    // openSecuritySettings falls back to the empty-state settings button when
    // no note is currently open (so no editor toolbar is rendered).
    await openSecuritySettings(page)
    await expect(page.locator('[data-testid="server-encrypt-toggle"]')).toBeVisible()
  })

  test('toggle is NOT rendered in keyless (no-encryption) mode', async ({ page }) => {
    await unlockKeyless(page)
    await clickNewNote(page)
    await page.locator('[data-testid="settings-btn"]').click()
    await page.locator('[data-testid="tab-security"]').click()
    // The entire security content is inside v-if="!isKeylessModeActive"
    await expect(page.locator('[data-testid="server-encrypt-toggle"]')).not.toBeVisible()
  })

  test('idle-timeout selector is visible in password mode', async ({ passwordPage: page }) => {
    await openSecuritySettings(page)
    await expect(page.locator('[data-testid="idle-timeout-select"]')).toBeVisible()
  })
})

// ─── Default state ────────────────────────────────────────────────────────────

test.describe('serverEncrypt default state', () => {
  test('serverEncrypt is ON by default in password mode (toggle background is accent)', async ({ passwordPage: page }) => {
    await openSecuritySettings(page)
    const toggle = page.locator('[data-testid="server-encrypt-toggle"]')
    const style = (await toggle.getAttribute('style')) ?? ''
    // When ON the toggle uses var(--accent)
    expect(style).toContain('var(--accent)')
    expect(style).not.toContain('var(--border-strong)')
  })
})

// ─── Content encryption verification ─────────────────────────────────────────

test.describe('note content on server', () => {
  test('with serverEncrypt OFF: PUT /api/notes body contains plaintext (no ENC1)', async ({ passwordPage: page }) => {
    // serverEncrypt defaults to ON in password mode; explicitly disable for this test
    await setServerEncrypt(page, false)
    await clickNewNote(page)
    const title = `PlainText-${Date.now()}`
    await typeInEditor(page, `# ${title}`)

    // Intercept the PUT request triggered by manualSave
    const saveRequest = page.waitForRequest(
      r => r.url().includes('/api/notes/') && r.method() === 'PUT',
      { timeout: 15_000 }
    )
    await manualSave(page)
    const req = await saveRequest
    const body = req.postData() ?? ''
    // Body is JSON: {"content": "..."} — should NOT contain ENC1 prefix
    expect(body).not.toContain('ENC1:')
    expect(body).toContain(title)
  })

  test('with serverEncrypt ON: PUT /api/notes body starts with ENC1 prefix', async ({ passwordPage: page }) => {
    await clickNewNote(page)
    await typeInEditor(page, `# PreEncrypt-${Date.now()}`)
    await manualSave(page)

    // Enable server-side encryption (triggers batch re-encryption of existing notes)
    await setServerEncrypt(page, true)

    // Create a new note and verify the PUT body is encrypted
    await clickNewNote(page)
    const title = `ENC1Note-${Date.now()}`
    await typeInEditor(page, `# ${title}`)

    const saveRequest = page.waitForRequest(
      r => r.url().includes('/api/notes/') && r.method() === 'PUT',
      { timeout: 15_000 }
    )
    await manualSave(page)
    const req = await saveRequest
    const body = req.postData() ?? ''
    // Body is JSON: {"content": "ENC1:..."} when server-side encryption is on
    expect(body).toContain('ENC1:')

    // Cleanup: restore plaintext state
    await setServerEncrypt(page, false)
  })
})

// ─── Data integrity ───────────────────────────────────────────────────────────

test.describe('data integrity after serverEncrypt toggle', () => {
  test('note content is readable in editor immediately after enabling serverEncrypt', async ({ passwordPage: page }) => {
    await clickNewNote(page)
    const content = `IntegrityOn-${Date.now()}`
    await typeInEditor(page, `# ${content}`)
    await manualSave(page)

    // Enable serverEncrypt — batch re-encrypts all notes and then reloads the editor
    await setServerEncrypt(page, true)

    // The editor should still display the original content after batch + reload
    await expect(page.locator('.ProseMirror')).toContainText(content, { timeout: 10_000 })

    // Cleanup
    await setServerEncrypt(page, false)
  })

  test('note content is readable in editor after disabling serverEncrypt', async ({ passwordPage: page }) => {
    await clickNewNote(page)
    const content = `IntegrityOff-${Date.now()}`
    await typeInEditor(page, `# ${content}`)
    await manualSave(page)

    // Enable then immediately disable (tests the decrypt-all batch)
    await setServerEncrypt(page, true)
    await setServerEncrypt(page, false)

    // The editor should still display the original content
    await expect(page.locator('.ProseMirror')).toContainText(content, { timeout: 10_000 })
  })

  test.skip('cloud-encryption badge appears in sidebar when serverEncrypt is ON', async ({ passwordPage: page }) => {
    // The cloudEncryptedBadge i18n key is defined but no component renders it yet.
    // Skip until the sidebar badge UI is implemented.
    await clickNewNote(page)
    await typeInEditor(page, `# BadgeTest-${Date.now()}`)
    await manualSave(page)

    await setServerEncrypt(page, true)

    // Sidebar shows a lock/shield badge when cloud encryption is active
    // The badge text is t.cloudEncryptedBadge
    await expect(
      page.locator('text=/Encrypted|已加密/i').first()
    ).toBeVisible({ timeout: 5_000 })

    // Cleanup
    await setServerEncrypt(page, false)
  })
})
