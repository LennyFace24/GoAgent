const { marked } = require('marked');
const katex = require('katex');

const latexExtension = {
  name: 'latex',
  level: 'inline',
  start(src) {
    return src.match(/\$/)?.index;
  },
  tokenizer(src) {
    const blockMatch = src.match(/^\$\$([\s\S]+?)\$\$/);
    if (blockMatch) {
      return {
        type: 'latex',
        raw: blockMatch[0],
        text: blockMatch[1].trim(),
        display: true,
      };
    }
    const inlineMatch = src.match(/^\$([^\$\n]+?)\$/);
    if (inlineMatch) {
      return {
        type: 'latex',
        raw: inlineMatch[0],
        text: inlineMatch[1].trim(),
        display: false,
      };
    }
  },
  renderer(token) {
    try {
      return katex.renderToString(token.text, {
        displayMode: token.display,
        throwOnError: false,
        trust: true,
      });
    } catch {
      return `<code class="latex-error">${token.text}</code>`;
    }
  },
};

marked.use(latexExtension);

const test1 = 'hello $E=mc^2$ world';
const test2 = 'a $$\\int_0^1 x^2 dx$$ b';
const test3 = 'no math here';

console.log('Test 1 (inline):', marked.parse(test1));
console.log('Test 2 (block):', marked.parse(test2));
console.log('Test 3 (none):', marked.parse(test3));
