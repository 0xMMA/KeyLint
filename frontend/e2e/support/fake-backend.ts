/**
 * A stand-in for the Go settings service, for specs that need a configured app.
 *
 * `ng serve` has no Wails bridge, so every binding call is a POST to
 * /wails/runtime that 404s and the app falls back to its browser-mode defaults:
 * no keys, no CLI, nothing saved. That is enough for layout checks but not for
 * anything about which provider is configured or in use. This answers the
 * settings service's calls from an in-memory state instead, so a spec can say
 * "Anthropic key in the keyring, CLI signed in" and see what a user sees.
 *
 * Method IDs are read from the generated bindings rather than copied here, so
 * regenerating them cannot leave this answering the wrong method. Calls to any
 * other service still fall through to `ng serve` and 404, as before.
 */
import { Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

export type KeySource = 'env' | 'keyring';

export interface FakeBackendState {
  settings: Record<string, unknown>;
  /** Where each provider's key comes from; absent means not set. */
  keys: Partial<Record<string, KeySource>>;
  claudeCode: { installed: boolean; path: string; version: string; loggedIn: boolean };
  /** ModelList.source per provider; anything absent answers "unreachable". */
  modelSources: Partial<Record<string, string>>;
  /** Every settings object the app saved (Save or SetActiveProvider), oldest first. */
  saves: Record<string, unknown>[];
  /** When set, SetActiveProvider fails with this message, as a failed write would. */
  failSetActiveProvider?: string;
}

/** What the real SetActiveProvider accepts; see selectableProviders in settings/service.go. */
const SELECTABLE_PROVIDERS = ['openai', 'claude', 'claude-code', 'ollama'];

export const BASE_SETTINGS: Record<string, unknown> = {
  active_provider: 'claude',
  models: {},
  providers: { ollama_url: '', aws_region: '' },
  shortcut_key: 'ctrl+g',
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: true,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};

const SIGNED_IN_CLI = {
  installed: true,
  path: 'C:\\Users\\you\\AppData\\Roaming\\npm\\claude.cmd',
  version: '2.1.0',
  loggedIn: true,
};

const NO_CLI = { installed: false, path: '', version: '', loggedIn: false };

export function fakeState(opts: {
  activeProvider: string;
  keys?: FakeBackendState['keys'];
  cliSignedIn?: boolean;
  modelSources?: FakeBackendState['modelSources'];
}): FakeBackendState {
  return {
    settings: { ...BASE_SETTINGS, active_provider: opts.activeProvider },
    keys: opts.keys ?? {},
    claudeCode: opts.cliSignedIn ? { ...SIGNED_IN_CLI } : { ...NO_CLI },
    // The CLI has no model endpoint; "fixed" is what the real service answers.
    modelSources: { 'claude-code': 'fixed', ...opts.modelSources },
    saves: [],
  };
}

/** methodID → method name, parsed from the generated settings bindings. */
function settingsMethodIDs(): Map<number, string> {
  const file = path.join(__dirname, '../../bindings/keylint/internal/features/settings/service.js');
  const src = fs.readFileSync(file, 'utf8');
  const ids = new Map<number, string>();
  for (const m of src.matchAll(/export function (\w+)\([^)]*\)\s*\{\s*return \$Call\.ByID\((\d+)/g)) {
    ids.set(Number(m[2]), m[1]);
  }
  if (ids.size === 0) throw new Error(`no binding IDs found in ${file}`);
  return ids;
}

/** Routes the settings service's binding calls to `state`. */
export async function installFakeBackend(page: Page, state: FakeBackendState): Promise<void> {
  const ids = settingsMethodIDs();

  await page.route('**/wails/runtime', async (route) => {
    let body: { args?: { methodID?: number; args?: unknown[] } } = {};
    try {
      body = route.request().postDataJSON() ?? {};
    } catch {
      // Not a JSON binding call.
    }
    const method = ids.get(body.args?.methodID ?? -1);
    const args = body.args?.args ?? [];
    const json = (value: unknown) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });
    const empty = () => route.fulfill({ status: 200, contentType: 'text/plain', body: '' });

    switch (method) {
      case 'Get':
        return json(state.settings);
      case 'Save':
        state.settings = structuredClone(args[0] as Record<string, unknown>);
        state.saves.push(state.settings);
        return empty();
      case 'SetActiveProvider': {
        const provider = args[0] as string;
        // The runtime turns a non-2xx answer into a rejected promise carrying
        // the body, which is how a Go error reaches the frontend.
        if (!SELECTABLE_PROVIDERS.includes(provider)) {
          return route.fulfill({ status: 500, contentType: 'text/plain', body: `settings: "${provider}" is not a provider KeyLint can use` });
        }
        if (state.failSetActiveProvider) {
          return route.fulfill({ status: 500, contentType: 'text/plain', body: state.failSetActiveProvider });
        }
        state.settings = { ...state.settings, active_provider: provider };
        state.saves.push(state.settings);
        return empty();
      }
      case 'GetKeyStatus': {
        const source = state.keys[args[0] as string];
        return json(source ? { is_set: true, source } : { is_set: false, source: 'none' });
      }
      case 'SetKey':
        state.keys[args[0] as string] = 'keyring';
        return empty();
      case 'DeleteKey':
        delete state.keys[args[0] as string];
        return empty();
      case 'GetClaudeCodeStatus':
        return json(state.claudeCode);
      case 'ListModels':
        return json({ models: [], source: state.modelSources[args[0] as string] ?? 'unreachable' });
      default:
        return route.fallback();
    }
  });
}
