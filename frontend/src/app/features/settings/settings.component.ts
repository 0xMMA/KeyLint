import { Component, OnInit, OnDestroy, ChangeDetectorRef, ElementRef, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ActivatedRoute, NavigationEnd, Router } from '@angular/router';
import { Subscription, filter } from 'rxjs';
import {
  ButtonComponent, InputDirective, MessageComponent, TabComponent, TabPanelComponent, TabsComponent,
} from '../../shared/ui';
import { ProviderCardComponent } from './provider-card/provider-card.component';
import { ActiveProviderComponent, FeatureChoice } from './active-provider/active-provider.component';
import { SettingsSectionComponent } from './settings-section/settings-section.component';
import { SettingsGeneralComponent } from './settings-general/settings-general.component';
import { AppPresetsComponent } from './app-presets/app-presets.component';
import { SettingsAboutComponent } from './settings-about/settings-about.component';
import { DEFAULT_MODELS, Feature, ModelOption, defaultOption, effortNote, fixSkipsReasoning, toModelOption } from './model-options';
import { SilentFixNoticeService } from '../fix/silent-fix-notice.service';
import { WailsService, type SilentFixNotice, Settings as AppSettings, KeyStatus, ClaudeCodeStatus, ModelInfo, BuildIdentity } from '../../core/wails.service';
import { noteForModelSource } from '../../core/model-source';
import { unavailableProviderName } from '../../core/constants';
import { LogService } from '../../core/log.service';
import { ENV_KEY_VARS, PROVIDER_OPTIONS, keyPlaceholder } from '../../core/providers';

/** A provider's stored choices; see FeatureModels in internal/features/settings/model.go. */
type ProviderModels = NonNullable<NonNullable<AppSettings['models']>[string]>;

/** Where a local Ollama listens when the URL field is left empty. */
const DEFAULT_OLLAMA_URL = 'http://localhost:11434';

interface ProviderKey {
  id: string;
  status: KeyStatus | null;
  editing: boolean;
  draftKey: string;
  saving: boolean;
}

/**
 * Settings (docs/ui/design.md, mockup "Settings"): four tabs, saved together
 * with the Save button at the bottom.
 *
 * This page owns the settings object and everything that has to stay in step
 * across tabs — the provider in use, its keys and model lists, Save and Reset.
 * The General, App Defaults and About tabs are their own components and edit
 * the same object in place; the AI Providers tab is drawn here, because every
 * row on it reaches into that shared state.
 */
