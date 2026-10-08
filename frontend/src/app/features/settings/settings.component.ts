import { Component, OnInit, OnDestroy, ChangeDetectorRef, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { InputTextModule } from 'primeng/inputtext';
import { SelectModule } from 'primeng/select';
import { ToggleSwitchModule, ToggleSwitch } from 'primeng/toggleswitch';
import { SliderModule } from 'primeng/slider';
import { Tabs, TabList, Tab, TabPanels, TabPanel } from 'primeng/tabs';
import { MessageModule } from 'primeng/message';
import { CardModule } from 'primeng/card';
import { TagModule } from 'primeng/tag';
import { ActivatedRoute } from '@angular/router';
import { versionLabel } from '../../core/version-label';
import { ProviderCardComponent } from './provider-card/provider-card.component';
import { DevChannelComponent } from './dev-channel/dev-channel.component';
import { WailsService, Settings as AppSettings, KeyStatus, UpdateInfo, AppPreset, ClaudeCodeStatus, ModelInfo, BuildIdentity } from '../../core/wails.service';
import { noteForModelSource } from '../../core/model-source';
import { DOCUMENT_TYPE_OPTIONS, unavailableProviderName } from '../../core/constants';
import { LogService } from '../../core/log.service';
import { ShortcutRecorderComponent } from './shortcut-recorder/shortcut-recorder.component';

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

/**
 * Which environment variable supplies each provider's key. The source of truth
 * is `envVars` in internal/features/settings/service.go: keep the two in sync,
 * or a card names a variable the backend does not read.
 */
const ENV_KEY_VARS: Readonly<Record<string, string>> = {
  openai: 'OPENAI_API_KEY',
  claude: 'ANTHROPIC_API_KEY',
};

/** Taps on the version that unlock the developer options, as on Android. */
const UNLOCK_TAPS = 7;

interface ProviderKey {
  id: string;
  status: KeyStatus | null;
  editing: boolean;
  draftKey: string;
  saving: boolean;
}

@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [
    ProviderCardComponent, DevChannelComponent,
    CommonModule, FormsModule,
    ButtonModule, InputTextModule, SelectModule, ToggleSwitchModule, SliderModule,
    Tabs, TabList, Tab, TabPanels, TabPanel, MessageModule, CardModule, TagModule,
    ShortcutRecorderComponent,
  ],
  template: `
    <div class="settings-page">
      <p-card>
        @if (settings) {
          <p-tabs [(value)]="activeTab">
            <p-tablist>
              <p-tab value="general">General</p-tab>
              <p-tab value="providers">AI Providers</p-tab>
              <p-tab value="app-defaults">App Defaults</p-tab>
              <p-tab value="about">About</p-tab>
            </p-tablist>

            <p-tabpanels>
              <!-- General tab -->
              <p-tabpanel value="general">
                <!-- Read-only: the provider is chosen on the AI Providers tab, next
                     to the keys and the CLI status that decide whether it works. -->
                <div class="form-group" data-testid="provider-pointer">
                  <label>AI Provider</label>
                  <div class="provider-pointer">
                    <span data-testid="provider-pointer-text">
                      @if (unavailableProvider) {
                        No provider in use
                      } @else if (activeProviderLabel; as label) {
                        Using: <strong>{{ label }}</strong>
                      } @else {
                        No provider chosen yet
                      }
                    </span>
                    <span class="provider-pointer-sep" aria-hidden="true">—</span>
                    <!-- An action, not a link: it switches the tab, so it is a button. -->
                    <button
                      type="button"
                      class="provider-pointer-link"
                      data-testid="provider-pointer-link"
                      (click)="openProvidersTab()"
                    >change under AI Providers</button>
                  </div>
                  @if (unavailableProvider; as name) {
                    <p-message data-testid="provider-unavailable" severity="warn" size="small">
                      {{ name }} is saved but not available yet, so KeyLint has no provider in use. Pick another one under AI Providers.
                    </p-message>
                  } @else if (activeProviderProblem; as why) {
                    <p-message data-testid="provider-pointer-problem" severity="warn" size="small">{{ why }}</p-message>
                  }
                </div>
                <!-- Shortcuts section -->
                <div class="form-group" data-testid="shortcut-mode-section">
                  <div class="toggle-row">
                    <div class="toggle-label-group">
                      <label>Double-tap mode</label>
                      <small class="hint-text">
                        @if (settings.shortcut_mode === 'double_tap') {
                          Hold your modifier keys, tap the trigger key once for Fix, twice for Pyramidize.
                        } @else {
                          Assign separate shortcuts for each action.
                        }
                      </small>
                    </div>
                    <p-toggle-switch
                      [ngModel]="settings.shortcut_mode === 'double_tap'"
                      (ngModelChange)="settings.shortcut_mode = $event ? 'double_tap' : 'independent'"
                    />
                  </div>
                </div>

                @if (settings.shortcut_mode === 'double_tap') {
                  <div class="form-group" data-testid="shortcut-fix-section">
                    <label>Shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_fix"
                      (valueChange)="settings.shortcut_fix = $event"
                    />
                    <small class="hint-text">Single tap → Fix · Double tap → Pyramidize</small>
                  </div>
                  <div class="form-group" data-testid="shortcut-delay-section">
                    <label>Double-tap delay: {{ settings.shortcut_double_tap_delay }}ms</label>
                    <p-slider
                      [(ngModel)]="settings.shortcut_double_tap_delay"
                      [min]="100"
                      [max]="500"
                      [step]="25"
                    />
                    <small class="hint-text">How long to wait for a second tap. Lower = faster but harder to trigger.</small>
                  </div>
                } @else {
                  <div class="form-group" data-testid="shortcut-fix-section">
                    <label>Fix shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_fix"
                      (valueChange)="settings.shortcut_fix = $event"
                    />
                    <small class="hint-text">Silently fixes clipboard text.</small>
                  </div>
                  <div class="form-group" data-testid="shortcut-pyramidize-section">
                    <label>Pyramidize shortcut</label>
                    <app-shortcut-recorder
                      [value]="settings.shortcut_pyramidize"
                      (valueChange)="settings.shortcut_pyramidize = $event"
                    />
                    <small class="hint-text">Opens the Pyramidize editor with clipboard text.</small>
                  </div>
                }
                <div class="form-group" data-testid="start-on-boot-section">
                  <div class="toggle-row">
                    <label>Start on Boot</label>
                    <p-toggle-switch [(ngModel)]="settings.start_on_boot" />
                  </div>
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
                <!-- Pyramidize follows this choice unless the user picked another
                     there for the session, and says so on its own page. -->
                <p class="provider-summary" data-testid="providers-summary" tabindex="-1">
                  @if (activeProviderLabel; as label) {
                    Fix sends your text to <strong>{{ label }}</strong>.
                    Pyramidize uses it too, unless you pick another provider there for this session.
                    To switch, press <em>Use this</em> on another provider.
                  } @else {
                    No provider is in use. Press <em>Use this</em> on the one KeyLint should send your text to.
                  }
                </p>
                @if (unavailableProvider; as name) {
                  <p-message data-testid="providers-tab-unavailable" severity="warn" size="small" styleClass="mb-3">
                    {{ name }} is saved but not available yet, so KeyLint has no provider in use.
                  </p-message>
                }
                <!-- Screen readers hear a switch land; the marker itself only moves. -->
                <div class="sr-only" aria-live="polite" data-testid="provider-announcement">{{ providerAnnouncement }}</div>
                @if (switchError) {
                  <!-- Focusable, for a failed switch with no card in use to return to.
                       The message is role="alert", so it announces itself. -->
                  <div class="switch-error" tabindex="-1" data-testid="provider-switch-error">
                    <p-message severity="error" size="small" styleClass="mb-3">{{ switchError }}</p-message>
                  </div>
                }
                <p class="hint-text">
                  Keys are stored in your OS keyring (Windows Credential Manager / libsecret on Linux).
                  Environment variables (<code>OPENAI_API_KEY</code>, <code>ANTHROPIC_API_KEY</code>) take priority and cannot be overridden here.
                </p>

                @for (p of providers; track p.value) {
                  <app-provider-card
                    [providerId]="p.value"
                    [label]="p.label"
                    [inUse]="settings.active_provider === p.value"
                    [problem]="providerProblem(p.value)"
                    [switching]="switchingTo === p.value"
                    [locked]="switchingTo !== null || formBusy"
                    (use)="useProvider(p.value)"
                  >
                    <span cardStatus class="card-status">
                      @if (p.value === 'claude-code') {
                        @if (claudeCodeStatus) {
                          @if (claudeCodeStatus.installed && claudeCodeStatus.loggedIn) {
                            <p-tag data-testid="claude-code-status-tag" value="● signed in" severity="success" />
                          } @else if (claudeCodeStatus.installed) {
                            <p-tag data-testid="claude-code-status-tag" value="not signed in" severity="warn" />
                          } @else {
                            <p-tag data-testid="claude-code-status-tag" value="not installed" severity="secondary" />
                          }
                        }
                      } @else if (p.value === 'ollama') {
                        @switch (modelSource['ollama']) {
                          @case ('live') { <p-tag data-testid="ollama-status-tag" value="● running" severity="success" /> }
                          @case ('empty') { <p-tag data-testid="ollama-status-tag" value="no models" severity="warn" /> }
                          @case ('unreachable') { <p-tag data-testid="ollama-status-tag" value="not reachable" severity="secondary" /> }
                        }
                      } @else if (keyFor(p.value)?.status; as status) {
                        @if (status.is_set && status.source === 'env') {
                          <p-tag [attr.data-testid]="'key-status-' + p.value" value="key from env var" severity="info" />
                        } @else if (status.is_set) {
                          <p-tag [attr.data-testid]="'key-status-' + p.value" value="● key set" severity="success" />
                        } @else {
                          <p-tag [attr.data-testid]="'key-status-' + p.value" value="no key" severity="secondary" />
                        }
                      }
                    </span>

                    @if (p.value === 'claude-code') {
                      <!-- Claude Code CLI needs no key: the user signs in themselves. -->
                      <div data-testid="claude-code-card">
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
                    } @else if (p.value === 'ollama') {
                      <!-- Ollama URL (not a secret) -->
                      <div class="form-group ollama-url">
                        <label for="ollama-url">Server URL</label>
                        <input
                          id="ollama-url"
                          data-testid="ollama-url-input"
                          pInputText
                          [(ngModel)]="settings.providers.ollama_url"
                          placeholder="http://localhost:11434"
                        />
                        <small class="hint-text">Leave empty for a local Ollama on the default port. Saved with the Save button below; <em>Use this</em> only switches the provider.</small>
                      </div>
                    } @else if (keyFor(p.value); as pk) {
                      @if (pk.status?.source === 'env') {
                        <p class="hint-text" [attr.data-testid]="'key-env-hint-' + pk.id">
                          Using the key from the <code>{{ envVarFor(pk.id) }}</code> environment variable.
                          It takes priority over a key saved here, so it can only be changed where it is set.
                        </p>
                      }
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
                      } @else if (pk.status?.source !== 'env') {
                        <div class="key-actions">
                          <p-button
                            [label]="pk.status?.is_set ? 'Update Key' : 'Set Key'"
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
                        </div>
                      }
                    }
                  </app-provider-card>
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
                <!-- Tapping the version seven times unlocks the developer options,
                     as on Android. A button, so it also works from the keyboard. -->
                <p data-testid="app-version">Version: <button type="button" class="version-tap" data-testid="version-tap" (click)="onVersionTap()">{{ versionLabel(appVersion) }}</button></p>
                <p class="hint-text unlock-hint" data-testid="unlock-hint" aria-live="polite">{{ unlockHint }}</p>

                @if (settings.developer_options) {
                  <div class="form-group" data-testid="developer-options-section">
                    <div class="toggle-row">
                      <div class="toggle-label-group">
                        <label for="developer-options-toggle">Developer options</label>
                        <small class="hint-text">Shows the dev channel below. Turn off to hide it again; tapping the version seven times brings it back.</small>
                      </div>
                      <p-toggle-switch
                        #developerOptionsToggle
                        inputId="developer-options-toggle"
                        data-testid="developer-options-toggle"
                        [ngModel]="settings.developer_options"
                        (ngModelChange)="setDeveloperOptions($event)"
                      />
                    </div>
                  </div>
                }
                @if (devOptionsError) {
                  <p-message data-testid="developer-options-error" severity="error" [text]="devOptionsError" styleClass="mb-2" />
                }

                <!-- The channel stays settable in a dev build: it decides which
                     release the dev channel's "latest release" offer installs. -->
                <div class="form-group mt-3" data-testid="update-channel-section">
                  <label>Update Channel</label>
                  <p-select
                    [(ngModel)]="settings.update_channel"
                    [options]="updateChannels"
                    optionLabel="label"
                    optionValue="value"
                  />
                  <small class="hint-text">Auto detects from your current version: pre-release versions check for pre-releases, stable versions check for stable only. Test builds count as pre-release.</small>
                </div>

                <!-- A dev build is 0.0.0, so every release would read as an
                     update: the dev channel replaces the normal check there. -->
                @if (!buildIdentity.is_dev_build) {
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
                }

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

                @if (settings.developer_options || buildIdentity.is_dev_build) {
                  <app-dev-channel [version]="appVersion" />
                }
              </p-tabpanel>
            </p-tabpanels>
          </p-tabs>

          @if (saved) {
            <p-message data-testid="saved-banner" severity="success" text="Settings saved!" styleClass="mt-3" />
          }
          @if (saveError) {
            <p-message data-testid="save-error" severity="error" [text]="saveError" styleClass="mt-3" />
          }
          @if (keyError) {
            <p-message severity="error" [text]="keyError" styleClass="mt-3" />
          }

          <div class="mt-4 flex gap-3">
            <!-- Held while a provider switch saves, and the switch is held while
                 these run: one landing in the middle of the other could
                 overwrite it or be overwritten by it. -->
            <p-button data-testid="save-btn" label="Save" icon="pi pi-check" [disabled]="switchingTo !== null || formBusy" (onClick)="save()" />
            <p-button data-testid="reset-btn" label="Reset to Defaults" icon="pi pi-refresh" severity="danger" outlined [disabled]="switchingTo !== null || formBusy" (onClick)="resetToDefaults()" />
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
    /* A long hint beside the switch would otherwise squeeze it narrower than
       its track, and the knob slides out past the edge (#26). */
    .toggle-row p-toggle-switch { flex-shrink: 0; }
    label { font-size: 0.875rem; color: var(--p-text-muted-color); }
    input { width: 100%; }

    .switch-error:focus { outline: none; }
    .provider-summary {
      margin: 0 0 0.75rem;
      font-size: 0.95rem;
    }
    .provider-pointer {
      display: flex;
      flex-wrap: wrap;
      align-items: baseline;
      gap: 0.4rem;
    }
    .provider-pointer-sep { color: var(--p-text-muted-color); }
    .provider-pointer-link {
      background: none;
      border: none;
      padding: 0;
      font: inherit;
      cursor: pointer;
      color: var(--p-primary-color);
      text-decoration: none;
    }
    .provider-pointer-link:hover { text-decoration: underline; }
    .card-status { display: inline-flex; gap: 0.5rem; }
    .ollama-url { margin-bottom: 0; }
    .ollama-url .hint-text { margin-bottom: 0; }

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
    .version-tap {
      background: none;
      border: none;
      padding: 0;
      font: inherit;
      color: inherit;
      cursor: default;
    }
    .unlock-hint { min-height: 1em; }
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
  /** Two-way bound to the tabs, so the General tab's pointer can switch it. */
  activeTab: string | number | undefined = 'general';

  appVersion = '';
  readonly versionLabel = versionLabel;
  /** What the running build says about itself; a release build until asked. */
  buildIdentity: BuildIdentity = { is_dev_build: false, kind: '', pr: 0, commit: '', tag: '' };
  @ViewChild('developerOptionsToggle') private developerOptionsToggle?: ToggleSwitch;
  /** Taps on the version so far, towards UNLOCK_TAPS. */
  private versionTaps = 0;
  unlockHint = '';
  devOptionsError = '';
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

  /**
   * The providers a user can choose, in the order their cards appear on the AI
   * Providers tab. The labels are what the General tab's pointer and the model
   * pickers show too.
   */
  readonly providers = [
    { label: 'OpenAI', value: 'openai' },
    { label: 'Anthropic API', value: 'claude' },
    { label: 'Claude Code (installed CLI)', value: 'claude-code' },
    { label: 'Ollama (local)', value: 'ollama' },
    // AWS Bedrock stays out until it works (#22, #23). A settings file that
    // still names it is explained, not broken — see unavailableProvider.
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
   * Providers whose model can be chosen. Derived from the provider list
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

  /** The provider whose "Use this" is being saved, so one switch runs at a time. */
  switchingTo: string | null = null;
  /** Why the last switch did not stick; cleared by the next one. */
  switchError = '';
  /** What the live region last announced about a switch. */
  providerAnnouncement = '';
  /** Save or Reset is running; a switch must wait for it. */
  formBusy = false;
  /** Why the last Save failed, shown beside the button. */
  saveError = '';

  /** Null until the first detection run finishes. */
  claudeCodeStatus: ClaudeCodeStatus | null = null;
  claudeCodeChecking = false;
  /** Detection can take seconds; the user may navigate away meanwhile. */
  private destroyed = false;

  providerKeys: ProviderKey[] = [
    { id: 'openai', status: null, editing: false, draftKey: '', saving: false },
    { id: 'claude', status: null, editing: false, draftKey: '', saving: false },
  ];

  constructor(
    private readonly route: ActivatedRoute,
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
    private readonly host: ElementRef<HTMLElement>,
  ) {}

  async ngOnInit(): Promise<void> {
    this.activeTab = this.route.snapshot.queryParamMap.get('tab') ?? 'general';
    this.settings = await this.wails.loadSettings();
    this.savedOllamaURL = this.settings?.providers?.ollama_url ?? '';
    this.log.info('settings: loaded');
    await this.refreshKeyStatuses();
    this.appVersion = await this.wails.getVersion();
    this.buildIdentity = await this.wails.getBuildIdentity();
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

  /** The active provider's display name, or null when none of the offered ones is saved. */
  get activeProviderLabel(): string | null {
    const active = this.settings?.active_provider;
    return this.providers.find(p => p.value === active)?.label ?? null;
  }

  /** Why the active provider cannot work right now, for the General tab's pointer. */
  get activeProviderProblem(): string | null {
    const active = this.settings?.active_provider;
    return active ? this.providerProblem(active) : null;
  }

  /**
   * Why a provider cannot work right now, or null when nothing is known to be
   * wrong — including while its status is still loading, so a card does not
   * flash a warning it is about to take back.
   *
   * Ollama is judged by whether its daemon answered, not by whether the URL
   * field is filled: an empty field means localhost on the default port, which
   * is where a local Ollama listens, so "no URL" would be a false alarm.
   */
  providerProblem(provider: string): string | null {
    switch (provider) {
      case 'claude-code': {
        const cli = this.claudeCodeStatus;
        if (!cli) return null;
        if (!cli.installed) return "Won't work yet: the Claude Code CLI is not installed on this machine.";
        if (!cli.loggedIn) return "Won't work yet: the Claude Code CLI is not signed in.";
        return null;
      }
      case 'ollama': {
        const where = this.savedOllamaURL || 'http://localhost:11434';
        switch (this.modelSource['ollama']) {
          case 'unreachable': return `Won't work yet: KeyLint cannot reach Ollama at ${where}. Start Ollama, or check the server URL.`;
          case 'empty': return "Won't work yet: Ollama has no models pulled. Pull one with `ollama pull`.";
          default: return null;
        }
      }
      default: {
        const status = this.keyFor(provider)?.status;
        if (!status || status.is_set) return null;
        return "Won't work yet: no API key is set.";
      }
    }
  }

  /** The key row for an API provider, or undefined for one that takes no key. */
  keyFor(provider: string): ProviderKey | undefined {
    return this.providerKeys.find(pk => pk.id === provider);
  }

  /** The environment variable the backend reads first; see envVars in settings/service.go. */
  envVarFor(provider: string): string {
    return ENV_KEY_VARS[provider] ?? '';
  }

  /**
   * Makes `provider` the one KeyLint uses, saved straight away so one click is
   * the whole switch.
   *
   * Only the provider is saved (SetActiveProvider). Anything else edited on
   * the page stays pending until Save: a switch that also committed a half-typed
   * URL or a Sensitive Logging toggle on a tab out of view would save things
   * the user never confirmed.
   *
   * The marker moves once the backend has the switch, not before, so it never
   * claims a provider the backend does not use.
   */
  async useProvider(provider: string): Promise<void> {
    if (!this.settings || this.switchingTo !== null || this.formBusy) return;
    if (this.settings.active_provider === provider) return;
    const target = this.settings;
    const label = this.providers.find(p => p.value === provider)?.label ?? provider;
    this.switchingTo = provider;
    this.switchError = '';
    this.cdr.detectChanges();
    let switched = false;
    try {
      await this.wails.setActiveProvider(provider);
      switched = true;
      this.log.info(`settings: active provider set to ${provider}`);
    } catch (e) {
      this.switchError = `Could not switch to ${label}: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.switchingTo = null;
    }
    if (switched && this.settings === target) {
      target.active_provider = provider;
      // The switch saved, so whatever the last Save said is no longer the news.
      this.saveError = '';
      this.refreshProviderStatus(provider);
    }
    // Announced once: a failure is already announced by the error message,
    // which is role="alert"; the live region carries only the success.
    this.providerAnnouncement = switched ? `KeyLint now uses ${label}.` : '';
    if (this.destroyed) return;
    this.cdr.detectChanges();
    // The pressed button is gone (on success it became the "In use" tag) or
    // no longer the point (on failure). Focus goes to the heading of the card
    // in use now; after a failure with no card in use (a saved Bedrock, or
    // nothing saved), to the error that explains it.
    const inUse = this.activeProviderLabel ? this.settings?.active_provider : null;
    if (inUse) {
      this.focusCardHeading(inUse);
    } else if (!switched) {
      (this.host.nativeElement as HTMLElement)
        .querySelector<HTMLElement>('[data-testid="provider-switch-error"]')?.focus();
    }
  }

  /**
   * Re-asks about the provider just switched to, so its card does not show an
   * answer from before the user set it up. The CLI probe is forced, the same
   * bypass the card's Re-check uses. Ollama has no such bypass: the backend
   * keeps a failed listing for 30 s, so a daemon started moments ago can still
   * read as unreachable until that passes.
   */
  private refreshProviderStatus(provider: string): void {
    if (provider === 'claude-code') {
      void this.recheckClaudeCode(true);
    } else if (provider === 'ollama') {
      void this.loadModelOptions();
    } else {
      const pk = this.keyFor(provider);
      if (!pk) return;
      void this.wails.getKeyStatus(provider).then(status => {
        pk.status = status;
        if (!this.destroyed) this.cdr.detectChanges();
      }).catch(() => { /* keep the answer the card already has */ });
    }
  }

  /** The General tab's pointer: show the AI Providers tab and move focus there. */
  openProvidersTab(): void {
    this.activeTab = 'providers';
    this.cdr.detectChanges();
    // The pointer sits in the panel that just hid, so focus would be lost.
    (this.host.nativeElement as HTMLElement)
      .querySelector<HTMLElement>('[data-testid="providers-summary"]')?.focus();
  }

  private focusCardHeading(provider: string): void {
    (this.host.nativeElement as HTMLElement)
      .querySelector<HTMLElement>(`[data-testid="provider-heading-${provider}"]`)?.focus();
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

  /**
   * One tap on the version. The last few taps count down, Android-style, and
   * the seventh turns the developer options on and saves that at once — there
   * is no Save to press for an unlock.
   */
  async onVersionTap(): Promise<void> {
    if (!this.settings) return;
    if (this.settings.developer_options) {
      this.unlockHint = 'Developer options are already on.';
      return;
    }
    this.versionTaps++;
    const left = UNLOCK_TAPS - this.versionTaps;
    if (left > 0) {
      if (left <= UNLOCK_TAPS - 3) {
        this.unlockHint = `${left} more ${left === 1 ? 'tap' : 'taps'} to turn on the developer options.`;
      }
      return;
    }
    this.versionTaps = 0;
    if (await this.setDeveloperOptions(true)) {
      this.unlockHint = 'Developer options are on.';
    }
  }

  /** Saves the developer options switch on its own; reports whether it stuck. */
  async setDeveloperOptions(enabled: boolean): Promise<boolean> {
    if (!this.settings) return false;
    this.devOptionsError = '';
    try {
      await this.wails.setDeveloperOptions(enabled);
      this.settings.developer_options = enabled;
      if (!enabled) this.unlockHint = '';
      this.log.info(`settings: developer options ${enabled ? 'on' : 'off'}`);
      return true;
    } catch (e) {
      // The switch has already moved, and its [ngModel] input has not
      // changed, so Angular will not move it back: do it here, so it shows
      // what is saved rather than what was asked for.
      this.developerOptionsToggle?.writeValue(this.settings.developer_options);
      this.devOptionsError = `Could not change the developer options: ${e instanceof Error ? e.message : String(e)}`;
      return false;
    } finally {
      if (!this.destroyed) this.cdr.detectChanges();
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
    if (!this.settings || this.formBusy) return;
    this.switchError = '';
    this.saveError = '';
    const ollamaURLChanged = this.settings.providers?.ollama_url !== this.savedOllamaURL;
    this.formBusy = true;
    try {
      await this.wails.saveSettings(this.settings);
    } catch (e) {
      this.saveError = `Could not save settings: ${e instanceof Error ? e.message : String(e)}`;
      return;
    } finally {
      this.formBusy = false;
      if (!this.destroyed) this.cdr.detectChanges();
    }
    this.savedOllamaURL = this.settings.providers?.ollama_url ?? '';
    this.log.info('settings: saved');
    if (ollamaURLChanged) {
      // Until the new address answers, nothing is known about it; the old
      // address's "unreachable" must not be shown under the new one.
      delete this.modelSource['ollama'];
      // A daemon at another address has other models pulled, so the picker
      // would otherwise keep showing the old machine's list.
      void this.loadModelOptions();
    }
    this.saved = true;
    this.cdr.detectChanges();
    setTimeout(() => { this.saved = false; this.cdr.detectChanges(); }, 3000);
  }

  async resetToDefaults(): Promise<void> {
    if (this.formBusy) return;
    this.formBusy = true;
    this.saveError = '';
    try {
      await this.wails.resetSettings();
      this.settings = await this.wails.loadSettings();
    } catch (e) {
      this.saveError = `Could not reset settings: ${e instanceof Error ? e.message : String(e)}`;
      return;
    } finally {
      this.formBusy = false;
      if (!this.destroyed) this.cdr.detectChanges();
    }
    this.savedOllamaURL = this.settings?.providers?.ollama_url ?? '';
    this.switchError = '';
    delete this.modelSource['ollama'];
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

  /**
   * Presets are saved by the backend the moment they change, outside save().
   * The settings object on this page must follow, or the next save() — the
   * Save button or a provider switch — writes back the list as it was when the
   * page opened and undoes the change.
   */
  private syncPresets(presets: AppPreset[]): void {
    this.presets = presets;
    if (this.settings) {
      this.settings.app_presets = presets.map(p => ({ ...p }));
    }
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
    this.syncPresets(await this.wails.getAppPresets());
    this.editingPreset = null;
    this.cdr.detectChanges();
  }

  async deletePreset(sourceApp: string): Promise<void> {
    await this.wails.deleteAppPreset(sourceApp);
    this.syncPresets(await this.wails.getAppPresets());
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
    this.syncPresets(await this.wails.getAppPresets());
    this.addingPreset = false;
    this.cdr.detectChanges();
  }
}
