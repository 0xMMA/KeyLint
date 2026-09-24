import { Component, OnInit, OnDestroy, ChangeDetectorRef } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { ToggleSwitchModule } from 'primeng/toggleswitch';
import { Tabs, TabList, Tab, TabPanels, TabPanel } from 'primeng/tabs';
import { MessageModule } from 'primeng/message';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';
import { ActivatedRoute } from '@angular/router';
import { versionLabel } from '../../core/version-label';
import { WailsService, Settings as AppSettings, KeyStatus, UpdateInfo, AppPreset, ClaudeCodeStatus, ModelInfo } from '../../core/wails.service';
import { noteForModelSource } from '../../core/model-source';
import { DOCUMENT_TYPE_OPTIONS, unavailableProviderName } from '../../core/constants';
import { LogService } from '../../core/log.service';

/**
 * Lets a user defer to KeyLint's default without knowing a model name.
 * PrimeNG only renders a placeholder while the value is null or undefined and
 * this component writes "", so an explicit option is what makes it visible.
 */
/**
 * One entry in a model picker.
 *
 * `label` is deliberately the model ID, not the display name: PrimeNG writes
 * optionLabel into the editable input and submits whatever stands there as the
 * value, so a display name there would be saved as a model ID that no provider
 * knows. The readable name lives in `display` and is rendered by the option
 * template instead.
 */
interface ModelOption {
  id: string;
  label: string;
  display: string;
}

const DEFAULT_MODEL_OPTION: ModelOption = { id: '', label: '', display: "KeyLint's default" };
const DEFAULT_MODEL_ONLY: ModelOption[] = [DEFAULT_MODEL_OPTION];

/** Picker entry for one model the provider reported. */
function toModelOption(model: ModelInfo): ModelOption {
  return { id: model.id, label: model.id, display: model.label || model.id };
}

