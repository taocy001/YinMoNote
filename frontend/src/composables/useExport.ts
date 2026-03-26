/**
 * Note export functionality.
 * Provides HTML, PDF, and Markdown export for a given Tiptap editor instance.
 */
import type { Editor as TiptapEditor } from '@tiptap/core'
import type { Ref } from 'vue'

// File System Access API — available in Chromium browsers, absent in Firefox/Safari.
declare global {
  interface Window {
    showSaveFilePicker?: (options?: {
      suggestedName?: string
      types?: Array<{ description: string; accept: Record<string, string[]> }>
    }) => Promise<FileSystemFileHandle>
  }
}

/**
 * Sanitizes HTML for export, removing potentially dangerous elements and attributes.
 * SEC-011: svg and style are blocked to prevent SVG-based XSS and CSS url() exfiltration.
 */
export function sanitizeExportHtml(html: string): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  const BLOCKED_TAGS = ['script', 'iframe', 'frame', 'frameset', 'form', 'object', 'embed', 'base', 'meta', 'link', 'svg', 'style']
  const BLOCKED_PROTO = /^(javascript|data|vbscript):/i
  BLOCKED_TAGS.forEach(tag => doc.querySelectorAll(tag).forEach(el => el.remove()))
  doc.querySelectorAll('*').forEach(el => {
    Array.from(el.attributes).forEach(attr => {
      if (/^on/i.test(attr.name)) el.removeAttribute(attr.name)
      if (attr.name === 'style' && /url\s*\(/i.test(attr.value)) el.removeAttribute(attr.name)
    })
    for (const attr of ['href', 'src', 'action', 'formaction', 'data']) {
      const val = el.getAttribute(attr)
      if (val && BLOCKED_PROTO.test(val.trim())) el.removeAttribute(attr)
    }
  })
  return doc.body.innerHTML
}

export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

/**
 * Save a Blob via the File System Access API (shows a native "Save As" dialog).
 * Falls back to the classic <a download> approach on browsers that do not
 * support showSaveFilePicker (e.g. Firefox, Safari).
 */
