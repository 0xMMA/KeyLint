import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { ChangeDetectorRef } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent, resetPyramidizeProviderSession } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockResolvedValue({
      documentType: 'EMAIL',
      language: 'en',
      fullDocument: 'Subject | Details | Actions\n\nBody text',
    }),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: 'Refined canvas text' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: 'Rewritten section' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue('Enhanced text.'),
  };
}

// The provider choice is module-level, outlives every fixture and is shared
// with other spec files (isolate: false): every test starts a fresh session.
beforeEach(() => resetPyramidizeProviderSession());
afterEach(() => resetPyramidizeProviderSession());

describe('TextEnhancementComponent (Pyramidize)', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    svcMock = makeEnhancementServiceMock();

    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;

    // Reset module-level state
    component.originalTextView = '';
    component.canvasTextView = '';
    component.docTypeView = 'email';
    component.isLoading = false;
    component.errorMessage = '';

    fixture.detectChanges();
    await fixture.whenStable();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    // Clean up module-level state
    component.originalTextView = '';
    component.canvasTextView = '';
  });

  // ── 1. Creates successfully ──

  it('creates successfully', () => {
    expect(component).toBeTruthy();
  });

  // ── 2. Original textarea renders with correct testid ──

  it('renders original textarea with data-testid="original-textarea"', () => {
    expect(el.querySelector('[data-testid="original-textarea"]')).toBeTruthy();
  });

  // ── 3. Pyramidize button disabled when originalText is empty ──

  it('Pyramidize button is disabled when originalText is empty', () => {
    component.originalTextView = '';
    fixture.detectChanges();
    const btn = el.querySelector<HTMLButtonElement>('[data-testid="pyramidize-btn"] button');
    expect(btn?.disabled).toBe(true);
  });

  // ── 4. Pyramidize button enabled when originalText has content ──

  it('Pyramidize button is enabled when originalText has content', async () => {
    component.originalTextView = 'Some text to pyramidize';
    fixture.detectChanges();
    await fixture.whenStable();
    const btn = el.querySelector<HTMLButtonElement>('[data-testid="pyramidize-btn"] button');
    expect(btn?.disabled).toBe(false);
  });

  // ── 5. Paste from clipboard button reads clipboard and sets originalText ──

  it('clicking paste from clipboard button reads clipboard and sets originalText', async () => {
    wailsMock.readClipboard.mockResolvedValue('pasted content from clipboard');
    component.originalTextView = ''; // ensure empty so button renders
    fixture.detectChanges();
    await fixture.whenStable();

    await component.pasteFromClipboard();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(component.originalTextView).toBe('pasted content from clipboard');
  });

  // ── 6. pyramidize() calls service with correct request ──

  it('pyramidize() calls service.pyramidize with correct request', async () => {
    component.originalTextView = 'Hello world';
    component.docTypeView = 'email';
    component.commStyleView = 'professional';
    component.relLevelView = 'professional';
    component.customInstructions = '';

    await component.pyramidize();

    expect(svcMock.pyramidize).toHaveBeenCalledWith(expect.objectContaining({
      text: 'Hello world',
      documentType: 'email',
      communicationStyle: 'professional',
      relationshipLevel: 'professional',
    }));
  });

  // ── 7. After pyramidize(), canvas tab has content and trace log has entries ──

  it('after pyramidize(), canvas has content and trace log has entries', async () => {
    component.originalTextView = 'Some text';

    await component.pyramidize();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(component.canvasTextView).toBe('Subject | Details | Actions\n\nBody text');
    expect(component.traceLogView.length).toBeGreaterThan(0);
    expect(component.traceLogView.some(e => e.label === 'Pyramidized')).toBe(true);
  });

  // ── 8. cancelOperation() calls service.cancelOperation() ──

  it('cancelOperation() calls service.cancelOperation', async () => {
    await component.cancelOperation();
    expect(svcMock.cancelOperation).toHaveBeenCalled();
    expect(component.isLoading).toBe(false);
  });

  // ── 9. applyGlobalInstruction() calls refineGlobal when there's canvas content and an instruction ──

  it('applyGlobalInstruction() calls service.refineGlobal with canvas and instruction', async () => {
    component.canvasTextView = 'Existing canvas content';
    component.originalTextView = 'original';
    component.globalInstruction = 'Make it shorter';

    await component.applyGlobalInstruction();

    expect(svcMock.refineGlobal).toHaveBeenCalledWith(expect.objectContaining({
      fullCanvas: 'Existing canvas content',
      instruction: 'Make it shorter',
    }));
    expect(component.canvasTextView).toBe('Refined canvas text');
  });

  it('applyGlobalInstruction() does nothing when instruction is empty', async () => {
    component.canvasTextView = 'Some canvas';
    component.globalInstruction = '';

    await component.applyGlobalInstruction();
    expect(svcMock.refineGlobal).not.toHaveBeenCalled();
  });

  it('applyGlobalInstruction() does nothing when canvas is empty', async () => {
    component.canvasTextView = '';
    component.globalInstruction = 'Make it shorter';

    await component.applyGlobalInstruction();
    expect(svcMock.refineGlobal).not.toHaveBeenCalled();
  });

  // ── 10. addCheckpoint() adds a trace entry ──

  it('addCheckpoint() adds a trace entry when canvas has content', () => {
    component.canvasTextView = 'Some canvas text';
    const before = component.traceLogView.length;

    component.addCheckpoint();

    expect(component.traceLogView.length).toBe(before + 1);
    expect(component.traceLogView[component.traceLogView.length - 1].label).toBe('Checkpoint');
  });

  it('addCheckpoint() does nothing when canvas is empty', () => {
    component.canvasTextView = '';
    const before = component.traceLogView.length;
    component.addCheckpoint();
    expect(component.traceLogView.length).toBe(before);
  });

  // ── 11. revertTo() restores canvas text and adds trace entry ──

  it('revertTo() restores canvas text and adds a trace entry', () => {
    component.canvasTextView = 'Current canvas';
    const snapshot = 'Old snapshot text';
    const entry = { id: 'test-id', label: 'Old version', snapshot, timestamp: new Date() };

    const before = component.traceLogView.length;
    component.revertTo(entry);

    expect(component.canvasTextView).toBe(snapshot);
    expect(component.traceLogView.length).toBe(before + 1);
    expect(component.traceLogView[component.traceLogView.length - 1].label).toContain('Reverted to');
  });

  // ── 12. copyAsMarkdown() writes via WailsService.writeClipboard ──

  it('copyAsMarkdown() writes canvasText via WailsService.writeClipboard', async () => {
    component.canvasTextView = '# Hello\n\nWorld';
    await component.copyAsMarkdown();

    expect(wailsMock.writeClipboard).toHaveBeenCalledWith('# Hello\n\nWorld');
  });

  // ── 13. shortcutPyramidize$ with empty originalText sets originalText from clipboard ──

  it('shortcutPyramidize$ with empty originalText sets originalText from clipboard', async () => {
    component.originalTextView = '';
    wailsMock.readClipboard.mockResolvedValue('clipboard hotkey content');
    wailsMock.getSourceApp.mockResolvedValue('TestApp');

    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(component.originalTextView).toBe('clipboard hotkey content');
  });

  // ── 14. shortcutPyramidize$ with existing originalText shows confirm dialog ──

  it('shortcutPyramidize$ with existing originalText shows confirm dialog', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);

    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(confirmSpy).toHaveBeenCalled();
    // Since user cancelled, originalText should remain unchanged
    expect(component.originalTextView).toBe('existing content');
  });

  it('shortcutPyramidize$ with existing originalText and confirm=true replaces content', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(component.originalTextView).toBe('new clipboard content');
  });

  // ── Additional tests ──

  it('renders API key banner when API key is not set', async () => {
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });
    // Force banner visible
    (component as unknown as { bannerDismissedView: boolean });
    component.apiKeySet = false;
    fixture.detectChanges();
    await fixture.whenStable();
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeTruthy();
  });

  it('opens the history from the toggle in the tab bar and closes it again', async () => {
    expect(el.querySelector('[data-testid="trace-log-panel"]')).toBeNull();
    const toggle = el.querySelector<HTMLButtonElement>('[data-testid="trace-log-toggle"] button')!;
    expect(toggle.getAttribute('aria-pressed')).toBe('false');

    toggle.click();
    await fixture.whenStable();
    expect(el.querySelector('[data-testid="trace-log-panel"]')).toBeTruthy();
    expect(toggle.getAttribute('aria-pressed')).toBe('true');

    el.querySelector<HTMLButtonElement>('[data-testid="trace-log-close"] button')!.click();
    await fixture.whenStable();
    expect(el.querySelector('[data-testid="trace-log-panel"]')).toBeNull();
  });

  it('pyramidize() does nothing when originalText is empty', async () => {
    component.originalTextView = '';
    await component.pyramidize();
    expect(svcMock.pyramidize).not.toHaveBeenCalled();
  });

  it('pyramidize() sets errorMessage on service failure', async () => {
    component.originalTextView = 'Some text';
    svcMock.pyramidize.mockRejectedValue(new Error('API failure'));

    await component.pyramidize();

    expect(component.errorMessage).toContain('API failure');
    expect(component.isLoading).toBe(false);
  });

  it('sendBack() sends the canvas and shows no error when it lands', async () => {
    component.canvasTextView = 'Final text';
    await component.sendBack();

    expect(svcMock.sendBack).toHaveBeenCalledWith('Final text');
    expect(el.querySelector('[data-testid="error-row"]')).toBeNull();
  });

  it('sendBack() shows why the text was not pasted, in the error row', async () => {
    component.canvasTextView = 'Final text';
    // What a failed Go binding call rejects with under Wails v3.
    svcMock.sendBack.mockRejectedValue(new Error(
      '{"message":"The original window is closed — the text is on the clipboard.","kind":"RuntimeError"}',
    ));

    await component.sendBack();
    fixture.detectChanges();
    await fixture.whenStable();

    const row = el.querySelector('[data-testid="error-row"]');
    expect(row?.textContent).toContain('The original window is closed — the text is on the clipboard.');
    expect(row?.textContent).not.toContain('RuntimeError');
    // Copying the error would overwrite the text it says is on the clipboard.
    expect(el.querySelector('[data-testid="copy-error-btn"]')).toBeNull();
  });

  it('keeps Copy error for other errors', async () => {
    component.originalTextView = 'Some text';
    svcMock.pyramidize.mockRejectedValue(new Error('API failure'));
    await component.pyramidize();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(el.querySelector('[data-testid="copy-error-btn"]')).toBeTruthy();
  });

  it('disables Send back while a send back is running', async () => {
    // The source app is read on init, so start a page that knows one.
    wailsMock.getSourceApp.mockResolvedValue('Untitled - Notepad');
    component.canvasTextView = 'Final text';
    const page = TestBed.createComponent(TextEnhancementComponent);
    page.detectChanges();
    await page.whenStable();
    page.detectChanges();
    const button = () =>
      (page.nativeElement as HTMLElement).querySelector<HTMLButtonElement>('[data-testid="send-back-btn"] button');
    expect(button()?.disabled).toBe(false);

    let finish!: () => void;
    svcMock.sendBack.mockReturnValue(new Promise<void>(r => (finish = r)));
    const running = page.componentInstance.sendBack();
    expect(button()?.disabled).toBe(true);

    finish();
    await running;
    expect(button()?.disabled).toBe(false);
    page.destroy();
  });

  it('sendBack() ignores a second click while the first is still running', async () => {
    component.canvasTextView = 'Final text';
    let finish!: () => void;
    svcMock.sendBack.mockReturnValue(new Promise<void>(r => (finish = r)));

    const first = component.sendBack();
    await component.sendBack();
    finish();
    await first;

    expect(svcMock.sendBack).toHaveBeenCalledTimes(1);
  });

  it('ngOnDestroy unsubscribes from shortcut events', async () => {
    component.ngOnDestroy();
    const prevReadCount = (wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length;
    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect((wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length).toBe(prevReadCount);
  });
});

