import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { ChangeDetectorRef } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent, resetPyramidizeSession } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService, type SourcePreset } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockImplementation(async (req: { documentType: string }) => ({
      documentType: req.documentType.toUpperCase(),
      language: 'en',
      fullDocument: `A ${req.documentType} result`,
    })),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: 'Refined canvas text' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: 'Rewritten' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue('Enhanced text.'),
  };
}

// The session is module-level, outlives every fixture and is shared with other
// spec files (isolate: false): every test starts a fresh one.
beforeEach(() => resetPyramidizeSession());
afterEach(() => resetPyramidizeSession());

const ORIGINAL = 'Hi Tom, the deployment failed last night.\nThe certificate had expired.';
const OUTLOOK: SourcePreset = { app: 'Outlook', documentType: 'email', remembered: false };

describe('TextEnhancementComponent (Pyramidize: Text → Form → Result)', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  async function mount(): Promise<void> {
    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    fixture.detectChanges();
    await settle();
  }

  /** Lets pending promises land, then renders. Zoneless: nothing marks the view by itself. */
  async function settle(): Promise<void> {
    for (let i = 0; i < 3; i++) {
      await new Promise(r => setTimeout(r));
      fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck();
      fixture.detectChanges();
      await fixture.whenStable();
    }
  }

  beforeEach(async () => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });
    svcMock = makeEnhancementServiceMock();
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();
    await mount();
  });

  afterEach(() => {
    vi.restoreAllMocks();
    fixture.destroy();
    document.querySelector('.cdk-overlay-container')?.replaceChildren();
  });

  const q = <T extends HTMLElement = HTMLElement>(testid: string): T | null =>
    el.querySelector<T>(`[data-testid="${testid}"]`);
  const button = (testid: string): HTMLButtonElement => q(testid)!.querySelector('button') ?? (q(testid) as HTMLButtonElement);
  const radio = (form: string) => el.querySelector<HTMLButtonElement>(`[role="radio"][data-form="${form}"]`)!;
  const currentStep = () => el.querySelector('[aria-current="step"]')?.getAttribute('data-testid');
  const traceLabels = () => component.traceLogView.map(e => e.label);

  async function press(key: string, target: EventTarget = document.body): Promise<void> {
    target.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true }));
    await settle();
  }

  /** Types the text on the Text step and goes on to Form. */
  async function typeText(text = ORIGINAL): Promise<void> {
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    area.value = text;
    area.dispatchEvent(new Event('input'));
    await settle();
    button('choose-form-btn').click();
    await settle();
  }

  /** The Pyramidize shortcut brings text from an app. */
  async function viaShortcut(text = ORIGINAL, title = 'Inbox - Outlook', preset: SourcePreset = OUTLOOK): Promise<void> {
    wailsMock.readClipboard.mockResolvedValue(text);
    wailsMock.getSourceApp.mockResolvedValue(title);
    wailsMock.getSourcePreset.mockResolvedValue(preset);
    wailsMock._shortcutPyramidize$.next('hotkey');
    await settle();
  }

  async function run(): Promise<void> {
    button('pyramidize-btn').click();
    await settle();
  }

  // ── The page ──

  it('has no options column and no provider, model, effort, style or relationship control on any step', async () => {
    const assertNone = () => {
      expect(el.querySelector('app-pyramidize-options, .pz-options')).toBeNull();
      for (const id of ['provider-select', 'model-select', 'effort-select', 'doc-type-select', 'comm-style-select', 'rel-level-select']) {
        expect(q(id), id).toBeNull();
      }
      const labels = Array.from(el.querySelectorAll('label')).map(l => l.textContent!.trim());
      for (const gone of ['Provider', 'Model', 'Effort', 'Style', 'Relationship', 'Tone', 'Audience']) {
        expect(labels.some(l => l.startsWith(gone)), gone).toBe(false);
      }
    };
    assertNone();
    await typeText();
    assertNone();
    await run();
    assertNone();
  });

  it('opens on the Text step when there is no text', () => {
    expect(currentStep()).toBe('step-intake');
    expect(q('original-textarea')).not.toBeNull();
    expect(button('choose-form-btn').disabled).toBe(true);
    expect(button('step-choice').disabled).toBe(true);
    expect(button('step-result').disabled).toBe(true);
  });

  it('goes from Text to Form with the typed text, and back with Edit text', async () => {
    await typeText();
    expect(currentStep()).toBe('step-choice');
    expect(el.querySelector('h1')!.textContent).toContain('What should it become?');
    expect(q('text-excerpt')!.textContent).toContain('Hi Tom, the deployment failed last night.');
    expect(q('text-excerpt')!.textContent).not.toContain('certificate');

    button('edit-text-btn').click();
    await settle();
    expect(currentStep()).toBe('step-intake');
    expect(q<HTMLTextAreaElement>('original-textarea')!.value).toBe(ORIGINAL);
  });

  it('pastes from the clipboard on the Text step', async () => {
    wailsMock.readClipboard.mockResolvedValue('pasted text');
    button('paste-from-clipboard-btn').click();
    await settle();
    expect(q<HTMLTextAreaElement>('original-textarea')!.value).toBe('pasted text');
    expect(button('choose-form-btn').disabled).toBe(false);
  });

  // ── Arriving with the shortcut, and what is remembered per app ──

  it('skips the Text step for text the shortcut brought, and preselects email tagged Default', async () => {
    await viaShortcut();
    expect(currentStep()).toBe('step-choice');
    expect(radio('email').getAttribute('aria-checked')).toBe('true');
    expect(radio('email').querySelector('.pz-form-tag')!.textContent!.trim()).toBe('Default');
    expect(q('source-chip')!.textContent!.trim()).toBe('From Outlook');
  });

  it('preselects the form remembered for the app', async () => {
    await viaShortcut(ORIGINAL, 'general | Acme - Microsoft Teams', { app: 'Teams', documentType: 'chat', remembered: true });
    expect(radio('chat').getAttribute('aria-checked')).toBe('true');
    expect(radio('chat').querySelector('.pz-form-tag')!.textContent!.trim()).toBe('Remembered for Teams');
    expect(q('remembered-note')!.textContent).toContain('Chat is remembered for Teams.');
    expect(q('remember-checkbox')).toBeNull();
  });

  it('writes the preset when the user runs with the box ticked — not when it is ticked', async () => {
    await viaShortcut();
    await press('3');
    const box = q('remember-checkbox')!;
    expect(box.textContent!.trim()).toBe('Remember Memo for Outlook');
    box.querySelector<HTMLInputElement>('input')!.click();
    await settle();
    expect(svcMock.setAppPreset).not.toHaveBeenCalled();

    // What the backend says after the write: asked again, not assumed.
    wailsMock.getSourcePreset.mockResolvedValue({ app: 'Outlook', documentType: 'memo', remembered: true });
    const asked = wailsMock.getSourcePreset.mock.calls.length;
    await run();
    expect(wailsMock.getSourcePreset.mock.calls.length).toBe(asked + 1);
    expect(svcMock.setAppPreset).toHaveBeenCalledExactlyOnceWith({ sourceApp: 'Outlook', documentType: 'memo' });
    expect(wailsMock.getSourcePreset).toHaveBeenLastCalledWith('Inbox - Outlook');

    button('try-another-form-btn').click();
    await settle();
    expect(q('remembered-note')!.textContent).toContain('Memo is remembered for Outlook.');
    expect(radio('memo').querySelector('.pz-form-tag')!.textContent!.trim()).toBe('Remembered for Outlook');
  });

  it('writes no preset when the box stays unticked', async () => {
    await viaShortcut();
    await press('2');
    await run();
    expect(svcMock.setAppPreset).not.toHaveBeenCalled();
    expect(svcMock.pyramidize).toHaveBeenCalledWith(expect.objectContaining({ documentType: 'chat' }));
  });

  it('forgets the remembered form with Forget', async () => {
    await viaShortcut(ORIGINAL, 'general | Acme - Microsoft Teams', { app: 'Teams', documentType: 'chat', remembered: true });
    wailsMock.getSourcePreset.mockResolvedValue({ app: 'Teams', documentType: 'email', remembered: false });
    const asked = wailsMock.getSourcePreset.mock.calls.length;
    button('forget-preset-btn').click();
    await settle();
    expect(svcMock.deleteAppPreset).toHaveBeenCalledExactlyOnceWith('Teams');
    expect(wailsMock.getSourcePreset.mock.calls.length).toBe(asked + 1);
    expect(wailsMock.getSourcePreset).toHaveBeenLastCalledWith('general | Acme - Microsoft Teams');
    expect(q('remembered-note')).toBeNull();
    expect(q('remember-checkbox')!.textContent!.trim()).toBe('Remember Chat for Teams');
    expect(radio('email').querySelector('.pz-form-tag')!.textContent!.trim()).toBe('Default');
  });

  it('offers nothing to remember, and no Send back, for pasted text', async () => {
    await typeText();
    expect(q('remember-checkbox')).toBeNull();
    expect(q('remembered-note')).toBeNull();
    expect(q('source-chip')!.textContent!.trim()).toBe('Pasted into KeyLint');
    await run();
    expect(q('send-back-btn')).toBeNull();
  });

  it('treats text pasted over a captured one as pasted, once the replace is confirmed', async () => {
    await viaShortcut();
    button('edit-text-btn').click();
    await settle();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    wailsMock.readClipboard.mockResolvedValue('other text');
    button('paste-from-clipboard-btn').click();
    await settle();
    button('choose-form-btn').click();
    await settle();
    expect(q('remember-checkbox')).toBeNull();
    expect(q('source-chip')!.textContent!.trim()).toBe('Pasted into KeyLint');
  });

  // ── Running ──

  it('sends the original text, the form and the instructions — and nothing else of the user\'s', async () => {
    await typeText();
    await press('3');
    const field = q<HTMLInputElement>('custom-instructions')!;
    field.value = '  in English  ';
    field.dispatchEvent(new Event('input'));
    await settle();
    await run();

    const req = svcMock.pyramidize.mock.calls[0][0];
    expect(req).toEqual(expect.objectContaining({
      text: ORIGINAL, documentType: 'memo', customInstructions: 'in English', provider: '', model: '', effort: '',
    }));
    expect(req).not.toHaveProperty('communicationStyle');
    expect(req).not.toHaveProperty('relationshipLevel');
  });

  it('shows the result titled with its form, in the editor', async () => {
    await typeText();
    await run();
    expect(currentStep()).toBe('step-result');
    expect(q('result-title')!.textContent!.trim()).toBe('Email');
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('A email result');
    expect(q('result-version')).toBeNull();
  });

  it('shows the working step, and Cancel goes back to Form without a result', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValueOnce(new Promise(r => (finish = r)));
    await typeText();
    await press('4');
    button('pyramidize-btn').click();
    await settle();

    expect(q('step-label')!.textContent!.trim()).toBe('Writing the wiki…');
    expect(el.querySelector('app-choice-step')).toBeNull();
    expect(button('step-intake').disabled).toBe(true);

    button('cancel-btn').click();
    await settle();
    expect(svcMock.cancelOperation).toHaveBeenCalled();
    expect(currentStep()).toBe('step-choice');
    expect(radio('wiki').getAttribute('aria-checked')).toBe('true');

    // A reply arriving after the cancel is dropped.
    finish({ documentType: 'WIKI', language: 'en', fullDocument: 'late' });
    await settle();
    expect(currentStep()).toBe('step-choice');
    expect(button('step-result').disabled).toBe(true);
    expect(q('error-row')).toBeNull();
  });

  it('going back keeps every earlier result and history entry, and runs again from the original', async () => {
    await typeText();
    await run();
    const editor = q<HTMLTextAreaElement>('canvas-textarea')!;
    editor.value = 'My edited email';
    editor.dispatchEvent(new Event('input'));
    await settle();

    button('try-another-form-btn').click();
    await settle();
    expect(currentStep()).toBe('step-choice');
    await press('2');
    await run();

    expect(svcMock.pyramidize).toHaveBeenCalledTimes(2);
    expect(svcMock.pyramidize.mock.calls[1][0]).toEqual(expect.objectContaining({ text: ORIGINAL, documentType: 'chat' }));
    expect(q('result-title')!.textContent!.trim()).toBe('Chat');
    const chips = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[data-testid="result-version"]'));
    expect(chips().map(c => c.textContent!.trim())).toEqual(['Email', 'Chat']);
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('A chat result');

    chips()[0].click();
    await settle();
    expect(q('result-title')!.textContent!.trim()).toBe('Email');
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('My edited email');

    expect(traceLabels()).toEqual(['Original', 'Pyramidized as email', 'Pyramidized as chat']);
  });

  it('takes a new snapshot of the original once it was edited', async () => {
    await typeText();
    await run();
    button('step-intake').click();
    await settle();
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    area.value = 'A new text';
    area.dispatchEvent(new Event('input'));
    await settle();
    button('choose-form-btn').click();
    await settle();
    await run();
    expect(svcMock.pyramidize.mock.calls[1][0].text).toBe('A new text');
    expect(traceLabels()).toEqual(['Original', 'Pyramidized as email', 'Original', 'Pyramidized as email']);
    expect(Array.from(el.querySelectorAll('[data-testid="result-version"]')).map(c => c.textContent!.trim()))
      .toEqual(['Email', 'Email 2']);
  });

  it('shows a failed run on the Form step, with Copy error and Retry', async () => {
    svcMock.pyramidize.mockRejectedValueOnce(new Error('API failure'));
    await typeText();
    await run();
    expect(currentStep()).toBe('step-choice');
    const row = q('error-row')!;
    expect(row.textContent).toContain('Pyramidize failed: API failure');
    expect(q('copy-error-btn')).not.toBeNull();

    button('retry-btn').click();
    await settle();
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(2);
    expect(currentStep()).toBe('step-result');
  });

  it('shows the error again when a retried run fails too', async () => {
    svcMock.pyramidize.mockRejectedValue(new Error('API failure'));
    await typeText();
    await run();
    button('retry-btn').click();
    await settle();
    expect(q('error-row')!.textContent).toContain('Pyramidize failed: API failure');
  });

  // ── Keys ──

  it('takes 1–5 and Enter on the Form step only', async () => {
    // Text step: a digit is nobody's.
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    area.value = ORIGINAL;
    area.dispatchEvent(new Event('input'));
    await settle();
    await press('2');
    expect(component.docTypeView).toBe('email');

    await press('Enter'); // the Text step's own Enter: on to Form
    expect(currentStep()).toBe('step-choice');
    await press('2');
    expect(radio('chat').getAttribute('aria-checked')).toBe('true');
    await press('Enter');
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
    expect(currentStep()).toBe('step-result');

    // Result step: neither picks nor runs.
    await press('4');
    await press('Enter');
    expect(component.docTypeView).toBe('chat');
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
  });

  it('keeps digits typed into the instructions as text', async () => {
    await typeText();
    const field = q<HTMLInputElement>('custom-instructions')!;
    await press('3', field);
    expect(component.docTypeView).toBe('email');
    await press('Enter', field);
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
  });

  // ── Settings ──

  it('says which model, provider and effort Settings name, with a link there', async () => {
    fixture.destroy();
    wailsMock.loadSettings.mockResolvedValue({
      ...defaultSettings,
      active_provider: 'claude-code',
      models: { 'claude-code': { fix: '', pyramidize: '', fix_effort: '', pyramidize_effort: 'high' } },
    });
    wailsMock.getClaudeCodeStatus.mockResolvedValue({ installed: true, path: '', version: '', loggedIn: true });
    await mount();
    await typeText();
    expect(q('run-settings')!.textContent).toContain('Sonnet (latest) · Claude Code (installed CLI) · effort high');
    expect(q<HTMLAnchorElement>('change-in-settings')!.getAttribute('href')).toBe('/settings?tab=providers');
  });

  it('names no effort for a provider that does nothing with it (Ollama)', async () => {
    fixture.destroy();
    wailsMock.loadSettings.mockResolvedValue({
      ...defaultSettings,
      active_provider: 'ollama',
      models: { ollama: { fix: '', pyramidize: 'qwen3', fix_effort: '', pyramidize_effort: '' } },
    });
    await mount();
    await typeText();
    expect(q('run-settings')!.textContent).toContain('qwen3 · Ollama (local)');
    expect(q('run-settings')!.textContent).not.toContain('effort');
  });

  it('shows the missing-key banner for a provider without a key', async () => {
    fixture.destroy();
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });
    await mount();
    expect(q('api-key-banner-message')!.textContent).toContain('No AI API key configured.');
  });

  // ── The result ──

  async function result(text = 'Some result'): Promise<void> {
    svcMock.pyramidize.mockResolvedValueOnce({ documentType: 'EMAIL', language: 'en', fullDocument: text });
    await typeText();
    await run();
  }

  it('switches the editor between editing and a formatted preview', async () => {
    await result('# Title\n\nBody');
    q<HTMLButtonElement>('segment-preview')!.click();
    await settle();
    expect(q('canvas-textarea')).toBeNull();
    expect(q('canvas-preview')!.querySelector('h1')!.textContent).toBe('Title');
    q<HTMLButtonElement>('segment-edit')!.click();
    await settle();
    expect(q('canvas-textarea')).not.toBeNull();
  });

  it('changes the whole text of the result shown on Ctrl+Enter, keeping its form', async () => {
    await result('Body');
    const input = q<HTMLInputElement>('global-instruction-input')!;
    input.value = 'Make it shorter';
    input.dispatchEvent(new Event('input'));
    await settle();
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true }));
    await settle();

    const req = svcMock.refineGlobal.mock.calls[0][0];
    expect(req).toEqual(expect.objectContaining({ fullCanvas: 'Body', originalText: ORIGINAL, instruction: 'Make it shorter', documentType: 'email' }));
    // Settings decide provider, model and effort: the page overrides none (E3 phase 3).
    expect(req).toEqual(expect.objectContaining({ provider: '', model: '', effort: '' }));
    expect(req).not.toHaveProperty('communicationStyle');
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('Refined canvas text');
    expect(traceLabels().at(-1)).toBe('Refined: "Make it shorter"');
  });

  it('makes Apply the primary action once an instruction is typed', async () => {
    await result();
    expect(q('apply-instruction-btn')!.querySelector('button')!.className).toContain('quiet');
    const input = q<HTMLInputElement>('global-instruction-input')!;
    input.value = 'shorter';
    input.dispatchEvent(new Event('input'));
    await settle();
    expect(q('apply-instruction-btn')!.querySelector('button')!.className).toContain('primary');
  });

  it('keeps the history out of reach while an edit runs', async () => {
    await result();
    button('trace-log-toggle').click();
    await settle();
    let finish!: (v: unknown) => void;
    svcMock.refineGlobal.mockReturnValueOnce(new Promise(r => (finish = r)));
    component.globalInstruction = 'shorter';
    const running = component.applyGlobalInstruction();
    await settle();
    expect(q('trace-log-panel')!.hasAttribute('inert')).toBe(true);
    expect(q('step-label')!.textContent!.trim()).toBe('Refining…');
    finish({ newCanvas: 'short' });
    await running;
    await settle();
    expect(q('trace-log-panel')!.hasAttribute('inert')).toBe(false);
  });

  it('counts, previews, pins and reverts history snapshots', async () => {
    await result('First version');
    button('trace-log-toggle').click();
    await settle();
    expect(button('trace-log-toggle').textContent!.trim()).toBe('History, 2 steps');

    const editor = q<HTMLTextAreaElement>('canvas-textarea')!;
    editor.value = 'Second version';
    editor.dispatchEvent(new Event('input'));
    await settle();

    const entries = () => el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]');
    const last = entries()[entries().length - 1];
    last.dispatchEvent(new MouseEvent('mouseenter'));
    await settle();
    expect(q('trace-peek-content')!.textContent).toBe('First version');

    last.click();
    await settle();
    expect(q('trace-peek-close')).not.toBeNull();
    button('trace-revert-btn').click();
    await settle();
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('First version');
    expect(traceLabels().at(-1)).toContain('Reverted to');
  });

  it('closes a pinned preview on Escape and gives the focus back to its entry', async () => {
    await result();
    button('trace-log-toggle').click();
    await settle();
    const entries = el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]');
    const last = entries[entries.length - 1];
    last.click();
    await settle();
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await settle();
    expect(q('trace-peek-overlay')).toBeNull();
    expect(document.activeElement).toBe(last);
  });

  it('adds a checkpoint from the history column', async () => {
    await result();
    button('trace-log-toggle').click();
    await settle();
    button('add-checkpoint-btn').click();
    await settle();
    expect(traceLabels().at(-1)).toBe('Checkpoint');
  });

  it('opens the selection bubble for selected text and splices into the result shown', async () => {
    await result('Some text to change');
    const textarea = q<HTMLTextAreaElement>('canvas-textarea')!;
    textarea.setSelectionRange(5, 9);
    textarea.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
    await settle();
    expect(q('selection-bubble')).not.toBeNull();

    const input = q<HTMLInputElement>('selection-instruction-input')!;
    input.value = 'bolder';
    input.dispatchEvent(new Event('input'));
    await settle();
    textarea.setSelectionRange(5, 9);
    await component.applySelectionInstruction();
    await settle();
    expect(svcMock.splice).toHaveBeenCalledWith(expect.objectContaining({
      selectedText: 'text', instruction: 'bolder', provider: '', model: '', effort: '',
    }));
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('Some Rewritten to change');
  });

  it('copies the result shown as Markdown', async () => {
    await result('# Hello');
    button('copy-markdown-btn').click();
    await settle();
    expect(wailsMock.writeClipboard).toHaveBeenCalledWith('# Hello');
  });

  describe('Send back', () => {
    beforeEach(async () => {
      await viaShortcut();
      await run();
    });

    it('sends the result shown back to the app the text came from', async () => {
      expect(button('send-back-btn').textContent).toContain('Send back to Outlook');
      button('send-back-btn').click();
      await settle();
      expect(svcMock.sendBack).toHaveBeenCalledWith('A email result');
      expect(q('error-row')).toBeNull();
    });

    it('says why the text was not pasted, without offering to copy the error', async () => {
      svcMock.sendBack.mockRejectedValue(new Error(
        '{"message":"The original window is closed — the text is on the clipboard.","kind":"RuntimeError"}',
      ));
      button('send-back-btn').click();
      await settle();
      const row = q('error-row')!;
      expect(row.textContent).toContain('The original window is closed — the text is on the clipboard.');
      expect(row.textContent).not.toContain('RuntimeError');
      expect(q('copy-error-btn')).toBeNull();
    });

    it('is disabled while a send back runs, and ignores a second click', async () => {
      let finish!: () => void;
      svcMock.sendBack.mockReturnValue(new Promise<void>(r => (finish = r)));
      const first = component.sendBack();
      await settle();
      expect(button('send-back-btn').disabled).toBe(true);
      await component.sendBack();
      finish();
      await first;
      await settle();
      expect(button('send-back-btn').disabled).toBe(false);
      expect(svcMock.sendBack).toHaveBeenCalledTimes(1);
    });
  });

  // ── PR #130 review round ──

  it('does not send back into another window after a declined replace (review 1)', async () => {
    await viaShortcut();
    await run();
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    await viaShortcut('slack text', 'general - Acme - Slack', { app: 'Slack', documentType: 'chat', remembered: true });
    expect(button('send-back-btn').textContent).toContain('Send back to Outlook');

    button('send-back-btn').click();
    await settle();
    expect(svcMock.sendBack).not.toHaveBeenCalled();
    expect(q('error-row')!.textContent).toContain('The shortcut was used in another window since');
    expect(q('copy-markdown-btn')).not.toBeNull();
  });

  it('starts one run for a double click, even while it remembers the form first (review 2)', async () => {
    await viaShortcut();
    q('remember-checkbox')!.querySelector<HTMLInputElement>('input')!.click();
    await settle();
    const btn = button('pyramidize-btn');
    btn.click();
    btn.click();
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await settle();
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
    expect(svcMock.setAppPreset).toHaveBeenCalledTimes(1);
  });

  it('offers no Remember box when nothing can be remembered for the app (review 3)', async () => {
    await viaShortcut(ORIGINAL, 'Max Mustermann', { app: '', documentType: 'email', remembered: false });
    expect(q('remember-checkbox')).toBeNull();
  });

  it('says so when the form could not be remembered, and still runs (review nit)', async () => {
    await viaShortcut();
    svcMock.setAppPreset.mockRejectedValueOnce(new Error('settings.json is locked'));
    q('remember-checkbox')!.querySelector<HTMLInputElement>('input')!.click();
    await settle();
    await run();
    expect(svcMock.pyramidize).toHaveBeenCalledTimes(1);
    expect(q('preset-notice')!.textContent).toContain('Could not remember Email for Outlook');
  });

  it('shows the run still working after leaving and coming back, and its result when it lands (review 4)', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValueOnce(new Promise(r => (finish = r)));
    await typeText();
    button('pyramidize-btn').click();
    await settle();
    fixture.destroy();

    await mount();
    expect(q('step-label')!.textContent!.trim()).toBe('Writing the email…');
    expect(q('cancel-btn')).not.toBeNull();
    expect(q('pyramidize-btn')).toBeNull();

    finish({ documentType: 'EMAIL', language: 'en', fullDocument: 'Landed after return' });
    await settle();
    expect(currentStep()).toBe('step-result');
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('Landed after return');
  });

  it('clears an error row when the step, the form or the instructions change (review 5)', async () => {
    svcMock.pyramidize.mockRejectedValue(new Error('API failure'));
    await typeText();
    await run();
    expect(q('error-row')).not.toBeNull();
    button('edit-text-btn').click();
    await settle();
    button('choose-form-btn').click();
    await settle();
    expect(q('error-row')).toBeNull();

    await run();
    expect(q('error-row')).not.toBeNull();
    await press('2');
    expect(q('error-row')).toBeNull();

    await run();
    const field = q<HTMLInputElement>('custom-instructions')!;
    field.value = 'shorter';
    field.dispatchEvent(new Event('input'));
    await settle();
    expect(q('error-row')).toBeNull();
  });

  it('never shows a failed whole-text change on the Form step (review 5)', async () => {
    await typeText();
    await run();
    svcMock.refineGlobal.mockRejectedValueOnce(new Error('refine broke'));
    component.globalInstruction = 'shorter';
    await component.applyGlobalInstruction();
    await settle();
    expect(q('error-row')!.textContent).toContain('Refine failed');
    button('try-another-form-btn').click();
    await settle();
    expect(q('error-row')).toBeNull();
  });

  it('reverts a snapshot into the version it came from, and shows that version (review 9)', async () => {
    await typeText();
    await run();
    button('try-another-form-btn').click();
    await settle();
    await press('2');
    await run();
    expect(q('result-title')!.textContent!.trim()).toBe('Chat');
    button('trace-log-toggle').click();
    await settle();
    const emailEntry = Array.from(el.querySelectorAll<HTMLButtonElement>('[data-testid="trace-entry"]'))
      .find(e => e.textContent!.includes('Pyramidized as email'))!;
    emailEntry.click();
    await settle();
    button('trace-revert-btn').click();
    await settle();
    expect(q('result-title')!.textContent!.trim()).toBe('Email');
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('A email result');
    Array.from(el.querySelectorAll<HTMLButtonElement>('[data-testid="result-version"]'))[1].click();
    await settle();
    expect(q<HTMLTextAreaElement>('canvas-textarea')!.value).toBe('A chat result');
  });

  it('announces the working step in a live region that is there before it starts (review 10)', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValueOnce(new Promise(r => (finish = r)));
    await typeText();
    const live = q('live-status')!;
    expect(live.getAttribute('aria-live')).toBe('polite');
    expect(live.textContent!.trim()).toBe('');
    button('pyramidize-btn').click();
    await settle();
    expect(q('live-status')).toBe(live);
    expect(live.textContent!.trim()).toBe('Writing the email…');
    finish({ documentType: 'EMAIL', language: 'en', fullDocument: 'x' });
    await settle();
  });

  it('changes a result against the text it was made from, not the Text step\'s current one (review 11)', async () => {
    await typeText();
    await run();
    button('step-intake').click();
    await settle();
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    area.value = 'A different text';
    area.dispatchEvent(new Event('input'));
    await settle();
    button('step-result').click();
    await settle();
    component.globalInstruction = 'shorter';
    await component.applyGlobalInstruction();
    expect(svcMock.refineGlobal.mock.calls[0][0].originalText).toBe(ORIGINAL);
  });

  it('keeps the session\'s app when the text is emptied and retyped, and Send back for its results (review 11, N4)', async () => {
    await viaShortcut();
    await run();
    button('step-intake').click();
    await settle();
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    area.value = '';
    area.dispatchEvent(new Event('input'));
    await settle();
    area.value = 'Retyped';
    area.dispatchEvent(new Event('input'));
    await settle();
    button('choose-form-btn').click();
    await settle();
    expect(q('remember-checkbox')).not.toBeNull();
    button('step-result').click();
    await settle();
    button('send-back-btn').click();
    await settle();
    expect(svcMock.sendBack).toHaveBeenCalledWith('A email result');
  });

  it('cancels the run in flight when a shortcut replaces the session (N1)', async () => {
    let finish!: (v: unknown) => void;
    svcMock.pyramidize.mockReturnValueOnce(new Promise(r => (finish = r)));
    await typeText();
    button('pyramidize-btn').click();
    await settle();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    await viaShortcut('new text');
    expect(svcMock.cancelOperation).toHaveBeenCalled();
    finish({ documentType: 'EMAIL', language: 'en', fullDocument: 'late' });
    await settle();
    expect(currentStep()).toBe('step-choice');
    expect(button('step-result').disabled).toBe(true);
  });

  it('asks before pasting over a session, and starts a new one on yes (N2)', async () => {
    await typeText();
    await run();
    button('step-intake').click();
    await settle();
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    wailsMock.readClipboard.mockResolvedValue('pasted');
    button('paste-from-clipboard-btn').click();
    await settle();
    expect(confirmSpy).toHaveBeenCalled();
    expect(q<HTMLTextAreaElement>('original-textarea')!.value).toBe(ORIGINAL);

    confirmSpy.mockReturnValue(true);
    button('paste-from-clipboard-btn').click();
    await settle();
    expect(q<HTMLTextAreaElement>('original-textarea')!.value).toBe('pasted');
    expect(button('step-result').disabled).toBe(true);
    expect(traceLabels()).toEqual([]);
  });

  // ── The shortcut while a session is open, and navigation ──

  it('asks before the shortcut replaces a session, and keeps it when declined', async () => {
    await typeText();
    await press('4');
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    await viaShortcut('new clipboard content', 'Slack', { app: 'Slack', documentType: 'chat', remembered: true });
    expect(confirmSpy).toHaveBeenCalled();
    expect(component.originalTextView).toBe(ORIGINAL);
    expect(component.docTypeView).toBe('wiki');
  });

  it('starts a new session when the replace is confirmed: new text, its form, no results', async () => {
    await typeText();
    await run();
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    await viaShortcut('new clipboard content', 'Slack', { app: 'Slack', documentType: 'chat', remembered: true });
    expect(component.originalTextView).toBe('new clipboard content');
    expect(currentStep()).toBe('step-choice');
    expect(radio('chat').getAttribute('aria-checked')).toBe('true');
    expect(button('step-result').disabled).toBe(true);
    expect(traceLabels()).toEqual(['Original']);
  });

  it('keeps the step, the pick and the results when the page is opened again', async () => {
    await typeText();
    await press('5');
    await run();
    fixture.destroy();
    await mount();
    expect(currentStep()).toBe('step-result');
    expect(q('result-title')!.textContent!.trim()).toBe('Presentation');
    expect(component.docTypeView).toBe('powerpoint');
  });

  it('stops listening for the shortcut once destroyed', async () => {
    component.ngOnDestroy();
    wailsMock._shortcutPyramidize$.next('hotkey');
    await new Promise(r => setTimeout(r));
    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
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
    fixture.detectChanges();
    await fixture.whenStable();
    await new Promise(r => setTimeout(r, 0));
  }

  beforeEach(() => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.readClipboard.mockResolvedValue('text copied before the page opened');
    wailsMock.getSourceApp.mockResolvedValue('Inbox - Outlook');
    wailsMock.getSourcePreset.mockResolvedValue(OUTLOOK);
  });

  afterEach(() => {
    fixture?.destroy();
    vi.restoreAllMocks();
  });

  it('loads the clipboard when a shortcut is pending on arrival, and opens on Form', async () => {
    wailsMock.takePendingPyramidize.mockReturnValueOnce(true);
    await mount();
    expect(fixture.componentInstance.originalTextView).toBe('text copied before the page opened');
    expect(fixture.componentInstance.shownStep).toBe('choice');
  });

  it('leaves the page alone when nothing is pending', async () => {
    await mount();
    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
    expect(fixture.componentInstance.shownStep).toBe('intake');
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
