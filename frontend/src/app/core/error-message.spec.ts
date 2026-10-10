import { describe, it, expect } from 'vitest';
import { describeError } from './error-message';

describe('describeError', () => {
  it('unwraps the CallError JSON a failed Go binding call rejects with', () => {
    const e = new Error(
      '{"message":"The original window is closed — the text is on the clipboard.","kind":"RuntimeError"}',
    );
    expect(describeError(e)).toBe('The original window is closed — the text is on the clipboard.');
  });

  it('passes a plain error message through', () => {
    expect(describeError(new Error('API failure'))).toBe('API failure');
  });

  it('passes text that only looks like JSON through', () => {
    expect(describeError(new Error('{not json'))).toBe('{not json');
    expect(describeError(new Error('{"code":3}'))).toBe('{"code":3}');
    expect(describeError(new Error('{}'))).toBe('{}');
  });

  it('handles non-Error rejections', () => {
    expect(describeError('boom')).toBe('boom');
    expect(describeError(42)).toBe('42');
  });
});
