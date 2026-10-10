import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { TooltipDirective } from './tooltip.directive';

@Component({
  standalone: true,
  imports: [TooltipDirective],
  template: `
    <button data-testid="target" [klTooltip]="text()" klTooltipPosition="right">Fix</button>
  `,
})
class Host {
  readonly text = signal('Fix');
}

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
});