interface ProviderKey {
  id: string;
  label: string;
  status: KeyStatus | null;
  editing: boolean;
  draftKey: string;
  saving: boolean;
}

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [
    CommonModule, FormsModule,
    ButtonModule, InputTextModule, SelectModule, ToggleSwitchModule,
    Tabs, TabList, Tab, TabPanels, TabPanel, MessageModule, CardModule, TagModule,
  ],
  template: `
    <div class="settings-page">
      <p-card>
        @if (settings) {
          <p-tabs [value]="activeTab">
            <p-tablist>
              <p-tab value="general">General</p-tab>
              <p-tab value="providers">AI Providers</p-tab>
              <p-tab value="app-defaults">App Defaults</p-tab>
              <p-tab value="about">About</p-tab>
            </p-tablist>

            <p-tabpanels>
              <!-- General tab -->
              <p-tabpanel value="general">
                <div class="form-group">
                  <label>Active Provider</label>
                  <p-select
                    data-testid="active-provider-select"
                    [(ngModel)]="settings.active_provider"
                    [options]="providers"
                    optionLabel="label"
                    optionValue="value"
                    placeholder="Choose a provider"
                  />
                  @if (unavailableProvider; as name) {
                    <p-message data-testid="provider-unavailable" severity="warn" size="small">
                      {{ name }} is not available yet, so KeyLint has no provider to use. Choose another one and save.
                    </p-message>
                  }
                </div>
                <div class="form-group">
                  <label>Shortcut Key</label>
                  <input data-testid="shortcut-input" pInputText [(ngModel)]="settings.shortcut_key" placeholder="ctrl+g" />
                </div>
                <div class="form-group" data-testid="start-on-boot-section">
                  <div class="toggle-row">
                    <label>Start on Boot</label>
                    <p-toggle-switch [(ngModel)]="settings.start_on_boot" />
                  </div>
                </div>
                <div class="form-group">
                  <label>Theme</label>
                  <p-select
                    [(ngModel)]="settings.theme_preference"
                    [options]="themes"
                    optionLabel="label"
                    optionValue="value"
                  />
                </div>
                <div class="form-group" data-testid="log-level-section">
                  <label>Log Level</label>
                  <p-select
                    [(ngModel)]="settings.log_level"
                    [options]="logLevels"
                    optionLabel="label"
                    optionValue="value"
                  />
                  <small class="hint-text">Writes to <code>~/.config/KeyLint/debug.log</code> (Linux) or <code>%AppData%/KeyLint/debug.log</code> (Windows). Takes effect on next launch.</small>
                </div>
                <div class="form-group" data-testid="sensitive-logging-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Sensitive Logging</label>
                      <small class="hint-text">Logs full API request payloads and responses. <strong>Do not share the log file while this is enabled.</strong> Takes effect on next launch.</small>
                    </div>
                    <p-toggle-switch [(ngModel)]="settings.sensitive_logging" [disabled]="settings.log_level === 'off'" />
                  </div>
                </div>
              </p-tabpanel>

              <!-- AI Providers / Keys tab -->
              <p-tabpanel value="providers">
                <p class="hint-text">
                  Keys are stored in your OS keyring (Windows Credential Manager / libsecret on Linux).
                  Environment variables (<code>OPENAI_API_KEY</code>, <code>ANTHROPIC_API_KEY</code>) take priority and cannot be overridden here.
                </p>

                <!-- Claude Code CLI needs no key: the user signs in themselves. -->
                <div class="key-row" data-testid="claude-code-card">
                  <div class="key-header">
                    <span class="key-label">Claude Code (installed CLI)</span>
                    @if (claudeCodeStatus) {
                      @if (claudeCodeStatus.installed && claudeCodeStatus.loggedIn) {
                        <p-tag data-testid="claude-code-status-tag" value="● signed in" severity="success" />
                      } @else if (claudeCodeStatus.installed) {
                        <p-tag data-testid="claude-code-status-tag" value="not signed in" severity="warn" />
                      } @else {
                        <p-tag data-testid="claude-code-status-tag" value="not installed" severity="secondary" />
                      }
                    }
                  </div>

                  @if (claudeCodeStatus?.installed) {
                    <p class="hint-text" data-testid="claude-code-detected">
                      Detected at <code>{{ claudeCodeStatus!.path }}</code>
                      @if (claudeCodeStatus!.version) {
                        <span> · version {{ claudeCodeStatus!.version }}</span>
                      }
                    </p>
                    <p class="hint-text" data-testid="claude-code-env-hint">
                      KeyLint runs the CLI with your subscription login. API-key variables in your
                      environment (<code>ANTHROPIC_API_KEY</code> and friends) are not passed through,
                      so the CLI uses the account you signed in with.
                    </p>

                    @if (!claudeCodeStatus!.loggedIn) {
                      <p class="hint-text" data-testid="claude-code-signin-hint">
                        Open a terminal, run <code>claude</code>, and sign in. KeyLint never reads or stores your credentials.
                      </p>
                    }
                  } @else if (claudeCodeStatus) {
                    <p class="hint-text" data-testid="claude-code-missing">
                      No Claude Code CLI found on this machine. Install it to use your own subscription instead of an API key.
                    </p>
                  }

                  <div class="key-actions">
                    <p-button
                      data-testid="claude-code-recheck"
                      label="Re-check"
                      icon="pi pi-refresh"
                      severity="secondary"
                      size="small"
                      (onClick)="recheckClaudeCode(true)"
                      [loading]="claudeCodeChecking"
                    />
                  </div>
                </div>

                @for (pk of providerKeys; track pk.id) {
                  <div class="key-row">
                    <div class="key-header">
                      <span class="key-label">{{ pk.label }}</span>
                      @if (pk.status) {
                        @if (pk.status.is_set && pk.status.source === 'env') {
                          <p-tag value="from env var" severity="info" />
                        } @else if (pk.status.is_set) {
                          <p-tag value="● set" severity="success" />
                        } @else {
                          <p-tag value="not set" severity="secondary" />
                        }
                      }
                    </div>

                    @if (pk.editing) {
                      <div class="key-edit">
                        <input pInputText
                          type="password"
                          [(ngModel)]="pk.draftKey"
                          [placeholder]="keyPlaceholder(pk.id)"
                          style="flex:1"
                        />
                        <p-button
                          label="Save"
                          icon="pi pi-check"
                          size="small"
                          (onClick)="saveKey(pk)"
                          [loading]="pk.saving"
                          [disabled]="!pk.draftKey"
                        />
                        <p-button
                          label="Cancel"
                          icon="pi pi-times"
                          severity="secondary"
                          size="small"
                          (onClick)="cancelEdit(pk)"
                        />
                      </div>
                    } @else {
                      <div class="key-actions">
                        @if (pk.status?.source !== 'env') {
                          <p-button
                            [label]="pk.status?.is_set ? 'Update' : 'Set Key'"
                            icon="pi pi-key"
                            severity="secondary"
                            size="small"
                            (onClick)="startEdit(pk)"
                          />
                          @if (pk.status?.is_set) {
                            <p-button
                              label="Clear"
                              icon="pi pi-trash"
                              severity="danger"
                              size="small"
                              (onClick)="clearKey(pk)"
                              [loading]="pk.saving"
                            />
                          }
                        }
                      </div>
                    }
                  </div>
                }

                <!-- Model selection per provider (#33 step 4) -->
                <div class="form-group mt-4">
                  <label>Models</label>
                  <small class="hint-text">
                    Which model each feature uses. Leave a field empty for KeyLint's default.
                    The lists come from the provider; type a name to use one that is not listed.
                  </small>
                </div>

                @for (mp of modelProviders; track mp.id) {
                  <div class="key-row" [attr.data-testid]="'models-' + mp.id">
                    <div class="key-header">
                      <span class="key-label">{{ mp.label }}</span>
                    </div>
                    @if (modelListNote(mp.id); as note) {
                      <small class="hint-text" [attr.data-testid]="'models-note-' + mp.id">{{ note }}</small>
                    }
                    <div class="form-group">
                      <label>Fix model</label>
                      <p-select
                        [attr.data-testid]="'model-fix-' + mp.id"
                        [editable]="allowsFreeText(mp.id)"
                        [options]="optionsFor(mp.id)"
                        optionLabel="label"
                        optionValue="id"
                        [ngModel]="modelFor(mp.id, 'fix')"
                        (ngModelChange)="setModel(mp.id, 'fix', $event)"
                      >
                        <ng-template #item let-option>
                          <span class="model-option-name">{{ option.display }}</span>
                          @if (option.id && option.id !== option.display) {
                            <small class="model-option-id">{{ option.id }}</small>
                          }
                        </ng-template>
                        <ng-template #selectedItem let-option>
                          {{ option?.display || option?.id }}
                        </ng-template>
                      </p-select>
                    </div>
                    <div class="form-group">
                      <label>Pyramidize model</label>
                      <p-select
                        [attr.data-testid]="'model-pyramidize-' + mp.id"
                        [editable]="allowsFreeText(mp.id)"
                        [options]="optionsFor(mp.id)"
                        optionLabel="label"
                        optionValue="id"
                        [ngModel]="modelFor(mp.id, 'pyramidize')"
                        (ngModelChange)="setModel(mp.id, 'pyramidize', $event)"
                      >
                        <ng-template #item let-option>
                          <span class="model-option-name">{{ option.display }}</span>
                          @if (option.id && option.id !== option.display) {
                            <small class="model-option-id">{{ option.id }}</small>
                          }
                        </ng-template>
                        <ng-template #selectedItem let-option>
                          {{ option?.display || option?.id }}
                        </ng-template>
                      </p-select>
                    </div>
                  </div>
                }

                <!-- Ollama URL (not a secret) -->
                <div class="form-group mt-4">
                  <label>Ollama Server URL</label>
                  <input pInputText [(ngModel)]="settings.providers.ollama_url" placeholder="http://localhost:11434" />
                  <small class="hint-text">Only needed when using Ollama as the provider.</small>
                </div>
              </p-tabpanel>

              <!-- App Defaults tab -->
              <p-tabpanel value="app-defaults">
                <div class="form-group mt-4">
                  <label>App Presets</label>
                  @if (presets.length === 0 && !addingPreset) {
                    <p class="hint-text">No app presets saved yet. Use Pyramidize with the global hotkey to detect apps automatically.</p>
                  }
                  @for (preset of presets; track preset.sourceApp) {
                    @if (editingPreset?.sourceApp === preset.sourceApp) {
                      <div class="preset-row editing">
                        <input pInputText [(ngModel)]="editPresetDraft.sourceApp" style="flex:1" />
                        <p-select [(ngModel)]="editPresetDraft.documentType" [options]="docTypeOptions" optionLabel="label" optionValue="value" style="width:140px" />
                        <p-button icon="pi pi-check" size="small" (onClick)="saveEditPreset()" />
                        <p-button icon="pi pi-times" size="small" severity="secondary" (onClick)="cancelEditPreset()" />
                      </div>
                    } @else {
                      <div class="preset-row">
                        <span style="flex:1">{{ preset.sourceApp }}</span>
                        <span class="preset-type">{{ preset.documentType }}</span>
                        <p-button icon="pi pi-pencil" size="small" severity="secondary" (onClick)="startEditPreset(preset)" />
                        <p-button icon="pi pi-trash" size="small" severity="danger" (onClick)="deletePreset(preset.sourceApp)" />
                      </div>
                    }
                  }

                  @if (addingPreset) {
                    <div class="preset-row editing">
                      <input pInputText [(ngModel)]="addPresetDraft.sourceApp" placeholder="App name" style="flex:1" />
                      <p-select [(ngModel)]="addPresetDraft.documentType" [options]="docTypeOptions" optionLabel="label" optionValue="value" style="width:140px" />
                      <p-button icon="pi pi-check" size="small" (onClick)="saveAddPreset()" [disabled]="!addPresetDraft.sourceApp" />
                      <p-button icon="pi pi-times" size="small" severity="secondary" (onClick)="cancelAddPreset()" />
                    </div>
                  } @else {
                    <p-button label="+ Add manually" size="small" severity="secondary" outlined (onClick)="startAddPreset()" />
                  }
                </div>
              </p-tabpanel>

              <!-- About tab -->
              <p-tabpanel value="about">
                <p>KeyLint — Wails v3 + Angular v21</p>
                <p>Built with Go, Angular, and PrimeNG.</p>
                <p data-testid="app-version">Version: {{ versionLabel(appVersion) }}</p>

                <div class="form-group mt-3" data-testid="update-channel-section">
                  <label>Update Channel</label>
                  <p-select
                    [(ngModel)]="settings.update_channel"
                    [options]="updateChannels"
                    optionLabel="label"
                    optionValue="value"
                  />
                  <small class="hint-text">Auto detects from your current version: pre-release versions check for pre-releases, stable versions check for stable only.</small>
                </div>

                <div class="mt-3">
                  <p-button
                    data-testid="check-update-btn"
                    label="Check for Updates"
                    icon="pi pi-refresh"
                    severity="secondary"
                    [loading]="updateChecking"
                    (onClick)="checkForUpdate()"
                  />
                </div>

                @if (updateInfo?.is_available) {
                  <div class="mt-3">
                    <p-message
                      data-testid="update-available-msg"
                      severity="info"
                      [text]="'Update available: v' + updateInfo!.latest_version + (updateInfo!.notes ? ' — ' + updateInfo!.notes : '')"
                      styleClass="mb-2"
                    />
                    <p-button
                      data-testid="install-update-btn"
                      label="Download and Install"
                      icon="pi pi-download"
                      [loading]="updateInstalling"
                      (onClick)="installUpdate()"
                    />
                  </div>
                }

                @if (updateSuccess) {
                  <p-message
                    data-testid="update-success-msg"
                    severity="success"
                    [text]="updateRestartRequired
                      ? 'Update is installing — the app will close shortly.'
                      : 'Update installed! Restart the app to use the new version.'"
                    styleClass="mt-3"
                  />
                }

                @if (updateError) {
                  <p-message
                    data-testid="update-error-msg"
                    severity="error"
                    [text]="updateError"
                    styleClass="mt-3"
                  />
                }
              </p-tabpanel>
            </p-tabpanels>
          </p-tabs>

          @if (saved) {
            <p-message data-testid="saved-banner" severity="success" text="Settings saved!" styleClass="mt-3" />
          }
          @if (keyError) {
            <p-message severity="error" [text]="keyError" styleClass="mt-3" />
          }

          <div class="mt-4 flex gap-3">
            <p-button data-testid="save-btn" label="Save" icon="pi pi-check" (onClick)="save()" />
            <p-button data-testid="reset-btn" label="Reset to Defaults" icon="pi pi-refresh" severity="danger" outlined (onClick)="resetToDefaults()" />
          </div>
        }
      </p-card>
    </div>
  `,
  styles: [`
    .settings-page { padding: 1.5rem; max-width: 700px; }
    .form-group {
      display: flex;
      flex-direction: column;
      gap: 0.4rem;
      margin-bottom: 1.25rem;
    }
    .toggle-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
    }
    .toggle-label-group {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
    }
    .toggle-label-group .hint-text { margin-bottom: 0; }
    label { font-size: 0.875rem; color: var(--p-text-muted-color); }
    input { width: 100%; }

    .key-row {
      border: 1px solid var(--p-content-border-color);
      border-radius: var(--p-border-radius-md, 6px);
      padding: 0.75rem 1rem;
      margin-bottom: 0.75rem;
    }
    .key-header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 0.5rem;
    }
    .key-label {
      font-weight: 600;
      font-size: 0.9rem;
    }
    .key-edit {
      display: flex;
      gap: 0.5rem;
      align-items: center;
      margin-top: 0.5rem;
    }
    .key-actions {
      display: flex;
      gap: 0.5rem;
    }
    .hint-text {
      font-size: 0.8rem;
      color: var(--p-text-muted-color);
      margin-bottom: 1rem;
    }
    .model-option-id {
      display: block;
      font-size: 0.75rem;
      color: var(--p-text-muted-color);
    }
    code {
      background: var(--p-content-hover-background);
      padding: 1px 4px;
      border-radius: 3px;
      font-family: monospace;
      font-size: 0.85em;
    }
    .preset-row {
      display: flex;
      align-items: center;
      gap: 0.5rem;
      padding: 0.5rem 0;
      border-bottom: 1px solid var(--p-content-border-color);
    }
    .preset-type {
      font-size: 0.8rem;
      color: var(--p-text-muted-color);
      text-transform: uppercase;
      width: 80px;
    }
  `],
})
export class SettingsComponent implements OnInit, OnDestroy {
  settings: AppSettings | null = null;
  saved = false;
  keyError = '';
  activeTab = 'general';

