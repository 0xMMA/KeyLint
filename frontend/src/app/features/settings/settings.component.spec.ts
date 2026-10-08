import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { SettingsComponent } from './settings.component';
import { WailsService, ClaudeCodeStatus, KeyStatus } from '../../core/wails.service';
import { createWailsMock, defaultSettings, defaultKeyStatus, defaultUpdateInfo, defaultClaudeCodeStatus, defaultModelList } from '../../../testing/wails-mock';

function makeActivatedRoute(tab?: string): Partial<ActivatedRoute> {
  return {
    snapshot: {
      queryParamMap: convertToParamMap(tab ? { tab } : {}),
    } as ActivatedRoute['snapshot'],
  };
}

// PrimeNG's overlay asks matchMedia whether it should go modal, and jsdom has
// no such function. Answering "no match" keeps the dropdown inline, which is
// what a desktop window does anyway. Test-only: the app itself must never ask
// matchMedia — see .claude/rules/architecture.md on dark mode.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener() {},
    removeListener() {},
    addEventListener() {},
    removeEventListener() {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// PrimeNG TabList uses ResizeObserver which is not available in jsdom
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

describe('SettingsComponent', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let component: SettingsComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute() },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;

    // Pre-initialize to avoid @if(settings) NG0100 from async ngOnInit
    component.settings = { ...defaultSettings };

    fixture.detectChanges();
    await fixture.whenStable();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // --- DOM tests ---

  it('renders tab list after settings load', () => {
    expect(el.querySelector('p-tablist')).toBeTruthy();
  });

  it('log-level section contains select and hint text', () => {
    const section = el.querySelector('[data-testid="log-level-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-select')).toBeTruthy();
    expect(section!.querySelector('small')).toBeTruthy();
  });

  it('sensitive-logging section contains both toggle and hint text', () => {
    const section = el.querySelector('[data-testid="sensitive-logging-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-toggle-switch')).toBeTruthy();
    expect(section!.querySelector('small')).toBeTruthy();
  });

  it('shortcut key input is present with correct initial value', () => {
    const input = el.querySelector<HTMLInputElement>('[data-testid="shortcut-input"]');
    expect(input).toBeTruthy();
    expect(input?.value).toBe('ctrl+g');
  });

  it('Save button is present', () => {
    expect(el.querySelector('[data-testid="save-btn"]')).toBeTruthy();
  });

  it('Reset to Defaults button is present', () => {
    expect(el.querySelector('[data-testid="reset-btn"]')).toBeTruthy();
  });

  it('saved banner appears in DOM after save', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="saved-banner"]')).toBeTruthy();
  });

  // --- Logic tests ---

  it('creates successfully', () => {
    expect(component).toBeTruthy();
  });

  it('loads settings on init', async () => {
    await component.ngOnInit();
    expect(wailsMock.loadSettings).toHaveBeenCalled();
    expect(component.settings).toMatchObject({ active_provider: 'openai' });
  });

  it('ngOnInit loads key statuses for all providers', async () => {
    await component.ngOnInit();
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('openai');
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('claude');
  });

  it('save() calls saveSettings with current settings', async () => {
    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ active_provider: 'openai' }),
    );
  });

  it('save() shows saved banner', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    expect(component.saved).toBe(true);
  });

  it('save() hides saved banner after 3 seconds', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    vi.advanceTimersByTime(3000);
    expect(component.saved).toBe(false);
  });

  it('save sends log_level to backend', async () => {
    component.settings!.log_level = 'warning';
    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ log_level: 'warning' }),
    );
  });

  it('save() does nothing when settings is null', async () => {
    component.settings = null;
    await component.save();
    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
  });

  it('resetToDefaults() calls resetSettings and reloads settings', async () => {
    await component.resetToDefaults();
    expect(wailsMock.resetSettings).toHaveBeenCalled();
    expect(wailsMock.loadSettings).toHaveBeenCalled();
  });

  it('saveKey() calls wails.setKey and refreshes status', async () => {
    wailsMock.setKey.mockResolvedValue(undefined);
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });

    const pk = component.providerKeys[0]; // openai
    pk.draftKey = 'sk-test';
    await component.saveKey(pk);

    expect(wailsMock.setKey).toHaveBeenCalledWith('openai', 'sk-test');
    expect(pk.status?.is_set).toBe(true);
    expect(pk.editing).toBe(false);
  });

  it('clearKey() calls wails.deleteKey and refreshes status', async () => {
    wailsMock.deleteKey.mockResolvedValue(undefined);
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });

    const pk = component.providerKeys[0];
    await component.clearKey(pk);

    expect(wailsMock.deleteKey).toHaveBeenCalledWith('openai');
    expect(pk.status?.is_set).toBe(false);
  });

  describe('activeTab from query param', () => {
    it('defaults to "general" when no tab param is present', async () => {
      // Outer beforeEach provides makeActivatedRoute() with no tab param
      await component.ngOnInit();
      expect(component.activeTab).toBe('general');
    });

    it('sets activeTab to "about" when tab=about query param is present', async () => {
      const route = TestBed.inject(ActivatedRoute);
      vi.spyOn(route.snapshot.queryParamMap, 'get').mockReturnValue('about');
      await component.ngOnInit();
      expect(component.activeTab).toBe('about');
    });
  });

  describe('About tab', () => {
    // Same wording as the sidebar (#21): one "v", and "dev" stays "dev".
    describe('version label', () => {
      for (const [raw, shown] of [
        ['v3.6.0', 'Version: v3.6.0'], ['3.6.0', 'Version: v3.6.0'], ['dev', 'Version: dev'],
        // Dev-channel builds are named by what they were built from.
        ['0.0.0-pr.12+abc1234def', 'Version: PR #12 · abc1234'], ['0.0.0-main+deadbee', 'Version: main · deadbee'],
      ] as const) {
        it(`shows "${raw}" as "${shown}"`, async () => {
          TestBed.resetTestingModule();
          const wailsMock = createWailsMock();
          wailsMock.getVersion.mockResolvedValue(raw);
          await TestBed.configureTestingModule({
            imports: [SettingsComponent],
            providers: [
              provideAnimationsAsync(),
              { provide: WailsService, useValue: wailsMock },
              { provide: ActivatedRoute, useValue: makeActivatedRoute('about') },
            ],
          }).compileComponents();
          const fixture = TestBed.createComponent(SettingsComponent);
          fixture.componentInstance.settings = { ...defaultSettings };
          fixture.detectChanges();
          await fixture.whenStable();
          await fixture.whenStable();
          fixture.detectChanges();

          const text = fixture.nativeElement.querySelector('[data-testid="app-version"]')?.textContent?.trim();
          expect(text).toBe(shown);
        });
      }
    });

    it('displays app version after init', async () => {
      wailsMock.getVersion.mockResolvedValue('3.6.0');
      await component.ngOnInit();
      fixture.detectChanges();
      expect(component.appVersion).toBe('3.6.0');
    });

    it('checkForUpdate() sets updateInfo when update is available', async () => {
      wailsMock.checkForUpdate.mockResolvedValue({
        ...defaultUpdateInfo,
        is_available: true,
        latest_version: '3.7.0',
        notes: 'Bug fixes',
      });
      await component.checkForUpdate();
      expect(component.updateInfo?.is_available).toBe(true);
      expect(component.updateInfo?.latest_version).toBe('3.7.0');
      expect(component.updateError).toBe('');
    });

    it('checkForUpdate() sets updateError when check throws', async () => {
      wailsMock.checkForUpdate.mockRejectedValue(new Error('network error'));
      await component.checkForUpdate();
      expect(component.updateError).toContain('network error');
      expect(component.updateInfo).toBeNull();
    });

    it('update channel selector renders in About tab', async () => {
      // Click on the About tab to activate it.
      const aboutTab = el.querySelector('p-tab[value="about"]') as HTMLElement;
      aboutTab?.click();
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      const section = el.querySelector('[data-testid="update-channel-section"]');
      expect(section).toBeTruthy();
      expect(section!.querySelector('p-select')).toBeTruthy();
    });

    it('installUpdate() sets updateSuccess on success', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: false });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(wailsMock.downloadAndInstall).toHaveBeenCalled();
    });

    it('installUpdate() shows restart message when restart_required is true', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: true });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(true);
    });

    describe('developer options', () => {
      // Renders Settings on the About tab with the given settings and build.
      async function renderAbout(opts: { developerOptions?: boolean; devBuild?: boolean } = {}) {
        TestBed.resetTestingModule();
        const mock = createWailsMock();
        mock.loadSettings.mockResolvedValue({ ...defaultSettings, developer_options: !!opts.developerOptions });
        if (opts.devBuild) {
          mock.getVersion.mockResolvedValue('0.0.0-pr.12+abc1234');
          mock.getBuildIdentity.mockResolvedValue({ is_dev_build: true, kind: 'pr', pr: 12, commit: 'abc1234', tag: 'v0.0.0-pr.12' });
        }
        await TestBed.configureTestingModule({
          imports: [SettingsComponent],
          providers: [
            provideAnimationsAsync(),
            { provide: WailsService, useValue: mock },
            { provide: ActivatedRoute, useValue: makeActivatedRoute('about') },
          ],
        }).compileComponents();
        const f = TestBed.createComponent(SettingsComponent);
        f.componentInstance.settings = { ...defaultSettings, developer_options: !!opts.developerOptions };
        f.detectChanges();
        await f.whenStable();
        await f.whenStable();
        f.detectChanges();
        const root: HTMLElement = f.nativeElement;
        const tap = async (times: number) => {
          for (let i = 0; i < times; i++) {
            (root.querySelector('[data-testid="version-tap"]') as HTMLButtonElement).click();
            await f.whenStable();
          }
          f.detectChanges();
          await f.whenStable();
          f.detectChanges();
        };
        return { f, root, mock, tap, q: (id: string) => root.querySelector(`[data-testid="${id}"]`) };
      }

      it('are hidden by default: no toggle, no dev channel', async () => {
        const { q } = await renderAbout();
        expect(q('developer-options-section')).toBeNull();
        expect(q('dev-channel')).toBeNull();
        expect(q('update-channel-section')).toBeTruthy();
      });

      it('count down from the third tap and unlock on the seventh, saving at once', async () => {
        const { q, tap, mock } = await renderAbout();

        await tap(2);
        expect(q('unlock-hint')?.textContent?.trim()).toBe('');
        await tap(4);
        expect(q('unlock-hint')?.textContent?.trim()).toBe('1 more tap to turn on the developer options.');
        expect(mock.setDeveloperOptions).not.toHaveBeenCalled();
        expect(q('dev-channel')).toBeNull();

        await tap(1);
        expect(mock.setDeveloperOptions).toHaveBeenCalledExactlyOnceWith(true);
        expect(mock.saveSettings).not.toHaveBeenCalled();
        expect(q('unlock-hint')?.textContent?.trim()).toBe('Developer options are on.');
        expect(q('developer-options-section')).toBeTruthy();
        expect(q('dev-channel')).toBeTruthy();
      });

      it('stay locked when the save fails, and say so', async () => {
        const { q, tap, mock } = await renderAbout();
        mock.setDeveloperOptions.mockRejectedValue(new Error('disk full'));
        await tap(7);
        expect(q('developer-options-error')?.textContent).toContain('disk full');
        expect(q('dev-channel')).toBeNull();
      });

      it('switch off again and hide the dev channel', async () => {
        const { f, q, mock } = await renderAbout({ developerOptions: true });
        expect(q('dev-channel')).toBeTruthy();

        (q('developer-options-toggle')?.querySelector('input') as HTMLInputElement).click();
        await f.whenStable();
        f.detectChanges();

        expect(mock.setDeveloperOptions).toHaveBeenCalledExactlyOnceWith(false);
        expect(q('developer-options-section')).toBeNull();
        expect(q('dev-channel')).toBeNull();
      });

      it('keep the switch on when turning them off fails', async () => {
        const { f, q, mock } = await renderAbout({ developerOptions: true });
        mock.setDeveloperOptions.mockRejectedValue(new Error('disk full'));

        (q('developer-options-toggle')?.querySelector('input') as HTMLInputElement).click();
        await f.whenStable();
        f.detectChanges();
        await f.whenStable();
        f.detectChanges();

        // NgModel writes a re-created control's value one microtask late.
        await f.whenStable();
        f.detectChanges();
        expect(q('developer-options-error')?.textContent).toContain('disk full');
        expect(q('dev-channel')).toBeTruthy();
        // What the switch draws: PrimeNG mirrors its state onto the host.
        expect(q('developer-options-toggle')?.getAttribute('data-p-checked')).toBe('true');
      });

      it('replace the normal update check with the dev channel in a dev build, even when off', async () => {
        const { q, mock } = await renderAbout({ devBuild: true });
        expect(q('check-update-btn')).toBeNull();
        // Still settable: it picks the release the "latest release" offer installs.
        expect(q('update-channel-section')).toBeTruthy();
        expect(q('dev-channel')).toBeTruthy();
        expect(mock.listDevBuilds).toHaveBeenCalled();
      });
    });

    it('installUpdate() shows standard success when restart_required is false', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: false });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(false);
    });
  });
});

