import { describe, it, expect } from 'vitest';
import { MarkdownPipe } from './markdown.pipe';

describe('MarkdownPipe', () => {
  const pipe = new MarkdownPipe();

  it('returns empty string for falsy input', () => {
    expect(pipe.transform('')).toBe('');
    expect(pipe.transform(null as unknown as string)).toBe('');
    expect(pipe.transform(undefined as unknown as string)).toBe('');
  });

  describe('headers', () => {
    it('renders h1', () => {
      expect(pipe.transform('# Title')).toBe('<h1>Title</h1>');
    });

    it('renders h2', () => {
      expect(pipe.transform('## Section')).toBe('<h2>Section</h2>');
    });

    it('renders h3', () => {
      expect(pipe.transform('### Subsection')).toBe('<h3>Subsection</h3>');
    });

    it('does not treat # mid-line as header', () => {
      expect(pipe.transform('not a # header')).toBe('<p>not a # header</p>');
    });
  });

  describe('lists', () => {
    it('renders unordered list with dashes', () => {
      const result = pipe.transform('- one\n- two\n- three');
      expect(result).toBe('<ul><li>one</li><li>two</li><li>three</li></ul>');
    });

    it('renders unordered list with asterisks', () => {
      const result = pipe.transform('* alpha\n* beta');
      expect(result).toBe('<ul><li>alpha</li><li>beta</li></ul>');
    });

    it('renders ordered list', () => {
      const result = pipe.transform('1. first\n2. second');
      expect(result).toBe('<ol><li>first</li><li>second</li></ol>');
    });

    it('closes list before paragraph', () => {
      const result = pipe.transform('- item\n\nParagraph');
      expect(result).toContain('</ul>');
      expect(result).toContain('<p>Paragraph</p>');
    });
  });

  describe('inline formatting', () => {
    it('renders bold', () => {
      expect(pipe.transform('**bold**')).toContain('<strong>bold</strong>');
    });

    it('renders italic', () => {
      expect(pipe.transform('*italic*')).toContain('<em>italic</em>');
    });

    it('renders bold+italic', () => {
      expect(pipe.transform('***both***')).toContain('<strong><em>both</em></strong>');
    });

    it('renders inline code', () => {
      expect(pipe.transform('use `code` here')).toContain('<code>code</code>');
    });
  });

  describe('HTML escaping', () => {
    it('escapes angle brackets', () => {
      const result = pipe.transform('<script>alert("xss")</script>');
      expect(result).not.toContain('<script>');
      expect(result).toContain('&lt;script&gt;');
    });

    it('escapes ampersands', () => {
      expect(pipe.transform('A & B')).toContain('A &amp; B');
    });
  });

  describe('horizontal rule', () => {
    it('renders hr from ---', () => {
      expect(pipe.transform('---')).toBe('<hr>');
    });
  });

  describe('paragraphs and breaks', () => {
    it('wraps plain text in p tags', () => {
      expect(pipe.transform('Hello world')).toBe('<p>Hello world</p>');
    });

    it('inserts br on blank lines', () => {
      const result = pipe.transform('Line 1\n\nLine 2');
      expect(result).toBe('<p>Line 1</p><br><p>Line 2</p>');
    });
  });

  describe('mixed content', () => {
    it('handles headers followed by lists', () => {
      const input = '## Tasks\n- Do this\n- Do that';
      const result = pipe.transform(input);
      expect(result).toBe('<h2>Tasks</h2><ul><li>Do this</li><li>Do that</li></ul>');
    });

    it('handles inline formatting in headers', () => {
      expect(pipe.transform('## **Bold** header')).toBe('<h2><strong>Bold</strong> header</h2>');
    });
  });

  // Send back pastes this HTML into Outlook and Teams, where the user may not
  // have looked at the Preview first (PR #133 review).
  describe('what the results use', () => {
    it('keeps a numbered list whose items are separated by blank lines as one list', () => {
      expect(pipe.transform('1. first\n\n2. second\n\n3. third'))
        .toBe('<ol><li>first</li><li>second</li><li>third</li></ol>');
    });

    it('keeps the numbering of a list a paragraph interrupts', () => {
      expect(pipe.transform('1. first\nA note\n2. second'))
        .toBe('<ol><li>first</li></ol><p>A note</p><ol start="2"><li>second</li></ol>');
    });

    it('keeps a bullet list separated by blank lines as one list', () => {
      expect(pipe.transform('- a\n\n- b')).toBe('<ul><li>a</li><li>b</li></ul>');
    });

    it('nests an indented bullet under the item above it', () => {
      expect(pipe.transform('- parent\n  - child\n\t- child 2\n- next'))
        .toBe('<ul><li>parent<ul><li>child</li><li>child 2</li></ul></li><li>next</li></ul>');
      expect(pipe.transform('1. step\n   - detail\n2. step'))
        .toBe('<ol><li>step<ul><li>detail</li></ul></li><li>step</li></ol>');
    });

    it('renders a fenced code block as preformatted text, untouched', () => {
      expect(pipe.transform('Run:\n```sh\nmake a * b <out>\n  **x**\n```\nDone'))
        .toBe('<p>Run:</p><pre><code>make a * b &lt;out&gt;\n  **x**</code></pre><p>Done</p>');
    });

    it('closes a code block the text never closes', () => {
      expect(pipe.transform('```\ncode')).toBe('<pre><code>code</code></pre>');
    });

    it('leaves an asterisk with spaces around it alone', () => {
      expect(pipe.transform('3 * 4 * 5')).toBe('<p>3 * 4 * 5</p>');
    });
  });
});
