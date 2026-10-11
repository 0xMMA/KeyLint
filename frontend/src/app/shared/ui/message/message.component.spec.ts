import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { MessageComponent, type MessageSeverity } from './message.component';

@Component({
  standalone: true,
  imports: [MessageComponent],
  template: `
    <kl-message data-testid="msg" [severity]="severity()" title="Fix took too long" closable (closed)="closed = true">
      <span data-testid="body">Try a shorter selection.</span>
    </kl-message>
    <kl-message data-testid="short" severity="success" text="Fixed and copied." />
  `,
})
class Host {
  readonly severity = signal<MessageSeverity>('warn');
  closed = false;
}

describe('kl-message', () => {
  let fixture: ComponentFixture<Host>;
  let el: HTMLElement;
  const q = (id: string) => el.querySelector<HTMLElement>(`[data-testid="${id}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  it('shows the title, the projected body and the severity icon', () => {
    expect(q('msg').querySelector('[data-testid="kl-message-title"]')?.textContent).toBe('Fix took too long');
    expect(q('body').textContent).toContain('shorter selection');
    expect(q('msg').querySelector('kl-icon[data-icon="triangle-alert"]')).toBeTruthy();
    expect(q('msg').getAttribute('data-severity')).toBe('warn');
  });

  it('is a polite status, and an alert only for errors', async () => {
    expect(q('msg').getAttribute('role')).toBe('status');
    fixture.componentInstance.severity.set('error');
    fixture.detectChanges();
    await fixture.whenStable();
    expect(q('msg').getAttribute('role')).toBe('alert');
    expect(q('msg').querySelector('kl-icon[data-icon="circle-x"]')).toBeTruthy();
  });

  it('takes a one-line text', () => {
    expect(q('short').textContent).toContain('Fixed and copied.');
    expect(q('short').querySelector('[data-testid="kl-message-close"]')).toBeNull();
  });

  it('a closable message hides itself and says so', async () => {
    const close = q('msg').querySelector<HTMLButtonElement>('[data-testid="kl-message-close"]')!;
    expect(close.getAttribute('aria-label')).toBe('Dismiss');
    close.click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(q('msg').hidden).toBe(true);
    expect(fixture.componentInstance.closed).toBe(true);
  });
});
