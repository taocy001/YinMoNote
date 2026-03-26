import { Node, InputRule, mergeAttributes } from '@tiptap/core'
import { VueNodeViewRenderer } from '@tiptap/vue-3'
import InlineMathView from '../components/InlineMathView.vue'

/**
 * InlineMath — renders $formula$ inline using KaTeX.
 * Typing $...$ and completing with a second $ triggers conversion to this node.
 * Markdown serialization outputs $formula$ so notes remain portable.
 */
export const InlineMath = Node.create({
  name: 'inlineMath',
  inline: true,
  group: 'inline',
  atom: true,

  addAttributes() {
    return {
      formula: { default: '' },
    }
  },

  parseHTML() {
    return [
      {
        tag: 'span[data-math-inline]',
        getAttrs: el => ({ formula: (el as HTMLElement).getAttribute('data-math-inline') ?? '' }),
      },
    ]
  },

  renderHTML({ node, HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes, { 'data-math-inline': node.attrs.formula })]
  },

  addNodeView() {
    return VueNodeViewRenderer(InlineMathView)
  },

  addInputRules() {
    const typeName = this.name
    return [
      new InputRule({
        find: /\$([^$\n]+)\$$/,
        handler: ({ range, match, commands }) => {
          commands.deleteRange(range)
          commands.insertContent({ type: typeName, attrs: { formula: match[1] } })
        },
      }),
    ]
  },

  addStorage() {
    return {
      markdown: {
        serialize(state: any, node: any) {
          state.write(`$${node.attrs.formula}$`)
        },
      },
    }
  },
})
