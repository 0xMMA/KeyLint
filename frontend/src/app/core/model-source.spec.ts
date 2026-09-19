import { describe, it, expect } from 'vitest';
import { noteForModelSource } from './model-source';

describe('noteForModelSource', () => {
  it('says nothing when the provider answered', () => {
    expect(noteForModelSource('live', 'openai', true)).toBe('');
  });

  it('says nothing for a provider with no endpoint to ask', () => {
    // The Claude Code CLI's three aliases are the whole list by design, so a
    // note here would be an alarm nobody can clear.
    expect(noteForModelSource('fixed', 'claude-code', true)).toBe('');
  });

  it('tells an Ollama user with nothing pulled what is actually wrong', () => {
    const note = noteForModelSource('empty', 'ollama', false);
    expect(note).toContain('No models pulled yet');
    expect(note).not.toContain('could not be reached');
  });

  it('does not tell an API user to pull a model', () => {
    expect(noteForModelSource('empty', 'openai', false)).toBe('This account lists no models.');
  });

  it('keeps a missing key apart from a missing provider', () => {
    expect(noteForModelSource('no-credentials', 'openai', true)).toContain('add a key');
    expect(noteForModelSource('unreachable', 'openai', true)).toContain('could not be reached');
  });

  it('does not claim the account is empty when the filter is what emptied it', () => {
    // An OpenAI project scoped to Responses-API-only models lists plenty; this
    // app can call none of it.
    expect(noteForModelSource('unusable', 'openai', true)).toContain('can be used here');
  });

  it('still explains an empty picker rather than going silent', () => {
    // A provider with no curated entries — a new one shipped before its list —
    // would otherwise show nothing and say nothing.
    for (const source of ['unreachable', 'no-credentials', 'unusable']) {
      const note = noteForModelSource(source, 'somethingnew', false);
      expect(note, source).not.toBe('');
      // ...and must not promise a list that is not on screen.
      expect(note, source).not.toContain('built-in list');
    }
  });

  it('says nothing for a source it has never heard of', () => {
    expect(noteForModelSource('static', 'openai', true)).toBe('');
  });
});
