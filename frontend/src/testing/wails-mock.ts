import { Subject } from 'rxjs';
import { vi } from 'vitest';
import type { Settings, KeyStatus, UpdateInfo, InstallResult, ClaudeCodeStatus, ModelList, BuildIdentity, DevChannel, SilentFixNotice, BedrockRegion } from '../app/core/wails.service';

export const defaultSettings: Settings = {
  active_provider: 'openai',
  models: {},
  providers: {
    ollama_url: '',
    aws_region: '',
    bedrock_auth: '',
    aws_profile: '',
    bedrock_routing: '',
  },
  shortcut_key: 'ctrl+g',
  shortcut_mode: 'double_tap',
  shortcut_fix: 'ctrl+g',
  shortcut_pyramidize: 'ctrl+shift+g',
  shortcut_double_tap_delay: 200,
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  developer_options: false,
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};

export const defaultKeyStatus: KeyStatus = { is_set: false, source: 'none' };

/** Default: the built-in list, which is what a picker shows offline. */
export const defaultModelList: ModelList = {
  models: [
    // An alias leads, as the Anthropic listing puts it, naming what it
    // resolves to today.
    { id: 'sonnet', label: 'Sonnet (latest)', resolved: 'claude-sonnet-5-5' },
    { id: 'claude-sonnet-4-6', label: 'Sonnet 4.6', resolved: '' },
    { id: 'claude-haiku-4-5-20251001', label: 'Haiku 4.5', resolved: '' },
  ],
  source: 'unreachable',
};

/**
 * A few of the regions the backend lists (llm.BedrockRegions): one per kind —
 * a geography, a geography Sonnet 5.5 cannot stay in, and worldwide only.
 */
export const bedrockRegions: BedrockRegion[] = [
  { id: 'eu-central-1', label: 'Europe (Frankfurt)', geography: 'eu', geography_name: 'the EU',
    geo_models: ['anthropic.claude-haiku-5-5', 'anthropic.claude-sonnet-5-5', 'anthropic.claude-opus-5-5', 'anthropic.claude-sonnet-5', 'anthropic.claude-haiku-4-5-20251001-v1:0', 'anthropic.claude-opus-4-8'] },
  { id: 'us-east-1', label: 'US East (N. Virginia)', geography: 'us', geography_name: 'the US',
    geo_models: ['anthropic.claude-haiku-5-5', 'anthropic.claude-sonnet-5-5', 'anthropic.claude-opus-5-5', 'anthropic.claude-sonnet-5', 'anthropic.claude-haiku-4-5-20251001-v1:0', 'anthropic.claude-opus-4-8'] },
  { id: 'ap-northeast-1', label: 'Asia Pacific (Tokyo)', geography: 'jp', geography_name: 'Japan',
    geo_models: ['anthropic.claude-haiku-5-5', 'anthropic.claude-opus-5-5', 'anthropic.claude-haiku-4-5-20251001-v1:0', 'anthropic.claude-opus-4-8'] },
  { id: 'ap-south-1', label: 'Asia Pacific (Mumbai)', geography: '', geography_name: '', geo_models: [] },
];

/** The Bedrock picker's list, as llm.ListModels answers it (source "known"). */
export const bedrockModelList: ModelList = {
  models: [
    { id: 'anthropic.claude-haiku-5-5', label: 'Claude Haiku 5.5', resolved: '' },
    { id: 'anthropic.claude-sonnet-5-5', label: 'Claude Sonnet 5.5', resolved: '' },
    { id: 'anthropic.claude-opus-5-5', label: 'Claude Opus 5.5', resolved: '' },
  ],
  source: 'known',
};

/** Default: no Claude Code CLI on the machine, so tests exercise the BYOK path. */
export const defaultClaudeCodeStatus: ClaudeCodeStatus = {
  installed: false,
  path: '',
  version: '',
  loggedIn: false,
};

export const defaultUpdateInfo: UpdateInfo = {
  is_available: false,
  latest_version: '',
  current_version: '3.6.0',
  release_url: '',
  notes: '',
  channel: '',
};

