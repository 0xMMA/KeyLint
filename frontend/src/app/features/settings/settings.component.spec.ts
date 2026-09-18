import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { SettingsComponent } from './settings.component';
import { WailsService, ClaudeCodeStatus } from '../../core/wails.service';
import { createWailsMock, defaultSettings, defaultKeyStatus, defaultUpdateInfo, defaultClaudeCodeStatus, defaultModelList } from '../../../testing/wails-mock';

function makeActivatedRoute(tab?: string): Partial<ActivatedRoute> {
  return {
    snapshot: {
      queryParamMap: convertToParamMap(tab ? { tab } : {}),
    } as ActivatedRoute['snapshot'],
  };
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
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('bedrock');
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

  async function render(source: 'live' | 'static' = 'live'): Promise<void> {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });
    wailsMock.listModels.mockResolvedValue({ ...defaultModelList, source });

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
    component.settings = { ...defaultSettings };
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
  });

  it('offers a fix and a Pyramidize model for every provider that has one', async () => {
    await render();

    for (const provider of ['openai', 'claude', 'claude-code', 'ollama']) {
      expect(el.querySelector(`[data-testid="model-fix-${provider}"]`), provider).not.toBeNull();
      expect(el.querySelector(`[data-testid="model-pyramidize-${provider}"]`), provider).not.toBeNull();
    }
    // Bedrock is still a stub and has nothing to choose.
    expect(el.querySelector('[data-testid="model-fix-bedrock"]')).toBeNull();
  });

  it('asks the backend for each provider list', async () => {
    await render();

    for (const provider of ['openai', 'claude', 'claude-code', 'ollama']) {
      expect(wailsMock.listModels).toHaveBeenCalledWith(provider);
    }
  });

  it('says when a list is the built-in one because the provider was unreachable', async () => {
    await render('static');

    expect(el.querySelector('[data-testid="models-static-openai"]')).not.toBeNull();
  });

  it('hides that hint when the list came from the provider', async () => {
    await render('live');

    expect(el.querySelector('[data-testid="models-static-openai"]')).toBeNull();
  });

  it('lets a model be typed for the API providers but not for the CLI', async () => {
    await render();

    // An editable PrimeNG select renders a text input; a closed one does not.
    expect(el.querySelector('[data-testid="model-fix-openai"] input')).not.toBeNull();
    // The CLI's three aliases are the whole list: a pinned API model ID there
    // freezes the generation the alias would follow.
    expect(el.querySelector('[data-testid="model-fix-claude-code"] input')).toBeNull();
    expect(component.allowsFreeText('claude-code')).toBe(false);
  });

  it('offers KeyLint\'s default as a selectable entry rather than a placeholder', async () => {
    await render();

    expect(component.optionsFor('openai')[0]).toEqual({ id: '', label: "KeyLint's default" });
  });

  it('never calls a provider list "built-in" when there is no endpoint to ask', async () => {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });
    wailsMock.listModels.mockResolvedValue({ ...defaultModelList, source: 'fixed' });

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
    component.settings = { ...defaultSettings };
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.querySelector('[data-testid="models-static-claude-code"]')).toBeNull();
  });

  it('uses the same provider labels as the Active Provider list', async () => {
    await render();

    for (const mp of component.modelProviders) {
      const active = component.providers.find(p => p.value === mp.id);
      expect(mp.label, mp.id).toBe(active!.label);
    }
  });

  it('keeps a model the provider does not list, so an unlisted one can be typed', async () => {
    await render();

    component.setModel('openai', 'fix', 'gpt-not-in-any-list');

    expect(component.modelFor('openai', 'fix')).toBe('gpt-not-in-any-list');
    expect(component.settings!.models!['openai']?.fix).toBe('gpt-not-in-any-list');
  });

  it('treats an empty field as "use the default" rather than as a model named ""', async () => {
    await render();

    component.setModel('claude', 'pyramidize', '   ');

    expect(component.modelFor('claude', 'pyramidize')).toBe('');
  });

  it('keeps the two features apart', async () => {
    await render();

    component.setModel('claude', 'fix', 'claude-haiku-4-5-20251001');
    component.setModel('claude', 'pyramidize', 'claude-opus-4-6');

    expect(component.modelFor('claude', 'fix')).toBe('claude-haiku-4-5-20251001');
    expect(component.modelFor('claude', 'pyramidize')).toBe('claude-opus-4-6');
  });
});
