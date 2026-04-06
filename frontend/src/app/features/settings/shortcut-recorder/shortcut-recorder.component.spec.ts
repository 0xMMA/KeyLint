import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { ShortcutRecorderComponent } from './shortcut-recorder.component';

describe('ShortcutRecorderComponent', () => {
  let fixture: ComponentFixture<ShortcutRecorderComponent>;
  let component: ShortcutRecorderComponent;
  let el: HTMLElement;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ShortcutRecorderComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(ShortcutRecorderComponent);
    component = fixture.componentInstance;
    component.value = 'ctrl+g';
    fixture.detectChanges();
    await fixture.whenStable();
    el = fixture.nativeElement;
  });

  it('renders the formatted key combo', () => {
    const display = el.querySelector('[data-testid="combo-display"]');
    expect(display?.textContent?.trim()).toBe('Ctrl + G');
  });

  it('shows Record button', () => {
    expect(el.querySelector('[data-testid="record-btn"]')).toBeTruthy();
  });

  it('enters recording mode on Record click', () => {
    el.querySelector<HTMLButtonElement>('[data-testid="record-btn"]')?.click();
    fixture.detectChanges();
    expect(component.recording).toBe(true);
    expect(el.querySelector('[data-testid="recording-indicator"]')).toBeTruthy();
  });

  it('exits recording mode on Escape', () => {
    component.recording = true;
    fixture.detectChanges();
    const event = new KeyboardEvent('keydown', { key: 'Escape' });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();
    expect(component.recording).toBe(false);
  });

  it('captures a key combo and emits valueChange', () => {
    const spy = vi.fn();
    component.valueChange.subscribe(spy);
    component.recording = true;
    fixture.detectChanges();

    const event = new KeyboardEvent('keydown', { key: 'k', ctrlKey: true, shiftKey: false, altKey: false, metaKey: false });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();

    expect(spy).toHaveBeenCalledWith('ctrl+k');
    expect(component.recording).toBe(false);
  });

  it('ignores modifier-only keypresses during recording', () => {
    const spy = vi.fn();
    component.valueChange.subscribe(spy);
    component.recording = true;
    fixture.detectChanges();

    const event = new KeyboardEvent('keydown', { key: 'Control', ctrlKey: true });
    el.querySelector<HTMLElement>('[data-testid="recorder-field"]')?.dispatchEvent(event);
    fixture.detectChanges();

    expect(spy).not.toHaveBeenCalled();
    expect(component.recording).toBe(true);
  });
});
