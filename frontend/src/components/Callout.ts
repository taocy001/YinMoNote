import { Node, mergeAttributes } from '@tiptap/core'
import { VueNodeViewRenderer } from '@tiptap/vue-3'
import CalloutView from './CalloutView.vue'

export type CalloutType = 'info' | 'warning' | 'tip' | 'danger'

/**
 * Default emoji for each callout type. Used when the emoji attr is empty.
 */
export const CALLOUT_DEFAULTS: Record<CalloutType, { emoji: string; label: string }> = {
  info:    { emoji: '💡', label: 'Info'    },
  warning: { emoji: '⚠️', label: 'Warning' },
  tip:     { emoji: '✅', label: 'Tip'     },
  danger:  { emoji: '🚨', label: 'Danger'  },
}

/**
 * Callout block — a coloured container with an optional emoji label.
 *
 * Stored in markdown as an HTML div so it survives round-trips:
 *   <div data-callout="info" data-emoji="💡">
 *
 *   Content here.
 *
 *   </div>
 *
 * The blank lines inside the div tell markdown parsers (CommonMark / markdown-it
 * with html:true) to treat the children as block elements.
 */
export const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,

  addAttributes() {
    return {
      type:  { default: 'info' as CalloutType },
      emoji: { default: '' },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'div[data-callout]',
        getAttrs: (el: HTMLElement) => ({
          type:  (el.getAttribute('data-callout') as CalloutType) || 'info',
          emoji: el.getAttribute('data-emoji') || '',
        }),
      },
    ]
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      'div',
      mergeAttributes(HTMLAttributes, {
        'data-callout': node.attrs.type,
        'data-emoji':   node.attrs.emoji || '',
      }),
      0,
    ]
  },

  addNodeView() {
    return VueNodeViewRenderer(CalloutView)
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          const type  = (node.attrs.type  as string) || 'info'
          const emoji = (node.attrs.emoji as string) || ''
          // HTML-escape the emoji attribute so a user-entered `"` character
          // cannot break the serialized attribute syntax on round-trip.
          const safeEmoji = emoji.replace(/&/g, '&amp;').replace(/"/g, '&quot;')
          const emojiAttr = safeEmoji ? ` data-emoji="${safeEmoji}"` : ''
          state.write(`<div data-callout="${type}"${emojiAttr}>\n\n`)
          state.renderContent(node)
          state.ensureNewLine()
          state.write('\n</div>')
          state.closeBlock(node)
        },
      },
    }
  },
})
