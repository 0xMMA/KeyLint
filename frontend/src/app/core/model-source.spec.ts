import { describe, it, expect } from 'vitest';
import { noteForModelSource } from './model-source';

describe('noteForModelSource', () => {
  it('says nothing when the provider answered', () => {
    expect(noteForModelSource('live', 'openai', true)).toBe('');
  });

  // Bedrock has no model list to ask (Anthropic's Models API is not offered
  // there): the picker says so truthfully, and that it takes other IDs.
  it('says where the Bedrock list comes from, truthfully', () => {
    const note = noteForModelSource('known', 'bedrock', true);
    // Bedrock does list foundation models; what KeyLint does not do is ask.
    expect(note).toContain('KeyLint does not ask AWS for a model list');
    expect(note).toContain('the Claude models KeyLint knows');
    expect(note).toMatch(/inference profile ARN/);
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

  // OpenRouter's catalogue is public: the list is live before any key is
  // set, and it is OpenRouter's, not the account's. The note has to say both,
  // and how to find one model among four hundred.
  it('calls the OpenRouter list the whole catalogue, searchable, and not a promise about the key', () => {
    const note = noteForModelSource('live', 'openrouter', true);
    expect(note).toContain("OpenRouter's");
    expect(note).toMatch(/type/i);
    expect(note).toMatch(/account/);
  });

  it('never calls OpenRouter\'s catalogue "this account\'s"', () => {
    for (const source of ['empty', 'unusable']) {
      const note = noteForModelSource(source, 'openrouter', source === 'unusable');
      expect(note, source).not.toContain('This account');
      expect(note, source).not.toContain("this account's");
      expect(note, source).toContain('OpenRouter');
    }
  });

  it('says why OpenRouter has not been asked yet', () => {
    expect(noteForModelSource('not-asked', 'openrouter', true)).toContain('in use');
  });

  it('says nothing for a source it has never heard of', () => {
    expect(noteForModelSource('static', 'openai', true)).toBe('');
  });
});
