import { diffWordsWithSpace } from 'diff';

/** One run of the result: unchanged, inserted by the fix, or deleted by it. */
export interface DiffSegment {
  kind: 'same' | 'ins' | 'del';
  text: string;
}

export interface WordDiff {
  segments: DiffSegment[];
  /**
   * How many places changed: a deletion and the insertion right after it are
   * one correction ("their" → "there"), as is a lone insertion or deletion.
   * A change of whitespace only (a doubled space, a line break) is not counted:
   * it has no visible mark to point at.
   */
  corrections: number;
}

/**
 * Give up on a word diff after this long and show the whole text as replaced.
 * A fix changes a few words, which diffs in well under a millisecond; this only
 * bites when the model rewrote a long text wholesale, where marks would be
 * noise anyway.
 */
const DIFF_TIMEOUT_MS = 300;

/**
 * The Fix result as proofreading marks: what the fix changed, word by word.
 *
 * Built on jsdiff (the `diff` package, BSD-3-Clause): Myers' O(ND) algorithm
 * over word and whitespace tokens, so `same + ins` spells the result exactly
 * and `same + del` the original — nothing is normalised away. Word tokens keep
 * apostrophes and accents together ("tomorrow’s" is one change, not three).
 */
export function wordDiff(before: string, after: string): WordDiff {
  if (before === after) {
    return { segments: after ? [{ kind: 'same', text: after }] : [], corrections: 0 };
  }
  const parts = diffWordsWithSpace(before, after, { timeout: DIFF_TIMEOUT_MS });
  if (!parts) {
    return {
      segments: [
        ...(before ? [{ kind: 'del' as const, text: before }] : []),
        ...(after ? [{ kind: 'ins' as const, text: after }] : []),
      ],
      corrections: 1,
    };
  }

  const segments: DiffSegment[] = [];
  let corrections = 0;
  let inChange = false;
  let changeVisible = false;
  const closeChange = () => {
    if (inChange && changeVisible) corrections++;
    inChange = false;
    changeVisible = false;
  };

  for (const part of parts) {
    if (!part.value) continue;
    const kind: DiffSegment['kind'] = part.added ? 'ins' : part.removed ? 'del' : 'same';
    if (kind === 'same') {
      closeChange();
    } else {
      inChange = true;
      if (/\S/.test(part.value)) changeVisible = true;
    }
    const last = segments[segments.length - 1];
    if (last && last.kind === kind) last.text += part.value;
    else segments.push({ kind, text: part.value });
  }
  closeChange();
  return { segments, corrections };
}
