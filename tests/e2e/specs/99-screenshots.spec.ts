/**
 * 99-screenshots — product screenshots and animated GIF for README.
 *
 * Run via:  tests/take-screenshots.sh
 * Output:   /screenshots/ (mounted from docs/images/ on host)
 *
 * Generates:
 *   frames/frame-01.png … frame-NN.png   GIF source frames
 *   editor.png                            static hero screenshot
 *
 * Strategy: use only keyboard input rules (no slash command UI) so the spec
 * works regardless of which app image version is installed.
 *   # text   → H1 (initial node is already heading)
 *   ## text  → H2 input rule
 *   - [x]    → task list input rule (first item only; subsequent items via Enter)
 *   ```lang  → code block input rule (CodeBlockLowlight)
 */
import { test } from '../fixtures'
import { clickNewNote, manualSave } from '../helpers/app'
import * as fs from 'fs'
import * as path from 'path'

const OUT    = process.env.SCREENSHOT_DIR ?? '/screenshots'
const FRAMES = path.join(OUT, 'frames')
let frameIdx = 0

async function frame(page: any) {
  frameIdx++
  const n = String(frameIdx).padStart(2, '0')
  await page.screenshot({ path: path.join(FRAMES, `frame-${n}.png`) })
}

async function shot(page: any, name: string) {
  await page.screenshot({ path: path.join(OUT, `${name}.png`) })
}

test.describe('Product screenshots', () => {
  test.setTimeout(180_000)
  // Only run when the screenshot output directory is explicitly provided
  // (i.e. when invoked via tests/take-screenshots.sh with /screenshots mounted).
  // Skip silently in regular CI/e2e runs to avoid writing to a non-existent path.
  test.skip(!process.env.SCREENSHOT_DIR, 'Set SCREENSHOT_DIR env var to enable screenshot capture')

  test('capture rich editor + GIF frames', async ({ unlockedPage: page }) => {
    fs.mkdirSync(FRAMES, { recursive: true })

    // Wide viewport for the "full app" look
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.waitForTimeout(300)

    // ── Create sidebar context notes ───────────────────────────────────────
    for (const title of ['Getting Started', 'Weekly Review', 'Reading List']) {
      await clickNewNote(page)
      const ed = page.locator('.ProseMirror')
      await ed.click({ position: { x: 4, y: 4 } })
      await page.keyboard.type(title)
      await manualSave(page)
      await page.waitForTimeout(200)
    }

    // ── Open the main demo note ────────────────────────────────────────────
    await clickNewNote(page)
    const editor = page.locator('.ProseMirror')
    await editor.click({ position: { x: 4, y: 4 } })

    // Frame 1 — clean editor with sidebar
    await frame(page)

    // ── H1 title (new notes start with an empty heading node) ─────────────
    await page.keyboard.type('Project Notes')
    await page.waitForTimeout(400)
    await frame(page)  // Frame 2 — title in H1

    // ── Paragraph ─────────────────────────────────────────────────────────
    await page.keyboard.press('Enter')
    await page.keyboard.type(
      'A Markdown note app with live rendering — every element renders as you type, with no mode switching.'
    )
    await page.waitForTimeout(400)
    await frame(page)  // Frame 3 — paragraph text

    // ── H2 via input rule (## + space) ────────────────────────────────────
    await page.keyboard.press('Enter')
    await page.keyboard.press('Enter')
    await page.keyboard.type('## ')
    await page.keyboard.type('Action Items')
    await page.keyboard.press('Enter')

    // ── Task list via input rule (- [x] triggers TaskList) ────────────────
    // First item: type the full markdown syntax to trigger the input rule
    await page.keyboard.type('- [x] Real-time rendering, no preview toggle')
    await page.keyboard.press('Enter')
    // Subsequent items: Enter continues the task list; type text directly
    await page.keyboard.type('Code blocks with syntax highlighting')
    await page.keyboard.press('Enter')
    await page.keyboard.type('KaTeX math and Mermaid diagrams')
    await page.waitForTimeout(500)
    await frame(page)  // Frame 4 — H2 + task list

    // ── Exit task list (Enter on empty item) ──────────────────────────────
    await page.keyboard.press('Enter')
    await page.keyboard.press('Enter')
    await page.waitForTimeout(200)

    // ── H2 for code section ───────────────────────────────────────────────
    await page.keyboard.type('## ')
    await page.keyboard.type('Code Example')
    await page.keyboard.press('Enter')
    await page.waitForTimeout(200)

    // ── Code block via backtick input rule (```python + Enter) ────────────
    // CodeBlockLowlight input rule: backtickInputRegex = /^```([a-z]+)?[\s\n]$/
    await page.keyboard.type('```python')
    await page.keyboard.press('Enter')
    await page.waitForTimeout(400)
    await frame(page)  // Frame 5 — empty code block appeared

    // Type Python code with syntax highlighting
    await page.keyboard.type('def fibonacci(n: int) -> int:')
    await page.keyboard.press('Enter')
    await page.keyboard.type('    if n <= 1: return n')
    await page.keyboard.press('Enter')
    await page.keyboard.type('    return fibonacci(n - 1) + fibonacci(n - 2)')
    await page.keyboard.press('Enter')
    await page.keyboard.press('Enter')
    await page.keyboard.type('print(fibonacci(10))  # 55')
    await page.waitForTimeout(500)
    await frame(page)  // Frame 6 — code block with Python syntax highlighting

    // ── Save and capture final screenshot ─────────────────────────────────
    await manualSave(page)
    await page.waitForTimeout(600)
    await frame(page)  // Frame 7 — saved state ("Saved" pill visible)

    await shot(page, 'editor')
  })
})
