import { Pipe, PipeTransform } from '@angular/core';

/**
 * MarkdownPipe converts basic markdown syntax to HTML.
 * Handles: headers (#, ##, ###), bold (**), italic (*), bullet lists (- or *),
 * ordered lists (1.), horizontal rules (---), and paragraph breaks.
 * Kept intentionally simple for testability; no external dependencies.
 */
@Pipe({ name: 'markdown', standalone: true, pure: true })
export class MarkdownPipe implements PipeTransform {
  transform(value: string): string {
    if (!value) return '';

    let html = value
      // Escape HTML special chars first
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');

    // Process line by line for block elements
    const lines = html.split('\n');
    const output: string[] = [];
    let inList = false;
    let inOrderedList = false;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      // Horizontal rule
      if (/^---+$/.test(line.trim())) {
        if (inList) { output.push('</ul>'); inList = false; }
        if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }
        output.push('<hr>');
        continue;
      }

      // H3
      const h3Match = line.match(/^###\s+(.+)/);
      if (h3Match) {
        if (inList) { output.push('</ul>'); inList = false; }
        if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }
        output.push(`<h3>${this.inlineFormat(h3Match[1])}</h3>`);
        continue;
      }

      // H2
      const h2Match = line.match(/^##\s+(.+)/);
      if (h2Match) {
        if (inList) { output.push('</ul>'); inList = false; }
        if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }
        output.push(`<h2>${this.inlineFormat(h2Match[1])}</h2>`);
        continue;
      }

      // H1
      const h1Match = line.match(/^#\s+(.+)/);
      if (h1Match) {
        if (inList) { output.push('</ul>'); inList = false; }
        if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }
        output.push(`<h1>${this.inlineFormat(h1Match[1])}</h1>`);
        continue;
      }

      // Unordered list
      const ulMatch = line.match(/^[-*+]\s+(.+)/);
      if (ulMatch) {
        if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }
        if (!inList) { output.push('<ul>'); inList = true; }
        output.push(`<li>${this.inlineFormat(ulMatch[1])}</li>`);
        continue;
      }

      // Ordered list
      const olMatch = line.match(/^\d+\.\s+(.+)/);
      if (olMatch) {
        if (inList) { output.push('</ul>'); inList = false; }
        if (!inOrderedList) { output.push('<ol>'); inOrderedList = true; }
        output.push(`<li>${this.inlineFormat(olMatch[1])}</li>`);
        continue;
      }

      // Close open lists on empty line or regular paragraph
      if (inList) { output.push('</ul>'); inList = false; }
      if (inOrderedList) { output.push('</ol>'); inOrderedList = false; }

      // Empty line becomes paragraph break
      if (line.trim() === '') {
        output.push('<br>');
        continue;
      }

      // Regular paragraph line
      output.push(`<p>${this.inlineFormat(line)}</p>`);
    }

    // Close any unclosed lists
    if (inList) output.push('</ul>');
    if (inOrderedList) output.push('</ol>');

    return output.join('');
  }

  private inlineFormat(text: string): string {
    return text
      // Bold+italic ***text***
      .replace(/\*\*\*(.+?)\*\*\*/g, '<strong><em>$1</em></strong>')
      // Bold **text**
      .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
      // Italic *text*
      .replace(/\*(.+?)\*/g, '<em>$1</em>')
      // Inline code `code`
      .replace(/`(.+?)`/g, '<code>$1</code>');
  }
}