describe('SettingsComponent — Claude Code provider card', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  // Renders the AI Providers tab with a given detection result.
  async function render(status: Partial<ClaudeCodeStatus>): Promise<void> {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });
    wailsMock.getClaudeCodeStatus.mockResolvedValue({ ...defaultClaudeCodeStatus, ...status });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute('providers') },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    // Pre-set both async results: letting them land mid-render trips NG0100.
    fixture.componentInstance.settings = { ...defaultSettings };
    fixture.componentInstance.claudeCodeStatus = { ...defaultClaudeCodeStatus, ...status };
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  function text(testid: string): string {
    return el.querySelector(`[data-testid="${testid}"]`)?.textContent?.trim() ?? '';
  }

  it('reports a signed-in CLI with its path and version', async () => {
    await render({ installed: true, loggedIn: true, path: '/home/dev/.local/bin/claude', version: '2.1.274' });

    expect(text('claude-code-status-tag')).toContain('signed in');
    expect(text('claude-code-detected')).toContain('/home/dev/.local/bin/claude');
    expect(text('claude-code-detected')).toContain('2.1.274');
    expect(el.querySelector('[data-testid="claude-code-signin-hint"]')).toBeNull();
  });

  it('tells an installed but signed-out user what to do', async () => {
    await render({ installed: true, loggedIn: false, path: '/usr/local/bin/claude', version: '2.1.274' });

    expect(text('claude-code-status-tag')).toContain('not signed in');
    expect(text('claude-code-signin-hint')).toContain('run');
    expect(text('claude-code-signin-hint')).toContain('sign in');
    // The sign-in happens in the user's own terminal, never inside KeyLint.
    expect(text('claude-code-signin-hint')).toContain('never reads or stores your credentials');
  });

  it('reports a machine without the CLI', async () => {
    await render({ installed: false });

    expect(text('claude-code-status-tag')).toContain('not installed');
    expect(text('claude-code-missing')).toContain('No Claude Code CLI found');
    expect(el.querySelector('[data-testid="claude-code-detected"]')).toBeNull();
  });

  it('says that environment API keys are not passed through to the CLI', async () => {
    await render({ installed: true, loggedIn: true, path: '/usr/local/bin/claude' });

    expect(text('claude-code-env-hint')).toContain('not passed through');
    expect(text('claude-code-env-hint')).toContain('subscription login');
  });

  it('never offers a key editor for the CLI', async () => {
    await render({ installed: true, loggedIn: true, path: '/usr/local/bin/claude' });

    const card = el.querySelector('[data-testid="claude-code-card"]');
    expect(card).not.toBeNull();
    expect(card!.querySelector('input')).toBeNull();
  });

  it('does not force a probe just because the screen opened', async () => {
    // The probe spawns processes and is cached on the Go side. Forcing on load
    // would defeat that for the caller most likely to be hit repeatedly:
    // Settings → Pyramidize → Settings is two navigations.
    await render({ installed: true, loggedIn: true });

    // ngOnInit reaches the probe behind five awaits, so asserting straight
    // after render() finds an empty call list and proves nothing — which is
    // what an earlier version of this test did.
    for (let i = 0; i < 10; i++) {
      await fixture.whenStable();
    }
    fixture.detectChanges();

    expect(wailsMock.getClaudeCodeStatus, 'the probe never ran, so this asserts nothing')
      .toHaveBeenCalled();
    for (const call of wailsMock.getClaudeCodeStatus.mock.calls) {
      expect(call[0] ?? false, 'opening Settings must take the cached answer').toBe(false);
    }
  });

  it('forces a fresh probe when the button is pressed', async () => {
    await render({ installed: true, loggedIn: true });
    wailsMock.getClaudeCodeStatus.mockClear();

    // Through the DOM, so the template binding is covered too: passing force
    // from the component but not from the button would be invisible otherwise.
    // The inner <button>: PrimeNG's (onClick) fires from there, not from the
    // host element carrying the test id.
    const host = el.querySelector('[data-testid="claude-code-recheck"]')!;
    (host.querySelector('button') ?? (host as HTMLElement)).click();
    await fixture.whenStable();

    expect(wailsMock.getClaudeCodeStatus).toHaveBeenCalledWith(true);
  });

  it('re-checks on demand, so signing in elsewhere is picked up', async () => {
    await render({ installed: true, loggedIn: false, path: '/usr/local/bin/claude' });
    expect(text('claude-code-status-tag')).toContain('not signed in');

    wailsMock.getClaudeCodeStatus.mockResolvedValue({
      ...defaultClaudeCodeStatus, installed: true, loggedIn: true, path: '/usr/local/bin/claude',
    });
    const recheck = el.querySelector<HTMLButtonElement>('[data-testid="claude-code-recheck"] button');
    expect(recheck).not.toBeNull();
    recheck!.click();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(text('claude-code-status-tag')).toContain('signed in');
  });

  it('runs detection itself rather than waiting to be asked', async () => {
    await render({ installed: true, loggedIn: true });
    // ngOnInit awaits five backend calls before it kicks off detection.
    await fixture.whenStable();
    await fixture.whenStable();

    expect(wailsMock.getClaudeCodeStatus).toHaveBeenCalled();
  });

  it('offers the CLI as an active provider', async () => {
    await render({ installed: true, loggedIn: true });

    const values = fixture.componentInstance.providers.map(p => p.value);
    expect(values).toContain('claude-code');
  });
});


