import { Component, OnInit, computed, signal } from '@angular/core';
import { Router } from '@angular/router';
import { Button } from 'primeng/button';
import { InputText } from 'primeng/inputtext';
import { Tag } from 'primeng/tag';
import { Message } from 'primeng/message';
import { WailsService, ClaudeCodeStatus, KeyStatus, Settings } from '../../core/wails.service';
import { ENV_KEY_VARS, KEY_PROVIDERS, PROVIDER_OPTIONS, cliTag, isKeyProvider, keyPlaceholder, keyTag } from '../../core/providers';
import { comboKeys } from '../../core/shortcut-format';

/** One line under each provider's name: what it is, in the user's terms. */
const PROVIDER_BLURBS: Readonly<Record<string, string>> = {
  'openai': 'Pay as you go with an OpenAI API key.',
  'claude': 'Pay as you go with an Anthropic API key.',
  'claude-code': 'Uses your Claude subscription through the installed Claude Code CLI. No key needed.',
  'ollama': 'Runs on this computer. Free, no key, needs Ollama installed.',
};

type Screen = 'welcome' | 'provider';

/**
 * First-run setup: what KeyLint does, then which AI it uses.
 *
 * The backend opens it on a first start (no settings file) and for an existing
 * setup with nothing usable; anyone else with a key, a signed-in CLI or a
 * working saved provider skips it (see welcome.Service.IsFirstRun). Even so, nothing in here blocks: "Set up later"
 * is on every screen, a key that is already stored is never asked for again,
 * and what already works is selected before the user touches anything.
 */
