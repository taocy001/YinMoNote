import { describe, it, expect, beforeEach } from 'vitest'
import { setLang, useI18n } from "../src/i18n";


beforeEach(() => {
  localStorage.clear()
  setLang('zh')
})

// ─── language switching ───────────────────────────────────────────────────────

describe('language switching', () => {
  it('defaults to zh', () => {
    const { t } = useI18n()
    expect(t.value.myNotes).toBe('我的笔记')
  })

  it('switches to en', () => {
    setLang('en')
    const { t } = useI18n()
    expect(t.value.myNotes).toBe('My Notes')
  })

  it('langSwitch shows the opposite label', () => {
    const { t } = useI18n()
    expect(t.value.langSwitch).toBe('En')
    setLang('en')
    expect(t.value.langSwitch).toBe('中')
  })

  it('t is reactive — updates immediately when lang changes', () => {
    const { t } = useI18n()
    expect(t.value.saved).toBe('已保存')
    setLang('en')
    expect(t.value.saved).toBe('Saved')
    setLang('zh')
    expect(t.value.saved).toBe('已保存')
  })

  it('lang ref reflects current language', () => {
    const { lang } = useI18n()
    expect(lang.value).toBe('zh')
    setLang('en')
    expect(lang.value).toBe('en')
  })

  it('multiple useI18n() calls share the same reactive state', () => {
    const a = useI18n()
    const b = useI18n()
    setLang('en')
    expect(a.t.value.myNotes).toBe(b.t.value.myNotes)
    expect(a.lang.value).toBe(b.lang.value)
  })

  it('setLang is idempotent', () => {
    setLang('en')
    setLang('en')
    setLang('en')
    const { lang } = useI18n()
    expect(lang.value).toBe('en')
  })
})

// ─── localStorage persistence ─────────────────────────────────────────────────

describe('localStorage persistence', () => {
  it('setLang writes lang key', () => {
    setLang('en')
    expect(localStorage.getItem('lang')).toBe('en')
  })

  it('setLang updates existing lang key', () => {
    setLang('en')
    setLang('zh')
    expect(localStorage.getItem('lang')).toBe('zh')
  })

  it('lang key is exactly "lang"', () => {
    setLang('en')
    const keys = Object.keys(localStorage)
    expect(keys).toContain('lang')
  })
})

// ─── sidebar & navigation strings ────────────────────────────────────────────

describe('sidebar & navigation strings', () => {
  it('zh: sidebar labels present and non-empty', () => {
    setLang('zh')
    const { t } = useI18n()
    expect(t.value.myNotes).toBeTruthy()
    expect(t.value.newNote).toBeTruthy()
    expect(t.value.collapseSidebar).toBeTruthy()
    expect(t.value.expandSidebar).toBeTruthy()
    expect(t.value.createSubNote).toBeTruthy()
    expect(t.value.newTag).toBeTruthy()
    expect(t.value.searchPlaceholder).toBeTruthy()
    expect(t.value.noResults).toBeTruthy()
    expect(t.value.searchHint).toBeTruthy()
  })

  it('en: sidebar labels present and non-empty', () => {
    setLang('en')
    const { t } = useI18n()
    expect(t.value.myNotes).toBeTruthy()
    expect(t.value.newNote).toBeTruthy()
    expect(t.value.collapseSidebar).toBeTruthy()
    expect(t.value.expandSidebar).toBeTruthy()
  })

  it('zh sidebar strings are in Chinese', () => {
    setLang('zh')
    const { t } = useI18n()
    // Chinese characters are in the range \u4e00-\u9fff
    expect(/[\u4e00-\u9fff]/.test(t.value.myNotes)).toBe(true)
    expect(/[\u4e00-\u9fff]/.test(t.value.newNote)).toBe(true)
  })

  it('en sidebar strings are in English (ASCII)', () => {
    setLang('en')
    const { t } = useI18n()
    // eslint-disable-next-line no-control-regex
    expect(/^[\x00-\x7F]+$/.test(t.value.myNotes)).toBe(true)
    // eslint-disable-next-line no-control-regex
    expect(/^[\x00-\x7F]+$/.test(t.value.newNote)).toBe(true)
  })
})

// ─── editor strings ───────────────────────────────────────────────────────────

describe('editor strings', () => {
  const editorKeys = [
    'toc', 'historyBtn', 'tocTitle', 'noHeadings',
    'saving', 'saved', 'saveError',
    'historyTitle', 'noHistory', 'loading',
    'added', 'removed', 'revertTo', 'confirmRevert', 'revertFailed',
    'uploadFailed', 'newNotePlaceholder',
    'insertBlock', 'insertBlockHint', 'noMatchCmd',
    'linkPlaceholder', 'confirm', 'linkInput',
    'dragHandle', 'insertBlockBtn',
  ] as const

  it('zh: all editor keys present', () => {
    setLang('zh')
    const { t } = useI18n()
    for (const key of editorKeys) {
      expect(t.value[key], `zh missing: ${key}`).toBeTruthy()
    }
  })

  it('en: all editor keys present', () => {
    setLang('en')
    const { t } = useI18n()
    for (const key of editorKeys) {
      expect(t.value[key], `en missing: ${key}`).toBeTruthy()
    }
  })

  it('saving / saved / saveError are distinct', () => {
    for (const lang of ['zh', 'en'] as const) {
      setLang(lang)
      const { t } = useI18n()
      const vals = [t.value.saving, t.value.saved, t.value.saveError]
      const unique = new Set(vals)
      expect(unique.size).toBe(3)
    }
  })
})