/** Default: a release build, not one from the dev channel. */
export const defaultBuildIdentity: BuildIdentity = { is_dev_build: false, kind: '', pr: 0, commit: '', tag: '' };

/** Default: nothing published on the dev channel. */
export const defaultDevChannel: DevChannel = {
  current: { ...defaultBuildIdentity },
  builds: [],
  orphaned: false,
  latest_release: '',
  latest_release_date: '',
  new_release_since_build: false,
  error: '',
};

export function createWailsMock() {
  const silentFixOpen$ = new Subject<SilentFixNotice>();
  const shortcutPyramidize$ = new Subject<string>();
  const settingsChanged$ = new Subject<void>();

  return {
    silentFixOpen$: silentFixOpen$.asObservable(),
    shortcutPyramidize$: shortcutPyramidize$.asObservable(),
    settingsChanged$: settingsChanged$.asObservable(),
    // Expose subjects so tests can trigger events
    _silentFixOpen$: silentFixOpen$,
    _shortcutPyramidize$: shortcutPyramidize$,
    _settingsChanged$: settingsChanged$,

    loadSettings: vi.fn().mockResolvedValue({ ...defaultSettings }),
    saveSettings: vi.fn().mockResolvedValue(undefined),
    setActiveProvider: vi.fn().mockResolvedValue(undefined),
    isFirstRun: vi.fn().mockResolvedValue(false),
    completeSetup: vi.fn().mockResolvedValue(undefined),
    readClipboard: vi.fn().mockResolvedValue('clipboard text'),
    writeClipboard: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue('Enhanced text.'),
    setShortcutPaused: vi.fn().mockResolvedValue(undefined),
    takePendingPyramidize: vi.fn().mockReturnValue(false),
    simulateShortcut: vi.fn().mockResolvedValue(undefined),
    getKeyStatus: vi.fn().mockResolvedValue({ ...defaultKeyStatus }),
    getKey: vi.fn().mockResolvedValue(''),
    setKey: vi.fn().mockResolvedValue(undefined),
    deleteKey: vi.fn().mockResolvedValue(undefined),
    resetSettings: vi.fn().mockResolvedValue(undefined),
    getClaudeCodeStatus: vi.fn().mockResolvedValue({ ...defaultClaudeCodeStatus }),
    setBedrock: vi.fn().mockResolvedValue(undefined),
    getBedrockRegions: vi.fn().mockResolvedValue(bedrockRegions.map(r => ({ ...r }))),
    testBedrock: vi.fn().mockResolvedValue('eu.anthropic.claude-haiku-5-5'),
    listModels: vi.fn().mockResolvedValue({ ...defaultModelList }),
    getVersion: vi.fn().mockResolvedValue('3.6.0'),
    checkForUpdate: vi.fn().mockResolvedValue({ ...defaultUpdateInfo }),
    downloadAndInstall: vi.fn().mockResolvedValue({ restart_required: false }),
    setDeveloperOptions: vi.fn().mockResolvedValue(undefined),
    getBuildIdentity: vi.fn().mockResolvedValue({ ...defaultBuildIdentity }),
    listDevBuilds: vi.fn().mockResolvedValue({ ...defaultDevChannel, builds: [] }),
    installDevBuild: vi.fn().mockResolvedValue({ restart_required: true } as InstallResult),
    installLatestRelease: vi.fn().mockResolvedValue({ restart_required: true } as InstallResult),
    log: vi.fn().mockResolvedValue(undefined),
    pasteToForeground: vi.fn().mockResolvedValue(undefined),
    ngOnDestroy: vi.fn(),

    // Pyramidize mocks
    pyramidize: vi.fn().mockResolvedValue({
      documentType: 'EMAIL',
      language: 'en',
      fullDocument: 'Subject | Details | Actions\n\nBody text',
    }),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: 'Refined canvas text' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: 'Rewritten section' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    getSourcePreset: vi.fn().mockResolvedValue({ app: '', documentType: 'email', remembered: false }),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
  };
}

export type WailsMock = ReturnType<typeof createWailsMock>;
