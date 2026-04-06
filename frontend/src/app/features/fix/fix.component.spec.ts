import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { FixComponent } from './fix.component';
import { TextEnhancementService } from '../text-enhancement/text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock } from '../../../testing/wails-mock';

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

  it('output textarea is empty before a fix is run', () => {
    component.outputText = '';
    fixture.detectChanges();
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

  it('shortcutSingle$ triggers fixClipboard', async () => {
    wailsMock.readClipboard.mockResolvedValue('shortcut clipboard');
    wailsMock._shortcutSingle$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(enhanceSpy).toHaveBeenCalledWith('shortcut clipboard');
  });

  it('ngOnDestroy unsubscribes from shortcut events', async () => {
    component.ngOnDestroy();
    wailsMock._shortcutSingle$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));
    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
  });
});