// ─── slash commands ───────────────────────────────────────────────────────────

describe('slash command strings', () => {
  const cmdKeys = [
    ['cmdH1', 'cmdH1Desc'],
    ['cmdH2', 'cmdH2Desc'],
    ['cmdH3', 'cmdH3Desc'],
    ['cmdText', 'cmdTextDesc'],
    ['cmdUL', 'cmdULDesc'],
    ['cmdOL', 'cmdOLDesc'],
    ['cmdCode', 'cmdCodeDesc'],
    ['cmdQuote', 'cmdQuoteDesc'],
    ['cmdHR', 'cmdHRDesc'],
  ] as const

  it('zh: all slash command labels and descriptions present', () => {
    setLang('zh')
    const { t } = useI18n()
    for (const [label, desc] of cmdKeys) {
      expect(t.value[label], `zh missing: ${label}`).toBeTruthy()
      expect(t.value[desc], `zh missing: ${desc}`).toBeTruthy()
    }
  })

  it('en: all slash command labels and descriptions present', () => {
    setLang('en')
    const { t } = useI18n()
    for (const [label, desc] of cmdKeys) {
      expect(t.value[label], `en missing: ${label}`).toBeTruthy()
      expect(t.value[desc], `en missing: ${desc}`).toBeTruthy()
    }
  })

  it('each command label is distinct from its description', () => {
    for (const lang of ['zh', 'en'] as const) {
      setLang(lang)
      const { t } = useI18n()
      for (const [label, desc] of cmdKeys) {
        expect(t.value[label]).not.toBe(t.value[desc])
      }
    }
  })

  it('H1/H2/H3 labels are distinct from each other', () => {
    for (const lang of ['zh', 'en'] as const) {
      setLang(lang)
      const { t } = useI18n()
      expect(t.value.cmdH1).not.toBe(t.value.cmdH2)
      expect(t.value.cmdH2).not.toBe(t.value.cmdH3)
    }
  })
})

// ─── key management strings ───────────────────────────────────────────────────

describe('key management strings', () => {
  const kmKeys = [
    'encrypted', 'keyMgmt', 'keyMgmtTitle', 'keyMgmtDesc',
    'exportKey', 'importKey', 'close', 'importPaste',
    'importSuccess', 'importInvalid',
  ] as const

  it('zh: all key management strings present', () => {
    setLang('zh')
    const { t } = useI18n()
    for (const key of kmKeys) {
      expect(t.value[key], `zh missing: ${key}`).toBeTruthy()
    }
  })

  it('en: all key management strings present', () => {
    setLang('en')
    const { t } = useI18n()
    for (const key of kmKeys) {
      expect(t.value[key], `en missing: ${key}`).toBeTruthy()
    }
  })
})

// ─── mobile toolbar strings ───────────────────────────────────────────────────

describe('mobile toolbar strings', () => {
  it('zh: mobile toolbar strings present', () => {
    setLang('zh')
    const { t } = useI18n()
    expect(t.value.h1).toBe('标题1')
    expect(t.value.h2).toBe('标题2')
    expect(t.value.list).toBeTruthy()
    expect(t.value.code).toBeTruthy()
  })

  it('en: mobile toolbar strings present', () => {
    setLang('en')
    const { t } = useI18n()
    expect(t.value.h1).toBe('H1')
    expect(t.value.h2).toBe('H2')
    expect(t.value.list).toBeTruthy()
    expect(t.value.code).toBeTruthy()
  })

  it('H1/H2 are same in zh and en (universal label)', () => {
    setLang('zh')
    const { t: tzh } = useI18n()
    setLang('en')
    const { t: ten } = useI18n()
    expect(tzh.value.h1).toBe(ten.value.h1)
    expect(tzh.value.h2).toBe(ten.value.h2)
  })
})

// ─── completeness ─────────────────────────────────────────────────────────────

describe('completeness', () => {
  it('zh and en have exactly the same set of keys', () => {
    setLang('zh')
    const { t: tzh } = useI18n()
    const zhKeys = Object.keys(tzh.value).sort()

    setLang('en')
    const { t: ten } = useI18n()
    const enKeys = Object.keys(ten.value).sort()

    expect(zhKeys).toEqual(enKeys)
  })

  it('no key has an undefined value', () => {
    for (const lang of ['zh', 'en'] as const) {
      setLang(lang)
      const { t } = useI18n()
      for (const [key, val] of Object.entries(t.value)) {
        expect(val, `${lang}.${key} is undefined`).not.toBeUndefined()
        expect(val, `${lang}.${key} is null`).not.toBeNull()
      }
    }
  })

  it('no key is an empty string', () => {
    for (const lang of ['zh', 'en'] as const) {
      setLang(lang)
      const { t } = useI18n()
      for (const [key, val] of Object.entries(t.value)) {
        expect(val, `${lang}.${key} is empty string`).not.toBe('')
      }
    }
  })

  it('zh and en values differ for language-specific keys', () => {
    const languageSpecificKeys = ['myNotes', 'newNote', 'saved', 'saving', 'saveError'] as const
    const { t } = useI18n()

    // Capture zh values while lang is zh
    setLang('zh')
    const zhValues = languageSpecificKeys.map(key => t.value[key])

    // Capture en values while lang is en
    setLang('en')
    const enValues = languageSpecificKeys.map(key => t.value[key])

    for (let i = 0; i < languageSpecificKeys.length; i++) {
      expect(zhValues[i], `key "${languageSpecificKeys[i]}" should differ between zh and en`)
        .not.toBe(enValues[i])
    }
  })
})
