/**
 * Extended Playwright fixtures for YinMoNote E2E tests.
 *
 * `unlockedPage` — a Page that has already passed the unlock modal via keyless
 * (No Encryption) mode.  Use this fixture in tests that only care about app
 * behaviour after login; it avoids repeating the unlock flow in every test.
 *
 * `passwordPage` — a Page unlocked with password mode (password: 'E2EPass123').
 * Use for tests that require encryption features (serverEncrypt toggle, etc.).
 */
import { test as base, type Page } from '@playwright/test'
import { unlockKeyless, initWithPassword, clearServerAuth } from './helpers/app'

type AppFixtures = {
  unlockedPage: Page
  passwordPage: Page
}

export const test = base.extend<AppFixtures>({
  unlockedPage: async ({ page }, use) => {
    await unlockKeyless(page)
    await use(page)
  },
  passwordPage: async ({ page }, use) => {
    await initWithPassword(page, 'E2EPass123')
    await use(page)
    // Reset server auth so subsequent keyless tests can reach the API without a token.
    await clearServerAuth(page)
  },
})

export { expect } from '@playwright/test'