// The shell navigates here on a Pyramidize shortcut, but the event has already
// gone by the time this page subscribes. WailsService keeps it pending until a
// page takes it, so the clipboard text still arrives.
describe('TextEnhancementComponent — Pyramidize shortcut from another page', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let wailsMock: ReturnType<typeof createWailsMock>;

  async function mount(): Promise<void> {
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(TextEnhancementComponent);
    fixture.componentInstance.originalTextView = '';
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r, 0));
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.readClipboard.mockResolvedValue('text copied before the page opened');
  });

  afterEach(() => {
    fixture?.componentInstance.ngOnDestroy();
    if (fixture) fixture.componentInstance.originalTextView = '';
    vi.restoreAllMocks();
  });

  it('loads the clipboard when a shortcut is pending on arrival', async () => {
    wailsMock.takePendingPyramidize.mockReturnValueOnce(true);

    await mount();

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(fixture.componentInstance.originalTextView).toBe('text copied before the page opened');
  });

  it('leaves the page alone when nothing is pending', async () => {
    await mount();

    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
    expect(fixture.componentInstance.originalTextView).toBe('');
  });

  it('takes the pending flag when the shortcut arrives while mounted', async () => {
    await mount();
    wailsMock.takePendingPyramidize.mockClear();

    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    // Otherwise the next visit to this page would load the clipboard again.
    expect(wailsMock.takePendingPyramidize).toHaveBeenCalled();
    expect(fixture.componentInstance.originalTextView).toBe('text copied before the page opened');
  });
});

