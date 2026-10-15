import { Pipe, PipeTransform } from '@angular/core';

const BULLET = /^[-*+]\s+(.+)/;
const NUMBERED = /^(\d+)\.\s+(.+)/;
/** A bullet indented under the item above it: one level of nesting. */
const SUB_BULLET = /^(?: {2,}|\t)[-*+]\s+(.+)/;
const FENCE = /^```/;

/**
 * MarkdownPipe converts basic markdown syntax to HTML.
 * Handles: headers (#, ##, ###), bold (**), italic (*), inline code, bullet
 * lists (- or *) with one level of indented sub-bullets, ordered lists (1.)
 * that keep their numbering, fenced code blocks, horizontal rules (---), and
 * paragraph breaks. A blank line between two items of the same list does not
 * end it.
 *
 * Its HTML is the Preview's, and Send back pastes it into apps that take
 * formatted text (Outlook, Teams), so what it gets wrong lands in the user's
 * mail. Kept intentionally simple for testability; no external dependencies.
 */
@Pipe({ name: 'markdown', standalone: true, pure: true })
export class MarkdownPipe implements PipeTransform {
  transform(value: string): string {
    if (!value) return '';

    const html = value
      // Escape HTML special chars first
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    const lines = html.split('\n');
    const output: string[] = [];
    let list: 'ul' | 'ol' | null = null;
    let itemOpen = false;
    let nested = false;
    let code: string[] | null = null;

    const closeItem = () => {
      if (nested) { output.push('</ul>'); nested = false; }
      if (itemOpen) { output.push('</li>'); itemOpen = false; }
    };
    const closeList = () => {
      closeItem();
      if (list) { output.push(`</${list}>`); list = null; }
    };
    const openItem = (type: 'ul' | 'ol', text: string, start = 1) => {
      if (list !== type) {
        closeList();
        output.push(type === 'ol' && start !== 1 ? `<ol start="${start}">` : `<${type}>`);
        list = type;
      } else {
        closeItem();
      }
      output.push(`<li>${this.inlineFormat(text)}`);
      itemOpen = true;
    };
    /** Whether the next non-blank line goes on with the list that is open. */
    const listGoesOn = (from: number): boolean => {
      const next = lines.slice(from).find(l => l.trim() !== '');
      if (next === undefined) return false;
      if (SUB_BULLET.test(next)) return true;
      return list === 'ul' ? BULLET.test(next) : NUMBERED.test(next);
    };

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      // Fenced code: kept as written, no inline formatting
      if (FENCE.test(line.trim())) {
        if (code === null) {
          closeList();
          code = [];
        } else {
          output.push(`<pre><code>${code.join('\n')}</code></pre>`);
          code = null;
        }
        continue;
      }
      if (code !== null) {
        code.push(line);
        continue;
      }

      // Horizontal rule
      if (/^---+$/.test(line.trim())) {
        closeList();
        output.push('<hr>');
        continue;
      }

      // Headers
      const header = line.match(/^(#{1,3})\s+(.+)/);
      if (header) {
        closeList();
        const level = header[1].length;
        output.push(`<h${level}>${this.inlineFormat(header[2])}</h${level}>`);
        continue;
      }

      // A sub-bullet under the open item; without one it is a bullet
      const sub = line.match(SUB_BULLET);
      if (sub && list && itemOpen) {
        if (!nested) { output.push('<ul>'); nested = true; }
        output.push(`<li>${this.inlineFormat(sub[1])}</li>`);
        continue;
      }
      if (sub) {
        openItem('ul', sub[1]);
        continue;
      }

      const bullet = line.match(BULLET);
      if (bullet) {
        openItem('ul', bullet[1]);
        continue;
      }

      const numbered = line.match(NUMBERED);
      if (numbered) {
        openItem('ol', numbered[2], Number(numbered[1]));
        continue;
      }

      if (line.trim() === '') {
        // A blank line between two items of one list does not end it
        if (list && listGoesOn(i + 1)) continue;
        closeList();
        // Empty line becomes paragraph break
        output.push('<br>');
        continue;
      }

      // Regular paragraph line
      closeList();
      output.push(`<p>${this.inlineFormat(line)}</p>`);
    }

    if (code !== null) output.push(`<pre><code>${(code as string[]).join('\n')}</code></pre>`);
    closeList();

    return output.join('');
  }

  private inlineFormat(text: string): string {
    return text
      // Bold+italic ***text***
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      // Bold **text**
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      // Italic *text*, not an asterisk with a space after or before it (3 * 4 * 5)
      .replace(/\*(?!\s)(.+?)(?<!\s)\*/g, '<em>$1</em>')
      // Inline code `code`
      .replace(/`(.+?)`/g, '<code>$1</code>');
  }
}