describe('SettingsComponent — model selection', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let component: SettingsComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  type StoredModels = Record<string, Partial<{ fix: string; pyramidize: string; fix_effort: string; pyramidize_effort: string }>>;

  /** Fills in the fields the binding type requires, so a test names only what it means. */
  function stored(models: StoredModels): NonNullable<typeof defaultSettings.models> {
    const out: NonNullable<typeof defaultSettings.models> = {};
    for (const [provider, m] of Object.entries(models)) {
      out[provider] = { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: '', ...m };
    }
    return out;
  }

  async function render(
    source: string = 'live',
    models: StoredModels = {},
    active: string = 'claude',
    listed = defaultModelList.models,
  ): Promise<void> {
    const settings = { ...defaultSettings, active_provider: active, models: stored(models) };
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue(structuredClone(settings));
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });
    wailsMock.listModels.mockResolvedValue({ models: listed, source });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute('providers') },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    component.settings = structuredClone(settings);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  /** Opens a PrimeNG select so its options are in the DOM. */
  function openDropdown(testid: string): void {
    el.querySelector<HTMLElement>(`[data-testid="${testid}"]`)!.click();
    fixture.detectChanges();
  }

  /** The option rows of whichever select is open. */
  function optionTexts(): string[] {
    return Array.from(document.querySelectorAll('.p-select-option'))
      .map(o => o.textContent?.trim() ?? '');
  }

  function q(testid: string): HTMLElement | null {
    return el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);
  }

  /** Presses a card's "Use this" and lets the switch land. */
  async function clickUse(provider: string): Promise<void> {
    q(`provider-use-${provider}`)!.querySelector('button')!.click();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r));
    fixture.detectChanges();
  }

  /** Opens a select and picks the option with this text. */
  function pick(testid: string, text: string): void {
    openDropdown(testid);
    Array.from(document.querySelectorAll<HTMLElement>('.p-select-option'))
      .find(o => o.textContent?.trim() === text)!.click();
    fixture.detectChanges();
  }

  it('shows the pickers of the provider in use, and only those', async () => {
    await render('live', {}, 'claude');

    expect(q('active-provider-name')?.textContent).toContain('Anthropic API');
    expect(q('model-fix-claude')).not.toBeNull();
    expect(q('model-pyramidize-claude')).not.toBeNull();
    // The old page listed every provider's pickers at once.
    for (const other of ['openai', 'claude-code', 'ollama']) {
      expect(q(`model-fix-${other}`), other).toBeNull();
    }
  });

  it('follows a switch, and switching back restores what was chosen there', async () => {
    await render('live', { claude: { fix: 'claude-sonnet-4-6' }, 'claude-code': { fix: 'opus' } }, 'claude');

    await clickUse('claude-code');
    expect(q('active-provider-name')?.textContent).toContain('Claude Code');
    expect(q('model-fix-claude-code')).not.toBeNull();
    expect(q('model-fix-claude')).toBeNull();
    expect(component.modelFor('claude-code', 'fix')).toBe('opus');

    await clickUse('claude');
    expect(q('model-fix-claude')!.querySelector('input')!.value).toBe('claude-sonnet-4-6');
  });

  it('shows no pickers when no provider is in use', async () => {
    await render('live', {}, 'bedrock');

    expect(q('active-provider')).toBeNull();
    expect(q('providers-summary')?.textContent).toContain('No provider is in use');
  });

  it('asks the backend for each provider list', async () => {
    await render();

    for (const provider of ['openai', 'claude', 'claude-code', 'ollama']) {
      expect(wailsMock.listModels).toHaveBeenCalledWith(provider);
    }
  });

  // Four situations, four sentences. "Not live" as one word would send a user
  // hunting for a network problem when the real answer is an unpasted key, or a
  // daemon that is running fine with nothing pulled.
  it('says the provider could not be reached', async () => {
    await render('unreachable', {}, 'openai');

    expect(q('models-note-openai')?.textContent).toContain('could not be reached');
  });

  it('says a key is missing rather than blaming the network', async () => {
    await render('no-credentials', {}, 'openai');

    const note = q('models-note-openai')?.textContent ?? '';
    expect(note).toContain('add a key');
    expect(note).not.toContain('could not be reached');
  });

  it('says an Ollama daemon has nothing pulled rather than calling it unreachable', async () => {
    wailsMock = createWailsMock();
    const settings = { ...defaultSettings, active_provider: 'ollama', models: {} };
    wailsMock.loadSettings.mockResolvedValue(structuredClone(settings));
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });
    wailsMock.listModels.mockImplementation(async (provider: string) =>
      provider === 'ollama'
        ? { models: [], source: 'empty' }
        : { ...defaultModelList, source: 'live' });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute('providers') },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    component.settings = structuredClone(settings);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();

    const note = q('models-note-ollama')?.textContent ?? '';
    expect(note).toContain('No models pulled yet');
    expect(note).not.toContain('could not be reached');
  });

  it('hides the note when the list came from the provider', async () => {
    await render('live', {}, 'openai');

    expect(q('models-note-openai')).toBeNull();
  });

  it('lets a model be typed for the API providers', async () => {
    await render('live', {}, 'openai');

    // An editable PrimeNG select renders a text input; a closed one does not.
    expect(q('model-fix-openai')!.querySelector('input')).not.toBeNull();
  });

  it('offers only the list for the CLI, whose aliases follow the generation', async () => {
    await render('fixed', {}, 'claude-code');

    expect(q('model-fix-claude-code')).not.toBeNull();
    expect(q('model-fix-claude-code')!.querySelector('input')).toBeNull();
  });

  it('offers KeyLint\'s default as a selectable entry that says what it is today', async () => {
    await render('live', {}, 'claude');

    // PrimeNG only renders a placeholder while the value is null, and this
    // component writes "" — so the default has to be a real option to be
    // reachable at all. It names the alias it stands for and, where the
    // listing says, the model that alias is today.
    openDropdown('model-pyramidize-claude');
    expect(optionTexts()[0]).toContain('Default: Sonnet (latest)');
    expect(optionTexts()[0]).toContain('claude-sonnet-5-5');
  });

  it('names the default in the empty field and the model it runs under it', async () => {
    await render('live', {}, 'claude');

    // The field holds no text for the default (see below), so the placeholder
    // says what the default is and the line under it which model that is now.
    const input = q('model-pyramidize-claude')!.querySelector<HTMLInputElement>('input')!;
    expect(input.value).toBe('');
    expect(input.getAttribute('placeholder')).toBe('Default: Sonnet (latest)');
    expect(q('model-effective-pyramidize')?.textContent?.trim()).toBe('Now claude-sonnet-5-5');
  });

  it('turns choosing the default back from the list into an empty setting', async () => {
    await render('live', { claude: { pyramidize: 'claude-sonnet-4-6' } }, 'claude');

    pick('model-pyramidize-claude', 'Default: Sonnet (latest)claude-sonnet-5-5');

    expect(component.modelFor('claude', 'pyramidize')).toBe('');
    expect(component.settings!.models!['claude']?.pyramidize).toBe('');
  });

  it('leaves the editable field empty for the default, so typing is not appended to a word', async () => {
    await render('live', {}, 'openai');

    // The field is prefilled with whatever label the selected option carries.
    // A readable one here would mean typing "gpt-4.1" without select-all
    // persists "KeyLint's defaultgpt-4.1".
    const input = q('model-fix-openai')!.querySelector<HTMLInputElement>('input')!;
    expect(input.value).toBe('');

    input.value = input.value + 'gpt-4.1';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();

    expect(component.modelFor('openai', 'fix')).toBe('gpt-4.1');
  });

  it('shows the model ID in the editable field, not the display name', async () => {
    // Anthropic is the provider whose display names differ from its IDs.
    await render('live', { claude: { fix: 'claude-sonnet-4-6' } }, 'claude');

    // PrimeNG writes optionLabel into this field and submits whatever stands
    // there as the value — so "Sonnet 4.6" here would persist as a model ID no
    // provider knows.
    expect(q('model-fix-claude')!.querySelector<HTMLInputElement>('input')?.value).toBe('claude-sonnet-4-6');
  });

  // The bug: PrimeNG's option row is a flex row, so name and ID sat side by
  // side with nothing between them ("Sonnet 4.6claude-sonnet-4-6").
  it('puts the readable name and the ID on separate lines in the dropdown', async () => {
    await render('live', {}, 'claude');

    openDropdown('model-fix-claude');
    const option = document.querySelector('[data-testid="model-option-claude-sonnet-4-6"]')!;
    expect(option).not.toBeNull();
    const name = option.querySelector('.model-option-name')!;
    const id = option.querySelector('.model-option-id')!;
    expect(name.textContent).toBe('Sonnet 4.6');
    expect(id.textContent).toBe('claude-sonnet-4-6');
    // One wrapper holds both, so the row's flex layout sees a single child;
    // the wrapper itself stacks them.
    expect(option.parentElement!.children.length).toBe(1);
    expect(getComputedStyle(option).flexDirection).toBe('column');

    const alias = document.querySelector('[data-testid="model-option-sonnet"]')!;
    expect(alias.querySelector('.model-option-name')!.textContent).toBe('Sonnet (latest)');
    expect(alias.querySelector('.model-option-id')!.textContent).toBe('claude-sonnet-5-5');
  });

  it('shows the readable name alone for a choice in the CLI picker', async () => {
    await render('fixed', { 'claude-code': { fix: 'sonnet' } }, 'claude-code');

    const shown = q('model-fix-claude-code')!.querySelector('.model-selected')?.textContent?.trim();
    expect(shown).toBe('Sonnet (latest)');
  });

  it('never calls a provider list "built-in" when there is no endpoint to ask', async () => {
    await render('fixed', {}, 'claude-code');

    expect(q('models-note-claude-code')).toBeNull();
  });

  it('uses the same provider labels as the provider cards', async () => {
    await render();

    for (const mp of component.modelProviders) {
      const active = component.providers.find(p => p.value === mp.id);
      expect(mp.label, mp.id).toBe(active!.label);
    }
  });

  it('keeps a model the provider does not list, so an unlisted one can be typed', async () => {
    await render('live', {}, 'openai');

    const input = q('model-fix-openai')!.querySelector<HTMLInputElement>('input')!;
    input.value = 'gpt-not-in-any-list';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();

    expect(component.settings!.models!['openai']?.fix).toBe('gpt-not-in-any-list');
  });

  it('treats an empty field as "use the default" rather than as a model named ""', async () => {
    await render();

    component.setModel('claude', 'pyramidize', '   ');

    expect(component.modelFor('claude', 'pyramidize')).toBe('');
  });

  // ── effort ──

  it('offers an effort per feature where the provider does something with it', async () => {
    for (const provider of ['claude', 'claude-code', 'openai']) {
      TestBed.resetTestingModule();
      await render(provider === 'claude-code' ? 'fixed' : 'live', {}, provider);
      expect(q(`effort-fix-${provider}`), provider).not.toBeNull();
      expect(q(`effort-pyramidize-${provider}`), provider).not.toBeNull();
      expect(q('effort-note'), provider).not.toBeNull();
    }
  });

  it('offers no effort for Ollama, which has nothing to send it to', async () => {
    await render('live', {}, 'ollama');

    expect(q('effort-fix-ollama')).toBeNull();
    expect(q('effort-note')).toBeNull();
  });

  it('starts on the model default and lists the levels the APIs take', async () => {
    await render('live', {}, 'claude');

    openDropdown('effort-fix-claude');
    expect(optionTexts()).toEqual(['Model default', 'Low', 'Medium', 'High', 'Extra high', 'Max']);
  });

  it('stores an effort chosen in the picker, for this provider and feature, and saves it', async () => {
    await render('live', {}, 'claude');

    pick('effort-pyramidize-claude', 'High');

    expect(component.effortFor('claude', 'pyramidize')).toBe('high');
    expect(component.effortFor('claude', 'fix')).toBe('');
    expect(component.effortFor('claude-code', 'pyramidize')).toBe('');

    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(expect.objectContaining({
      models: expect.objectContaining({ claude: expect.objectContaining({ pyramidize_effort: 'high' }) }),
    }));
  });

  it('says Fix keeps a Haiku from reasoning, until an effort is chosen', async () => {
    await render('live', {}, 'claude');
    expect(q('fix-fast-note')?.textContent).toContain('without reasoning');

    pick('effort-fix-claude', 'Low');
    expect(component.effortFor('claude', 'fix')).toBe('low');
    expect(q('fix-fast-note')).toBeNull();
  });

  it('names the default alias even when no list could be loaded', async () => {
    await render('unreachable', {}, 'claude', []);

    const input = q('model-fix-claude')!.querySelector<HTMLInputElement>('input')!;
    expect(input.getAttribute('placeholder')).toBe('Default: Haiku (latest)');
    expect(q('fix-fast-note')?.textContent).toContain('Fix asks Haiku (latest)');
  });

  it('does not promise a quick Fix where the model keeps its own default', async () => {
    await render('live', { claude: { fix: 'sonnet' } }, 'claude');
    expect(q('fix-fast-note')).toBeNull();

    // The CLI has no switch for it, so saying so would be false.
    TestBed.resetTestingModule();
    await render('fixed', {}, 'claude-code');
    expect(q('fix-fast-note')).toBeNull();
  });

  it('re-asks after the Ollama URL is saved, the way saving a key does', async () => {
    await render();
    const before = wailsMock.listModels.mock.calls.length;

    component.settings!.providers = { ...component.settings!.providers, ollama_url: 'http://elsewhere:11434' };
    await component.save();
    await fixture.whenStable();

    // The daemon at the new address has other models pulled, so the old list is
    // not stale — it is the wrong machine's.
    expect(wailsMock.listModels.mock.calls.length).toBeGreaterThan(before);
  });

  it('does not re-ask when a save left the Ollama URL alone', async () => {
    await render();
    const before = wailsMock.listModels.mock.calls.length;

    await component.save();
    await fixture.whenStable();

    expect(wailsMock.listModels.mock.calls.length).toBe(before);
  });

  it('re-asks after a reset, which puts the Ollama URL back to its default', async () => {
    await render();
    const before = wailsMock.listModels.mock.calls.length;

    await component.resetToDefaults();
    await fixture.whenStable();

    expect(wailsMock.listModels.mock.calls.length).toBeGreaterThan(before);
  });

  it('keeps the two features apart', async () => {
    await render();

    component.setModel('claude', 'fix', 'claude-haiku-4-5-20251001');
    component.setModel('claude', 'pyramidize', 'claude-opus-4-6');

    expect(component.modelFor('claude', 'fix')).toBe('claude-haiku-4-5-20251001');
    expect(component.modelFor('claude', 'pyramidize')).toBe('claude-opus-4-6');
  });
});