describe('TextEnhancementComponent — Claude Code provider', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });

    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  afterEach(async () => {
    // Provider choice lives in module-level state that outlives the fixture.
    component.providerView = 'claude';
    await component.onProviderChange();
  });

  it('offers the CLI as a provider', () => {
    expect(component.providerOptions.map(p => p.value)).toContain('claude-code');
  });

  it('reads the model list from the backend rather than a hardcoded table', async () => {
    wailsMock.listModels.mockResolvedValue({
      models: [{ id: 'opus', label: 'Opus' }, { id: 'sonnet', label: 'Sonnet' }],
      source: 'live',
    });

    component.providerView = 'claude-code';
    await component.onProviderChange();
    fixture.detectChanges();

    expect(wailsMock.listModels).toHaveBeenCalledWith('claude-code');
    // A leading entry lets the user say "whatever Settings says" without
    // knowing a model name; the rest come from the provider.
    expect(component.currentModelOptions.map(m => m.id)).toEqual(['', 'opus', 'sonnet']);
    // Switching provider must not carry a model from the previous one.
    expect(component.modelView).toBe('');
  });

  /** Switches provider and returns whatever note the panel then shows. */
  async function noteFor(provider: string, list: { models: Array<{ id: string; label: string }>; source: string }): Promise<string> {
    wailsMock.listModels.mockResolvedValue(list);

    component.providerView = provider;
    await component.onProviderChange();
    fixture.detectChanges();

    return el.querySelector('[data-testid="model-list-note"]')?.textContent?.trim() ?? '';
  }

  it('tells the user when the list is the built-in one because the provider was unreachable', async () => {
    const note = await noteFor('ollama', { models: [{ id: 'llama3.2', label: 'llama3.2' }], source: 'unreachable' });

    expect(note).toContain('could not be reached');
  });

  it('tells a user without a key to add one instead of blaming the network', async () => {
    const note = await noteFor('openai', { models: [{ id: 'gpt-4.1', label: 'gpt-4.1' }], source: 'no-credentials' });

    expect(note).toContain('add a key');
    expect(note).not.toContain('could not be reached');
  });

  it('tells an Ollama user with nothing pulled what is actually wrong', async () => {
    // A running daemon with no models is not an unreachable one, and saying so
    // would send the user after a problem that is not there.
    const note = await noteFor('ollama', { models: [], source: 'empty' });

    expect(note).toContain('No models pulled yet');
    expect(note).not.toContain('could not be reached');
  });

  it('stays quiet when the provider answered', async () => {
    const note = await noteFor('ollama', { models: [{ id: 'llama3.2', label: 'llama3.2' }], source: 'live' });

    expect(note).toBe('');
  });

  it('stays quiet for a provider that has no endpoint to ask', async () => {
    // The CLI's three aliases are the whole list by design, so "built-in" would
    // be an alarm nobody can clear.
    const note = await noteFor('claude-code', { models: [{ id: 'sonnet', label: 'Sonnet' }], source: 'fixed' });

    expect(note).toBe('');
  });

  it('warns when the CLI is installed but signed out, instead of failing at call time', async () => {
    wailsMock.getClaudeCodeStatus.mockResolvedValue({
      installed: true, loggedIn: false, path: '/usr/local/bin/claude', version: '2.1.274',
    });

    component.providerView = 'claude-code';
    await component.onProviderChange();
    fixture.detectChanges();

    expect(component.apiKeySet).toBe(false);
    expect(el.querySelector('[data-testid="api-key-banner"]')).not.toBeNull();
    // "No AI API key configured" is the one thing that is never wrong here.
    const message = el.querySelector('[data-testid="api-key-banner-message"]')!.textContent!;
    expect(message).toContain('not signed in');
    expect(message).toContain('sign in');
    expect(message).not.toContain('API key');
  });

  it('warns when the CLI is not installed at all', async () => {
    wailsMock.getClaudeCodeStatus.mockResolvedValue({
      installed: false, loggedIn: false, path: '', version: '',
    });

    component.providerView = 'claude-code';
    await component.onProviderChange();
    fixture.detectChanges();

    // Without this the test also passes when only half the branch is reverted.
    expect(component.apiKeySet).toBe(false);
    expect(el.querySelector('[data-testid="api-key-banner"]')).not.toBeNull();
    const message = el.querySelector('[data-testid="api-key-banner-message"]')!.textContent!;
    expect(message).toContain('not found on this machine');
    expect(message).not.toContain('API key');
  });

  it('keeps saying "API key" for the providers that actually need one', async () => {
    component.providerView = 'openai';
    await component.onProviderChange();
    fixture.detectChanges();

    expect(el.querySelector('[data-testid="api-key-banner-message"]')!.textContent)
      .toContain('No AI API key configured');
  });

  it('ignores a stale probe when the user switches provider again', async () => {
    // claude-code costs two process spawns, so a quick second switch can
    // finish first; the slower answer must not overwrite it.
    let resolveSlow: (v: unknown) => void = () => {};
    wailsMock.getClaudeCodeStatus.mockReturnValueOnce(new Promise(resolve => { resolveSlow = resolve; }));

    component.providerView = 'claude-code';
    const slowSwitch = component.onProviderChange();

    component.providerView = 'openai';
    await component.onProviderChange();

    resolveSlow({ installed: false, loggedIn: false, path: '', version: '' });
    await slowSwitch;
    fixture.detectChanges();

    expect(el.querySelector('[data-testid="api-key-banner-message"]')!.textContent)
      .toContain('No AI API key configured');
  });

  it('stays quiet when the CLI is installed and signed in', async () => {
    wailsMock.getClaudeCodeStatus.mockResolvedValue({
      installed: true, loggedIn: true, path: '/usr/local/bin/claude', version: '2.1.274',
    });

    component.providerView = 'claude-code';
    await component.onProviderChange();
    fixture.detectChanges();

    expect(component.apiKeySet).toBe(true);
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeNull();
    // The keyring has no answer for this provider and must not be asked.
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalledWith('claude-code');
  });

  it('shows no missing-key warning for Ollama, which needs no credential at all', async () => {
    expect(el.querySelector('[data-testid="api-key-banner"]')).not.toBeNull();

    component.providerView = 'ollama';
    await component.onProviderChange();
    fixture.detectChanges();

    expect(component.apiKeySet).toBe(true);
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeNull();
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalledWith('ollama');
  });
});

