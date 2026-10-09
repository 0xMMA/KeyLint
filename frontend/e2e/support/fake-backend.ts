/**
 * A stand-in for the Go settings service, for specs that need a configured app.
 *
 * `ng serve` has no Wails bridge, so every binding call is a POST to
 * /wails/runtime that 404s and the app falls back to its browser-mode defaults:
 * no keys, no CLI, nothing saved. That is enough for layout checks but not for
 * anything about which provider is configured or in use. This answers the
 * settings service's calls from an in-memory state instead, so a spec can say
 * "Anthropic key in the keyring, CLI signed in" and see what a user sees. The
 * welcome service's two calls are answered too when a spec sets `firstRun`, so
 * the setup wizard can be driven the same way.
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
  /** ModelList.models per provider; anything absent answers an empty list. */
  modelLists: Partial<Record<string, ModelEntry[]>>;
  /** Every settings object the app saved (Save, SetActiveProvider or SetDeveloperOptions), oldest first. */
  saves: Record<string, unknown>[];
  /** When set, SetActiveProvider fails with this message, as a failed write would. */
  failSetActiveProvider?: string;
  /**
   * What IsFirstRun answers while setup is not complete. The decision itself
   * (first start, or nothing usable) is welcome.Service's and is tested in Go;
   * this only says whether the wizard opens. Undefined leaves the welcome
   * service unanswered (it 404s and the app treats that as "not first run").
   */
  firstRun?: boolean;
  /** How many times the app called CompleteSetup. */
  completeSetupCalls: number;
}

/** One picker entry, as internal/llm ModelInfo serialises it. */
export interface ModelEntry { id: string; label: string; resolved: string }

/** What the real Anthropic listing looks like: aliases first, naming their model. */
export const ANTHROPIC_MODELS: ModelEntry[] = [
  { id: 'opus', label: 'Opus (latest)', resolved: 'claude-opus-5-5' },
  { id: 'sonnet', label: 'Sonnet (latest)', resolved: 'claude-sonnet-5-5' },
  { id: 'haiku', label: 'Haiku (latest)', resolved: 'claude-haiku-5-5' },
  { id: 'claude-haiku-5-5', label: 'Claude Haiku 5.5', resolved: '' },
  { id: 'claude-sonnet-5-5', label: 'Claude Sonnet 5.5', resolved: '' },
  { id: 'claude-opus-5-5', label: 'Claude Opus 5.5', resolved: '' },
  { id: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6', resolved: '' },
  { id: 'claude-haiku-4-5-20251001', label: 'Claude Haiku 4.5', resolved: '' },
];

/** The CLI's fixed alias list; see claudeCodeAliases in internal/llm/models.go. */
export const CLI_ALIASES: ModelEntry[] = [
  { id: 'opus', label: 'Opus (latest)', resolved: '' },
  { id: 'sonnet', label: 'Sonnet (latest)', resolved: '' },
  { id: 'haiku', label: 'Haiku (latest)', resolved: '' },
  { id: 'fable', label: 'Fable (latest)', resolved: '' },
];

/** What the real SetActiveProvider accepts; see selectableProviders in settings/service.go. */
const SELECTABLE_PROVIDERS = ['openai', 'claude', 'claude-code', 'ollama'];

export const BASE_SETTINGS: Record<string, unknown> = {
  active_provider: 'claude',
  models: {},
  providers: { ollama_url: '', aws_region: '' },
  shortcut_key: 'ctrl+g',
  shortcut_mode: 'double_tap',
  shortcut_fix: 'ctrl+g',
  shortcut_pyramidize: 'ctrl+shift+g',
  shortcut_double_tap_delay: 200,
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: true,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  developer_options: false,
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
  modelLists?: FakeBackendState['modelLists'];
}): FakeBackendState {
  return {
    settings: { ...BASE_SETTINGS, active_provider: opts.activeProvider },
    keys: opts.keys ?? {},
    claudeCode: opts.cliSignedIn ? { ...SIGNED_IN_CLI } : { ...NO_CLI },
    // The CLI has no model endpoint; "fixed" is what the real service answers.
    modelSources: { 'claude-code': 'fixed', ...opts.modelSources },
    modelLists: { 'claude-code': CLI_ALIASES, ...opts.modelLists },
    saves: [],
    completeSetupCalls: 0,
  };
}

/** methodID → method name, parsed from one service's generated bindings. */
function methodIDs(feature: string): Map<number, string> {
  const file = path.join(__dirname, `../../bindings/keylint/internal/features/${feature}/service.js`);
  const src = fs.readFileSync(file, 'utf8');
  const ids = new Map<number, string>();
  for (const m of src.matchAll(/export function (\w+)\([^)]*\)\s*\{\s*return \$Call\.ByID\((\d+)/g)) {
    ids.set(Number(m[2]), m[1]);
  }
  if (ids.size === 0) throw new Error(`no binding IDs found in ${file}`);
  return ids;
}

/** Routes the settings service's binding calls (and the welcome service's, see firstRun) to `state`. */
export async function installFakeBackend(page: Page, state: FakeBackendState): Promise<void> {
  const ids = methodIDs('settings');
  const welcomeIDs = methodIDs('welcome');

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

    const welcomeMethod = welcomeIDs.get(body.args?.methodID ?? -1);
    if (welcomeMethod && state.firstRun !== undefined) {
      switch (welcomeMethod) {
        case 'IsFirstRun':
          return json(state.firstRun && !state.settings['completed_setup']);
        case 'CompleteSetup':
          state.completeSetupCalls++;
          state.settings = { ...state.settings, completed_setup: true };
          return empty();
      }
    }

    switch (method) {
      case 'Get':
        return json(state.settings);
      case 'Save':
        // Like the real Save, completed_setup is not the screen's to change.
        state.settings = { ...structuredClone(args[0] as Record<string, unknown>), completed_setup: state.settings['completed_setup'] };
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
      case 'SetDeveloperOptions':
        state.settings = { ...state.settings, developer_options: args[0] as boolean };
        state.saves.push(state.settings);
        return empty();
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
        return json({
          models: state.modelLists[args[0] as string] ?? [],
          source: state.modelSources[args[0] as string] ?? 'unreachable',
        });
      default:
        return route.fallback();
    }
  });
}
