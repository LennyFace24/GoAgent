const { marked } = require('marked');
const katex = require('katex');

function renderLatex(tex, displayMode) {
  try {
    return katex.renderToString(tex, { displayMode, throwOnError: false, trust: true });
  } catch {
    return displayMode
      ? `<pre class="latex-error">${tex}</pre>`
      : `<code class="latex-error">${tex}</code>`;
  }
}

function splitSegments(html) {
  const tagRe = /<(code|pre|script|style)[\s>][\s\S]*?<\/\1>|<(code|pre|script|style)[\s>][\s\S]*?\/>/gi;
  const segments = [];
  let lastIndex = 0;
  let m;
  while ((m = tagRe.exec(html)) !== null) {
    if (m.index > lastIndex) {
      segments.push({ type: 'text', content: html.slice(lastIndex, m.index) });
    }
    segments.push({ type: 'skip', content: m[0] });
    lastIndex = m.index + m[0].length;
  }
  if (lastIndex < html.length) {
    segments.push({ type: 'text', content: html.slice(lastIndex) });
  }
  return segments;
}

function processTextSegments(html) {
  html = html.replace(/\$\$([\s\S]+?)\$\$/g, (_, tex) => renderLatex(tex.trim(), true));
  html = html.replace(/(?<!\$)\$(?!\$)((?:[^$\n\\]|\\.)+?)\$(?!\$)/g, (_, tex) => renderLatex(tex.trim(), false));
  return html;
}

const latexExtension = {
  hooks: {
    postprocess(html) {
      const segments = splitSegments(html);
      return segments.map(seg => {
        if (seg.type === 'skip') return seg.content;
        return processTextSegments(seg.content);
      }).join('');
    },
  },
};

marked.use(latexExtension);

// Tests
console.log('=== Test 1: Inline math ===');
const r1 = marked.parse('hello $E=mc^2$ world');
console.log(r1);
console.log('Has katex:', r1.includes('katex'));

console.log('\n=== Test 2: Block math ===');
const r2 = marked.parse('before\n\n$$\\int_0^1 x^2 dx$$\n\nafter');
console.log(r2);
console.log('Has katex:', r2.includes('katex'));

console.log('\n=== Test 3: Math in code block (should NOT render) ===');
const r3 = marked.parse('`$E=mc^2$`');
console.log(r3);
console.log('Has katex:', r3.includes('katex'));

console.log('\n=== Test 4: No math ===');
const r4 = marked.parse('hello world no math');
console.log(r4);
console.log('Has katex:', r4.includes('katex'));