@Component({
  selector: 'app-welcome-wizard',
  standalone: true,
  imports: [Button, InputText, Tag, Message],
  template: `
    <main class="wizard">
      <div class="wizard-column">
        <p class="wordmark" aria-hidden="true">KeyLint</p>

        @switch (screen()) {
          @case ('welcome') {
            <section data-testid="wizard-welcome" class="screen" aria-labelledby="welcome-title">
              <div class="keys" role="img" [attr.aria-label]="'Shortcut: ' + shortcutKeys().join(' + ')">
                @for (key of shortcutKeys(); track $index) {
                  @if (!$first) { <span class="keys-plus" aria-hidden="true">+</span> }
                  <kbd class="keycap" [class.trigger]="$last">{{ key }}</kbd>
                }
              </div>

              <h1 id="welcome-title">Fix any text right where you wrote it.</h1>
              <p class="lede">
                Select text in any app and press <strong>{{ shortcutKeys().join('+') }}</strong>.
                KeyLint corrects the grammar and spelling and puts the result back in place.
              </p>
              @if (doubleTap()) {
                <p class="lede secondary">
                  Hold {{ modifierKeys() }} and press {{ triggerKey() }} twice instead to open Pyramidize, which restructures longer text.
                </p>
              }

              <div class="actions">
                <p-button
                  data-testid="wizard-skip"
                  label="Set up later"
                  [text]="true"
                  severity="secondary"
                  [loading]="skipping()"
                  (onClick)="skip()"
                />
                <p-button data-testid="wizard-next" label="Choose an AI" (onClick)="screen.set('provider')" />
              </div>
            </section>
          }

          @case ('provider') {
            <section data-testid="wizard-provider" class="screen" aria-labelledby="provider-title">
              <h1 id="provider-title">Which AI should fix your text?</h1>
              <p class="lede">You can change this any time under Settings › AI Providers.</p>

              <div class="options" role="radiogroup" aria-labelledby="provider-title">
                @for (p of providers; track p.value) {
                  <div class="option" [class.selected]="selected() === p.value" [attr.data-testid]="'wizard-option-' + p.value">
                    <label class="option-head">
                      <input
                        type="radio"
                        name="provider"
                        [value]="p.value"
                        [checked]="selected() === p.value"
                        [attr.data-testid]="'wizard-radio-' + p.value"
                        (change)="choose(p.value)"
                      />
                      <span class="option-text">
                        <span class="option-name">{{ p.label }}</span>
                        <span class="option-blurb">{{ blurbs[p.value] }}</span>
                      </span>
                      <span class="option-status" aria-live="polite" [attr.data-testid]="'wizard-status-' + p.value">
                        @if (p.value === 'claude-code') {
                          @if (cli(); as status) {
                            @let tag = cliTag(status);
                            <p-tag [value]="tag.value" [severity]="tag.severity" />
                          } @else {
                            <span class="checking" data-testid="wizard-cli-checking">
                              <i class="pi pi-spin pi-spinner" aria-hidden="true"></i> checking
                            </span>
                          }
                        } @else if (keys()[p.value]; as status) {
                          @if (status.is_set) {
                            @let tag = keyTag(status);
                            <p-tag [value]="tag.value" [severity]="tag.severity" />
                          }
                        }
                      </span>
                    </label>

                    @if (selected() === p.value) {
                      <div class="option-detail" [attr.data-testid]="'wizard-detail-' + p.value">
                        @switch (p.value) {
                          @case ('claude-code') {
                            @if (!cli()) {
                              <p class="note">Looking for the CLI on this computer…</p>
                            } @else if (cliReady()) {
                              <p class="note">KeyLint runs the CLI with your own login and never sees your credentials.</p>
                            } @else {
                              <p class="note" data-testid="wizard-cli-hint">
                                @if (cli()!.installed) {
                                  The CLI is installed but not signed in. Open a terminal, run <code>claude</code>, sign in, then check again.
                                } @else {
                                  The Claude Code CLI is not on this computer. Install it, sign in, then check again — or pick another option.
                                }
                              </p>
                              <p-button
                                data-testid="wizard-recheck"
                                label="Check again"
                                icon="pi pi-refresh"
                                size="small"
                                severity="secondary"
                                [outlined]="true"
                                [loading]="rechecking()"
                                (onClick)="recheck()"
                              />
                            }
                          }
                          @case ('ollama') {
                            <p class="note">Make sure Ollama is running. KeyLint looks for it at <code>localhost:11434</code>; a different address goes in Settings.</p>
                          }
                          @default {
                            @if (keys()[p.value]?.source === 'env' && !replacing()) {
                              <p class="note" data-testid="wizard-key-stored">
                                Using the key from the <code>{{ envVar(p.value) }}</code> environment variable.
                              </p>
                            } @else if (keys()[p.value]?.is_set && !replacing()) {
                              <p class="note" data-testid="wizard-key-stored">
                                A key is already saved for {{ p.label }}.
                                <button type="button" class="inline-action" data-testid="wizard-replace-key" (click)="startReplacing()">Replace it</button>
                              </p>
                            } @else {
                              <label class="key-label" for="wizard-key">Paste your API key</label>
                              <input
                                pInputText
                                id="wizard-key"
                                data-testid="wizard-key-input"
                                type="password"
                                autocomplete="off"
                                spellcheck="false"
                                [placeholder]="keyPlaceholder(p.value)"
                                [value]="apiKey()"
                                (input)="typeKey($any($event.target).value)"
                              />
                              <p class="note">
                                Saved in your system's keyring, never in a file.
                                @if (replacing()) {
                                  <button type="button" class="inline-action" data-testid="wizard-keep-key" (click)="keepSavedKey()">Keep the saved key</button>
                                }
                              </p>
                            }
                          }
                        }
                      </div>
                    }
                  </div>
                }
              </div>

              <div class="actions">
                <p-button data-testid="wizard-back" label="Back" [text]="true" severity="secondary" (onClick)="screen.set('welcome')" />
                <span class="actions-spacer"></span>
                <p-button
                  data-testid="wizard-skip"
                  label="Set up later"
                  [text]="true"
                  severity="secondary"
                  [loading]="skipping()"
                  [disabled]="finishing()"
                  (onClick)="skip()"
                />
                <p-button
                  data-testid="wizard-finish"
                  label="Start using KeyLint"
                  [loading]="finishing()"
                  [disabled]="!canFinish() || skipping()"
                  (onClick)="finish()"
                />
              </div>
            </section>
          }
        }

        @if (error(); as message) {
          <p-message data-testid="wizard-error" severity="error" size="small" styleClass="wizard-error">{{ message }}</p-message>
        }
      </div>
    </main>
  `,
  styles: [`
    :host { display: block; }

    .wizard {
      min-height: 100vh;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 2rem 1rem;
      background: var(--p-surface-950, #09090b);
      color: var(--p-text-color, #f4f4f5);
    }
    .wizard-column {
      width: 100%;
      max-width: 34rem;
    }
    .wordmark {
      margin: 0 0 2.5rem;
      font-weight: 700;
      font-size: 1.05rem;
      letter-spacing: -0.025em;
      color: var(--p-text-muted-color, #a1a1aa);
    }

    h1 {
      margin: 0 0 0.75rem;
      font-size: 1.75rem;
      line-height: 1.2;
      font-weight: 650;
      letter-spacing: -0.02em;
    }
    .lede {
      margin: 0 0 0.75rem;
      font-size: 1rem;
      line-height: 1.55;
      color: var(--p-text-color, #f4f4f5);
      max-width: 32rem;
    }
    .lede.secondary { color: var(--p-text-muted-color, #a1a1aa); }

    /* The shortcut is the product, so it is the picture: real keycaps. */
    .keys {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 2.25rem;
    }
    .keys-plus { color: var(--p-text-muted-color, #a1a1aa); font-size: 1.25rem; }
    .keycap {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 4rem;
      height: 4rem;
      padding: 0 1.1rem;
      border-radius: 0.75rem;
      font-family: inherit;
      font-size: 1.25rem;
      font-weight: 600;
      line-height: 1;
      color: var(--p-text-color, #f4f4f5);
      background: linear-gradient(180deg, var(--p-surface-800, #27272a), var(--p-surface-900, #18181b));
      border: 1px solid var(--p-surface-600, #52525b);
      box-shadow: 0 5px 0 var(--p-surface-700, #3f3f46), 0 8px 18px rgba(0, 0, 0, 0.45);
      transform: translateY(0);
    }
    .keycap.trigger {
      border-color: var(--p-primary-color, #f97316);
      color: var(--p-primary-color, #f97316);
      animation: press 1.4s ease-in-out 0.6s 1 both;
    }
    /* One press, once, when the screen opens: this is what the user will do. */
    @keyframes press {
      0%, 30%, 100% { transform: translateY(0); box-shadow: 0 5px 0 var(--p-surface-700, #3f3f46), 0 8px 18px rgba(0, 0, 0, 0.45); }
      12% { transform: translateY(4px); box-shadow: 0 1px 0 var(--p-surface-700, #3f3f46), 0 2px 6px rgba(0, 0, 0, 0.45); }
    }
    @media (prefers-reduced-motion: reduce) {
      .keycap.trigger { animation: none; }
    }

    /* One list with dividers rather than a stack of cards: it is one choice. */
    .options {
      margin: 1.5rem 0 1rem;
      border: 1px solid var(--p-surface-700, #3f3f46);
      border-radius: 0.75rem;
      overflow: hidden;
      background: var(--p-surface-900, #18181b);
    }
    .option + .option { border-top: 1px solid var(--p-surface-800, #27272a); }
    /* The same marker as the provider in use on Settings › AI Providers. */
    .option.selected {
      box-shadow: inset 3px 0 0 var(--p-primary-color, #f97316);
      background: color-mix(in srgb, var(--p-primary-color, #f97316) 5%, var(--p-surface-900, #18181b));
    }
    .option-head {
      display: flex;
      align-items: center;
      gap: 0.85rem;
      padding: 0.85rem 1rem;
      cursor: pointer;
    }
    .option-head input[type="radio"] {
      flex: none;
      width: 1.05rem;
      height: 1.05rem;
      margin: 0;
      accent-color: var(--p-primary-color, #f97316);
    }
    .option-head input[type="radio"]:focus-visible {
      outline: 2px solid var(--p-primary-color, #f97316);
      outline-offset: 3px;
    }
    .option-text { flex: 1; display: flex; flex-direction: column; gap: 0.15rem; min-width: 0; }
    .option-name { font-weight: 600; font-size: 0.95rem; }
    .option-blurb { font-size: 0.85rem; line-height: 1.4; color: var(--p-text-muted-color, #a1a1aa); }
    .option-status { flex: none; }
    .checking { font-size: 0.8rem; color: var(--p-text-muted-color, #a1a1aa); white-space: nowrap; }

    .option-detail {
      padding: 0 1rem 1rem 2.9rem;
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 0.6rem;
    }
    .option-detail input { width: 100%; }
    .key-label { font-size: 0.85rem; font-weight: 600; }
    .note {
      margin: 0;
      font-size: 0.85rem;
      line-height: 1.5;
      color: var(--p-text-muted-color, #a1a1aa);
    }
    code {
      font-size: 0.8rem;
      padding: 0.05rem 0.3rem;
      border-radius: 4px;
      background: var(--p-surface-800, #27272a);
    }
    .inline-action {
      background: none;
      border: 0;
      padding: 0;
      font: inherit;
      color: var(--p-primary-color, #f97316);
      cursor: pointer;
      text-decoration: underline;
      text-underline-offset: 2px;
    }
    .inline-action:focus-visible { outline: 2px solid var(--p-primary-color, #f97316); outline-offset: 2px; }

    .actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      gap: 0.5rem;
      margin-top: 2rem;
      flex-wrap: wrap;
    }
    .actions-spacer { flex: 1; }
    :host ::ng-deep .wizard-error { margin-top: 1rem; }

    @media (max-width: 480px) {
      h1 { font-size: 1.45rem; }
      .keycap { min-width: 3.25rem; height: 3.25rem; }
      .option-detail { padding-left: 1rem; }
    }
  `],
})
export class WelcomeWizardComponent implements OnInit {
  readonly providers = PROVIDER_OPTIONS;
  readonly blurbs = PROVIDER_BLURBS;
  readonly cliTag = cliTag;
  readonly keyTag = keyTag;
  readonly keyPlaceholder = keyPlaceholder;

