import type { ClaudeCodeStatus, KeyStatus, Settings } from './wails.service';
import { bedrockConnectionOf, usesBedrockKey } from './bedrock';

/**
 * The providers a user can choose, in the order Settings › AI Providers lists
 * them and the setup wizard offers them. The labels are what the General tab's
 * pointer and the model pickers show too.
 */
export const PROVIDER_OPTIONS: ReadonlyArray<{ label: string; value: string }> = [
  { label: 'Anthropic API', value: 'claude' },
  // Next to the Anthropic API: the other way to reach Claude.
  { label: 'Claude Code (installed CLI)', value: 'claude-code' },
  { label: 'OpenRouter', value: 'openrouter' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'AWS Bedrock', value: 'bedrock' },
  { label: 'Ollama (local)', value: 'ollama' },
];

/**
 * A provider's name without the parenthetical ("Claude Code", "Ollama"), for
 * tight places like a chip; the provider ID only if KeyLint does not know it.
 */
export function providerShortLabel(provider: string): string {
  const label = PROVIDER_OPTIONS.find(o => o.value === provider)?.label;
  return label ? label.replace(/\s*\(.*\)$/, '') : provider;
}

/**
 * The providers that work with an API key alone. AWS Bedrock is not one: it
 * needs a region too, and with this computer's AWS sign-in no key at all —
 * see needsKey.
 */
export const KEY_PROVIDERS: ReadonlyArray<string> = ['openai', 'claude', 'openrouter'];

export function isKeyProvider(provider: string | null | undefined): boolean {
  return !!provider && KEY_PROVIDERS.includes(provider);
}

/**
 * Whether the provider needs a key as these settings configure it: a key
 * provider always, AWS Bedrock in key mode (its default). Mirrors
 * llm.UsesAPIKey.
 */
export function needsKey(provider: string | null | undefined, settings: Settings | null | undefined): boolean {
  if (provider === 'bedrock') return usesBedrockKey(bedrockConnectionOf(settings));
  return isKeyProvider(provider);
}

/**
 * Which environment variable supplies each provider's key. The source of truth
 * is `envVars` in internal/features/settings/service.go: keep the two in sync,
 * or a card names a variable the backend does not read.
 */
export const ENV_KEY_VARS: Readonly<Record<string, string>> = {
  openai: 'OPENAI_API_KEY',
  claude: 'ANTHROPIC_API_KEY',
  openrouter: 'OPENROUTER_API_KEY',
  bedrock: 'AWS_BEARER_TOKEN_BEDROCK',
};

export function keyPlaceholder(provider: string): string {
  switch (provider) {
    case 'openai': return 'sk-…';
    case 'claude': return 'sk-ant-…';
    case 'openrouter': return 'sk-or-…';
    case 'bedrock': return 'Bedrock API key';
    default: return 'API key';
  }
}

/** A connection's state as a tag: the same words on the settings cards and in the wizard. */
export interface ConnectionTag {
  value: string;
  severity: 'success' | 'warn' | 'secondary' | 'info';
}

export function cliTag(status: ClaudeCodeStatus): ConnectionTag {
  if (status.installed && status.loggedIn) return { value: 'signed in', severity: 'success' };
  if (status.installed) return { value: 'not signed in', severity: 'warn' };
  return { value: 'not installed', severity: 'secondary' };
}

export function keyTag(status: KeyStatus): ConnectionTag {
  if (status.is_set && status.source === 'env') return { value: 'key from env var', severity: 'info' };
  if (status.is_set) return { value: 'key set', severity: 'success' };
  return { value: 'no key', severity: 'secondary' };
}
