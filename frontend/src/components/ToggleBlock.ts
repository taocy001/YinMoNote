import { Node, mergeAttributes } from '@tiptap/core'
import { VueNodeViewRenderer } from '@tiptap/vue-3'
import ToggleBlockView from './ToggleBlockView.vue'

/**
 * ToggleBlock — a collapsible block with an editable title and inner content.
 *
 * Stored in markdown as an HTML details/summary element:
 *   <details open>
 *   <summary>Toggle title</summary>
 *
 *   Content here.
 *
 *   </details>
 *
 * Round-trip: parseHTML extracts the summary text as the title attribute and
 * strips the <summary> element before parsing the remaining children as blocks.
 */
export const ToggleBlock = Node.create({
  name: 'toggleBlock',
  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      open:  { default: true },
      title: { default: 'Toggle' },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'details',
        getAttrs: (el: HTMLElement) => ({
          open:  el.hasAttribute('open'),
          title: el.querySelector('summary')?.textContent?.trim() || 'Toggle',
        }),
        // Strip <summary> so ProseMirror only parses the block children as content
        contentElement(el: HTMLElement) {
          const div = el.ownerDocument.createElement('div')
          el.childNodes.forEach(child => {
            if ((child as HTMLElement).tagName?.toLowerCase() !== 'summary') {
              div.appendChild(child.cloneNode(true))
            }
          })
          return div
        },
      },
    ]
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      'details',
      mergeAttributes(HTMLAttributes, {
        open: node.attrs.open ? '' : undefined,
      }),
      ['summary', node.attrs.title as string],
      0,
    ]
  },

  addNodeView() {
    return VueNodeViewRenderer(ToggleBlockView)
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const title  = (node.attrs.title as string) || 'Toggle'
          const isOpen = node.attrs.open !== false
          // Escape any HTML special chars in the title
          const safeTitle = title.replace(/</g, '&lt;').replace(/>/g, '&gt;')
          state.write(`<details${isOpen ? ' open' : ''}>\n<summary>${safeTitle}</summary>\n\n`)
          state.renderContent(node)
          state.ensureNewLine()
          state.write('\n</details>')
          state.closeBlock(node)
        },
      },
    }
  },
})