@Component({
  selector: 'app-settings',
  standalone: true,
  imports: [
    FormsModule,
    ButtonComponent, InputDirective, MessageComponent, TabComponent, TabPanelComponent, TabsComponent,
    ProviderCardComponent, ActiveProviderComponent, SettingsSectionComponent,
    SettingsGeneralComponent, AppPresetsComponent, SettingsAboutComponent,
  ],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="settings-page">
      <h1 class="m-0 mb-2 text-2xl font-bold tracking-tight text-fg-strong">Settings</h1>
      @if (settings) {
        <kl-tabs [(value)]="activeTab" ariaLabel="Settings sections">
          <kl-tab value="general">General</kl-tab>
          <kl-tab value="providers">AI providers</kl-tab>
          <kl-tab value="app-defaults">App defaults</kl-tab>
          <kl-tab value="about">About</kl-tab>

          <kl-tab-panel value="general">
            <div class="panel">
              <app-settings-general
                [settings]="settings"
                [providerLabel]="activeProviderLabel"
                [unavailableProvider]="unavailableProvider"
                [providerProblem]="activeProviderProblem"
                (openProviders)="openProvidersTab()"
              />
            </div>
          </kl-tab-panel>

          <!-- AI Providers: what is in use on top, every connection below. -->
          <kl-tab-panel value="providers">
            <div class="panel">
              <!-- A clicked hotkey-fix notification sent the user here: say why,
                   once, until it is dismissed or something here changes. -->
              @if (silentFixNote; as note) {
                <kl-message data-testid="silent-fix-note" severity="warn" closable (closed)="silentFixNote = null">
                  <span><strong>{{ note.title }}:</strong> {{ note.body }}</span>
                </kl-message>
              }
              @if (unavailableProvider; as name) {
                <kl-message data-testid="providers-tab-unavailable" severity="warn">
                  {{ name }} is saved but not available yet, so KeyLint has no provider in use.
                </kl-message>
              }
              <!-- Screen readers hear a switch land; the marker itself only moves. -->
              <div class="sr-only" aria-live="polite" data-testid="provider-announcement">{{ providerAnnouncement }}</div>
              @if (switchError) {
                <!-- Focusable, for a failed switch with no card in use to return to.
                     The message is role="alert", so it announces itself. -->
                <div class="switch-error" tabindex="-1" data-testid="provider-switch-error">
                  <kl-message severity="error">{{ switchError }}</kl-message>
                </div>
              }

              <!-- Fix and Pyramidize both follow this choice: the Pyramidize page
                   has no provider switch since E3 phase 3. -->
              @if (activeProviderLabel && settings.active_provider; as active) {
                <app-active-provider
                  [providerId]="active"
                  [label]="activeProviderLabel!"
                  [fixOptions]="optionsFor(active, 'fix')"
                  [pyramidizeOptions]="optionsFor(active, 'pyramidize')"
                  [fixModel]="modelFor(active, 'fix')"
                  [pyramidizeModel]="modelFor(active, 'pyramidize')"
                  [fixEffort]="effortFor(active, 'fix')"
                  [pyramidizeEffort]="effortFor(active, 'pyramidize')"
                  [editable]="allowsFreeText(active)"
                  [note]="modelListNote(active)"
                  [showEffort]="effortNoteFor(active) !== null"
                  [effortNote]="effortNoteFor(active)"
                  [fixFastNote]="fixSkipsReasoningFor(active)"
                  (modelChange)="onModelChange(active, $event)"
                  (effortChange)="onEffortChange(active, $event)"
                >
                  @if (activeProviderProblem; as why) {
                    <kl-message status data-testid="active-provider-problem" severity="warn">{{ why }}</kl-message>
                  }
                </app-active-provider>
              } @else {
                <!-- The focus target of the General tab's pointer, as the
                     in-use block's header is when there is one. -->
                <p class="no-provider" data-testid="providers-summary" tabindex="-1">
                  No provider is in use. Press <em>Use this</em> on the one KeyLint should send your text to.
                </p>
              }

              <app-settings-section list heading="Connections" description="Set up as many as you like. Only the one in use receives your text.">
                @for (p of providers; track p.value) {
                  <app-provider-card
                    [providerId]="p.value"
                    [label]="p.label"
                    [inUse]="settings.active_provider === p.value"
                    [ready]="isReady(p.value)"
                    [problem]="providerProblem(p.value)"
                    [switching]="switchingTo === p.value"
                    [locked]="switchingTo !== null || formBusy"
                    (use)="useProvider(p.value)"
                  >
                    <!-- The status in words under the name, as the mockup's calm
                         list has it; the dot beside the name carries it at a glance.
                         One element per slot: projection by selector only reaches
                         an element at the root, not one of several inside an @if. -->
                    <div cardSummary>
                      @if (p.value === 'claude-code') {
                        <!-- The CLI needs no key: the user signs in themselves. -->
                        <div data-testid="claude-code-card">
                          @if (claudeCodeStatus?.installed) {
                            <p class="m-0" data-testid="claude-code-detected">
                              @if (claudeCodeStatus!.loggedIn) {
                                <span data-testid="claude-code-status-tag">Signed in</span>
                              } @else {
                                <span class="font-semibold text-warn" data-testid="claude-code-status-tag">Not signed in</span>
                              }, found at <code>{{ claudeCodeStatus!.path }}</code>@if (claudeCodeStatus!.version) {<span> · version {{ claudeCodeStatus!.version }}</span>}
                            </p>
                            @if (!claudeCodeStatus!.loggedIn) {
                              <p class="m-0" data-testid="claude-code-signin-hint">
                                Open a terminal, run <code>claude</code>, and sign in. KeyLint never reads or stores your credentials.
                              </p>
                            }
                          } @else if (claudeCodeStatus) {
                            <p class="m-0" data-testid="claude-code-missing">
                              <span data-testid="claude-code-status-tag">Not installed</span>: no Claude Code CLI found on this machine. Install it to use your own subscription instead of an API key.
                            </p>
                          } @else {
                            <p class="m-0">Looking for the CLI…</p>
                          }
                        </div>
                      } @else if (p.value === 'ollama') {
                        <span data-testid="ollama-status-tag" [class.text-warn]="modelSource['ollama'] === 'empty'">{{ ollamaSummary }}</span>
                      } @else if (keyFor(p.value); as pk) {
                        @if (pk.status?.source === 'env') {
                          <span [attr.data-testid]="'key-env-hint-' + pk.id">
                            <span [attr.data-testid]="'key-status-' + pk.id">Key from the environment:</span>
                            <code>{{ envVarFor(pk.id) }}</code> takes priority over a key saved here, so it can only be changed where it is set.
                          </span>
                        } @else if (pk.status?.is_set) {
                          <span [attr.data-testid]="'key-status-' + pk.id">Key saved in your system keyring</span>
                        } @else if (pk.status) {
                          <span [attr.data-testid]="'key-status-' + pk.id">No key yet</span>
                        }
                      }
                    </div>

                    <div cardActions class="contents">
                      @if (p.value === 'claude-code') {
                        <kl-button
                          data-testid="claude-code-recheck"
                          size="sm"
                          variant="ghost"
                          icon="refresh-cw"
                          loadingLabel="Checking…"
                          [loading]="claudeCodeChecking"
                          (click)="recheckClaudeCode(true)"
                        >Check again</kl-button>
                      } @else if (keyFor(p.value); as pk) {
                        @if (!pk.editing && pk.status?.source !== 'env') {
                          <kl-button
                            [attr.data-testid]="'key-edit-' + pk.id"
                            size="sm"
                            icon="key-round"
                            [variant]="pk.status?.is_set ? 'ghost' : 'quiet'"
                            (click)="startEdit(pk)"
                          >{{ pk.status?.is_set ? 'Edit key' : 'Add key' }}</kl-button>
                          @if (pk.status?.is_set) {
                            <kl-button
                              [attr.data-testid]="'key-clear-' + pk.id"
                              size="sm"
                              variant="ghost"
                              icon="trash"
                              iconOnly
                              [ariaLabel]="'Remove the ' + p.label + ' key'"
                              [loading]="pk.saving"
                              (click)="clearKey(pk)"
                            />
                          }
                        }
                      }
                    </div>

                    @if (p.value === 'claude-code') {
                      @if (claudeCodeStatus?.installed) {
                        <p class="detail-hint" data-testid="claude-code-env-hint">
                          KeyLint runs the CLI with your subscription login. API-key variables in your
                          environment (<code>ANTHROPIC_API_KEY</code> and friends) are not passed through,
                          so the CLI uses the account you signed in with.
                        </p>
                      }
                    } @else if (p.value === 'ollama') {
                      <!-- The URL is not a secret, so it is a plain field saved with Save. -->
                      <div class="flex flex-col gap-1.5">
                        <label for="ollama-url" class="text-[13px] font-medium text-muted">Server URL</label>
                        <input
                          id="ollama-url"
                          data-testid="ollama-url-input"
                          klInput
                          class="max-w-md"
                          [(ngModel)]="settings.providers.ollama_url"
                          placeholder="http://localhost:11434"
                        />
                        <small class="detail-hint">Leave empty for a local Ollama on the default port. Saved with the Save button below; <em>Use this</em> only switches the provider.</small>
                      </div>
                    } @else if (keyFor(p.value); as pk) {
                      @if (pk.editing) {
                        <div class="flex flex-wrap items-center gap-2">
                          <input
                            klInput
                            type="password"
                            class="min-w-48 flex-1"
                            autocomplete="off"
                            spellcheck="false"
                            [attr.data-testid]="'key-input-' + pk.id"
                            [attr.aria-label]="p.label + ' API key'"
                            [(ngModel)]="pk.draftKey"
                            [placeholder]="keyPlaceholder(pk.id)"
                            (keydown.enter)="saveKey(pk)"
                            (keydown.escape)="cancelEdit(pk)"
                          />
                          <kl-button
                            [attr.data-testid]="'key-save-' + pk.id"
                            variant="primary"
                            icon="check"
                            loadingLabel="Saving…"
                            [loading]="pk.saving"
                            [disabled]="!pk.draftKey"
                            (click)="saveKey(pk)"
                          >Save key</kl-button>
                          <kl-button [attr.data-testid]="'key-cancel-' + pk.id" variant="ghost" (click)="cancelEdit(pk)">Cancel</kl-button>
                        </div>
                      }
                    }
                  </app-provider-card>
                }
                <p sectionFooter class="detail-hint">
                  Keys are stored in your OS keyring (Windows Credential Manager, libsecret on Linux).
                  Environment variables (<code>OPENAI_API_KEY</code>, <code>ANTHROPIC_API_KEY</code>) take priority and cannot be overridden here.
                </p>
              </app-settings-section>
              @if (keyError) {
                <kl-message data-testid="key-error" severity="error" [text]="keyError" />
              }
            </div>
          </kl-tab-panel>

          <kl-tab-panel value="app-defaults">
            <div class="panel">
              <app-app-presets [settings]="settings" />
            </div>
          </kl-tab-panel>

          <kl-tab-panel value="about">
            <div class="panel">
              <app-settings-about [settings]="settings" [version]="appVersion" [buildIdentity]="buildIdentity" />
            </div>
          </kl-tab-panel>
        </kl-tabs>

        <div class="footer">
          @if (saved) {
            <kl-message data-testid="saved-banner" class="mt-4" severity="success" text="Settings saved!" />
          }
          @if (saveError) {
            <kl-message data-testid="save-error" class="mt-4" severity="error" [text]="saveError" />
          }

          @if (confirmingReset) {
            <!-- Asked in place, not in a browser dialog: what goes and what
                 stays is the whole question, so it is spelled out. -->
            <div
              class="reset-confirm mt-6"
              role="group"
              aria-labelledby="reset-confirm-title"
              data-testid="reset-confirm"
              (keydown.escape)="cancelReset()"
            >
              <p id="reset-confirm-title" class="m-0 font-semibold text-fg-strong">Reset all settings to their defaults?</p>
              <p class="mt-1 mb-4 text-sm text-muted" data-testid="reset-confirm-detail">
                Shortcuts, models and effort, app presets, the Ollama address, logging, update and
                developer options go back to their defaults.
                <strong class="text-soft">Kept:</strong> your API keys in the OS keyring, the provider in use, and your completed setup.
              </p>
              <div class="flex gap-4">
                <kl-button data-testid="reset-confirm-btn" variant="danger" icon="rotate-ccw" [disabled]="switchingTo !== null || formBusy" (click)="confirmReset()">Reset</kl-button>
                <kl-button data-testid="reset-cancel-btn" (click)="cancelReset()">Cancel</kl-button>
              </div>
            </div>
          } @else {
            <div class="mt-6 flex gap-4">
              <!-- Held while a provider switch saves, and the switch is held while
                   these run: one landing in the middle of the other could
                   overwrite it or be overwritten by it. -->
              <kl-button data-testid="save-btn" variant="primary" icon="check" [disabled]="switchingTo !== null || formBusy" (click)="save()">Save</kl-button>
              <kl-button data-testid="reset-btn" variant="danger" icon="rotate-ccw" [disabled]="switchingTo !== null || formBusy" (click)="requestReset()">Reset to defaults</kl-button>
            </div>
          }
        </div>
      }
    </div>
  `,
  styles: [`
    .settings-page { padding: 32px 44px 40px; }
    .panel, .footer { max-width: 820px; }
    .panel { display: flex; flex-direction: column; gap: 24px; padding-top: 28px; }
    .footer { display: flex; flex-direction: column; margin-top: 8px; }
    .switch-error:focus { outline: none; }
    .no-provider { margin: 0; font-size: 14px; color: var(--kl-muted); }
    .no-provider:focus { outline: none; }
    .no-provider:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 4px; }
    .detail-hint { margin: 0; font-size: 13px; line-height: 1.5; color: var(--kl-muted); max-width: 72ch; }
    .reset-confirm {
      padding: 16px 20px; border-radius: var(--kl-radius-sheet);
      border: 1px solid color-mix(in srgb, var(--kl-error), transparent 55%);
      background: color-mix(in srgb, var(--kl-error), transparent 94%);
    }
    code { font-size: 12px; padding: 1px 4px; border-radius: 3px; background: var(--kl-selected); color: var(--kl-text-soft); }
    @media (max-width: 600px) { .settings-page { padding: 24px 16px 32px; } }
  `],
})
export class SettingsComponent implements OnInit, OnDestroy {
  settings: AppSettings | null = null;
  saved = false;
  keyError = '';
  /** Two-way bound to the tabs, so the General tab's pointer can switch it. */
  activeTab = 'general';

  appVersion = '';
  /** What the running build says about itself; a release build until asked. */
  buildIdentity: BuildIdentity = { is_dev_build: false, kind: '', pr: 0, commit: '', tag: '' };

  /**
   * The providers a user can choose, in the order their rows appear on the AI
   * Providers tab. The labels are what the General tab's pointer and the model
   * pickers show too.
   */
  readonly providers = PROVIDER_OPTIONS;

  /**
   * Providers whose model can be chosen. Derived from the provider list
   * rather than repeated, so the two cannot drift apart.
   *
   * Computed once: a getter would hand the template a new array on every
   * change-detection pass.
   */
  readonly modelProviders = this.providers
    .map(p => ({ id: p.value, label: p.label }));

  /** Picker contents per provider and feature, the default entry first; see optionsFor. */
  modelSelectOptions: Record<string, Record<Feature, ModelOption[]>> = {};
  /** The default entry alone, per provider and feature, until a list arrives. */
  private readonly fallbackOptions: Record<string, ModelOption[]> = {};

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
  private readonly subs = new Subscription();
  /** Why a hotkey-fix notification sent the user to this tab, if one did. */
  silentFixNote: SilentFixNotice | null = null;

  providerKeys: ProviderKey[] = [
    { id: 'openai', status: null, editing: false, draftKey: '', saving: false },
    { id: 'claude', status: null, editing: false, draftKey: '', saving: false },
  ];

  constructor(
    private readonly route: ActivatedRoute,
    private readonly router: Router,
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
    private readonly host: ElementRef<HTMLElement>,
    private readonly silentNotices: SilentFixNoticeService,
  ) {}

  async ngOnInit(): Promise<void> {
    this.activeTab = this.route.snapshot.queryParamMap.get('tab') ?? 'general';
    // A later ?tab= while this page is open (the sidebar's version opens
    // About) reuses this component rather than making a new one. Read from
    // each finished navigation, not from queryParamMap: picking a tab leaves
    // the URL as it was, so the version link clicked again is a navigation to
    // the same URL (the shell sends it with onSameUrlNavigation: 'reload'),
    // and queryParamMap only reports a change.
    this.subs.add(this.router.events.pipe(filter(e => e instanceof NavigationEnd)).subscribe(e => {
      const url = this.router.parseUrl(e.urlAfterRedirects);
      const tab = url.queryParamMap.get('tab');
      const onSettings = url.root.children['primary']?.segments[0]?.path === 'settings';
      if (tab && onSettings) {
        this.activeTab = tab;
        this.cdr.markForCheck();
      }
    }));
    // A hotkey-fix notification clicked to get here — or clicked while this
    // page is already open, which the router does not report as a new
    // navigation.
    const pendingNote = this.silentNotices.take('providers');
    if (pendingNote) this.showSilentFixNote(pendingNote);
    this.subs.add(this.silentNotices.notices$.subscribe(() => {
      const note = this.silentNotices.take('providers');
      if (note) {
        this.showSilentFixNote(note);
        this.cdr.detectChanges();
      }
    }));
    this.settings = await this.wails.loadSettings();
    this.savedOllamaURL = this.settings?.providers?.ollama_url ?? '';
    this.log.info('settings: loaded');
    await this.refreshKeyStatuses();
    this.appVersion = await this.wails.getVersion();
    this.buildIdentity = await this.wails.getBuildIdentity();
    this.cdr.detectChanges();

    // Detection spawns processes, so the rest of the screen must not wait for it.
    void this.recheckClaudeCode();
    // Same for the model lists: four providers, each a round trip.
    void this.loadModelOptions();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.subs.unsubscribe();
  }

  private showSilentFixNote(note: SilentFixNotice): void {
    this.silentFixNote = note;
    this.activeTab = 'providers';
  }

  /** The note's problem was acted on. Called after an await, so it marks the view. */
  private dropSilentFixNote(): void {
    if (!this.silentFixNote) return;
    this.silentFixNote = null;
    this.cdr.markForCheck();
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

  /** The Ollama row's status line: where KeyLint looks, and what it found there. */
  get ollamaSummary(): string {
    const where = (this.savedOllamaURL || DEFAULT_OLLAMA_URL).replace(/^https?:\/\//, '');
    switch (this.modelSource['ollama']) {
      case 'live': return `Running at ${where}`;
      case 'empty': return `Running at ${where}, no models pulled`;
      case 'unreachable': return `Not running at ${where}`;
      default: return `Looking for Ollama at ${where}…`;
    }
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
        const where = this.savedOllamaURL || DEFAULT_OLLAMA_URL;
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

  /** Set up and answering, for the filled dot on a connection row. */
  isReady(provider: string): boolean {
    switch (provider) {
      case 'claude-code': return !!this.claudeCodeStatus?.installed && !!this.claudeCodeStatus.loggedIn;
      case 'ollama': return this.modelSource['ollama'] === 'live';
      default: return !!this.keyFor(provider)?.status?.is_set;
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
      // The user acted on what the note said; it is no longer the news.
      this.dropSilentFixNote();
      // The switch saved, so whatever the last Save said is no longer the news.
      this.saveError = '';
      this.refreshProviderStatus(provider);
    }
    // Announced once: a failure is already announced by the error message,
    // which is role="alert"; the live region carries only the success.
    this.providerAnnouncement = switched ? `KeyLint now uses ${label}.` : '';
    if (this.destroyed) return;
    this.cdr.detectChanges();
    // The pressed button is gone (on success it became the "In use" mark) or
    // no longer the point (on failure). Focus goes to the heading of the row
    // in use now; after a failure with no row in use (a saved Bedrock, or
    // nothing saved), to the error that explains it.
    const inUse = this.activeProviderLabel ? this.settings?.active_provider : null;
    if (inUse) {
      this.focusCardHeading(inUse);
    } else if (!switched) {
      this.host.nativeElement
        .querySelector<HTMLElement>('[data-testid="provider-switch-error"]')?.focus();
    }
  }

  /**
   * Re-asks about the provider just switched to, so its row does not show an
   * answer from before the user set it up. The CLI probe is forced, the same
   * bypass the row's Check again uses. Ollama has no such bypass: the backend
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
      }).catch(() => { /* keep the answer the row already has */ });
    }
  }

  /** The General tab's pointer: show the AI Providers tab and move focus there. */
  openProvidersTab(): void {
    this.activeTab = 'providers';
    this.cdr.detectChanges();
    // The pointer sits in the panel that just hid, so focus would be lost.
    this.host.nativeElement
      .querySelector<HTMLElement>('[data-testid="providers-summary"]')?.focus();
  }

  private focusCardHeading(provider: string): void {
    this.host.nativeElement
      .querySelector<HTMLElement>(`[data-testid="provider-heading-${provider}"]`)?.focus();
  }

  /** Reads the configured model, or "" when the default applies. */
  modelFor(provider: string, feature: Feature): string {
    return this.settings?.models?.[provider]?.[feature] ?? '';
  }

  /** Reads the configured effort, or "" when the model's default applies. */
  effortFor(provider: string, feature: Feature): string {
    return this.settings?.models?.[provider]?.[`${feature}_effort`] ?? '';
  }

  /** Stores a model choice; an empty value means "use KeyLint's default". */
  setModel(provider: string, feature: Feature, model: string): void {
    this.updateProviderModels(provider, entry => { entry[feature] = (model ?? '').trim(); });
  }

  /** Stores an effort choice; an empty value means "send none". */
  setEffort(provider: string, feature: Feature, effort: string): void {
    this.updateProviderModels(provider, entry => { entry[`${feature}_effort`] = effort ?? ''; });
  }

  onModelChange(provider: string, choice: FeatureChoice): void {
    this.setModel(provider, choice.feature, choice.value);
  }

  onEffortChange(provider: string, choice: FeatureChoice): void {
    this.setEffort(provider, choice.feature, choice.value);
  }

  /**
   * Copies the provider's entry before changing it, so the object a pending
   * change detection pass still holds is not edited under it.
   */
  private updateProviderModels(provider: string, change: (entry: ProviderModels) => void): void {
    if (!this.settings) return;
    const models = { ...(this.settings.models ?? {}) };
    const entry: ProviderModels = { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: '', ...(models[provider] ?? {}) };
    change(entry);
    models[provider] = entry;
    this.settings.models = models;
  }

  /**
   * The provider's models for one feature, with a leading entry naming
   * KeyLint's default for it. The picker shows its placeholder only while the
   * value is null, and this component writes "" — so an explicit option is
   * what makes the default visible and selectable.
   */
  optionsFor(provider: string, feature: Feature): ModelOption[] {
    // Before the list arrives (or when it never does) the default entry still
    // names the default, so the field and the Fix note never read a bare
    // "Default".
    // Kept, not rebuilt: a new array on every change-detection pass would
    // read as a changed binding.
    return this.modelSelectOptions[provider]?.[feature]
      ?? (this.fallbackOptions[`${provider}/${feature}`] ??= [defaultOption(provider, feature, [])]);
  }

  /** What effort does on this provider, or null where it does nothing. */
  effortNoteFor(provider: string): string | null {
    return effortNote(provider);
  }

  /** Whether Fix will ask its model on this provider not to reason first. */
  fixSkipsReasoningFor(provider: string): boolean {
    if (this.effortFor(provider, 'fix') !== '') return false;
    const model = this.modelFor(provider, 'fix') || (provider === 'claude' || provider === 'openai'
      ? this.defaultModelId(provider, 'fix') : '');
    return fixSkipsReasoning(provider, model);
  }

  private defaultModelId(provider: string, feature: Feature): string {
    return DEFAULT_MODELS[provider]?.[feature] ?? '';
  }

  /**
   * Whether a model outside the list can be typed. The Claude Code CLI's
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
      const listed = (list?.models ?? []).map(toModelOption);
      this.modelSelectOptions[mp.id] = {
        fix: [defaultOption(mp.id, 'fix', listed), ...listed],
        pyramidize: [defaultOption(mp.id, 'pyramidize', listed), ...listed],
      };
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
      // "Check again" after signing in: the problem the note was about is gone.
      if (force && this.claudeCodeStatus.installed && this.claudeCodeStatus.loggedIn) {
        this.dropSilentFixNote();
      }
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
    return keyPlaceholder(provider);
  }

  startEdit(pk: ProviderKey): void {
    pk.editing = true;
    pk.draftKey = '';
    // The field replaces the button that was pressed, so focus follows it.
    this.cdr.detectChanges();
    this.host.nativeElement.querySelector<HTMLInputElement>(`[data-testid="key-input-${pk.id}"]`)?.focus();
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
      this.dropSilentFixNote();
    } catch (e) {
      this.keyError = `Failed to save key: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      pk.saving = false;
      if (!this.destroyed) this.cdr.markForCheck();
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
      if (!this.destroyed) this.cdr.markForCheck();
    }
    // The model list depends on this key.
    void this.loadModelOptions();
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

  /** Reset to Defaults asks first, in place; see confirmReset. */
  confirmingReset = false;

  requestReset(): void {
    if (this.switchingTo !== null || this.formBusy) return;
    this.confirmingReset = true;
    this.cdr.detectChanges();
    // Focus lands on Cancel: Enter straight after the click must not reset.
    this.host.nativeElement
      .querySelector<HTMLElement>('[data-testid="reset-cancel-btn"] button')?.focus();
  }

  cancelReset(): void {
    this.confirmingReset = false;
    this.cdr.detectChanges();
    this.host.nativeElement.querySelector<HTMLElement>('[data-testid="reset-btn"] button')?.focus();
  }

  async confirmReset(): Promise<void> {
    this.confirmingReset = false;
    await this.resetToDefaults();
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
}
