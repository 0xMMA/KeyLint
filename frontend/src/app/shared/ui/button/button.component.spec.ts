import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { ButtonComponent, type ButtonVariant } from './button.component';

@Component({
  standalone: true,
  imports: [ButtonComponent],
  template: `
    <kl-button data-testid="btn" [variant]="variant()" icon="sparkles" [loading]="loading()"
      [disabled]="disabled()" loadingLabel="Fixing…" (click)="clicks = clicks + 1">
      Fix
      <span klButtonEnd data-testid="hint"><kbd>Ctrl</kbd> <kbd>Enter</kbd></span>
    </kl-button>
    <kl-button data-testid="icon-only" icon="x" iconOnly ariaLabel="Close" />
  `,
})
class Host {
  readonly variant = signal<ButtonVariant>('primary');
  readonly loading = signal(false);
  readonly disabled = signal(false);
  clicks = 0;
}

describe('kl-button', () => {
  let fixture: ComponentFixture<Host>;
  let el: HTMLElement;
  const button = (id = 'btn') => el.querySelector<HTMLButtonElement>(`[data-testid="${id}"] button`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
  }

  it('renders one real button with its label, icon and projected end content', () => {
    const b = button();
    expect(b.getAttribute('type')).toBe('button');
    expect(b.textContent).toContain('Fix');
    expect(b.querySelector('kl-icon[data-icon="sparkles"]')).toBeTruthy();
    expect(b.querySelector('[data-testid="hint"]')?.textContent).toContain('Ctrl');
  });

  it('applies the variant as a class', async () => {
    expect(button().classList).toContain('primary');
    fixture.componentInstance.variant.set('danger');
    await render();
    expect(button().classList).toContain('danger');
    expect(button().classList).not.toContain('primary');
  });

  it('fires (click) on the host when the inner button is clicked', () => {
    button().click();
    expect(fixture.componentInstance.clicks).toBe(1);
  });

  it('does not fire while disabled', async () => {
    fixture.componentInstance.disabled.set(true);
    await render();
    expect(button().disabled).toBe(true);
    button().click();
    expect(fixture.componentInstance.clicks).toBe(0);
  });

  it('while loading: spinner instead of icon, the loading label, aria-busy, no clicks', async () => {
    fixture.componentInstance.loading.set(true);
    await render();
    const b = button();
    expect(b.getAttribute('aria-busy')).toBe('true');
    expect(b.querySelector('[data-testid="kl-button-spinner"]')).toBeTruthy();
    expect(b.querySelector('kl-icon[data-icon="sparkles"]')).toBeNull();
    expect(b.textContent).toContain('Fixing…');
    expect(b.textContent).not.toMatch(/\bFix\b(?!ing)/);
    b.click();
    expect(fixture.componentInstance.clicks).toBe(0);
  });

  it('gives an icon-only button its accessible name', () => {
    expect(button('icon-only').getAttribute('aria-label')).toBe('Close');
    expect(button('icon-only').classList).toContain('icon-only');
  });
});
