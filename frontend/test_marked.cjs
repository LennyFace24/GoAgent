const { marked, Lexer } = require('marked');

// Check what methods the inline lexer has
const lex = new Lexer();
console.log('Lexer methods:', Object.getOwnPropertyNames(Object.getPrototypeOf(lex)));
console.log('Lexer inlineTokens:', typeof lex.inlineTokens);

// Check default inline rules
const rules = lex.rules;
console.log('Has inline rules:', !!rules.inline);
if (rules.inline) {
  console.log('Inline rule keys:', Object.keys(rules.inline));
}
