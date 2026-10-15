import { describe, it, expect } from 'vitest';
import { DEFAULT_MODELS, defaultOption, effortNote, fixSkipsReasoning, toModelOption } from './model-options';

describe('model-options', () => {
  // The owner's choice (2026-10-15); the source of truth is defaultModels in
  // internal/llm/models.go.
  it('names OpenRouter\'s defaults as explicit vendor/model IDs', () => {
    expect(DEFAULT_MODELS['openrouter']).toEqual({ fix: 'anthropic/claude-haiku-5.5', pyramidize: 'anthropic/claude-sonnet-5.5' });
  });

  it('names the OpenRouter default by the catalogue\'s own name once the list is there', () => {
    const listed = [toModelOption({ id: 'anthropic/claude-haiku-5.5', label: 'Anthropic: Claude Haiku 5.5', resolved: '' })];
    const option = defaultOption('openrouter', 'fix', listed);
    expect(option.display).toBe('Default: Anthropic: Claude Haiku 5.5');
    expect(option.secondary).toBe('anthropic/claude-haiku-5.5');
  });

  it('says what effort does on OpenRouter', () => {
    expect(effortNote('openrouter')).toMatch(/reasoning/);
  });

  // Mirrors IsFastTier in internal/llm/families.go (TestIsFastTier): the note
  // under the Fix picker must say what the backend does.
  it('knows which OpenRouter models Fix asks not to reason', () => {
    const fast = [
      'anthropic/claude-haiku-5.5', 'anthropic/claude-haiku-4.5', '~anthropic/claude-haiku-latest',
      'openai/gpt-6-luna', 'openai/gpt-5.6-luna', '~openai/gpt-luna-latest',
    ];
    const notFast = [
      'anthropic/claude-sonnet-5.5', 'openai/gpt-6-astra', 'openai/gpt-6-luna-pro',
      'somehost/claude-haiku-5.5', 'google/gemini-3.8-flash', '',
    ];
    for (const m of fast) expect(fixSkipsReasoning('openrouter', m), m).toBe(true);
    for (const m of notFast) expect(fixSkipsReasoning('openrouter', m), m).toBe(false);
  });

  it('keeps the other providers as they were', () => {
    expect(fixSkipsReasoning('claude', 'haiku')).toBe(true);
    expect(fixSkipsReasoning('openai', 'gpt-6-luna')).toBe(true);
    expect(fixSkipsReasoning('openai', 'gpt-6-luna-pro')).toBe(false);
    // The CLI has no switch for it, and Ollama nothing to send it to.
    expect(fixSkipsReasoning('claude-code', 'haiku')).toBe(false);
    expect(fixSkipsReasoning('ollama', 'anthropic/claude-haiku-5.5')).toBe(false);
  });

  // Base IDs; the routing prefix is added in Go at call time.
  it('names the Bedrock defaults as base IDs', () => {
    expect(DEFAULT_MODELS['bedrock']).toEqual({ fix: 'anthropic.claude-haiku-5-5', pyramidize: 'anthropic.claude-sonnet-5-5' });
  });

  it('says what effort does on Bedrock', () => {
    expect(effortNote('bedrock')).toMatch(/reasoning/);
  });

  // Mirrors IsFastTier for Bedrock IDs, bare or behind a routing prefix.
  it('knows which Bedrock models Fix asks not to reason', () => {
    const fast = ['anthropic.claude-haiku-5-5', 'eu.anthropic.claude-haiku-5-5', 'global.anthropic.claude-haiku-4-5-20251001-v1:0'];
    const notFast = [
      'anthropic.claude-sonnet-5-5', 'us.anthropic.claude-sonnet-5-5', 'global.anthropic.claude-opus-5-5',
      'arn:aws:bedrock:eu-central-1:123456789012:application-inference-profile/haiku', 'xx.anthropic.claude-haiku-5-5',
    ];
    for (const m of fast) expect(fixSkipsReasoning('bedrock', m), m).toBe(true);
    for (const m of notFast) expect(fixSkipsReasoning('bedrock', m), m).toBe(false);
  });
});
