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
  /** Every provider ListModels was called for, in order: what the app asked a provider for. */
  listModelsCalls: string[];
  /** Every settings object the app saved (Save, SetActiveProvider, SetDeveloperOptions or SetBedrock), oldest first. */
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
  /** Every connection the app saved through SetBedrock, oldest first. */
  bedrockSaves: Record<string, string>[];
  /** How many times the app called TestBedrock: never unless the user clicked. */
  testBedrockCalls: number;
  /** When set, TestBedrock fails with this message; otherwise it answers bedrockTestAnswer. */
  failTestBedrock?: string;
  /** The model ID TestBedrock reports reaching. */
  bedrockTestAnswer: string;
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

/**
 * A slice of OpenRouter's public catalogue as ListModels hands it over: its
 * names, explicit vendor/model IDs, no alias (internal/llm/openrouter.go).
 */
export const OPENROUTER_MODELS: ModelEntry[] = [
  { id: 'anthropic/claude-haiku-5.5', label: 'Anthropic: Claude Haiku 5.5', resolved: '' },
  { id: 'anthropic/claude-sonnet-5.5', label: 'Anthropic: Claude Sonnet 5.5', resolved: '' },
  { id: 'anthropic/claude-opus-5.5', label: 'Anthropic: Claude Opus 5.5', resolved: '' },
  { id: 'openai/gpt-6-luna', label: 'OpenAI: GPT-6 Luna', resolved: '' },
  { id: 'openai/gpt-6-astra', label: 'OpenAI: GPT-6 Astra', resolved: '' },
  { id: 'google/gemini-3.8-flash', label: 'Google: Gemini 3.8 Flash', resolved: '' },
  { id: 'deepseek/deepseek-v4.1-flash', label: 'DeepSeek: DeepSeek V4.1 Flash', resolved: '' },
  { id: 'meta-llama/llama-5-70b-instruct:free', label: 'Meta: Llama 5 70B Instruct (free)', resolved: '' },
];

/** The Bedrock picker as llm.ListModels answers it, source "known" (internal/llm/bedrock.go). */
export const BEDROCK_MODELS: ModelEntry[] = [
  { id: 'anthropic.claude-haiku-5-5', label: 'Claude Haiku 5.5', resolved: '' },
  { id: 'anthropic.claude-sonnet-5-5', label: 'Claude Sonnet 5.5', resolved: '' },
  { id: 'anthropic.claude-opus-5-5', label: 'Claude Opus 5.5', resolved: '' },
  { id: 'anthropic.claude-sonnet-5', label: 'Claude Sonnet 5', resolved: '' },
  { id: 'anthropic.claude-haiku-4-5-20251001-v1:0', label: 'Claude Haiku 4.5', resolved: '' },
  { id: 'anthropic.claude-opus-4-8', label: 'Claude Opus 4.8', resolved: '' },
];

const ALL_BEDROCK = BEDROCK_MODELS.map(m => m.id);
const NO_JP = ['anthropic.claude-sonnet-5-5', 'anthropic.claude-sonnet-5'];

/** A slice of llm.BedrockRegions(): a geography, Japan (where Sonnet cannot stay), and worldwide only. */
export const BEDROCK_REGIONS = [
  { id: 'us-east-1', label: 'US East (N. Virginia)', geography: 'us', geography_name: 'the US', geo_models: ALL_BEDROCK },
  { id: 'eu-central-1', label: 'Europe (Frankfurt)', geography: 'eu', geography_name: 'the EU', geo_models: ALL_BEDROCK },
  { id: 'ap-northeast-1', label: 'Asia Pacific (Tokyo)', geography: 'jp', geography_name: 'Japan', geo_models: ALL_BEDROCK.filter(m => !NO_JP.includes(m)) },
  { id: 'ap-south-1', label: 'Asia Pacific (Mumbai)', geography: '', geography_name: '', geo_models: [] },
];

