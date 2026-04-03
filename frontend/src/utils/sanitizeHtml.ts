/**
 * Whitelist-based HTML sanitization for pasted content.
 *
 * Only tags in ALLOWED_TAGS and attributes in ALLOWED_ATTRS are preserved.
 * All other elements are removed. href/src attributes with dangerous
 * protocols (javascript:, data:, vbscript:) are stripped.
 *
 * Uses a whitelist approach for stronger XSS prevention than tag blacklisting.
 */

const ALLOWED_TAGS = new Set([
  'p', 'span', 'strong', 'em', 'b', 'i', 'u', 's', 'mark', 'code', 'sub', 'sup', 'a', 'br', 'wbr',
  'h1', 'h2', 'h3', 'h4', 'h5', 'h6', 'blockquote', 'ol', 'ul', 'li', 'pre',
  'table', 'thead', 'tbody', 'tr', 'td', 'th', 'hr', 'div', 'img', 'figure', 'figcaption',
  'dl', 'dt', 'dd', 'details', 'summary', 'abbr', 'del', 'ins', 'small', 'label',
])

const ALLOWED_ATTRS = new Set([
  'href', 'src', 'alt', 'title', 'class', 'id',
  'colspan', 'rowspan', 'width', 'height', 'target', 'rel',
  // Table column-width: format is comma-separated integers, no XSS risk.
  // Required so that copy-pasting a table within the app preserves column widths.
  'colwidth',
  // Cell background colour set by the table background-colour feature.
  // Stored as a named/hex CSS colour string; no XSS risk.
  'data-bg-color',
])

export function sanitizePastedHtml(html: string): string {
  const template = document.createElement('template')
  template.innerHTML = html
  const walker = document.createTreeWalker(template.content, NodeFilter.SHOW_ELEMENT)
  const toRemove: Element[] = []
  let node = walker.nextNode()
  while (node) {
    const el = node as Element
    const tag = el.tagName.toLowerCase()
    if (!ALLOWED_TAGS.has(tag)) {
      toRemove.push(el)
    } else {
      for (const attr of Array.from(el.attributes)) {
        // Strip all whitespace/control chars before protocol check to defeat
        // HTML entity encoding bypasses like java&#x09;script: or java\nscript:
        // eslint-disable-next-line no-control-regex
        const val = attr.value.replace(/[\s\x00-\x1f]/g, '')
        if (!ALLOWED_ATTRS.has(attr.name) ||
            (['href', 'src'].includes(attr.name) &&
             /^(javascript|data|vbscript):/i.test(val))) {
          el.removeAttribute(attr.name)
        }
      }
    }
    node = walker.nextNode()
  }
  toRemove.forEach(el => el.remove())
  const div = document.createElement('div')
  div.appendChild(template.content.cloneNode(true))
  return div.innerHTML
}
