import { describe, it, expect } from 'vitest';
import { wordDiff, type DiffSegment } from './word-diff';

const spell = (segments: DiffSegment[], keep: 'ins' | 'del') =>
  segments.filter(s => s.kind === 'same' || s.kind === keep).map(s => s.text).join('');

describe('wordDiff', () => {
  it('reports nothing for an unchanged text', () => {
    expect(wordDiff('All good here.', 'All good here.')).toEqual({
      segments: [{ kind: 'same', text: 'All good here.' }],
      corrections: 0,
    });
    expect(wordDiff('', '')).toEqual({ segments: [], corrections: 0 });
  });

  it('marks a replaced word as one deletion followed by one insertion', () => {
    const { segments, corrections } = wordDiff('so quick.', 'so quickly.');
    expect(segments).toEqual([
      { kind: 'same', text: 'so ' },
      { kind: 'del', text: 'quick' },
      { kind: 'ins', text: 'quickly' },
      { kind: 'same', text: '.' },
    ]);
    expect(corrections).toBe(1);
  });

  it('keeps a word with an apostrophe whole', () => {
    const { segments } = wordDiff('before tomorrows meeting', 'before tomorrow’s meeting');
    expect(segments.filter(s => s.kind !== 'same')).toEqual([
      { kind: 'del', text: 'tomorrows' },
      { kind: 'ins', text: 'tomorrow’s' },
    ]);
  });

  it('spells the result exactly from same + ins, and the original from same + del', () => {
    const before = 'Hi Sarah,\n\nthanks for sending the numbers so quick. I had a look and their are two things\nto flag before tomorrows meeting.\n\nBest,\nJonas';
    const after = 'Hi Sarah,\n\nThanks for sending the numbers so quickly. I had a look, and there are two things\nto flag before tomorrow’s meeting.\n\nBest,\nJonas';
    const { segments, corrections } = wordDiff(before, after);
    expect(spell(segments, 'ins')).toBe(after);
    expect(spell(segments, 'del')).toBe(before);
    // thanks→Thanks, quick→quickly, look→look,, their→there, tomorrows→tomorrow’s
    expect(corrections).toBe(5);
  });

  it('counts the mockup’s eight corrections', () => {
    const before = 'thanks for sending the quarterly numbers so quick. I had a look and their are two things I want to flag before tomorrows meeting: the travel budget for Q3 is allready over by 12%, and the vendor invoices from August dont show up in the totals. Could you check with finance wether those invoices where booked to the wrong cost center? Happy to walk through it together if thats easier.';
    const after = 'thanks for sending the quarterly numbers so quickly. I had a look and there are two things I want to flag before tomorrow’s meeting: the travel budget for Q3 is already over by 12%, and the vendor invoices from August don’t show up in the totals. Could you check with finance whether those invoices were booked to the wrong cost center? Happy to walk through it together if that’s easier.';
    expect(wordDiff(before, after).corrections).toBe(8);
  });

  it('counts a lone insertion or deletion as a correction', () => {
    expect(wordDiff('I saw cat.', 'I saw the cat.').corrections).toBe(1);
    expect(wordDiff('It is is fine.', 'It is fine.').corrections).toBe(1);
  });

  it('does not count a whitespace-only change, but keeps it in the text', () => {
    const { segments, corrections } = wordDiff('one  two', 'one two');
    expect(corrections).toBe(0);
    expect(spell(segments, 'ins')).toBe('one two');
  });

  it('handles an empty side', () => {
    expect(wordDiff('', 'New text')).toEqual({ segments: [{ kind: 'ins', text: 'New text' }], corrections: 1 });
    expect(wordDiff('Old text', '')).toEqual({ segments: [{ kind: 'del', text: 'Old text' }], corrections: 1 });
  });
});