  appVersion = '';
  readonly versionLabel = versionLabel;
  updateInfo: UpdateInfo | null = null;
  updateChecking = false;
  updateInstalling = false;
  updateError = '';
  updateSuccess = false;
  updateRestartRequired = false;

  // App Defaults tab state
  presets: AppPreset[] = [];
  qualityThreshold = 0.65;
  editingPreset: AppPreset | null = null;
  editPresetDraft: AppPreset = { sourceApp: '', documentType: 'email' };
  addingPreset = false;
  addPresetDraft: AppPreset = { sourceApp: '', documentType: 'email' };

  readonly providers = [
    { label: 'OpenAI', value: 'openai' },
    { label: 'Anthropic Claude', value: 'claude' },
    { label: 'Claude Code (installed CLI)', value: 'claude-code' },
    { label: 'Ollama (local)', value: 'ollama' },
    // AWS Bedrock stays out until it works (#22, #23). A settings file that
    // still names it is explained, not broken — see unavailableProvider.
  ];

  readonly themes = [
    { label: 'Dark', value: 'dark' },
    { label: 'Light', value: 'light' },
    { label: 'System', value: 'system' },
  ];

  readonly updateChannels = [
    { label: 'Auto (detect from version)', value: '' },
    { label: 'Stable', value: 'stable' },
    { label: 'Pre-release', value: 'pre-release' },
  ];

