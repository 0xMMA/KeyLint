import { Subject } from 'rxjs';
import { vi } from 'vitest';
import type { Settings, KeyStatus, UpdateInfo, InstallResult, ClaudeCodeStatus, ModelList } from '../app/core/wails.service';

export const defaultSettings: Settings = {
  active_provider: 'openai',
  models: {},
  providers: {
    ollama_url: '',
    aws_region: '',
  },
  shortcut_key: 'ctrl+g',
  start_on_boot: false,
  theme_preference: 'dark',
  completed_setup: false,
  log_level: 'off',
  sensitive_logging: false,
  update_channel: '',
  app_presets: [],
  pyramidize_quality_threshold: 0.65,
};

export const defaultKeyStatus: KeyStatus = { is_set: false, source: 'none' };

/** Default: the built-in list, which is what a picker shows offline. */
export const defaultModelList: ModelList = {
  models: [
    { id: 'claude-sonnet-4-6', label: 'Sonnet 4.6' },
    { id: 'claude-haiku-4-5-20251001', label: 'Haiku 4.5' },
  ],
  source: 'unreachable',
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

export function createWailsMock() {
  const shortcutTriggered$ = new Subject<string>();
  const settingsChanged$ = new Subject<void>();

  return {
    shortcutTriggered$: shortcutTriggered$.asObservable(),
    settingsChanged$: settingsChanged$.asObservable(),
    // Expose subjects so tests can trigger events
    _shortcutTriggered$: shortcutTriggered$,
    _settingsChanged$: settingsChanged$,

    loadSettings: vi.fn().mockResolvedValue({ ...defaultSettings }),
    saveSettings: vi.fn().mockResolvedValue(undefined),
    isFirstRun: vi.fn().mockResolvedValue(false),
    completeSetup: vi.fn().mockResolvedValue(undefined),
    readClipboard: vi.fn().mockResolvedValue('clipboard text'),
    writeClipboard: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue('Enhanced text.'),
    simulateShortcut: vi.fn().mockResolvedValue(undefined),
    getKeyStatus: vi.fn().mockResolvedValue({ ...defaultKeyStatus }),
    getKey: vi.fn().mockResolvedValue(''),
    setKey: vi.fn().mockResolvedValue(undefined),
    deleteKey: vi.fn().mockResolvedValue(undefined),
    resetSettings: vi.fn().mockResolvedValue(undefined),
    getClaudeCodeStatus: vi.fn().mockResolvedValue({ ...defaultClaudeCodeStatus }),
    listModels: vi.fn().mockResolvedValue({ ...defaultModelList }),
    getVersion: vi.fn().mockResolvedValue('3.6.0'),
    checkForUpdate: vi.fn().mockResolvedValue({ ...defaultUpdateInfo }),
    downloadAndInstall: vi.fn().mockResolvedValue({ restart_required: false }),
    log: vi.fn().mockResolvedValue(undefined),
    pasteToForeground: vi.fn().mockResolvedValue(undefined),
    ngOnDestroy: vi.fn(),

    // Pyramidize mocks
    pyramidize: vi.fn().mockResolvedValue({
      documentType: 'EMAIL',
      language: 'en',
      fullDocument: 'Subject | Details | Actions\n\nBody text',
      headers: ['Header 1'],
      qualityScore: 0.9,
      qualityFlags: [],
      appliedRefinement: false,
      refinementWarning: '',
      detectedType: 'EMAIL',
      detectedLang: 'en',
      detectedConfidence: 0.95,
    }),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: 'Refined canvas text' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: 'Rewritten section' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
    getQualityThreshold: vi.fn().mockResolvedValue(0.65),
    setQualityThreshold: vi.fn().mockResolvedValue(undefined),
  };
}

export type WailsMock = ReturnType<typeof createWailsMock>;
