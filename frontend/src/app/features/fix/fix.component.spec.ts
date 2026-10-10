import { describe, it, expect, beforeEach, vi } from 'vitest';
import { ChangeDetectorRef } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { FixComponent } from './fix.component';
import { TextEnhancementService } from '../text-enhancement/text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';
import { SilentFixNoticeService } from './silent-fix-notice.service';

describe('FixComponent', () => {
  let fixture: ComponentFixture<FixComponent>;
  let component: FixComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let enhanceSpy: ReturnType<typeof vi.fn>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    enhanceSpy = vi.fn().mockResolvedValue('Fixed text output.');

    await TestBed.configureTestingModule({
      imports: [FixComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: { enhance: enhanceSpy } },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(FixComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    // The view choice is module state, like the texts; start every test on the marks.
    component.setView('changes');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  /** The result as the user would copy it: the shown text without what was struck out. */
  function resultText(): string {
    const output = el.querySelector('[data-testid="fix-output"]')!.cloneNode(true) as HTMLElement;
    output.querySelectorAll('del, [data-testid="fix-empty"]').forEach(n => n.remove());
    return output.textContent ?? '';
  }

  const q = (testid: string) => el.querySelector<HTMLElement>(`[data-testid="${testid}"]`);

  async function render(): Promise<void> {
    // Zoneless: a field set from the test does not mark the view dirty, and an
    // unmarked view is skipped (then checkNoChanges reports NG0100).
    fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  // --- DOM tests ---

  it('renders the text field, the result and the Fix button', () => {
    expect(el.querySelector('textarea[data-testid="fix-input"]')).toBeTruthy();
    expect(q('fix-output')).toBeTruthy();
    expect(el.querySelector('[data-testid="fix-btn"] button')?.textContent).toContain('Fix');
    expect(el.querySelector('label[for="fix-input"]')?.textContent).toContain('Your text');
    expect(q('fix-shortcut-hint')?.textContent).toContain('Ctrl');
  });

  it('shows an instruction, not a result, before a fix is run', async () => {
    // The result keeps module-level state across navigation, and spec files
    // share a process (isolate: false), so another file's last result can be
    // in it.
    component.outputText = '';
    await render();
    expect(q('fix-empty')?.textContent).toContain('appears here');
    expect(resultText().trim()).toBe('');
    expect(q('fix-view')).toBeNull();
    expect(q('fix-correction-count')).toBeNull();
  });

  it('counts the characters of the text', async () => {
    component.inputText = 'their going';
    await render();
    expect(q('fix-char-count')?.textContent?.trim()).toBe('11 characters');
    component.inputText = 'a';
    await render();
    expect(q('fix-char-count')?.textContent?.trim()).toBe('1 character');
  });

  it('shows the result after a successful fix', async () => {
    component.inputText = 'bad grammer';
    await component.fix();
    await render();
    expect(resultText()).toBe('Fixed text output.');
  });

  it('marks the corrections against the text that was fixed, and counts them', async () => {
    enhanceSpy.mockResolvedValue("They're going home quickly.");
    component.inputText = 'their going home quick.';
    await component.fix();
    await render();

    const marks = (tag: string) => Array.from(el.querySelectorAll(`[data-testid="fix-output"] ${tag}`)).map(n => n.textContent);
    expect(marks('del')).toEqual(['their', 'quick']);
    expect(marks('ins')).toEqual(["They're", 'quickly']);
    expect(resultText()).toBe("They're going home quickly.");
    expect(q('fix-correction-count')?.textContent?.trim()).toBe('2 corrections');

    // Editing the text afterwards does not move the marks: they belong to the result.
    component.inputText = 'something else entirely';
    await render();
    expect(marks('del')).toEqual(['their', 'quick']);
  });

  it('says so when the fix changed nothing', async () => {
    enhanceSpy.mockResolvedValue('Already fine.');
    component.inputText = 'Already fine.';
    await component.fix();
    await render();
    expect(el.querySelector('[data-testid="fix-output"] ins, [data-testid="fix-output"] del')).toBeNull();
    expect(q('fix-correction-count')?.textContent?.trim()).toBe('No corrections');
  });

  it('switches between the marks and the clean text, and keeps the choice across visits', async () => {
    enhanceSpy.mockResolvedValue('there');
    component.inputText = 'their';
    await component.fix();
    await render();
    expect(q('segment-changes')?.getAttribute('aria-pressed')).toBe('true');
    q('segment-clean')!.click();
    await render();
    expect(el.querySelector('[data-testid="fix-output"] del')).toBeNull();
    expect(q('fix-clean')?.textContent).toBe('there');

    const again = TestBed.createComponent(FixComponent);
    again.detectChanges();
    await again.whenStable();
    expect(again.nativeElement.querySelector('[data-testid="segment-clean"]')?.getAttribute('aria-pressed')).toBe('true');
    again.destroy();
    component.setView('changes');
  });

  it('runs the fix on Ctrl+Enter from the text field', async () => {
    component.inputText = 'bad grammer';
    await render();
    const input = el.querySelector<HTMLTextAreaElement>('[data-testid="fix-input"]')!;
    const event = new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, cancelable: true });
    input.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(true);
    expect(enhanceSpy).toHaveBeenCalledWith('bad grammer');
    // Let the fix finish; it repaints by itself.
    await vi.waitFor(() => expect(component.loading).toBe(false));
    await fixture.whenStable();
  });

  it('shows the busy state on the button while fixing', async () => {
    let finish!: (v: string) => void;
    enhanceSpy.mockReturnValue(new Promise<string>(r => (finish = r)));
    component.inputText = 'bad grammer';
    const running = component.fix();
    await render();
    const button = el.querySelector<HTMLButtonElement>('[data-testid="fix-btn"] button')!;
    expect(button.getAttribute('aria-busy')).toBe('true');
    expect(button.textContent).toContain('Fixing…');
    finish('done');
    await running;
    await render();
    expect(button.getAttribute('aria-busy')).toBeNull();
  });

  it('copies the clean result with the copy button', async () => {
    enhanceSpy.mockResolvedValue('there');
    component.autoCopy = false;
    component.inputText = 'their';
    await component.fix();
    await render();
    expect(wailsMock.writeClipboard).not.toHaveBeenCalled();
    el.querySelector<HTMLButtonElement>('[data-testid="fix-copy-btn"] button')!.click();
    await render();
    expect(wailsMock.writeClipboard).toHaveBeenCalledWith('there');
    expect(q('fix-copy-btn')?.textContent).toContain('Copied');
  });

  it('names the provider and model the fix is sent to', async () => {
    // defaultSettings: OpenAI with nothing stored, so KeyLint's Fix default.
    expect(q('fix-provider')?.textContent?.trim()).toBe('OpenAI');
    expect(q('fix-model')?.textContent?.trim()).toBe('gpt-6-luna');

    wailsMock.loadSettings.mockResolvedValue({
      ...defaultSettings,
      active_provider: 'claude-code',
      models: { 'claude-code': { fix: 'sonnet', pyramidize: '', fix_effort: '', pyramidize_effort: '' } },
    });
    wailsMock._settingsChanged$.next();
    await render();
    await render();
    expect(q('fix-provider')?.textContent?.trim()).toBe('Claude Code');
    // Named as the listing names it (wails-mock lists "Sonnet (latest)").
    expect(q('fix-model')?.textContent?.trim()).toBe('Sonnet (latest)');
  });

  it('shows error message in DOM on service failure', async () => {
    enhanceSpy.mockRejectedValue(new Error('API error'));
    component.inputText = 'some text';
    await component.fix();
    await render();

    const error = q('fix-error')!;
    expect(error.getAttribute('role')).toBe('alert');
    expect(error.textContent).toContain('API error');
  });

  it('error message is absent when there is no error', () => {
    expect(q('fix-error')).toBeFalsy();
  });

  it('confirms a fix that went to the clipboard', async () => {
    component.autoCopy = true;
    component.inputText = 'bad grammer';
    await component.fix();
    await render();
    expect(q('fix-done')?.getAttribute('role')).toBe('status');
    expect(q('fix-done')?.textContent).toContain('Fixed and written to clipboard!');
  });

  // --- Logic tests ---

  it('autoCopy defaults to true', () => {
    expect(component.autoCopy).toBe(true);
  });

  it('fix() calls enhance and writes result to clipboard when autoCopy is on', async () => {
    component.autoCopy = true;
    component.inputText = 'bad grammer';
    await component.fix();
    expect(enhanceSpy).toHaveBeenCalledWith('bad grammer');
    expect(component.outputText).toBe('Fixed text output.');
    expect(wailsMock.writeClipboard).toHaveBeenCalledWith('Fixed text output.');
    expect(component.loading).toBe(false);
  });

  it('fix() does not write clipboard when autoCopy is off', async () => {
    component.autoCopy = false;
    component.inputText = 'bad grammer';
    await component.fix();
    expect(component.outputText).toBe('Fixed text output.');
    expect(wailsMock.writeClipboard).not.toHaveBeenCalled();
  });

  it('fix() does nothing when inputText is blank', async () => {
    component.inputText = '   ';
    await component.fix();
    expect(enhanceSpy).not.toHaveBeenCalled();
  });

  it('fix() sets error on service failure', async () => {
    enhanceSpy.mockRejectedValue(new Error('API error'));
    component.inputText = 'some text';
    await component.fix();
    expect(component.error).toBe('API error');
    expect(component.loading).toBe(false);
  });

  // The hotkey fix runs in Go; this page shows a clicked notification.
  // shell-silent-fix.spec.ts drives the same through the real router.
  describe('a clicked hotkey-fix notification', () => {
    const notice = {
      id: 'silentfix-3', title: 'Fixed, not pasted', body: 'You switched windows — the fix is on the clipboard.',
      target: 'fix', detail: '', input: 'their going home', output: "They're going home.",
    };

    it('shows what happened, with the text and the result in place', async () => {
      TestBed.inject(SilentFixNoticeService).present(notice);
      await render();

      expect(q('silent-fix-notice-title')?.textContent).toContain('Fixed, not pasted');
      expect(q('silent-fix-notice-body')?.textContent).toContain('You switched windows');
      expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-input"]')?.value).toBe('their going home');
      expect(resultText()).toBe("They're going home.");
      expect(el.querySelector('[data-testid="fix-output"] del')?.textContent).toBe('their');
    });

    it('shows the full error under the reason, and clears a stale result', async () => {
      component.outputText = 'an older result';
      TestBed.inject(SilentFixNoticeService).present({
        ...notice, title: 'Fix took too long', body: 'The model took too long — try a shorter selection or a faster model.',
        detail: 'Claude took too long to answer — try a shorter selection or a faster model', output: '',
      });
      await render();

      expect(q('silent-fix-notice-detail')?.textContent).toContain('Claude took too long to answer');
      expect(resultText().trim()).toBe('');
      expect(q('fix-empty')).toBeTruthy();
    });

    it('goes away when the user runs the fix again', async () => {
      TestBed.inject(SilentFixNoticeService).present(notice);
      await render();
      expect(q('silent-fix-notice')).toBeTruthy();

      await component.fix();
      await render();
      expect(q('silent-fix-notice')).toBeFalsy();
    });
  });
});
