import { describe, it, expect } from 'vitest';
import { ENV_KEY_VARS, KEY_PROVIDERS, PROVIDER_OPTIONS, isKeyProvider, keyPlaceholder, needsKey, providerShortLabel } from './providers';
import { defaultSettings } from '../../testing/wails-mock';

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

  // Both ways to reach Claude sit side by side, so a user choosing between
  // a key and the subscription sees them together (owner, 2026-10-16).
  it('lists Claude Code next to the Anthropic API, the local daemon last', () => {
    expect(PROVIDER_OPTIONS.map(o => o.value)).toEqual(['openai', 'claude', 'claude-code', 'openrouter', 'bedrock', 'ollama']);
  });

  // The source of truth is envVars in internal/features/settings/service.go.
  it('names the environment variable the backend reads for the OpenRouter key', () => {
    expect(ENV_KEY_VARS['openrouter']).toBe('OPENROUTER_API_KEY');
  });

  it('shows the shape of an OpenRouter key in an empty key field', () => {
    expect(keyPlaceholder('openrouter')).toBe('sk-or-…');
  });

  // AWS Bedrock (roadmap E6): a key in key mode, nothing with this computer's
  // AWS sign-in. A key alone does not make it work (it needs a region too),
  // so it is not one of the providers that work with a key alone.
  it('offers AWS Bedrock, which needs a key only in key mode', () => {
    expect(PROVIDER_OPTIONS.find(o => o.value === 'bedrock')?.label).toBe('AWS Bedrock');
    expect(providerShortLabel('bedrock')).toBe('AWS Bedrock');
    expect(isKeyProvider('bedrock')).toBe(false);
    const keyMode = { ...defaultSettings, providers: { ...defaultSettings.providers, bedrock_auth: 'api-key' } };
    const signIn = { ...defaultSettings, providers: { ...defaultSettings.providers, bedrock_auth: 'aws-signin' } };
    expect(needsKey('bedrock', keyMode)).toBe(true);
    expect(needsKey('bedrock', defaultSettings)).toBe(true);
    expect(needsKey('bedrock', signIn)).toBe(false);
    expect(needsKey('openai', signIn)).toBe(true);
    expect(needsKey('claude-code', keyMode)).toBe(false);
  });

  // AWS's own variable for a Bedrock key; AWS_SECRET_ACCESS_KEY is not one.
  it('names AWS_BEARER_TOKEN_BEDROCK for the Bedrock key', () => {
    expect(ENV_KEY_VARS['bedrock']).toBe('AWS_BEARER_TOKEN_BEDROCK');
    expect(keyPlaceholder('bedrock')).toBe('Bedrock API key');
  });
});
