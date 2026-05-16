import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { latexExtension } from '../../latex'

marked.use(latexExtension)

export function useMarkdown() {
  function render(text: string): string {
    const raw = marked.parse(text, { breaks: true, gfm: true }) as string
    return DOMPurify.sanitize(raw, {
      ADD_TAGS: ['math', 'semantics', 'mrow', 'mi', 'mo', 'mn', 'msup', 'msub', 'mfrac',
        'msqrt', 'mroot', 'mstyle', 'munder', 'mover', 'munderover', 'mspace',
        'mpadded', 'menclose', 'annotation', 'annotation-xml',
        'mglyph', 'maligngroup', 'malignmark', 'mtable', 'mtr', 'mtd', 'mlabeledtr'],
      ADD_ATTR: ['mathvariant', 'mathsize', 'mathcolor', 'mathbackground',
        'scriptsizemultiplier', 'scriptminsize', 'accent', 'accentunder',
        'form', 'fence', 'separator', 'stretchy', 'symmetric', 'largeop',
        'movablelimits', 'data-latex'],
    })
  }

  return { render }
}