// Settings › AI Providers marks one provider as in use. This panel follows it
// on every visit unless the user picked another here this session, and then
// says so, so the two pages never disagree silently.
describe('TextEnhancementComponent — provider follows Settings', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  /** One visit to the page: a fresh component, as navigation creates. */
  async function visit(settingsProvider: string): Promise<void> {
    fixture?.destroy();
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, active_provider: settingsProvider });
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    // ngOnInit awaits several backend calls the fixture does not track.
    await new Promise(r => setTimeout(r));
    fixture.detectChanges();
  }

  async function pick(provider: string): Promise<void> {
    component.providerView = provider;
    await component.onProviderChange();
    fixture.detectChanges();
  }

  function selectText(): string {
    return el.querySelector('[data-testid="provider-select"]')?.textContent?.trim() ?? '';
  }

  function override(): HTMLElement | null {
    return el.querySelector('[data-testid="provider-session-override"]');
  }

  beforeEach(() => resetPyramidizeProviderSession());
  afterEach(() => {
    fixture?.destroy();
    resetPyramidizeProviderSession();
  });

  it('follows a switch made in Settings on the next visit', async () => {
    await visit('claude');
    expect(selectText()).toContain('Anthropic API');

    await visit('ollama');

    expect(selectText()).toContain('Ollama (local)');
    expect(override()).toBeNull();
  });

  it('keeps a provider picked here for the session, and says Settings uses another', async () => {
    await visit('claude');
    await pick('openai');

    expect(override()!.textContent).toContain('Settings uses Anthropic API');

    await visit('claude');
    expect(selectText()).toContain('OpenAI');
    expect(override()!.textContent).toContain('Settings uses Anthropic API');
  });

  it('names Settings\' new provider when it changes under a session pick', async () => {
    await visit('claude');
    await pick('openai');

    await visit('claude-code');

    expect(selectText()).toContain('OpenAI');
    expect(override()!.textContent).toContain('Settings uses Claude Code (installed CLI)');
  });

  it('goes back to Settings\' provider from the override note, and follows it again', async () => {
    await visit('claude');
    await pick('openai');

    el.querySelector<HTMLElement>('[data-testid="provider-follow-settings"]')!.click();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(selectText()).toContain('Anthropic API');
    expect(override()).toBeNull();
    await visit('ollama');
    expect(selectText()).toContain('Ollama (local)');
  });

  it('treats picking Settings\' own provider as no override', async () => {
    await visit('claude');
    await pick('openai');
    await pick('claude');

    expect(override()).toBeNull();
    await visit('ollama');
    expect(selectText()).toContain('Ollama (local)');
  });

  it('ends the override once Settings catches up, and follows Settings again', async () => {
    await visit('claude');
    await pick('openai');

    await visit('openai');
    expect(override()).toBeNull();

    await visit('ollama');
    expect(selectText()).toContain('Ollama (local)');
  });

  // Settings names a provider that cannot run, so the user had to pick one.
  // That is no preference to protect, and a link back would be a dead end.
  it('treats a pick forced by an unavailable Settings provider as no override', async () => {
    await visit('bedrock');
    await pick('openai');

    expect(override()).toBeNull();
    expect(el.querySelector('[data-testid="provider-follow-settings"]')).toBeNull();
    expect(el.querySelector('[data-testid="provider-forced-note"]')!.textContent).toContain('AWS Bedrock');

    await visit('bedrock');
    expect(selectText()).toContain('OpenAI');

    await visit('claude');
    expect(selectText()).toContain('Anthropic API');
    expect(el.querySelector('[data-testid="provider-forced-note"]')).toBeNull();
  });

  it('starts from Settings\' default model again after following a switch', async () => {
    await visit('claude');
    component.modelView = 'claude-haiku-4-5-20251001';

    await visit('openai');

    expect(component.modelView).toBe('');
  });
});

