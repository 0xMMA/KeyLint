import type { ClaudeCodeStatus, KeyStatus } from './wails.service';

/**
 * The providers a user can choose, in the order Settings › AI Providers lists
 * them and the setup wizard offers them. The labels are what the General tab's
 * pointer and the model pickers show too.
 *
 * AWS Bedrock stays out until it works (#22, #23). A settings file that still
 * names it is explained, not broken — see unavailableProviderName.
 */
export const PROVIDER_OPTIONS: ReadonlyArray<{ label: string; value: string }> = [
  { label: 'OpenAI', value: 'openai' },
  { label: 'Anthropic API', value: 'claude' },
  { label: 'Claude Code (installed CLI)', value: 'claude-code' },
  { label: 'Ollama (local)', value: 'ollama' },
];

/** The providers that work with an API key, and nothing else. */
export const KEY_PROVIDERS: ReadonlyArray<string> = ['openai', 'claude'];

export function isKeyProvider(provider: string | null | undefined): boolean {
  return !!provider && KEY_PROVIDERS.includes(provider);
}

/**
 * Which environment variable supplies each provider's key. The source of truth
 * is `envVars` in internal/features/settings/service.go: keep the two in sync,
 * or a card names a variable the backend does not read.
 */
export const ENV_KEY_VARS: Readonly<Record<string, string>> = {
  openai: 'OPENAI_API_KEY',
  claude: 'ANTHROPIC_API_KEY',
};

export function keyPlaceholder(provider: string): string {
  switch (provider) {
    case 'openai': return 'sk-…';
    case 'claude': return 'sk-ant-…';
    default: return 'API key';
  }
}

/** A connection's state as a tag: the same words on the settings cards and in the wizard. */
export interface ConnectionTag {
  value: string;
  severity: 'success' | 'warn' | 'secondary' | 'info';
}

export function cliTag(status: ClaudeCodeStatus): ConnectionTag {
  if (status.installed && status.loggedIn) return { value: '● signed in', severity: 'success' };
  if (status.installed) return { value: 'not signed in', severity: 'warn' };
  return { value: 'not installed', severity: 'secondary' };
}

export function keyTag(status: KeyStatus): ConnectionTag {
  if (status.is_set && status.source === 'env') return { value: 'key from env var', severity: 'info' };
  if (status.is_set) return { value: '● key set', severity: 'success' };
  return { value: 'no key', severity: 'secondary' };
}