  readonly screen = signal<Screen>('welcome');
  /** The provider picked, or null while nothing is. */
  readonly selected = signal<string | null>(null);
  /** Null while detection runs: the option shows "checking", it does not vanish. */
  readonly cli = signal<ClaudeCodeStatus | null>(null);
  readonly keys = signal<Record<string, KeyStatus | undefined>>({});
  readonly settings = signal<Settings | null>(null);
  readonly apiKey = signal('');
  /** The user asked to replace a key that is already stored. */
  readonly replacing = signal(false);
  readonly rechecking = signal(false);
  readonly finishing = signal(false);
  readonly skipping = signal(false);
  readonly error = signal<string | null>(null);

  /** Once the user picks something, the defaults stop moving the selection. */
  private userPicked = false;

  readonly cliReady = computed(() => !!this.cli()?.installed && !!this.cli()?.loggedIn);

  readonly shortcutKeys = computed(() => {
    const keys = comboKeys(this.settings()?.shortcut_fix || 'ctrl+g');
    return keys.length ? keys : ['Ctrl', 'G'];
  });
  readonly triggerKey = computed(() => this.shortcutKeys().at(-1) ?? 'G');
  readonly modifierKeys = computed(() => this.shortcutKeys().slice(0, -1).join('+') || 'the modifier keys');
  readonly doubleTap = computed(() => (this.settings()?.shortcut_mode ?? 'double_tap') === 'double_tap');