// "If I have both an Anthropic API key and the Claude Code CLI, I cannot see or
// define which is prioritized." One provider is in use, the AI Providers tab
// shows which, and switching is one click there.
describe('SettingsComponent — which provider is in use', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  interface Setup {
    active: string;
    tab?: string;
    keys?: Record<string, KeyStatus['source']>;
    cli?: Partial<ClaudeCodeStatus>;
    ollamaSource?: string;
    ollamaURL?: string;
  }

  async function render(setup: Setup): Promise<void> {
    TestBed.resetTestingModule();
    const settings = {
      ...defaultSettings,
      active_provider: setup.active,
      providers: { ...defaultSettings.providers, ollama_url: setup.ollamaURL ?? '' },
    };
    const cli = { ...defaultClaudeCodeStatus, ...setup.cli };
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockImplementation(async () => structuredClone(settings));
    wailsMock.getKeyStatus.mockImplementation(async (provider: string) => {
      const source = setup.keys?.[provider];
      return source ? { is_set: true, source } : { ...defaultKeyStatus };
    });
    wailsMock.getClaudeCodeStatus.mockResolvedValue(cli);
    wailsMock.listModels.mockImplementation(async (provider: string) =>
      provider === 'ollama'
        ? { models: [], source: setup.ollamaSource ?? 'live' }
        : { ...defaultModelList, source: 'live' });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute(setup.tab ?? 'providers') },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    // Pre-set the async results: letting them land mid-render trips NG0100.
    fixture.componentInstance.settings = structuredClone(settings);
    fixture.componentInstance.claudeCodeStatus = cli;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  function q(testid: string): HTMLElement | null {
    return el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);
  }

  /** Which cards carry the "In use" marker, by provider ID. */
  function inUse(): string[] {
    return Array.from(el.querySelectorAll('[data-testid^="provider-in-use-"]'))
      .map(t => t.getAttribute('data-testid')!.replace('provider-in-use-', ''));
  }

  /** Lets promise chains the fixture does not track run out, then repaints. */
  async function settle(): Promise<void> {
    await fixture.whenStable();
    await new Promise(r => setTimeout(r));
    fixture.detectChanges();
  }

  async function clickUse(provider: string): Promise<void> {
    q(`provider-use-${provider}`)!.querySelector('button')!.click();
    await settle();
  }

  const BOTH = { keys: { claude: 'keyring' as const }, cli: { installed: true, loggedIn: true } };

  it('marks exactly the saved provider as in use, with both configured', async () => {
    await render({ active: 'claude-code', ...BOTH });

    expect(inUse()).toEqual(['claude-code']);
    for (const other of ['openai', 'claude', 'ollama']) {
      expect(q(`provider-use-${other}`), other).not.toBeNull();
    }
    expect(q('provider-use-claude-code')).toBeNull();
    expect(q('providers-summary')!.textContent).toContain('Claude Code (installed CLI)');
  });

  it('gives every offered provider a card, in a stable order', async () => {
    await render({ active: 'openai' });

    const cards = Array.from(el.querySelectorAll('[data-testid^="provider-card-"]'))
      .map(c => c.getAttribute('data-testid'));
    expect(cards).toEqual(['provider-card-openai', 'provider-card-claude', 'provider-card-claude-code', 'provider-card-ollama']);
  });

  it('switches with one click, saving only the provider', async () => {
    await render({ active: 'claude-code', ...BOTH });

    await clickUse('claude');

    expect(wailsMock.setActiveProvider).toHaveBeenCalledTimes(1);
    expect(wailsMock.setActiveProvider).toHaveBeenCalledWith('claude');
    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
    expect(inUse()).toEqual(['claude']);
    expect(q('provider-use-claude-code')).not.toBeNull();
    expect(q('providers-summary')!.textContent).toContain('Anthropic API');
    expect(q('provider-announcement')!.textContent).toContain('KeyLint now uses Anthropic API.');
  });

  // A switch must not commit edits the user has not confirmed, least of all
  // on a tab out of view (Sensitive Logging writes full payloads to the log).
  it('leaves other unsaved edits pending, for the Save button', async () => {
    await render({ active: 'claude-code', ...BOTH });
    fixture.componentInstance.settings!.log_level = 'debug';
    fixture.componentInstance.settings!.sensitive_logging = true;

    await clickUse('claude');

    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
    q('save-btn')!.querySelector('button')!.click();
    await settle();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(expect.objectContaining({
      active_provider: 'claude',
      sensitive_logging: true,
    }));
  });

  it('keeps the marker where it was when the switch fails, with focus on it', async () => {
    await render({ active: 'claude-code', ...BOTH });
    document.body.appendChild(el);
    wailsMock.setActiveProvider.mockRejectedValueOnce(new Error('disk full'));

    await clickUse('claude');

    expect(inUse()).toEqual(['claude-code']);
    expect(q('provider-switch-error')!.textContent).toContain('disk full');
    expect(document.activeElement).toBe(q('provider-heading-claude-code'));
    el.remove();
  });

  it('shows progress on the pressed card and moves the marker once saved', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.setActiveProvider.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('provider-use-claude')!.querySelector('button')!.click();
    fixture.detectChanges();

    expect(q('provider-use-claude')!.querySelector('.p-button-loading')).not.toBeNull();
    expect(inUse()).toEqual(['claude-code']);
    finish();
    await settle();
    expect(inUse()).toEqual(['claude']);
  });

  it('runs one switch at a time', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.setActiveProvider.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('provider-use-claude')!.querySelector('button')!.click();
    fixture.detectChanges();
    expect(q('provider-use-openai')!.querySelector('button')!.disabled).toBe(true);
    await fixture.componentInstance.useProvider('openai');
    finish();
    await settle();

    expect(wailsMock.setActiveProvider).toHaveBeenCalledTimes(1);
    expect(inUse()).toEqual(['claude']);
  });

  it('lets a provider without a key be chosen, and says why it will not work yet', async () => {
    await render({ active: 'claude-code', ...BOTH });

    await clickUse('openai');

    expect(inUse()).toEqual(['openai']);
    expect(q('provider-not-ready-openai')!.textContent).toContain('no API key');
  });

  it('warns about a CLI that is not signed in', async () => {
    await render({ active: 'claude-code', cli: { installed: true, loggedIn: false } });

    expect(q('provider-not-ready-claude-code')!.textContent).toContain('not signed in');
  });

  it('warns about a CLI that is not installed', async () => {
    await render({ active: 'claude-code', cli: { installed: false } });

    expect(q('provider-not-ready-claude-code')!.textContent).toContain('not installed');
  });

  it('warns when Ollama cannot be reached, naming the default address for an empty URL', async () => {
    await render({ active: 'ollama', ollamaSource: 'unreachable' });

    expect(q('provider-not-ready-ollama')!.textContent).toContain('http://localhost:11434');
  });

  it('names the configured Ollama address when one is set', async () => {
    await render({ active: 'ollama', ollamaSource: 'unreachable', ollamaURL: 'http://gpu-box:11434' });

    expect(q('provider-not-ready-ollama')!.textContent).toContain('http://gpu-box:11434');
  });

  // An empty field is localhost on the default port, which is where Ollama
  // listens. Warning about it would be a false alarm.
  it('does not warn about a reachable Ollama with an empty URL', async () => {
    await render({ active: 'ollama', ollamaSource: 'live' });

    expect(inUse()).toEqual(['ollama']);
    expect(q('provider-not-ready-ollama')).toBeNull();
  });

  it('does not warn about the provider in use when it works', async () => {
    await render({ active: 'claude', ...BOTH });

    expect(inUse()).toEqual(['claude']);
    expect(el.querySelector('[data-testid^="provider-not-ready-"]')).toBeNull();
  });

  // The status tags already say "no key"; a warning on every unused card
  // would bury the one that matters.
  it('keeps warnings to the card in use', async () => {
    await render({ active: 'claude-code', cli: { installed: true, loggedIn: true } });

    expect(q('key-status-openai')!.textContent).toContain('no key');
    expect(q('provider-not-ready-openai')).toBeNull();
  });

  it('says when an environment variable supplies the key, and which one', async () => {
    await render({ active: 'claude', keys: { claude: 'env' } });

    expect(q('key-status-claude')!.textContent).toContain('env var');
    expect(q('key-env-hint-claude')!.textContent).toContain('ANTHROPIC_API_KEY');
    expect(q('provider-card-claude')!.textContent).not.toContain('Set Key');
    expect(q('provider-not-ready-claude')).toBeNull();
  });

  it('does not save a half-typed Ollama URL with a switch', async () => {
    await render({ active: 'claude', ...BOTH });

    const input = q('provider-card-ollama')!.querySelector<HTMLInputElement>('[data-testid="ollama-url-input"]')!;
    input.value = 'http://gpu-bo';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await clickUse('ollama');

    expect(wailsMock.setActiveProvider).toHaveBeenCalledWith('ollama');
    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
    expect((q('ollama-url-input') as HTMLInputElement).value).toBe('http://gpu-bo');
  });

  it('holds Save and Reset while a switch is saving', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.setActiveProvider.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('provider-use-claude')!.querySelector('button')!.click();
    fixture.detectChanges();

    expect(q('save-btn')!.querySelector('button')!.disabled).toBe(true);
    expect(q('reset-btn')!.querySelector('button')!.disabled).toBe(true);
    finish();
    await settle();
    expect(q('save-btn')!.querySelector('button')!.disabled).toBe(false);
  });

  it('does not let a reset start in the middle of a switch', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.setActiveProvider.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('provider-use-claude')!.querySelector('button')!.click();
    fixture.detectChanges();
    q('reset-btn')!.querySelector('button')!.click();
    finish();
    await settle();

    expect(wailsMock.resetSettings).not.toHaveBeenCalled();
    expect(inUse()).toEqual(['claude']);
  });

  it('holds every "Use this" while a reset runs', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.resetSettings.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('reset-btn')!.querySelector('button')!.click();
    fixture.detectChanges();

    for (const p of ['openai', 'claude', 'ollama']) {
      expect(q(`provider-use-${p}`)!.querySelector('button')!.disabled, p).toBe(true);
    }
    await fixture.componentInstance.useProvider('claude');
    expect(wailsMock.setActiveProvider).not.toHaveBeenCalled();
    finish();
    await settle();
    expect(q('provider-use-claude')!.querySelector('button')!.disabled).toBe(false);
  });

  it('holds every "Use this" while a save runs', async () => {
    await render({ active: 'claude-code', ...BOTH });
    let finish!: () => void;
    wailsMock.saveSettings.mockImplementationOnce(() => new Promise<void>(r => { finish = r; }));

    q('save-btn')!.querySelector('button')!.click();
    fixture.detectChanges();

    expect(q('provider-use-claude')!.querySelector('button')!.disabled).toBe(true);
    finish();
    await settle();
    expect(q('provider-use-claude')!.querySelector('button')!.disabled).toBe(false);
  });

  it('names the provider on each "Use this" button, keeping the visible words first', async () => {
    await render({ active: 'claude-code', ...BOTH });

    expect(q('provider-use-claude')!.querySelector('button')!.getAttribute('aria-label')).toBe('Use this: Anthropic API');
  });

  it('moves focus to the heading of the card it marks, since the pressed button goes away', async () => {
    await render({ active: 'claude-code', ...BOTH });
    document.body.appendChild(el);

    await clickUse('claude');

    expect(document.activeElement).toBe(q('provider-heading-claude'));
    expect(document.activeElement!.textContent).toContain('Anthropic API');
    el.remove();
  });

  it('clears a failed switch\'s error once a later save goes through', async () => {
    await render({ active: 'claude-code', ...BOTH });
    wailsMock.setActiveProvider.mockRejectedValueOnce(new Error('disk full'));
    await clickUse('claude');
    expect(q('provider-switch-error')).not.toBeNull();

    q('save-btn')!.querySelector('button')!.click();
    await settle();

    expect(q('provider-switch-error')).toBeNull();
  });

  it('announces a failed switch once, through the error, not the live region too', async () => {
    await render({ active: 'claude-code', ...BOTH });
    wailsMock.setActiveProvider.mockRejectedValueOnce(new Error('disk full'));

    await clickUse('claude');

    expect(q('provider-switch-error')!.querySelector('[role="alert"]')).not.toBeNull();
    expect(q('provider-announcement')!.textContent!.trim()).toBe('');
  });

  it('focuses the error after a failed switch when no card is in use', async () => {
    await render({ active: 'bedrock', ...BOTH });
    document.body.appendChild(el);
    wailsMock.setActiveProvider.mockRejectedValueOnce(new Error('disk full'));

    await clickUse('claude');

    expect(document.activeElement).toBe(q('provider-switch-error'));
    el.remove();
  });

  it('clears an earlier Save error once a switch goes through', async () => {
    await render({ active: 'claude-code', ...BOTH });
    wailsMock.saveSettings.mockRejectedValueOnce(new Error('disk full'));
    await fixture.componentInstance.save();
    fixture.detectChanges();
    expect(q('save-error')).not.toBeNull();

    await clickUse('claude');

    expect(q('save-error')).toBeNull();
  });

  it('re-probes the CLI when switching to it, bypassing the cached answer', async () => {
    await render({ active: 'claude', ...BOTH });
    wailsMock.getClaudeCodeStatus.mockClear();

    await clickUse('claude-code');

    expect(wailsMock.getClaudeCodeStatus).toHaveBeenCalledWith(true);
  });

  it('re-reads the key status when switching to an API provider', async () => {
    await render({ active: 'claude-code', ...BOTH });
    wailsMock.getKeyStatus.mockClear();
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });

    await clickUse('openai');

    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('openai');
    expect(q('key-status-openai')!.textContent).toContain('key set');
    expect(q('provider-not-ready-openai')).toBeNull();
  });

  it('shows a failed Save beside the button instead of throwing', async () => {
    await render({ active: 'claude-code', ...BOTH });
    wailsMock.saveSettings.mockRejectedValueOnce(new Error('disk full'));

    await expect(fixture.componentInstance.save()).resolves.toBeUndefined();
    fixture.detectChanges();

    expect(q('save-error')!.textContent).toContain('disk full');
    expect(q('saved-banner')).toBeNull();
  });

  // Presets are saved by the backend on their own. Save sends the whole
  // settings object, so a stale preset list in it would undo the change.
  it('does not undo a preset added in the same visit when saving', async () => {
    await render({ active: 'claude-code', ...BOTH });
    const added = { sourceApp: 'Outlook', documentType: 'email' };
    wailsMock.getAppPresets.mockResolvedValue([added]);
    fixture.componentInstance.startAddPreset();
    fixture.componentInstance.addPresetDraft = { ...added };
    await fixture.componentInstance.saveAddPreset();

    await fixture.componentInstance.save();

    expect(wailsMock.saveSettings).toHaveBeenLastCalledWith(expect.objectContaining({
      app_presets: [added],
    }));
  });

  it('drops the old address\'s Ollama warning once a new URL is saved', async () => {
    await render({ active: 'ollama', ollamaSource: 'unreachable' });
    expect(q('provider-not-ready-ollama')).not.toBeNull();
    // The new address has not answered yet.
    wailsMock.listModels.mockImplementation(() => new Promise(() => {}));

    const input = q('ollama-url-input') as HTMLInputElement;
    input.value = 'http://gpu-box:11434';
    input.dispatchEvent(new Event('input'));
    q('save-btn')!.querySelector('button')!.click();
    await settle();

    expect(q('provider-not-ready-ollama')).toBeNull();
  });

  describe('General tab', () => {
    it('names the provider in use instead of offering a dropdown', async () => {
      await render({ active: 'claude-code', tab: 'general', ...BOTH });

      expect(q('provider-pointer-text')!.textContent).toContain('Claude Code (installed CLI)');
      expect(q('active-provider-select')).toBeNull();
    });

    it('opens the AI Providers tab from the pointer', async () => {
      await render({ active: 'claude-code', tab: 'general', ...BOTH });
      const providersPanel = () => el.querySelector<HTMLElement>('p-tabpanel[data-p-active="true"]');
      expect(providersPanel()!.contains(q('provider-pointer'))).toBe(true);

      q('provider-pointer-link')!.click();
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      expect(providersPanel()!.contains(q('provider-card-claude'))).toBe(true);
    });

    // The tabs are two-way bound. One-way, a deep link to AI Providers would
    // leave the property on "providers" after the user clicked General, and
    // the pointer setting it to "providers" again would change nothing.
    it('opens AI Providers from the pointer after a deep link and a tab click', async () => {
      await render({ active: 'claude-code', tab: 'providers', ...BOTH });
      const activePanel = () => el.querySelector<HTMLElement>('p-tabpanel[data-p-active="true"]');
      const generalTab = Array.from(el.querySelectorAll<HTMLElement>('[role="tab"]'))
        .find(t => t.textContent?.trim() === 'General')!;

      generalTab.click();
      await settle();
      expect(activePanel()!.contains(q('provider-pointer'))).toBe(true);

      q('provider-pointer-link')!.click();
      await settle();
      expect(activePanel()!.contains(q('provider-card-claude'))).toBe(true);
    });

    it('moves focus to the AI Providers tab, out of the panel that hides', async () => {
      await render({ active: 'claude-code', tab: 'general', ...BOTH });
      document.body.appendChild(el);

      q('provider-pointer-link')!.click();
      await fixture.whenStable();

      expect(document.activeElement).toBe(q('providers-summary'));
      el.remove();
    });

    it('repeats the warning when the provider in use cannot work', async () => {
      await render({ active: 'claude', tab: 'general' });

      expect(q('provider-pointer-problem')!.textContent).toContain('no API key');
    });

    it('stays quiet when the provider in use works', async () => {
      await render({ active: 'claude', tab: 'general', ...BOTH });

      expect(q('provider-pointer-problem')).toBeNull();
      expect(q('provider-unavailable')).toBeNull();
    });
  });
});


