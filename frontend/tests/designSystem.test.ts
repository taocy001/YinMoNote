/**
 * Design System Tests
 *
 * Validates that the design system CSS variables and utility classes
 * are correctly defined and consistent across light/dark themes.
 */
import { describe, it, expect, beforeAll } from 'vitest'
import { readFileSync } from 'fs'
import { resolve } from 'path'

// Read source files once
const indexHtml = readFileSync(resolve(__dirname, '../index.html'), 'utf-8')
const styleCss = readFileSync(resolve(__dirname, '../src/style.css'), 'utf-8')

// ─── Type scale variables ─────────────────────────────────────────────────────

describe('type scale CSS variables', () => {
  const typeVars = ['--text-xs', '--lh-xs', '--text-sm', '--lh-sm', '--text-base', '--lh-base', '--text-lg', '--lh-lg', '--text-xl', '--lh-xl']

  it.each(typeVars)('%s is defined in :root', (varName) => {
    expect(indexHtml).toContain(varName)
  })

  it('defines exactly 5 type scale levels', () => {
    const textVars = indexHtml.match(/--text-(xs|sm|base|lg|xl):/g) || []
    expect(textVars.length).toBe(5)
  })

  it('each text size has a matching line-height', () => {
    const sizes = ['xs', 'sm', 'base', 'lg', 'xl']
    for (const s of sizes) {
      expect(indexHtml).toContain(`--text-${s}:`)
      expect(indexHtml).toContain(`--lh-${s}:`)
    }
  })
})

// ─── Animation easing variables ───────────────────────────────────────────────

describe('animation easing CSS variables', () => {
  const animVars = ['--ease-micro', '--duration-micro', '--ease-normal', '--duration-normal', '--ease-expressive', '--duration-expressive']

  it.each(animVars)('%s is defined in :root', (varName) => {
    expect(indexHtml).toContain(varName)
  })

  it('micro duration is shorter than normal', () => {
    const microMatch = indexHtml.match(/--duration-micro:\s*(\d+)ms/)
    const normalMatch = indexHtml.match(/--duration-normal:\s*(\d+)ms/)
    expect(microMatch).not.toBeNull()
    expect(normalMatch).not.toBeNull()
    expect(Number(microMatch![1])).toBeLessThan(Number(normalMatch![1]))
  })

  it('normal duration is shorter than expressive', () => {
    const normalMatch = indexHtml.match(/--duration-normal:\s*(\d+)ms/)
    const expressiveMatch = indexHtml.match(/--duration-expressive:\s*(\d+)ms/)
    expect(normalMatch).not.toBeNull()
    expect(expressiveMatch).not.toBeNull()
    expect(Number(normalMatch![1])).toBeLessThan(Number(expressiveMatch![1]))
  })
})

// ─── Semantic color variables ─────────────────────────────────────────────────

describe('semantic color CSS variables', () => {
  const semanticColors = ['--color-success', '--color-success-light', '--color-warning', '--color-warning-light', '--color-danger', '--color-danger-light', '--color-info', '--color-info-light']

  it.each(semanticColors)('%s is defined in :root', (varName) => {
    expect(indexHtml).toContain(varName)
  })

  it.each(semanticColors)('%s is defined in .dark', (varName) => {
    // Extract .dark block
    const darkBlock = indexHtml.match(/\.dark\s*\{[^}]+\}/s)?.[0] || ''
    expect(darkBlock).toContain(varName)
  })
})

// ─── Animation utility classes ────────────────────────────────────────────────

describe('animation utility classes in style.css', () => {
  it('defines transition-micro class', () => {
    expect(styleCss).toContain('.transition-micro')
    expect(styleCss).toContain('var(--duration-micro)')
    expect(styleCss).toContain('var(--ease-micro)')
  })

  it('defines transition-normal class', () => {
    expect(styleCss).toContain('.transition-normal')
    expect(styleCss).toContain('var(--duration-normal)')
    expect(styleCss).toContain('var(--ease-normal)')
  })

  it('defines transition-expressive class', () => {
    expect(styleCss).toContain('.transition-expressive')
    expect(styleCss).toContain('var(--duration-expressive)')
    expect(styleCss).toContain('var(--ease-expressive)')
  })

  it('defines anim-pop-in class with keyframes', () => {
    expect(styleCss).toContain('.anim-pop-in')
    expect(styleCss).toContain('@keyframes popIn')
  })

  it('defines anim-slide-up class with keyframes', () => {
    expect(styleCss).toContain('.anim-slide-up')
    expect(styleCss).toContain('@keyframes slideUp')
  })

  it('defines focus-ring class', () => {
    expect(styleCss).toContain('.focus-ring:focus-visible')
  })
})

// ─── Type scale utility classes ───────────────────────────────────────────────

describe('type scale utility classes in style.css', () => {
  const tsClasses = ['.ts-xs', '.ts-sm', '.ts-base', '.ts-lg', '.ts-xl']

  it.each(tsClasses)('%s is defined', (cls) => {
    expect(styleCss).toContain(cls)
  })
})

// ─── No non-standard font sizes remain ────────────────────────────────────────

describe('font size standardization', () => {
  const appVue = readFileSync(resolve(__dirname, '../src/App.vue'), 'utf-8')
  const allVueFiles = ['App.vue', 'components/CommandPalette.vue', 'components/Editor.vue',
    'components/SettingsPanel.vue', 'components/TabBar.vue', 'components/UnlockModal.vue',
    'components/ResetModal.vue', 'components/SearchResults.vue', 'components/HistoryPanel.vue',
    'components/ImageView.vue'].map(f => readFileSync(resolve(__dirname, `../src/${f}`), 'utf-8'))
  const allVue = allVueFiles.join('\n')

  it('no text-[9px] anywhere', () => { expect(allVue).not.toContain('text-[9px]') })
  it('no text-[10px] anywhere', () => { expect(allVue).not.toContain('text-[10px]') })
  it('no text-[12px] anywhere', () => { expect(allVue).not.toContain('text-[12px]') })
  it('no text-[11px] (should use ts-xs)', () => { expect(allVue).not.toContain('text-[11px]') })
  it('no text-[13px] (should use ts-sm)', () => { expect(allVue).not.toContain('text-[13px]') })
  it('no text-[14px] (should use ts-base)', () => { expect(allVue).not.toContain('text-[14px]') })

  it('ts-xs is actually used in components', () => { expect(allVue).toContain('ts-xs') })
  it('ts-sm is actually used in components', () => { expect(allVue).toContain('ts-sm') })
})

// ─── Core surface colors exist ────────────────────────────────────────────────

describe('surface color CSS variables', () => {
  const surfaceVars = ['--bg-app', '--bg-sidebar', '--bg-editor', '--bg-hover', '--bg-active', '--accent', '--accent-light', '--text-primary', '--text-secondary', '--text-muted', '--border', '--border-strong']

  it.each(surfaceVars)('%s is defined in :root', (varName) => {
    expect(indexHtml).toContain(varName)
  })
})