// Pyramidize is the page's main action. Apply only does something once there
// is an instruction, so until then it must not wear the same accent (#29).
describe('TextEnhancementComponent — Apply button emphasis', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: createWailsMock() },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    component.originalTextView = '';
    component.canvasTextView = 'Some canvas text';
    component.isLoading = false;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  afterEach(() => {
    component.originalTextView = '';
    component.canvasTextView = '';
  });

  function applyButton(): HTMLButtonElement {
    return el.querySelector<HTMLButtonElement>('[data-testid="apply-instruction-btn"] button')!;
  }

  async function typeInstruction(text: string): Promise<void> {
    const input = el.querySelector<HTMLInputElement>('[data-testid="global-instruction-input"]')!;
    input.value = text;
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  it('is quiet while the instruction is empty', () => {
    expect(applyButton().classList).toContain('quiet');
    expect(applyButton().classList).not.toContain('primary');
  });

  it('becomes the primary action once an instruction is typed', async () => {
    await typeInstruction('make it shorter');

    expect(applyButton().disabled).toBe(false);
    expect(applyButton().classList).toContain('primary');
  });

  it('recedes again when the instruction is only whitespace', async () => {
    await typeInstruction('make it shorter');
    await typeInstruction('   ');

    expect(applyButton().classList).toContain('quiet');
  });
});