// Bedrock is a stub that only returns an error (#22) until #23 implements it.
describe('SettingsComponent — AWS Bedrock is not offered yet', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let component: SettingsComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  async function render(activeProvider: string, tab?: string): Promise<void> {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, active_provider: activeProvider });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute(tab) },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    component.settings = { ...defaultSettings, active_provider: activeProvider };
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  it('offers no Bedrock card among the providers', async () => {
    await render('openai', 'providers');

    // Positive first, so an unrendered tab cannot pass the absence check.
    expect(el.querySelector('[data-testid="provider-card-openai"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="provider-card-bedrock"]')).toBeNull();
    expect(el.querySelector('[data-testid="provider-use-bedrock"]')).toBeNull();
  });

  it('has no AWS key field on the AI Providers tab', async () => {
    await render('openai', 'providers');

    // Positive first, so an unrendered tab cannot pass the absence checks.
    expect(el.querySelector('[data-testid="key-status-openai"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="key-status-claude"]')).not.toBeNull();
    expect(el.textContent).not.toMatch(/AWS/);
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalledWith('bedrock');
  });

  it('says nothing about availability when the saved provider works', async () => {
    await render('openai');

    expect(el.querySelector('[data-testid="provider-unavailable"]')).toBeNull();
  });

  // A user who picked Bedrock before it was hidden. The dropdown cannot show
  // it any more, so the empty field needs a reason next to it.
  it('explains an empty provider field when Bedrock was saved', async () => {
    await render('bedrock');

    const note = el.querySelector('[data-testid="provider-unavailable"]');
    expect(note).not.toBeNull();
    expect(note!.textContent).toContain('AWS Bedrock is saved but not available yet');
    expect(el.querySelector('[data-testid="provider-pointer-text"]')!.textContent).toContain('No provider in use');
  });

  // No card can carry the marker, so the tab itself has to say why.
  it('says on the AI Providers tab that no offered provider is in use', async () => {
    await render('bedrock', 'providers');

    expect(el.querySelector('[data-testid="providers-tab-unavailable"]')?.textContent)
      .toContain('AWS Bedrock');
    expect(el.querySelector('[data-testid^="provider-in-use-"]')).toBeNull();
  });

  // Switching on the user's behalf would send their text to a service they
  // never chose, so the saved value stays until they pick one.
  it('does not switch a saved Bedrock provider behind the user\'s back', async () => {
    await render('bedrock');

    await component.save();

    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ active_provider: 'bedrock' }),
    );
  });

  it('drops the explanation once the user picks a provider', async () => {
    await render('bedrock', 'providers');

    el.querySelector<HTMLElement>('[data-testid="provider-use-openai"] button')!.click();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r));
    fixture.detectChanges();

    expect(el.querySelector('[data-testid="provider-unavailable"]')).toBeNull();
    expect(el.querySelector('[data-testid="providers-tab-unavailable"]')).toBeNull();
    expect(wailsMock.setActiveProvider).toHaveBeenCalledWith('openai');
  });
});


// Only the dark theme is styled (#24). A dropdown with one entry is not a
// choice, so the control is gone until a light theme exists (#25).
describe('SettingsComponent — theme', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let component: SettingsComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  async function render(themePreference: string): Promise<void> {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, theme_preference: themePreference });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute() },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    component.settings = { ...defaultSettings, theme_preference: themePreference };
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  it('shows no theme control on the General tab', async () => {
    await render('dark');

    const labels = Array.from(el.querySelectorAll('label')).map(l => l.textContent?.trim());
    expect(labels).toContain('Start on Boot');
    expect(labels).not.toContain('Theme');
  });

  // The field stays in the model for #25; saving must not rewrite it.
  it('saves a stored theme preference unchanged', async () => {
    await render('light');

    await component.save();

    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ theme_preference: 'light' }),
    );
  });
});
