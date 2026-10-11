import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { Component, signal, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { TooltipDirective } from './tooltip.directive';
import { ButtonComponent } from '../button/button.component';

@Component({
  standalone: true,
  imports: [TooltipDirective],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <button data-testid="target" [klTooltip]="text()" klTooltipPosition="right">Fix</button>
  `,
})
class Host {
  readonly text = signal('Fix');
}

/** On a kl-button, as the copy-error and history buttons use it. */
@Component({
  standalone: true,
  imports: [TooltipDirective, ButtonComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-button data-testid="host" icon="copy" iconOnly ariaLabel="Copy error" klTooltip="Copy error" klTooltipPosition="top" />
    <input data-testid="elsewhere" />
  `,
})
class ButtonHost {}

describe('klTooltip', () => {
  let fixture: ComponentFixture<Host>;
  let target: HTMLButtonElement;
  const tooltip = () => document.querySelector<HTMLElement>('[role="tooltip"]');

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(async () => {
    vi.useFakeTimers();
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    fixture.detectChanges();
    target = fixture.nativeElement.querySelector('[data-testid="target"]');
  });

  afterEach(() => {
    fixture.destroy();
    vi.useRealTimers();
    document.querySelectorAll('.cdk-overlay-container').forEach(c => (c.innerHTML = ''));
  });

  it('shows after the hover delay, on <body>, and describes its host', async () => {
    target.dispatchEvent(new MouseEvent('mouseenter'));
    expect(tooltip()).toBeNull();
    vi.advanceTimersByTime(300);
    await render();
    const tip = tooltip()!;
    expect(tip.textContent?.trim()).toBe('Fix');
    expect(fixture.nativeElement.contains(tip)).toBe(false);
    expect(target.getAttribute('aria-describedby')).toBe(tip.id);
  });

  it('hides on mouse leave, and on Escape', async () => {
    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    target.dispatchEvent(new MouseEvent('mouseleave'));
    await render();
    expect(tooltip()).toBeNull();
    expect(target.getAttribute('aria-describedby')).toBeNull();

    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    target.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await render();
    expect(tooltip()).toBeNull();
  });

  it('shows on keyboard focus without waiting', async () => {
    target.focus();
    await render();
    expect(tooltip()?.textContent?.trim()).toBe('Fix');
    target.blur();
    await render();
    expect(tooltip()).toBeNull();
  });

  it('an empty text means no tooltip, and emptying it hides one that is up', async () => {
    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    expect(tooltip()).toBeTruthy();
    fixture.componentInstance.text.set('');
    await render();
    expect(tooltip()).toBeNull();
    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    expect(tooltip()).toBeNull();
  });

  it('updates a tooltip that is up when its text changes', async () => {
    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    fixture.componentInstance.text.set('Expand sidebar');
    await render();
    expect(tooltip()?.textContent?.trim()).toBe('Expand sidebar');
  });

  it('Escape hides it wherever the focus is', async () => {
    target.dispatchEvent(new MouseEvent('mouseenter'));
    vi.advanceTimersByTime(300);
    await render();
    expect(tooltip()).toBeTruthy();
    // Focus is on <body>, not on the host: the key goes to the document.
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    await render();
    expect(tooltip()).toBeNull();
  });

  it('a press that did not focus the host does not stop a later keyboard focus from showing it', async () => {
    target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
    await render();
    expect(tooltip()).toBeNull();
    target.focus();
    await render();
    expect(tooltip()?.textContent?.trim()).toBe('Fix');
  });

  it('a click that focuses the host still shows nothing', async () => {
    target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    target.focus();
    target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
    await render();
    expect(tooltip()).toBeNull();
  });
});

/** A help button whose click exists to show the text. */
@Component({
  standalone: true,
  imports: [TooltipDirective],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `<button data-testid="target" klTooltip="What it does" klTooltipOnClick>?</button>`,
})
class HelpHost {}

describe('klTooltip with klTooltipOnClick', () => {
  const tooltip = () => document.querySelector<HTMLElement>('[role="tooltip"]');

  afterEach(() => {
    document.querySelectorAll('.cdk-overlay-container').forEach(c => (c.innerHTML = ''));
  });

  it('a click (or a tap) toggles it, and a click elsewhere still takes it away', async () => {
    await TestBed.configureTestingModule({ imports: [HelpHost] }).compileComponents();
    const fixture = TestBed.createComponent(HelpHost);
    fixture.detectChanges();
    const target: HTMLButtonElement = fixture.nativeElement.querySelector('[data-testid="target"]');
    const settle = async () => { fixture.detectChanges(); await fixture.whenStable(); };

    // A tap: pointerdown, focus, pointerup, click — the focus shows nothing, the click does.
    target.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    target.focus();
    target.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
    target.click();
    await settle();
    expect(tooltip()?.textContent?.trim()).toBe('What it does');

    target.click();
    await settle();
    expect(tooltip()).toBeNull();

    target.click();
    await settle();
    target.blur();
    await settle();
    expect(tooltip()).toBeNull();
    fixture.destroy();
  });
});

describe('klTooltip on a kl-button', () => {
  afterEach(() => {
    document.querySelectorAll('.cdk-overlay-container').forEach(c => (c.innerHTML = ''));
  });

  it('describes the inner <button> that takes the focus, not the wrapper', async () => {
    await TestBed.configureTestingModule({ imports: [ButtonHost] }).compileComponents();
    const fixture = TestBed.createComponent(ButtonHost);
    fixture.detectChanges();
    await fixture.whenStable();
    const host = fixture.nativeElement.querySelector('[data-testid="host"]') as HTMLElement;
    const inner = host.querySelector('button')!;
    inner.focus();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const tip = document.querySelector<HTMLElement>('[role="tooltip"]')!;
    expect(tip).toBeTruthy();
    expect(inner.getAttribute('aria-describedby')).toBe(tip.id);
    expect(host.hasAttribute('aria-describedby')).toBe(false);

    inner.blur();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(inner.hasAttribute('aria-describedby')).toBe(false);
    fixture.destroy();
  });
});
