import { Injectable, OnDestroy, isDevMode } from '@angular/core';
import { Subject, Observable } from 'rxjs';
import { Events } from '@wailsio/runtime';

// Generated bindings — auto-updated by `wails3 generate bindings`
import * as SettingsService from '../../../bindings/keylint/internal/features/settings/service.js';
import * as WelcomeService from '../../../bindings/keylint/internal/features/welcome/service.js';
import * as ClipboardService from '../../../bindings/keylint/internal/features/clipboard/service.js';
import * as SimulateService from '../../../bindings/keylint/simulateservice.js';
import * as EnhanceService from '../../../bindings/keylint/internal/features/enhance/service.js';
import * as UpdaterService from '../../../bindings/keylint/internal/features/updater/service.js';
import * as LoggerService from '../../../bindings/keylint/internal/features/logger/service.js';
import * as PyramidizeService from '../../../bindings/keylint/internal/features/pyramidize/service.js';
import { Settings, KeyStatus } from '../../../bindings/keylint/internal/features/settings/models.js';
import { UpdateInfo } from '../../../bindings/keylint/internal/features/updater/models.js';
import type { PyramidizeRequest, PyramidizeResult, RefineGlobalRequest, RefineGlobalResult, SpliceRequest, SpliceResult, AppPreset } from '../../../bindings/keylint/internal/features/pyramidize/models.js';

export type { Settings, KeyStatus, UpdateInfo };
export type { PyramidizeRequest, PyramidizeResult, RefineGlobalRequest, RefineGlobalResult, SpliceRequest, SpliceResult, AppPreset };


// Default settings used when the Wails backend is unavailable (browser dev / Playwright mode).
const BROWSER_MODE_DEFAULTS: Settings = {
  active_provider: 'claude',
  providers: { ollama_url: '', aws_region: '' },
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

@Injectable({ providedIn: 'root' })
export class WailsService implements OnDestroy {
  private readonly shortcutTriggered = new Subject<string>();
  private readonly settingsChanged = new Subject<void>();
  private readonly unsubscribers: Array<() => void> = [];

  /** Emits whenever the global shortcut fires (real hotkey or simulated). */
  readonly shortcutTriggered$: Observable<string> = this.shortcutTriggered.asObservable();
  /** Emits whenever settings are saved from the backend. */
  readonly settingsChanged$: Observable<void> = this.settingsChanged.asObservable();

  constructor() {
    this.listenToEvents();
  }

  private listenToEvents(): void {
    this.unsubscribers.push(
      Events.On('shortcut:triggered', (ev) => {
        this.shortcutTriggered.next(ev.data as string);
      }),
      Events.On('settings:changed', () => {
        this.settingsChanged.next();
      }),
    );
  }

  loadSettings(): Promise<Settings> {
    try {
      return SettingsService.Get().catch(() => ({ ...BROWSER_MODE_DEFAULTS }));
    } catch {
      return Promise.resolve({ ...BROWSER_MODE_DEFAULTS });
    }
  }

  saveSettings(s: Settings): Promise<void> {
    return SettingsService.Save(s);
  }

  isFirstRun(): Promise<boolean> {
    try {
      return WelcomeService.IsFirstRun().catch(() => false);
    } catch {
      return Promise.resolve(false);
    }
  }

  completeSetup(): Promise<void> {
    return WelcomeService.CompleteSetup();
  }

  readClipboard(): Promise<string> {
    return ClipboardService.Read();
  }

  writeClipboard(text: string): Promise<void> {
    return ClipboardService.Write(text);
  }

  pasteToForeground(): Promise<void> {
    try {
      return ClipboardService.PasteToForeground().catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  getKeyStatus(provider: string): Promise<KeyStatus> {
    try {
      return SettingsService.GetKeyStatus(provider).catch(() => ({ is_set: false, source: 'none' }));
    } catch {
      return Promise.resolve({ is_set: false, source: 'none' });
    }
  }

  getKey(provider: string): Promise<string> {
    try {
      // Do NOT swallow async errors here — let Wails RPC failures surface as real errors
      // so "key not configured" is not confused with "key retrieval failed".
      // Only the synchronous catch handles browser mode (Wails runtime unavailable).
      return SettingsService.GetKey(provider);
    } catch {
      return Promise.resolve(''); // Browser/Playwright mode: Wails runtime not initialised
    }
  }

  setKey(provider: string, key: string): Promise<void> {
    try {
      return SettingsService.SetKey(provider, key).catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  deleteKey(provider: string): Promise<void> {
    try {
      return SettingsService.DeleteKey(provider).catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  resetSettings(): Promise<void> {
    try {
      return SettingsService.ResetToDefaults().catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  enhance(text: string): Promise<string> {
    // API call made from Go — avoids WebKit WebView network-policy issues on Linux.
    return EnhanceService.Enhance(text);
  }

  getVersion(): Promise<string> {
    try {
      return UpdaterService.GetVersion().catch(() => 'dev');
    } catch {
      return Promise.resolve('dev');
    }
  }

  checkForUpdate(): Promise<UpdateInfo> {
    return UpdaterService.CheckForUpdate();
  }

  downloadAndInstall(): Promise<void> {
    return UpdaterService.DownloadAndInstall();
  }

  simulateShortcut(): Promise<void> {
    if (!isDevMode()) return Promise.resolve();
    return SimulateService.SimulateShortcut();
  }

  log(level: string, msg: string): Promise<void> {
    try {
      return LoggerService.Log(level, msg).catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  // ── Pyramidize RPCs ────────────────────────────────────────────────────────

  pyramidize(req: PyramidizeRequest): Promise<PyramidizeResult> {
    return PyramidizeService.Pyramidize(req);
  }

  refineGlobal(req: RefineGlobalRequest): Promise<RefineGlobalResult> {
    return PyramidizeService.RefineGlobal(req);
  }

  splice(req: SpliceRequest): Promise<SpliceResult> {
    return PyramidizeService.Splice(req);
  }

  cancelOperation(): Promise<void> {
    try {
      return PyramidizeService.CancelOperation().catch(() => {});
    } catch {
      return Promise.resolve();
    }
  }

  sendBack(text: string): Promise<void> {
    return PyramidizeService.SendBack(text);
  }

  getSourceApp(): Promise<string> {
    try {
      return PyramidizeService.GetSourceApp().catch(() => '');
    } catch {
      return Promise.resolve('');
    }
  }

  getAppPresets(): Promise<AppPreset[]> {
    try {
      return PyramidizeService.GetAppPresets().catch(() => []);
    } catch {
      return Promise.resolve([]);
    }
  }

  setAppPreset(preset: AppPreset): Promise<void> {
    return PyramidizeService.SetAppPreset(preset);
  }

  deleteAppPreset(sourceApp: string): Promise<void> {
    return PyramidizeService.DeleteAppPreset(sourceApp);
  }

  getQualityThreshold(): Promise<number> {
    try {
      return PyramidizeService.GetQualityThreshold().catch(() => 0.65);
    } catch {
      return Promise.resolve(0.65);
    }
  }

  setQualityThreshold(v: number): Promise<void> {
    return PyramidizeService.SetQualityThreshold(v);
  }

  ngOnDestroy(): void {
    this.unsubscribers.forEach(fn => fn());
    this.shortcutTriggered.complete();
    this.settingsChanged.complete();
  }
}
