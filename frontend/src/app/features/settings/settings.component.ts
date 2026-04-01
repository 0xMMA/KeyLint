import { Component, OnInit, ChangeDetectorRef } from '@angular/core';
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
import { WailsService, Settings as AppSettings, KeyStatus, UpdateInfo, AppPreset } from '../../core/wails.service';
import { DOCUMENT_TYPE_OPTIONS } from '../../core/constants';
import { LogService } from '../../core/log.service';

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
                    [(ngModel)]="settings.active_provider"
                    [options]="providers"
                    optionLabel="label"
                    optionValue="value"
                  />
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
                <div class="form-group" data-testid="debug-logging-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Debug Logging</label>
                      <small class="hint-text">When enabled, writes a <code>debug.log</code> to the app config folder. Takes effect on next launch.</small>
                    </div>
                    <p-toggle-switch [(ngModel)]="settings.debug_logging" />
                  </div>
                </div>
                <div class="form-group" data-testid="sensitive-logging-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Sensitive Logging</label>
                      <small class="hint-text">Logs full API request payloads and responses. <strong>Do not share the log file while this is enabled.</strong> Takes effect on next launch.</small>
                    </div>
                    <p-toggle-switch [(ngModel)]="settings.sensitive_logging" [disabled]="!settings.debug_logging" />
                  </div>
                </div>
              </p-tabpanel>

              <!-- AI Providers / Keys tab -->
              <p-tabpanel value="providers">
                <p class="hint-text">
                  Keys are stored in your OS keyring (Windows Credential Manager / libsecret on Linux).
                  Environment variables (<code>OPENAI_API_KEY</code>, <code>ANTHROPIC_API_KEY</code>) take priority and cannot be overridden here.
                </p>

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
                <p data-testid="app-version">Version: {{ appVersion }}</p>

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
                    text="Update installed! Restart the app to use the new version."
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
export class SettingsComponent implements OnInit {
  settings: AppSettings | null = null;
  saved = false;
  keyError = '';
  activeTab = 'general';

  appVersion = '';
  updateInfo: UpdateInfo | null = null;
  updateChecking = false;
  updateInstalling = false;
  updateError = '';
  updateSuccess = false;

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
    { label: 'Ollama (local)', value: 'ollama' },
    { label: 'AWS Bedrock', value: 'bedrock' },
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

  readonly docTypeOptions = DOCUMENT_TYPE_OPTIONS;

  providerKeys: ProviderKey[] = [
    { id: 'openai',  label: 'OpenAI API Key',      status: null, editing: false, draftKey: '', saving: false },
    { id: 'claude',  label: 'Anthropic API Key',    status: null, editing: false, draftKey: '', saving: false },
    { id: 'bedrock', label: 'AWS Secret Access Key', status: null, editing: false, draftKey: '', saving: false },
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
    this.log.info('settings: loaded');
    await this.refreshKeyStatuses();
    this.appVersion = await this.wails.getVersion();
    this.presets = await this.wails.getAppPresets();
    this.qualityThreshold = await this.wails.getQualityThreshold();
    this.cdr.detectChanges();
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
      case 'bedrock': return 'AWS secret access key';
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
    try {
      await this.wails.downloadAndInstall();
      this.updateSuccess = true;
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
    await this.wails.saveSettings(this.settings);
    this.log.info('settings: saved');
    this.saved = true;
    this.cdr.detectChanges();
    setTimeout(() => { this.saved = false; this.cdr.detectChanges(); }, 3000);
  }

  async resetToDefaults(): Promise<void> {
    await this.wails.resetSettings();
    this.settings = await this.wails.loadSettings();
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