  readonly logLevels = [
    { label: 'Off', value: 'off' },
    { label: 'Trace', value: 'trace' },
    { label: 'Debug', value: 'debug' },
    { label: 'Info', value: 'info' },
    { label: 'Warning', value: 'warning' },
    { label: 'Error', value: 'error' },
  ];

  readonly docTypeOptions = DOCUMENT_TYPE_OPTIONS;

  /**
   * Providers whose model can be chosen. Derived from the Active Provider list
   * rather than repeated, so the two cannot drift apart.
   *
   * Computed once: a getter would hand the template a new array on every
   * change-detection pass.
   */
  readonly modelProviders = this.providers
    .map(p => ({ id: p.value, label: p.label }));

  /** Picker contents including the leading default entry; see optionsFor. */
  modelSelectOptions: Record<string, ModelOption[]> = {};

  /** Picker contents per provider, and whether they are live or built-in. */
  modelOptions: Record<string, ModelInfo[]> = {};
  modelSource: Record<string, string> = {};

  /** The Ollama URL as last persisted; see save(). */
  private savedOllamaURL = '';

  /** Null until the first detection run finishes. */
  claudeCodeStatus: ClaudeCodeStatus | null = null;
  claudeCodeChecking = false;
  /** Detection can take seconds; the user may navigate away meanwhile. */
  private destroyed = false;

