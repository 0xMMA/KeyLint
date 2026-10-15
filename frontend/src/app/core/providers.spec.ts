import { describe, it, expect } from 'vitest';
import { ENV_KEY_VARS, KEY_PROVIDERS, PROVIDER_OPTIONS, isKeyProvider, keyPlaceholder, providerShortLabel } from './providers';

describe('providers', () => {
  // OpenRouter (roadmap E10): one key for many vendors' models, set up like
  // the other key providers — a card on AI Providers and an option in the
  // setup wizard, both read from these lists.
  it('offers OpenRouter as a provider that takes an API key', () => {
    expect(PROVIDER_OPTIONS.find(o => o.value === 'openrouter')?.label).toBe('OpenRouter');
    expect(KEY_PROVIDERS).toContain('openrouter');
    expect(isKeyProvider('openrouter')).toBe(true);
    expect(providerShortLabel('openrouter')).toBe('OpenRouter');
  });

  it('lists the key providers together, ahead of the CLI and the local daemon', () => {
    expect(PROVIDER_OPTIONS.map(o => o.value)).toEqual(['openai', 'claude', 'openrouter', 'claude-code', 'ollama']);
  });

  // The source of truth is envVars in internal/features/settings/service.go.
  it('names the environment variable the backend reads for the OpenRouter key', () => {
    expect(ENV_KEY_VARS['openrouter']).toBe('OPENROUTER_API_KEY');
  });

  it('shows the shape of an OpenRouter key in an empty key field', () => {
    expect(keyPlaceholder('openrouter')).toBe('sk-or-…');
  });
});
