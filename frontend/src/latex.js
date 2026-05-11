import katex from 'katex'

export const latexExtension = {
  name: 'latex',
  level: 'inline',
  start(src) {
    return src.match(/\$|\\\(/)?.index
  },
  tokenizer(src) {
    // block math: $$...$$
    const blockMatch = src.match(/^\$\$([\s\S]+?)\$\$/)
    if (blockMatch) {
      return {
        type: 'latex',
        raw: blockMatch[0],
        text: blockMatch[1].trim(),
        display: true,
      }
    }
    // inline math: $...$  (not greedy, no newlines)
    const inlineMatch = src.match(/^\$([^\$\n]+?)\$/)
    if (inlineMatch) {
      return {
        type: 'latex',
        raw: inlineMatch[0],
        text: inlineMatch[1].trim(),
        display: false,
      }
    }
  },
  renderer(token) {
    try {
      return katex.renderToString(token.text, {
        displayMode: token.display,
        throwOnError: false,
        trust: true,
      })
    } catch {
      return token.display
        ? `<pre class="latex-error">${token.text}</pre>`
        : `<code class="latex-error">${token.text}</code>`
    }
  },
}