  /**
   * Whether "Start using KeyLint" can go: something is picked, and a provider
   * that needs a key has one — stored, from the environment, or typed. Never a
   * dead end: "Set up later" is always next to it.
   */
  readonly canFinish = computed(() => {
    const p = this.selected();
    if (!p) return false;
    if (!isKeyProvider(p)) return true;
    if (this.apiKey().trim()) return true;
    return !!this.keys()[p]?.is_set && !this.replacing();
  });

  constructor(
    private readonly wails: WailsService,
    private readonly router: Router,
  ) {}

  ngOnInit(): void {
    // Independent, so a slow CLI probe cannot hold up the keys or the
    // shortcut on the first screen.
    void this.wails.loadSettings().then(s => { this.settings.set(s); this.applyDefault(); }).catch(() => {});
    void Promise.all(
      KEY_PROVIDERS.map(async p => [p, await this.wails.getKeyStatus(p)] as const),
    ).then(entries => { this.keys.set(Object.fromEntries(entries)); this.applyDefault(); }).catch(() => {});
    void this.wails.getClaudeCodeStatus().then(st => { this.cli.set(st); this.applyDefault(); }).catch(() => {});
  }

  choose(provider: string): void {
    this.userPicked = true;
    if (provider !== this.selected()) {
      this.apiKey.set('');
      this.replacing.set(false);
    }
    this.selected.set(provider);
    this.error.set(null);
  }