async function saveWithPicker(blob: Blob, suggestedName: string, description: string, mimeType: string, ext: string): Promise<void> {
  if (typeof window.showSaveFilePicker === 'function') {
    try {
      const handle = await window.showSaveFilePicker({
        suggestedName,
        types: [{ description, accept: { [mimeType]: [ext] } }],
      })
      const writable = await handle.createWritable()
      await writable.write(blob)
      await writable.close()
      return
    } catch (e: any) {
      if (e?.name === 'AbortError') return // user cancelled
      // API rejected for another reason — fall through to legacy download
    }
  }
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a'); a.href = url; a.download = suggestedName
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export function useExport(editorRef: Ref<TiptapEditor | null | undefined>, getFullContent: () => string) {
  const exportHTML = async () => {
    if (!editorRef.value) return
    const html = sanitizeExportHtml(editorRef.value.getHTML())
    const title = editorRef.value.state.doc.firstChild?.textContent?.trim() || 'note'
    const safeTitle = escapeHtml(title)
    const doc = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>${safeTitle}</title>
<style>
  body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 800px; margin: 40px auto; padding: 0 24px; line-height: 1.8; color: #1C1B22; }
  h1 { font-size: 2.2em; font-weight: 800; letter-spacing: -0.03em; border-bottom: 2px solid #5E6AD2; padding-bottom: 0.3em; margin-bottom: 1em; }
  h2 { font-size: 1.5em; font-weight: 700; margin-top: 1.5em; }
  h3 { font-size: 1.2em; font-weight: 600; }
  pre { background: #F7F6F3; border: 1px solid #E5E2DB; border-radius: 10px; padding: 1em; overflow-x: auto; }
  code { background: #F7F6F3; padding: 0.15em 0.4em; border-radius: 4px; font-family: monospace; font-size: 0.875em; }
  blockquote { border-left: 3px solid #5E6AD2; padding: 0.75em 1em; background: #EEF0FF; border-radius: 0 8px 8px 0; color: #6B6A78; font-style: italic; }
  table { border-collapse: collapse; width: 100%; margin: 1em 0; }
  td, th { border: 1px solid #E5E2DB; padding: 8px 12px; }
  th { background: #EFEDE8; font-weight: 600; }
  img { max-width: 100%; border-radius: 10px; }
  a { color: #5E6AD2; }
</style>
</head>
<body>${html}</body>
</html>`
    const filename = title.replace(/[^a-z0-9\u4e00-\u9fff\-_]/gi, '_') + '.html'
    await saveWithPicker(new Blob([doc], { type: 'text/html' }), filename, 'HTML', 'text/html', '.html')
  }

  const exportPDF = () => {
    if (!editorRef.value) return
    const html = sanitizeExportHtml(editorRef.value.getHTML())
    const title = editorRef.value.state.doc.firstChild?.textContent?.trim() || 'note'
    const safeTitle = escapeHtml(title)
    // @page margin:0 eliminates the browser's default header/footer area
    // (which prints the URL and page number). Body padding provides the
    // actual content margins so text doesn't touch the paper edge.
    const doc = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>${safeTitle}</title>
<style>
  @page { margin: 0; }
  body { font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 800px; margin: 0 auto; padding: 15mm 20mm; line-height: 1.8; color: #1C1B22; }
  h1 { font-size: 2em; font-weight: 800; border-bottom: 2px solid #5E6AD2; padding-bottom: 0.3em; }
  h2 { font-size: 1.5em; font-weight: 700; margin-top: 1.5em; }
  pre { background: #F7F6F3; border: 1px solid #E5E2DB; border-radius: 10px; padding: 1em; }
  code { background: #F7F6F3; padding: 0.2em 0.4em; border-radius: 3px; font-size: 0.9em; }
  blockquote { border-left: 3px solid #5E6AD2; padding-left: 1em; color: #6B6A78; }
  table { border-collapse: collapse; width: 100%; }
  td, th { border: 1px solid #E5E2DB; padding: 8px 12px; }
  th { background: #EFEDE8; font-weight: 600; }
  img { max-width: 100%; }
</style>
</head>
<body>${html}</body>
</html>`
    const iframe = document.createElement('iframe')
    iframe.style.cssText = 'position:fixed;top:-9999px;left:-9999px;width:1px;height:1px;border:0'
    const cleanup = () => {
      if (document.body.contains(iframe)) document.body.removeChild(iframe)
    }
    // Set srcdoc BEFORE appending so the browser loads the actual content
    // directly, avoiding a spurious about:blank onload that would fire
    // print() on an empty document and never trigger a second load event.
    iframe.srcdoc = doc
    iframe.addEventListener('load', () => {
      // The print dialog derives the default PDF filename from document.title.
      // With srcdoc iframes the browser uses the PARENT page's title, so we
      // temporarily swap it to the note title and restore it after printing.
      const prevTitle = document.title
      document.title = title
      iframe.contentWindow?.focus()
      iframe.contentWindow?.print()
      const restore = () => { document.title = prevTitle; cleanup() }
      iframe.contentWindow?.addEventListener('afterprint', restore, { once: true })
      setTimeout(restore, 60_000)
    }, { once: true })
    document.body.appendChild(iframe)
  }

  const exportMarkdown = async () => {
    const md = getFullContent()
    const title = editorRef.value?.state.doc.firstChild?.textContent?.trim() || 'note'
    const filename = title.replace(/[^a-z0-9\u4e00-\u9fff\-_]/gi, '_') + '.md'
    await saveWithPicker(new Blob([md], { type: 'text/markdown; charset=utf-8' }), filename, 'Markdown', 'text/markdown', '.md')
  }

  return { exportHTML, exportPDF, exportMarkdown }
}
