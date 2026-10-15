import { Component, OnInit, computed, signal, ChangeDetectionStrategy } from '@angular/core';
import { Router } from '@angular/router';
import { ButtonComponent, InputDirective, MessageComponent, SpinnerComponent, TagComponent } from '../../shared/ui';
import { WailsService, ClaudeCodeStatus, KeyStatus, Settings } from '../../core/wails.service';
import { ENV_KEY_VARS, KEY_PROVIDERS, PROVIDER_OPTIONS, cliTag, isKeyProvider, keyPlaceholder, keyTag } from '../../core/providers';
import { comboKeys } from '../../core/shortcut-format';

/** One line under each provider's name: what it is, in the user's terms. */
const PROVIDER_BLURBS: Readonly<Record<string, string>> = {
  'openai': 'Pay as you go with an OpenAI API key.',
  'claude': 'Pay as you go with an Anthropic API key.',
  'openrouter': 'Pay as you go with one OpenRouter key, for models from many vendors.',
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
  imports: [ButtonComponent, InputDirective, MessageComponent, SpinnerComponent, TagComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <main class="wizard">
      <div class="w-full max-w-[34rem]">
        <p class="mt-0 mb-10 text-xl font-bold tracking-tight" aria-hidden="true"><span class="text-fg-strong">Key</span><span class="text-primary">Lint</span></p>

        @switch (screen()) {
          @case ('welcome') {
            <section data-testid="wizard-welcome" aria-labelledby="welcome-title">
              <div class="keys" role="img" [attr.aria-label]="'Shortcut: ' + shortcutKeys().join(' + ')">
                @for (key of shortcutKeys(); track $index) {
                  @if (!$first) { <span class="text-xl text-muted" aria-hidden="true">+</span> }
                  <kbd class="keycap" [class.trigger]="$last">{{ key }}</kbd>
                }
              </div>

              <h1 id="welcome-title">Fix any text right where you wrote it.</h1>
              <p class="lede">
                Select text in any app and press <strong class="text-fg-strong">{{ shortcutKeys().join('+') }}</strong>.
                KeyLint corrects the grammar and spelling and puts the result back in place.
              </p>
              @if (doubleTap()) {
                <p class="lede text-muted">
                  Hold {{ modifierKeys() }} and press {{ triggerKey() }} twice instead to open Pyramidize, which restructures longer text.
                </p>
              }

              <div class="actions">
                <kl-button data-testid="wizard-skip" variant="ghost" size="lg" [loading]="skipping()" (click)="skip()">Set up later</kl-button>
                <kl-button data-testid="wizard-next" variant="primary" size="lg" (click)="screen.set('provider')">Choose an AI</kl-button>
              </div>
            </section>
          }

          @case ('provider') {
            <section data-testid="wizard-provider" aria-labelledby="provider-title">
              <h1 id="provider-title">Which AI should fix your text?</h1>
              <p class="lede text-muted">You can change this any time under Settings › AI providers.</p>

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
                      <span class="flex min-w-0 flex-1 flex-col gap-0.5">
                        <span class="text-[15px] font-semibold text-fg-strong">{{ p.label }}</span>
                        <span class="text-[13px] leading-snug text-muted">{{ blurbs[p.value] }}</span>
                      </span>
                      <span class="flex-none" aria-live="polite" [attr.data-testid]="'wizard-status-' + p.value">
                        @if (p.value === 'claude-code') {
                          @if (cli(); as status) {
                            @let tag = cliTag(status);
                            <kl-tag [value]="tag.value" [severity]="tag.severity" />
                          } @else {
                            <span class="inline-flex items-center gap-1.5 whitespace-nowrap text-[13px] text-muted" data-testid="wizard-cli-checking">
                              <kl-spinner [size]="14" /> checking
                            </span>
                          }
                        } @else if (keys()[p.value]; as status) {
                          @if (status.is_set) {
                            @let tag = keyTag(status);
                            <kl-tag [value]="tag.value" [severity]="tag.severity" />
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
                              <kl-button
                                data-testid="wizard-recheck"
                                size="sm"
                                icon="refresh-cw"
                                loadingLabel="Checking…"
                                [loading]="rechecking()"
                                (click)="recheck()"
                              >Check again</kl-button>
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
                              <label class="text-[13px] font-semibold text-soft" for="wizard-key">Paste your API key</label>
                              <input
                                klInput
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
                <kl-button data-testid="wizard-back" variant="ghost" size="lg" icon="chevron-left" (click)="screen.set('welcome')">Back</kl-button>
                <span class="flex-1"></span>
                <kl-button
                  data-testid="wizard-skip"
                  variant="ghost"
                  size="lg"
                  [loading]="skipping()"
                  [disabled]="finishing()"
                  (click)="skip()"
                >Set up later</kl-button>
                <kl-button
                  data-testid="wizard-finish"
                  variant="primary"
                  size="lg"
                  loadingLabel="Saving…"
                  [loading]="finishing()"
                  [disabled]="!canFinish() || skipping()"
                  (click)="finish()"
                >Start using KeyLint</kl-button>
              </div>
            </section>
          }
        }

        @if (error(); as message) {
          <kl-message class="mt-4" data-testid="wizard-error" severity="error">{{ message }}</kl-message>
        }
      </div>
    </main>
  `,
  styles: [`
    :host { display: block; }
    .wizard {
      min-height: 100vh; display: flex; align-items: center; justify-content: center;
      padding: 32px 16px; background: var(--kl-ground); color: var(--kl-text);
    }
    h1 { margin: 0 0 12px; font-size: 28px; line-height: 1.2; font-weight: 700; letter-spacing: -0.02em; color: var(--kl-text-strong); }
    .lede { margin: 0 0 12px; font-size: 16px; line-height: 1.55; max-width: 32rem; }

    /* The shortcut is the product, so it is the picture: real keycaps. */
    .keys { display: flex; align-items: center; gap: 12px; margin-bottom: 36px; }
    .keycap {
      display: inline-flex; align-items: center; justify-content: center;
      min-width: 4rem; height: 4rem; padding: 0 1.1rem; border-radius: 12px;
      font: 600 20px/1 var(--kl-font-ui); color: var(--kl-text);
      background: linear-gradient(180deg, var(--kl-raised), var(--kl-rail));
      border: 1px solid var(--kl-line-strong);
      box-shadow: 0 5px 0 var(--kl-line-control), 0 8px 18px rgba(0, 0, 0, 0.45);
    }
    .keycap.trigger { border-color: var(--kl-primary); color: var(--kl-primary-text); animation: press 1.4s ease-in-out 0.6s 1 both; }
    /* One press, once, when the screen opens: this is what the user will do. */
    @keyframes press {
      0%, 30%, 100% { transform: translateY(0); box-shadow: 0 5px 0 var(--kl-line-control), 0 8px 18px rgba(0, 0, 0, 0.45); }
      12% { transform: translateY(4px); box-shadow: 0 1px 0 var(--kl-line-control), 0 2px 6px rgba(0, 0, 0, 0.45); }
    }
    @media (prefers-reduced-motion: reduce) { .keycap.trigger { animation: none; } }

    /* One list with dividers rather than a stack of cards: it is one choice. */
    .options {
      margin: 24px 0 16px; overflow: hidden;
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .option + .option { border-top: 1px solid var(--kl-line); }
    /* The same marker as the provider in use on Settings › AI Providers. */
    .option.selected { background: var(--kl-sheet-alt); box-shadow: inset 3px 0 0 var(--kl-primary); }
    .option-head { display: flex; align-items: center; gap: 14px; padding: 14px 18px; cursor: pointer; }
    .option-head input { flex: none; width: 17px; height: 17px; margin: 0; accent-color: var(--kl-primary); }
    .option-head input:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 3px; }
    .option-detail { display: flex; flex-direction: column; align-items: flex-start; gap: 10px; padding: 0 18px 16px 49px; }
    .note { margin: 0; font-size: 13px; line-height: 1.5; color: var(--kl-muted); }
    code { font-size: 12px; padding: 1px 4px; border-radius: 3px; background: var(--kl-selected); color: var(--kl-text-soft); }
    .inline-action {
      font: inherit; color: var(--kl-link); background: none; border: 0; padding: 0; cursor: pointer;
      text-decoration: underline; text-underline-offset: 2px;
    }
    .inline-action:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 8px; margin-top: 32px; }

    @media (max-width: 480px) {
      h1 { font-size: 23px; }
      .keycap { min-width: 3.25rem; height: 3.25rem; }
      .option-detail { padding-left: 18px; }
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
