import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { FixComponent } from './fix.component';
import { TextEnhancementService } from '../text-enhancement/text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock } from '../../../testing/wails-mock';
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
    fixture.detectChanges();
    await fixture.whenStable();
  });

  // --- DOM tests ---

  it('renders input textarea, output textarea, and Fix button', () => {
    expect(el.querySelector('[data-testid="fix-input"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="fix-output"]')).toBeTruthy();
    expect(el.querySelector('[data-testid="fix-btn"]')).toBeTruthy();
  });

  it('output textarea is empty before a fix is run', async () => {
    // The result box keeps module-level state across navigation, and spec
    // files share a process (isolate: false), so another file's last result
    // can be in it. ngModel writes the cleared value asynchronously.
    component.outputText = '';
    fixture.detectChanges();
    await fixture.whenStable();
    const output = el.querySelector<HTMLTextAreaElement>('[data-testid="fix-output"]');
    expect(output?.value).toBe('');
  });

  it('shows fix result in output textarea after a successful fix', async () => {
    component.inputText = 'bad grammer';
    await component.fix();
    fixture.detectChanges();
    await fixture.whenStable();

    const output = el.querySelector<HTMLTextAreaElement>('[data-testid="fix-output"]');
    expect(output?.value).toBe('Fixed text output.');
  });

  it('shows error message in DOM on service failure', async () => {
    enhanceSpy.mockRejectedValue(new Error('API error'));
    component.inputText = 'some text';
    await component.fix();
    fixture.detectChanges();
    await fixture.whenStable();

    expect(el.querySelector('[data-testid="fix-error"]')).toBeTruthy();
  });

  it('error message is absent when there is no error', () => {
    expect(el.querySelector('[data-testid="fix-error"]')).toBeFalsy();
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

    async function render(): Promise<void> {
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
    }

    it('shows what happened, with the text and the result in place', async () => {
      TestBed.inject(SilentFixNoticeService).present(notice);
      await render();

      expect(el.querySelector('[data-testid="silent-fix-notice-title"]')?.textContent).toContain('Fixed, not pasted');
      expect(el.querySelector('[data-testid="silent-fix-notice-body"]')?.textContent).toContain('You switched windows');
      expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-input"]')?.value).toBe('their going home');
      expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-output"]')?.value).toBe("They're going home.");
    });

    it('shows the full error under the reason, and clears a stale result', async () => {
      component.outputText = 'an older result';
      TestBed.inject(SilentFixNoticeService).present({
        ...notice, title: 'Fix took too long', body: 'The model took too long — try a shorter selection or a faster model.',
        detail: 'Claude took too long to answer — try a shorter selection or a faster model', output: '',
      });
      await render();

      expect(el.querySelector('[data-testid="silent-fix-notice-detail"]')?.textContent).toContain('Claude took too long to answer');
      expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-output"]')?.value).toBe('');
    });

    it('goes away when the user runs the fix again', async () => {
      TestBed.inject(SilentFixNoticeService).present(notice);
      await render();
      expect(el.querySelector('[data-testid="silent-fix-notice"]')).toBeTruthy();

      await component.fix();
      await render();
      expect(el.querySelector('[data-testid="silent-fix-notice"]')).toBeFalsy();
    });
  });
});