// The page in the E11 look: what the user sees and can reach by keyboard. The
// behaviour underneath is covered above through the component's methods.
describe('TextEnhancementComponent — page', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  beforeEach(async () => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    svcMock = makeEnhancementServiceMock();
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    component.originalTextView = '';
    component.canvasTextView = '';
    component.docTypeView = 'email';
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r));
    await refresh();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    component.originalTextView = '';
    component.canvasTextView = '';
    component.docTypeView = 'email';
    if (component.traceLogOpenView) component.toggleTraceLog();
    component.onTabChange('original');
    component.setPreviewMode(false);
    fixture.destroy();
    // kl-select renders its options into the CDK container on <body>.
    document.querySelector('.cdk-overlay-container')?.replaceChildren();
  });

  /** Zoneless: a field set from here does not mark the view dirty by itself. */
  async function refresh(): Promise<void> {
    fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck();
    fixture.detectChanges();
    await fixture.whenStable();
  }

  function q<T extends HTMLElement = HTMLElement>(testid: string): T | null {
    return el.querySelector<T>(`[data-testid="${testid}"]`);
  }

  function button(testid: string): HTMLButtonElement {
    return q(testid)!.querySelector('button')!;
  }

  function ctrlEnter(target: HTMLElement): void {
    target.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true }));
  }

  // ADR-003: the user picks the form; nothing detects it out of sight.
  it('offers the five forms, Chat among them, and no automatic detection', async () => {
    expect(q('doc-type-select')!.textContent).toContain('Email');

    q('doc-type-select')!.querySelector<HTMLButtonElement>('button')!.click();
    await refresh();
    const labels = Array.from(document.querySelectorAll<HTMLElement>('.cdk-overlay-container [role="option"]'))
      .map(o => o.textContent!.trim());
    expect(labels).toEqual(['Email', 'Chat', 'Memo', 'Wiki', 'PowerPoint']);
  });

  it('sends the picked form, never "auto"', async () => {
    component.originalTextView = 'Hi Sarah, two things';
    await component.pyramidize();

    expect(svcMock.pyramidize).toHaveBeenCalledWith(expect.objectContaining({ documentType: 'email' }));
  });

  it('preselects the app preset for text the shortcut brought in', async () => {
    wailsMock.readClipboard.mockResolvedValue('quick note for the channel');
    wailsMock.getDefaultDocType.mockResolvedValue('chat');

    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    await refresh();

    expect(q('doc-type-select')!.textContent).toContain('Chat');
    expect(component.docTypeView).toBe('chat');
  });

  it('keeps the user\'s pick when only navigating back to the page', async () => {
    component.docTypeView = 'wiki';
    wailsMock.getDefaultDocType.mockResolvedValue('chat');
    await component.ngOnInit();
    await refresh();

    expect(component.docTypeView).toBe('wiki');
  });

  it('labels every option with a visible label', () => {
    for (const id of ['pz-provider', 'pz-model', 'pz-doc-type', 'pz-style', 'pz-relationship', 'pz-instructions']) {
      expect(el.querySelector(`label[for="${id}"]`), id).not.toBeNull();
      expect(el.querySelector(`#${id}`), id).not.toBeNull();
    }
  });

  it('offers to pyramidize again once the editor holds a result', async () => {
    component.originalTextView = 'Some text';
    await refresh();
    expect(button('pyramidize-btn').textContent).toContain('Pyramidize');
    expect(button('pyramidize-btn').textContent).not.toContain('again');

    component.canvasTextView = 'A result';
    await refresh();
    expect(button('pyramidize-btn').textContent).toContain('Pyramidize again');
  });

  it('pyramidizes on Ctrl+Enter in the instruction field, as the button hint says', async () => {
    component.originalTextView = 'Some text';
    await refresh();

    ctrlEnter(q('custom-instructions')!);
    await fixture.whenStable();

    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
  });

  it('ignores Ctrl+Enter in the instruction field while there is nothing to pyramidize', async () => {
    ctrlEnter(q('custom-instructions')!);
    await fixture.whenStable();

    expect(svcMock.pyramidize).not.toHaveBeenCalled();
  });

  it('pyramidizes on Ctrl+Enter in the original text', async () => {
    component.originalTextView = 'Some text';
    await refresh();

    ctrlEnter(q('original-textarea')!);
    await fixture.whenStable();

    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
  });

  it('shows the step over the editor while it works, and keeps the text out of reach', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValue(new Promise(r => (finish = r)));
    component.originalTextView = 'Some text';
    await refresh();

    const running = component.pyramidize();
    await refresh();

    // One call, so one step: no "Step 1/2" and no "Detecting" any more.
    expect(q('step-label')!.textContent!.trim()).toBe('Structuring…');
    expect(q('cancel-btn')).not.toBeNull();
    expect(el.querySelector('kl-tabs')!.hasAttribute('inert')).toBe(true);
    expect(button('pyramidize-btn').getAttribute('aria-busy')).toBe('true');

    finish({ fullDocument: 'Done', documentType: 'EMAIL', language: 'en' });
    await running;
    await refresh();
    expect(q('step-indicator')).toBeNull();
    expect(el.querySelector('kl-tabs')!.hasAttribute('inert')).toBe(false);
    expect(el.querySelector('kl-tab[data-value="canvas"]')!.getAttribute('aria-selected')).toBe('true');
  });

  it('switches the editor between editing and a formatted preview', async () => {
    component.canvasTextView = '# Title\n\nBody';
    component.onTabChange('canvas');
    await refresh();
    expect(q('canvas-textarea')).not.toBeNull();

    q<HTMLButtonElement>('segment-preview')!.click();
    await refresh();

    expect(q('canvas-textarea')).toBeNull();
    expect(q('canvas-preview')!.querySelector('h1')!.textContent).toBe('Title');
    q<HTMLButtonElement>('segment-edit')!.click();
    await refresh();
    expect(q('canvas-textarea')).not.toBeNull();
  });

  it('counts the history steps on its toggle', async () => {
    const before = component.traceLogView.length;
    component.canvasTextView = 'Text';
    component.addCheckpoint();
    await refresh();
    const n = before + 1;
    expect(button('trace-log-toggle').textContent!.trim()).toBe(`History, ${n} ${n === 1 ? 'step' : 'steps'}`);
  });

  it('previews a history entry on hover, pins it on click, and reverts from the pinned preview', async () => {
    component.canvasTextView = 'First version';
    component.addCheckpoint();
    component.canvasTextView = 'Second version';
    button('trace-log-toggle').click();
    await refresh();

    const entries = el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]');
    const last = entries[entries.length - 1];
    expect(last.textContent).toContain('Checkpoint');

    last.dispatchEvent(new MouseEvent('mouseenter'));
    await refresh();
    expect(q('trace-peek-content')!.textContent).toBe('First version');
    expect(q('trace-peek-close')).toBeNull();

    last.click();
    await refresh();
    expect(last.getAttribute('aria-current')).toBe('true');
    expect(q('trace-peek-close')).not.toBeNull();

    button('trace-revert-btn').click();
    await refresh();
    expect(component.canvasTextView).toBe('First version');
    expect(q('trace-peek-overlay')).toBeNull();
  });

  it('closes a pinned preview on Escape', async () => {
    component.canvasTextView = 'A version';
    component.addCheckpoint();
    button('trace-log-toggle').click();
    await refresh();
    const entries = el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]');
    entries[entries.length - 1].click();
    await refresh();
    expect(q('trace-peek-overlay')).not.toBeNull();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await refresh();

    expect(q('trace-peek-overlay')).toBeNull();
  });

  it('adds a checkpoint from the history column', async () => {
    component.canvasTextView = 'Worth keeping';
    button('trace-log-toggle').click();
    await refresh();
    const before = component.traceLogView.length;

    button('add-checkpoint-btn').click();
    await refresh();

    expect(component.traceLogView.length).toBe(before + 1);
  });

  it('keeps the chosen model when the provider in use is picked again', async () => {
    component.modelView = 'claude-opus-4';
    await refresh();
    const before = wailsMock.listModels.mock.calls.length;

    q('provider-select')!.querySelector<HTMLButtonElement>('button')!.click();
    await refresh();
    const current = component.providerOptions.find(o => o.value === component.providerView)!;
    const option = Array.from(document.querySelectorAll<HTMLElement>('.cdk-overlay-container [role="option"]'))
      .find(o => o.textContent!.trim() === current.label)!;
    option.click();
    await refresh();

    expect(component.modelView).toBe('claude-opus-4');
    expect(wailsMock.listModels.mock.calls.length).toBe(before);
  });

  it('shows the error again when a retried Pyramidize fails too', async () => {
    component.originalTextView = 'Some text';
    svcMock.pyramidize.mockRejectedValue(new Error('API failure'));
    await component.pyramidize();
    await component.retry();
    await refresh();

    expect(q('error-row')!.textContent).toContain('Pyramidize failed: API failure');
  });

  it('keeps the history out of reach while a step runs', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValue(new Promise(r => (finish = r)));
    component.originalTextView = 'Some text';
    component.canvasTextView = '';
    button('trace-log-toggle').click();
    await refresh();

    const running = component.pyramidize();
    await refresh();
    expect(q('trace-log-panel')!.hasAttribute('inert')).toBe(true);

    finish({ fullDocument: 'Done', documentType: 'EMAIL', language: 'en' });
    await running;
    await refresh();
    expect(q('trace-log-panel')!.hasAttribute('inert')).toBe(false);
  });

  it('gives the focus back to the history entry when a pinned preview closes', async () => {
    component.canvasTextView = 'A version';
    component.addCheckpoint();
    button('trace-log-toggle').click();
    await refresh();
    const entries = el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]');
    const last = entries[entries.length - 1];
    last.click();
    await refresh();

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await refresh();

    expect(document.activeElement).toBe(last);
  });

  it('opens the selection bubble for text selected in the editor, and not for a plain click', async () => {
    component.canvasTextView = 'Some text to change';
    component.onTabChange('canvas');
    await refresh();
    const textarea = q<HTMLTextAreaElement>('canvas-textarea')!;
    expect(textarea.value).toBe('Some text to change');

    // Only the textarea knows: Chromium keeps the document selection collapsed.
    textarea.setSelectionRange(5, 9);
    textarea.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
    await refresh();
    expect(q('selection-bubble')).not.toBeNull();

    textarea.setSelectionRange(3, 3);
    textarea.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
    await refresh();
    expect(q('selection-bubble')).toBeNull();
  });

  it('gives the selection bubble an Apply that waits for an instruction, and closes it on Escape', async () => {
    component.canvasTextView = 'Some text to change';
    component.onTabChange('canvas');
    component.showSelectionBubble = true;
    await refresh();

    expect(button('selection-apply-btn').disabled).toBe(true);
    const input = q<HTMLInputElement>('selection-instruction-input')!;
    input.value = 'more formal';
    input.dispatchEvent(new Event('input'));
    await refresh();
    expect(component.selectionInstruction).toBe('more formal');
    expect(button('selection-apply-btn').disabled).toBe(false);

    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await refresh();
    expect(q('selection-bubble')).toBeNull();
  });

  it('shows a failure as an error message with Retry', async () => {
    component.originalTextView = 'Some text';
    svcMock.pyramidize.mockRejectedValueOnce(new Error('API failure'));
    await component.pyramidize();
    await refresh();

    const row = q('error-row')!;
    expect(row.getAttribute('role')).toBe('alert');
    expect(row.textContent).toContain('Pyramidize failed: API failure');

    button('retry-btn').click();
    await fixture.whenStable();
    await refresh();
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(2);
  });
});

