import katex from 'katex'

function renderLatex(tex, displayMode) {
  try {
    return katex.renderToString(tex, {
      displayMode,
      throwOnError: false,
      trust: true,
    })
  } catch {
    const escaped = tex.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    return displayMode
      ? `<pre class="latex-error">${escaped}</pre>`
      : `<code class="latex-error">${escaped}</code>`
  }
}

/**
 * Split HTML into segments: "html" (skip) vs "text" (process for LaTeX).
 * We skip anything inside <code>, <pre>, and <script>/<style> tags.
 */
function splitSegments(html) {
  // Match opening/closing tags we want to skip
  const tagRe = /<(code|pre|script|style)[\s>][\s\S]*?<\/\1>|<(code|pre|script|style)[\s>][\s\S]*?\/>/gi
  const segments = []
  let lastIndex = 0
  let m

  while ((m = tagRe.exec(html)) !== null) {
    if (m.index > lastIndex) {
      segments.push({ type: 'text', content: html.slice(lastIndex, m.index) })
    }
    segments.push({ type: 'skip', content: m[0] })
    lastIndex = m.index + m[0].length
  }
  if (lastIndex < html.length) {
    segments.push({ type: 'text', content: html.slice(lastIndex) })
  }
  return segments
}

function processTextSegments(html) {
  // 1. Block math: $$...$$ (greedy across newlines)
  html = html.replace(/\x24\x24([\s\S]+?)\x24\x24/g, (_, tex) => renderLatex(tex.trim(), true))
  // 2. Inline math: $...$ (no newlines, non-greedy)
  html = html.replace(/(?<!\x24)\x24(?!\x24)((?:[^\x24\n\\]|\\.)+?)\x24(?!\x24)/g, (_, tex) => renderLatex(tex.trim(), false))
  return html
}

export const latexExtension = {
  hooks: {
    postprocess(html) {
      const segments = splitSegments(html)
      return segments.map(seg => {
        if (seg.type === 'skip') return seg.content
        return processTextSegments(seg.content)
      }).join('')
    },
  },
}
