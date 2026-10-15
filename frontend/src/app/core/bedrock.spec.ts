import { describe, it, expect } from 'vitest';
import { defaultSettings } from '../../testing/wails-mock';
import { bedrockBaseModel, bedrockConnectionOf, bedrockProblem, bedrockReady, withBedrockConnection } from './bedrock';

describe('bedrock', () => {
  const settingsWith = (providers: Record<string, string>) =>
    ({ ...defaultSettings, providers: { ...defaultSettings.providers, ...providers } });

  // A settings file that names no mode or routing: a key, kept in the
  // geography — the same defaults as Settings.Bedrock() in Go.
  it('reads the defaults the backend uses', () => {
    expect(bedrockConnectionOf(defaultSettings)).toEqual({ region: '', auth: 'api-key', profile: '', routing: 'geo' });
    expect(bedrockConnectionOf(settingsWith({ bedrock_auth: 'nonsense', bedrock_routing: 'anywhere' })))
      .toEqual({ region: '', auth: 'api-key', profile: '', routing: 'geo' });
    expect(bedrockConnectionOf(settingsWith({ aws_region: 'eu-west-1', bedrock_auth: 'aws-signin', aws_profile: 'work', bedrock_routing: 'global' })))
      .toEqual({ region: 'eu-west-1', auth: 'aws-signin', profile: 'work', routing: 'global' });
  });

  it('writes a connection back without touching the rest', () => {
    const next = withBedrockConnection(settingsWith({ ollama_url: 'http://gpu:11434' }), { region: 'us-west-2', auth: 'aws-signin', profile: '', routing: 'global' });
    expect(next.providers).toEqual({ ollama_url: 'http://gpu:11434', aws_region: 'us-west-2', bedrock_auth: 'aws-signin', aws_profile: '', bedrock_routing: 'global' });
  });

  it('is ready with a region and, in key mode, a key', () => {
    const key = { is_set: true, source: 'keyring' };
    const none = { is_set: false, source: 'none' };
    expect(bedrockReady({ region: '', auth: 'aws-signin', profile: '', routing: 'geo' }, none)).toBe(false);
    expect(bedrockReady({ region: 'eu-central-1', auth: 'aws-signin', profile: '', routing: 'geo' }, none)).toBe(true);
    expect(bedrockReady({ region: 'eu-central-1', auth: 'api-key', profile: '', routing: 'geo' }, none)).toBe(false);
    expect(bedrockReady({ region: 'eu-central-1', auth: 'api-key', profile: '', routing: 'geo' }, key)).toBe(true);
  });

  it('says why it cannot work yet, and nothing while the key status is loading', () => {
    expect(bedrockProblem({ region: '', auth: 'api-key', profile: '', routing: 'geo' }, null)).toBe("Won't work yet: choose a region.");
    expect(bedrockProblem({ region: 'eu-central-1', auth: 'api-key', profile: '', routing: 'geo' }, null)).toBeNull();
    expect(bedrockProblem({ region: 'eu-central-1', auth: 'api-key', profile: '', routing: 'geo' }, { is_set: false, source: 'none' }))
      .toBe("Won't work yet: no API key is set.");
    // Whether the sign-in works can only be known by trying.
    expect(bedrockProblem({ region: 'eu-central-1', auth: 'aws-signin', profile: '', routing: 'geo' }, { is_set: false, source: 'none' })).toBeNull();
  });

  // Mirrors splitBedrockPrefix in internal/llm/bedrock.go.
  it('reads the base model behind a routing prefix', () => {
    expect(bedrockBaseModel('eu.anthropic.claude-haiku-5-5')).toBe('anthropic.claude-haiku-5-5');
    expect(bedrockBaseModel('us-gov.anthropic.claude-opus-4-8')).toBe('anthropic.claude-opus-4-8');
    expect(bedrockBaseModel('anthropic.claude-sonnet-5-5')).toBe('anthropic.claude-sonnet-5-5');
    expect(bedrockBaseModel('xx.anthropic.claude-haiku-5-5')).toBe('xx.anthropic.claude-haiku-5-5');
  });
});