// The Effort picker under Model: Settings' levels, a session override like the
// model (never written to settings), sent with every call the page makes.
describe('TextEnhancementComponent — effort', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  async function open(settings: Partial<typeof defaultSettings> = {}): Promise<void> {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, ...settings });
    svcMock = makeEnhancementServiceMock();
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();
    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    component.originalTextView = '';
    component.canvasTextView = '';
    fixture.detectChanges();
    await fixture.whenStable();
    await refresh();
  }

  async function refresh(): Promise<void> {
    fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck();
    fixture.detectChanges();
    await fixture.whenStable();
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);
  const overlayOptions = () => Array.from(document.querySelectorAll<HTMLElement>('.cdk-overlay-container [role="option"]'));

  async function pickEffort(label: string): Promise<void> {
    q('effort-select')!.querySelector<HTMLButtonElement>('button')!.click();
    await refresh();
    overlayOptions().find(o => o.textContent!.trim() === label)!.click();
    await refresh();
  }

  async function effortLabels(): Promise<string[]> {
    q('effort-select')!.querySelector<HTMLButtonElement>('button')!.click();
    await refresh();
    const labels = overlayOptions().map(o => o.textContent!.trim());
    q('effort-select')!.querySelector<HTMLButtonElement>('button')!.click();
    await refresh();
    return labels;
  }

  afterEach(() => {
    component.originalTextView = '';
    component.canvasTextView = '';
    fixture.destroy();
    document.querySelector('.cdk-overlay-container')?.replaceChildren();
  });

  it('offers Settings’ levels under Model, led by "Model default", with what effort does there', async () => {
    await open({ active_provider: 'claude' });
    expect(q('effort-select')).not.toBeNull();
    expect(await effortLabels()).toEqual(['Model default', 'Low', 'Medium', 'High', 'Extra high', 'Max']);
    expect(q('effort-select')!.textContent).toContain('Model default');
  });

  it('explains effort in a tooltip on the info button next to the label, not as a line under the picker', async () => {
    await open({ active_provider: 'claude' });
    const info = q('effort-info')!;
    expect(info.getAttribute('aria-label')).toBe('What effort does');
    const tip = () => document.querySelector('[data-testid="kl-tooltip"]');

    // A click shows it (a help button does nothing else), a second hides it.
    info.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await refresh();
    expect(tip()!.textContent).toContain('more reasoning');
    info.dispatchEvent(new MouseEvent('click', { bubbles: true }));
    await refresh();
    expect(tip()).toBeNull();
  });

  it('describes the effort picker with the same text for a screen reader', async () => {
    await open({ active_provider: 'claude' });
    const trigger = q('effort-select')!.querySelector('[role="combobox"]')!;
    const note = document.getElementById(trigger.getAttribute('aria-describedby')!)!;
    expect(note.textContent).toContain('more reasoning');
    expect(note.classList).toContain('sr-only');
  });

  it('names Settings’ level first when Settings has one, and offers "Model default" to send none', async () => {
    await open({
      active_provider: 'claude',
      models: { claude: { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: 'high' } },
    });
    expect(await effortLabels()).toEqual(['Settings: High', 'Model default', 'Low', 'Medium', 'High', 'Extra high', 'Max']);
    expect(q('effort-select')!.textContent).toContain('Settings: High');
  });

  it('falls back to following Settings when "Model default" was picked and Settings has no level any more', async () => {
    const high = { claude: { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: 'high' } };
    await open({ active_provider: 'claude', models: high });
    await pickEffort('Model default');
    expect(component.effortView).toBe('none');
    fixture.destroy();
    TestBed.resetTestingModule();

    // Settings' level went back to "Model default" meanwhile; same provider.
    await open({ active_provider: 'claude' });
    await refresh(); // ngModel writes the corrected value on the next turn
    expect(component.effortView).toBe('');
    expect(q('effort-select')!.textContent).toContain('Model default');
  });

  it('is not there on a provider that does nothing with effort (Ollama)', async () => {
    await open({ active_provider: 'ollama' });
    expect(q('model-select')).not.toBeNull();
    expect(q('effort-select')).toBeNull();
    expect(q('effort-note')).toBeNull();
  });

  it('is not there for a provider that is not available yet', async () => {
    await open({ active_provider: 'bedrock' });
    expect(q('effort-select')).toBeNull();
  });

  it('sends the chosen effort with Pyramidize, a whole-text change and a splice; never saves it', async () => {
    await open({ active_provider: 'claude' });
    await pickEffort('Extra high');

    component.originalTextView = 'Some text';
    await component.pyramidize();
    expect(svcMock.pyramidize).toHaveBeenCalledWith(expect.objectContaining({ provider: 'claude', effort: 'xhigh' }));

    component.canvasTextView = 'Canvas';
    component.globalInstruction = 'Shorter';
    await component.applyGlobalInstruction();
    expect(svcMock.refineGlobal).toHaveBeenCalledWith(expect.objectContaining({ effort: 'xhigh' }));

    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
  });

  it('sends "none" for "Model default" over a level in Settings, and "" while it follows Settings', async () => {
    await open({
      active_provider: 'claude',
      models: { claude: { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: 'high' } },
    });
    component.originalTextView = 'Some text';
    await component.pyramidize();
    expect(svcMock.pyramidize).toHaveBeenLastCalledWith(expect.objectContaining({ effort: '' }));

    await pickEffort('Model default');
    component.canvasTextView = '';
    await component.pyramidize();
    expect(svcMock.pyramidize).toHaveBeenLastCalledWith(expect.objectContaining({ effort: 'none' }));
  });

  it('goes back to following Settings when the provider changes, as the model does', async () => {
    await open({ active_provider: 'claude' });
    await pickEffort('Max');
    expect(component.effortView).toBe('max');
    component.providerView = 'openai';
    await component.onProviderChange();
    await refresh();
    expect(component.effortView).toBe('');
    expect(component.modelView).toBe('');
  });
});