  /**
   * Preselects what already works: the saved provider if it can run, then a
   * signed-in CLI, then a provider with a stored key. Nothing at all when
   * nothing works — the user picks; OpenAI is not assumed.
   */
  private applyDefault(): void {
    if (this.userPicked) return;
    this.selected.set(this.defaultProvider());
  }

  private defaultProvider(): string | null {
    const keys = this.keys();
    const saved = this.settings()?.active_provider;
    if (saved) {
      if (isKeyProvider(saved) && keys[saved]?.is_set) return saved;
      if (saved === 'claude-code' && this.cliReady()) return saved;
      if (saved === 'ollama') return saved;
    }
    if (this.cliReady()) return 'claude-code';
    return KEY_PROVIDERS.find(p => keys[p]?.is_set) ?? null;
  }

  /** Typing a key, or asking to replace one, is a choice: detection must not move it. */
  typeKey(value: string): void {
    this.userPicked = true;
    this.apiKey.set(value);
  }

  startReplacing(): void {
    this.userPicked = true;
    this.replacing.set(true);
  }

  keepSavedKey(): void {
    this.replacing.set(false);
    this.apiKey.set('');
  }

  envVar(provider: string): string {
    return ENV_KEY_VARS[provider] ?? '';
  }

  async recheck(): Promise<void> {
    this.rechecking.set(true);
    try {
      this.cli.set(await this.wails.getClaudeCodeStatus(true));
    } finally {
      this.rechecking.set(false);
    }
  }

  async finish(): Promise<void> {
    const provider = this.selected();
    if (!provider || !this.canFinish()) return;
    this.finishing.set(true);
    this.error.set(null);
    try {
      // The key first: if the keyring refuses it, nothing has moved yet, and
      // "Set up later" leaves the previous provider in place.
      const key = this.apiKey().trim();
      if (key && isKeyProvider(provider)) {
        // Rejects when the keyring refuses; the status check below also catches
        // a backend that reports success without storing anything.
        await this.wails.setKey(provider, key);
        const status = await this.wails.getKeyStatus(provider);
        if (!status.is_set) {
          throw new Error('the key could not be saved to your system keyring');
        }
        this.keys.update(k => ({ ...k, [provider]: status }));
        this.apiKey.set('');
        this.replacing.set(false);
      }
      // The one field, not a whole Settings object: a save of everything would
      // write back whatever this screen happened to load.
      await this.wails.setActiveProvider(provider);
      await this.wails.completeSetup();
      await this.router.navigate(['/']);
    } catch (err) {
      this.error.set(`Could not save your choice: ${describe(err)}. Try again, or set it up later.`);
    } finally {
      this.finishing.set(false);
    }
  }

  /** Leaves setup without choosing anything; Settings › AI Providers has it all. */
  async skip(): Promise<void> {
    this.skipping.set(true);
    this.error.set(null);
    try {
      await this.wails.completeSetup();
      await this.router.navigate(['/']);
    } catch (err) {
      this.error.set(`Could not close setup: ${describe(err)}.`);
    } finally {
      this.skipping.set(false);
    }
  }
}

function describe(err: unknown): string {
  if (err instanceof Error) return err.message;
  return typeof err === 'string' && err ? err : 'unknown error';
}