/** What the real SetActiveProvider accepts; see selectableProviders in settings/service.go. */
const SELECTABLE_PROVIDERS = ['openai', 'claude', 'claude-code', 'openrouter', 'bedrock', 'ollama'];

export const BASE_SETTINGS: Record<string, unknown> = {
  active_provider: 'claude',
  models: {},
  providers: { ollama_url: '', aws_region: '', bedrock_auth: '', aws_profile: '', bedrock_routing: '' },
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
    // Bedrock has no list to ask; "known" is the build's own.
    modelSources: { 'claude-code': 'fixed', bedrock: 'known', ...opts.modelSources },
    modelLists: { 'claude-code': CLI_ALIASES, bedrock: BEDROCK_MODELS, ...opts.modelLists },
    listModelsCalls: [],
    saves: [],
    completeSetupCalls: 0,
    bedrockSaves: [],
    testBedrockCalls: 0,
    bedrockTestAnswer: 'eu.anthropic.claude-haiku-5-5',
  };
}

/** methodID → method name, parsed from one service's generated bindings. */
export function methodIDs(feature: string): Map<number, string> {
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
      case 'Save': {
        // Like the real Save, completed_setup is not the screen's to change,
        // and neither is the Bedrock connection (SetBedrock writes it).
        const sent = structuredClone(args[0] as Record<string, unknown>);
        const kept = state.settings['providers'] as Record<string, string>;
        state.settings = {
          ...sent,
          completed_setup: state.settings['completed_setup'],
          providers: {
            ...(sent['providers'] as Record<string, string>),
            aws_region: kept['aws_region'], bedrock_auth: kept['bedrock_auth'],
            aws_profile: kept['aws_profile'], bedrock_routing: kept['bedrock_routing'],
          },
        };
        state.saves.push(state.settings);
        return empty();
      }
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
      case 'BedrockRegions':
        return json(BEDROCK_REGIONS);
      case 'SetBedrock': {
        // The real SetBedrock's checks (settings/service.go), so a spec
        // cannot pass on a connection the backend would refuse.
        const sent = args[0] as Record<string, string>;
        const b = { ...sent, region: (sent['region'] ?? '').trim(), profile: (sent['profile'] ?? '').trim() };
        const fail = (why: string) => route.fulfill({ status: 500, contentType: 'text/plain', body: `settings: ${why}` });
        if (b.region && !/^[a-z]{2}(-[a-z]+)+-[0-9]+$/.test(b.region)) return fail(`"${b.region}" is not an AWS region`);
        if (!['api-key', 'aws-signin'].includes(b['auth'])) return fail(`"${b['auth']}" is not a way to connect to AWS Bedrock`);
        if (!['geo', 'global'].includes(b['routing'])) return fail(`"${b['routing']}" is not a Bedrock routing`);
        // eslint-disable-next-line no-control-regex
        if (b.profile.length > 128 || /[\x00-\x1f\x7f]/.test(b.profile)) return fail('that is not an AWS profile name');
        state.bedrockSaves.push({ ...b });
        state.settings = {
          ...state.settings,
          providers: {
            ...(state.settings['providers'] as Record<string, string>),
            aws_region: b.region, bedrock_auth: b['auth'], aws_profile: b.profile, bedrock_routing: b['routing'],
          },
        };
        state.saves.push(state.settings);
        return empty();
      }
      case 'TestBedrock':
        state.testBedrockCalls++;
        if (state.failTestBedrock) {
          return route.fulfill({ status: 500, contentType: 'text/plain', body: state.failTestBedrock });
        }
        return json(state.bedrockTestAnswer);
      case 'ListModels':
        state.listModelsCalls.push(args[0] as string);
        return json({
          models: state.modelLists[args[0] as string] ?? [],
          source: state.modelSources[args[0] as string] ?? 'unreachable',
        });
      default:
        return route.fallback();
    }
  });
}