  providerKeys: ProviderKey[] = [
    { id: 'openai',  label: 'OpenAI API Key',      status: null, editing: false, draftKey: '', saving: false },
    { id: 'claude',  label: 'Anthropic API Key',    status: null, editing: false, draftKey: '', saving: false },
  ];

  constructor(
    private readonly route: ActivatedRoute,
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
  ) {}

  async ngOnInit(): Promise<void> {
    this.activeTab = this.route.snapshot.queryParamMap.get('tab') ?? 'general';
    this.settings = await this.wails.loadSettings();
    this.savedOllamaURL = this.settings?.providers?.ollama_url ?? '';
    this.log.info('settings: loaded');
    await this.refreshKeyStatuses();
    this.appVersion = await this.wails.getVersion();
    this.presets = await this.wails.getAppPresets();
    this.qualityThreshold = await this.wails.getQualityThreshold();
    this.cdr.detectChanges();

    // Detection spawns processes, so the rest of the screen must not wait for it.
    void this.recheckClaudeCode();
    // Same for the model lists: four providers, each a round trip.
    void this.loadModelOptions();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  /** See UNAVAILABLE_PROVIDERS: the saved value is explained, not switched. */
  get unavailableProvider(): string | null {
    return unavailableProviderName(this.settings?.active_provider);
  }

  /** Reads the configured model, or "" when the default applies. */
  modelFor(provider: string, feature: 'fix' | 'pyramidize'): string {
    return this.settings?.models?.[provider]?.[feature] ?? '';
  }

  /** Stores a model choice; an empty value means "use KeyLint's default". */
  setModel(provider: string, feature: 'fix' | 'pyramidize', model: string): void {
    if (!this.settings) return;
    const models = { ...(this.settings.models ?? {}) };
    const entry = { fix: '', pyramidize: '', ...(models[provider] ?? {}) };
    entry[feature] = (model ?? '').trim();
    models[provider] = entry;
    this.settings.models = models;
  }

  /**
   * The provider's models, with a leading entry for KeyLint's own default.
   * PrimeNG only renders a placeholder while the value is null or undefined,
   * and this component writes "" — so an explicit option is what makes the
   * default visible and selectable.
   */
  optionsFor(provider: string): ModelOption[] {
    return this.modelSelectOptions[provider] ?? DEFAULT_MODEL_ONLY;
  }

  /**
   * Whether a model outside the list can be typed. The Claude Code CLI's three
   * aliases are the whole list the picker offers, because an alias follows the
   * generation where a pinned API model ID freezes it. The backend still
   * accepts a pinned ID from a hand-edited settings.json and only logs it —
   * this is the picker steering the choice, not a rejection.
   */
  allowsFreeText(provider: string): boolean {
    return provider !== 'claude-code';
  }

  /**
   * What to say about where this picker's list came from, or null when the list
   * needs no explaining.
   *
   * Each case is a different thing for the user to do, so each gets its own
   * sentence. "Not live" as one word would send someone hunting for a network
   * problem when the real answer is that they have not pasted a key, or that
   * the daemon is running fine and has nothing pulled.
   */
  modelListNote(provider: string): string {
    return noteForModelSource(
      this.modelSource[provider],
      provider,
      (this.modelOptions[provider]?.length ?? 0) > 0,
    );
  }

  /**
   * Counts reload rounds. Saving a key, clearing one and changing the Ollama URL
   * each start a round, and a user doing two of those in a row would otherwise
   * have the slower round land last and overwrite the newer answer.
   */
  private modelLoadRound = 0;

  /** Fetches every provider's model list in parallel; failures fall back. */
  private async loadModelOptions(): Promise<void> {
    const round = ++this.modelLoadRound;
    // Rendered as each provider answers: one that is wedged would otherwise
    // leave all four pickers empty for as long as its timeout.
    await Promise.all(this.modelProviders.map(async mp => {
      const list = await this.wails.listModels(mp.id).catch(() => null);
      if (round !== this.modelLoadRound) return;
      this.modelOptions[mp.id] = list?.models ?? [];
      this.modelSelectOptions[mp.id] = [DEFAULT_MODEL_OPTION, ...(list?.models ?? []).map(toModelOption)];
      this.modelSource[mp.id] = list?.source ?? 'unreachable';
      if (!this.destroyed) {
        this.cdr.detectChanges();
      }
    }));
  }

  /**
   * Loads the Claude Code status, or re-probes it.
   *
   * force belongs to the button, not to the screen opening: a settings visit
   * that always forced a probe would defeat the cache for the caller most
   * likely to be hit repeatedly — Settings → Pyramidize → Settings inside a
   * minute is two navigations and four process spawns.
   */
  async recheckClaudeCode(force = false): Promise<void> {
    this.claudeCodeChecking = true;
    this.cdr.detectChanges();
    try {
      this.claudeCodeStatus = await this.wails.getClaudeCodeStatus(force);
    } finally {
      this.claudeCodeChecking = false;
      // Detection can outlive the screen. Refreshing a destroyed view does NOT
      // throw on this Angular version — measured, see shell-routing.spec.ts —
      // so this guard is about not doing pointless work, not about safety. The
      // comment it replaces claimed the opposite and sent #38's investigation
      // after a phantom.
      if (!this.destroyed) {
        this.cdr.detectChanges();
      }
    }
  }

  private async refreshKeyStatuses(): Promise<void> {
    await Promise.all(
      this.providerKeys.map(async pk => {
        pk.status = await this.wails.getKeyStatus(pk.id);
      }),
    );
  }

  keyPlaceholder(provider: string): string {
    switch (provider) {
      case 'openai':  return 'sk-…';
      case 'claude':  return 'sk-ant-…';
      default:        return 'API key';
    }
  }

  startEdit(pk: ProviderKey): void {
    pk.editing = true;
    pk.draftKey = '';
  }

  cancelEdit(pk: ProviderKey): void {
    pk.editing = false;
    pk.draftKey = '';
  }

  async saveKey(pk: ProviderKey): Promise<void> {
    if (!pk.draftKey) return;
    pk.saving = true;
    this.keyError = '';
    try {
      await this.wails.setKey(pk.id, pk.draftKey);
      pk.editing = false;
      pk.draftKey = '';
      pk.status = await this.wails.getKeyStatus(pk.id);
      this.log.info(`settings: key saved for ${pk.id}`);
    } catch (e) {
      this.keyError = `Failed to save key: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      pk.saving = false;
    }
    // The model list depends on this key.
    void this.loadModelOptions();
  }

  async clearKey(pk: ProviderKey): Promise<void> {
    pk.saving = true;
    this.keyError = '';
    try {
      await this.wails.deleteKey(pk.id);
      pk.status = await this.wails.getKeyStatus(pk.id);
      this.log.info(`settings: key cleared for ${pk.id}`);
    } catch (e) {
      this.keyError = `Failed to clear key: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      pk.saving = false;
    }
    // The model list depends on this key.
    void this.loadModelOptions();
  }

  async checkForUpdate(): Promise<void> {
    this.updateChecking = true;
    this.updateError = '';
    this.updateInfo = null;
    try {
      this.updateInfo = await this.wails.checkForUpdate();
      if (!this.updateInfo.is_available) {
        this.updateError = 'You are already on the latest version.';
      }
    } catch (e) {
      this.updateError = `Update check failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.updateChecking = false;
      this.cdr.detectChanges();
    }
  }

  async installUpdate(): Promise<void> {
    this.updateInstalling = true;
    this.updateError = '';
    this.updateSuccess = false;
    this.updateRestartRequired = false;
    try {
      const result = await this.wails.downloadAndInstall();
      this.updateSuccess = true;
      this.updateRestartRequired = result.restart_required;
      this.updateInfo = null;
    } catch (e) {
      this.updateError = `Install failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.updateInstalling = false;
      this.cdr.detectChanges();
    }
  }

  async save(): Promise<void> {
    if (!this.settings) return;
    const ollamaURLChanged = this.settings.providers?.ollama_url !== this.savedOllamaURL;
    await this.wails.saveSettings(this.settings);
    this.savedOllamaURL = this.settings.providers?.ollama_url ?? '';
    this.log.info('settings: saved');
    if (ollamaURLChanged) {
      // A daemon at another address has other models pulled, so the picker
      // would otherwise keep showing the old machine's list.
      void this.loadModelOptions();
    }
    this.saved = true;
    this.cdr.detectChanges();
    setTimeout(() => { this.saved = false; this.cdr.detectChanges(); }, 3000);
  }

  async resetToDefaults(): Promise<void> {
    await this.wails.resetSettings();
    this.settings = await this.wails.loadSettings();
    this.savedOllamaURL = this.settings?.providers?.ollama_url ?? '';
    // A reset puts the Ollama URL back to its default, so the pickers are now
    // showing whatever the previous address had pulled.
    void this.loadModelOptions();
    this.saved = true;
    this.cdr.detectChanges();
    setTimeout(() => { this.saved = false; this.cdr.detectChanges(); }, 3000);
  }

  // ── App Defaults tab methods ──

  async saveThreshold(): Promise<void> {
    await this.wails.setQualityThreshold(this.qualityThreshold);
  }

  startEditPreset(preset: AppPreset): void {
    this.editingPreset = preset;
    this.editPresetDraft = { ...preset };
  }

  cancelEditPreset(): void {
    this.editingPreset = null;
  }

  async saveEditPreset(): Promise<void> {
    await this.wails.setAppPreset(this.editPresetDraft);
    this.presets = await this.wails.getAppPresets();
    this.editingPreset = null;
    this.cdr.detectChanges();
  }

  async deletePreset(sourceApp: string): Promise<void> {
    await this.wails.deleteAppPreset(sourceApp);
    this.presets = await this.wails.getAppPresets();
    this.cdr.detectChanges();
  }

  startAddPreset(): void {
    this.addingPreset = true;
    this.addPresetDraft = { sourceApp: '', documentType: 'email' };
  }

  cancelAddPreset(): void {
    this.addingPreset = false;
  }

  async saveAddPreset(): Promise<void> {
    if (!this.addPresetDraft.sourceApp) return;
    await this.wails.setAppPreset(this.addPresetDraft);
    this.presets = await this.wails.getAppPresets();
    this.addingPreset = false;
    this.cdr.detectChanges();
  }
}
