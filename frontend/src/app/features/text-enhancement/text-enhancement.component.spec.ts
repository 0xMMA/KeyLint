import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock } from '../../../testing/wails-mock';

// PrimeNG TabList uses ResizeObserver which is not available in jsdom
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockResolvedValue({
      documentType: 'EMAIL',
      language: 'en',
      fullDocument: 'Subject | Details | Actions\n\nBody text',
      headers: ['Header 1'],
      qualityScore: 0.9,
      qualityFlags: [],
      appliedRefinement: false,
      refinementWarning: '',
      detectedType: 'EMAIL',
      detectedLang: 'en',
      detectedConfidence: 0.95,
    }),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: 'Refined canvas text' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: 'Rewritten section' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
    getQualityThreshold: vi.fn().mockResolvedValue(0.65),
    setQualityThreshold: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue('Enhanced text.'),
  };
}

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
        provideAnimationsAsync(),
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
    component.docTypeView = 'auto';
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

  // ── 13. shortcutTriggered$ with empty originalText sets originalText from clipboard ──

  it('shortcutTriggered$ with empty originalText sets originalText from clipboard', async () => {
    component.originalTextView = '';
    wailsMock.readClipboard.mockResolvedValue('clipboard hotkey content');
    wailsMock.getSourceApp.mockResolvedValue('TestApp');

    wailsMock._shortcutTriggered$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(component.originalTextView).toBe('clipboard hotkey content');
  });

  // ── 14. shortcutTriggered$ with existing originalText shows confirm dialog ──

  it('shortcutTriggered$ with existing originalText shows confirm dialog', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);

    wailsMock._shortcutTriggered$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(confirmSpy).toHaveBeenCalled();
    // Since user cancelled, originalText should remain unchanged
    expect(component.originalTextView).toBe('existing content');
  });

  it('shortcutTriggered$ with existing originalText and confirm=true replaces content', async () => {
    component.originalTextView = 'existing content';
    wailsMock.readClipboard.mockResolvedValue('new clipboard content');
    vi.spyOn(window, 'confirm').mockReturnValue(true);

    wailsMock._shortcutTriggered$.next('hotkey');
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

  it('renders trace log panel', () => {
    expect(el.querySelector('[data-testid="trace-log-panel"]')).toBeTruthy();
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

  it('ngOnDestroy unsubscribes from shortcut events', async () => {
    component.ngOnDestroy();
    const prevReadCount = (wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length;
    wailsMock._shortcutTriggered$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect((wailsMock.readClipboard as ReturnType<typeof vi.fn>).mock.calls.length).toBe(prevReadCount);
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
        provideAnimationsAsync(),
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
