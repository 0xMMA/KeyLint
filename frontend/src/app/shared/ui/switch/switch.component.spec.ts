import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { SwitchComponent } from './switch.component';

@Component({
  standalone: true,
  imports: [SwitchComponent, FormsModule],
  template: `
    <kl-switch data-testid="boot" [(ngModel)]="boot" [disabled]="locked()">Start on boot</kl-switch>
    <kl-switch data-testid="named" [(checked)]="plain" ariaLabel="Sensitive logging" />
  `,
})
class Host {
  readonly boot = signal(false);
  readonly plain = signal(true);
  readonly locked = signal(false);
}

describe('kl-switch', () => {
  let fixture: ComponentFixture<Host>;
  let host: Host;
  let el: HTMLElement;
  const q = <T extends HTMLElement = HTMLElement>(sel: string) => el.querySelector<T>(sel)!;

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    el = fixture.nativeElement;
    await render();
  });

  it('switch: a role="switch" button named by its text, flipped by a click', async () => {
    const sw = q<HTMLButtonElement>('[data-testid="boot"] [role="switch"]');
    expect(sw.tagName).toBe('BUTTON');
    expect(sw.getAttribute('aria-checked')).toBe('false');
    const labelId = sw.getAttribute('aria-labelledby')!;
    expect(el.querySelector(`#${labelId}`)?.textContent).toContain('Start on boot');
    sw.click();
    await render();
    expect(host.boot()).toBe(true);
    expect(sw.getAttribute('aria-checked')).toBe('true');
  });

  it('switch: clicking the label text flips it too', async () => {
    q('[data-testid="boot"] .kl-switch-label').click();
    await render();
    expect(host.boot()).toBe(true);
  });

  it('switch: follows the model it is given and does nothing while disabled', async () => {
    host.boot.set(true);
    host.locked.set(true);
    await render();
    const sw = q<HTMLButtonElement>('[data-testid="boot"] [role="switch"]');
    expect(sw.getAttribute('aria-checked')).toBe('true');
    expect(sw.disabled).toBe(true);
    q('[data-testid="boot"] .kl-switch-label').click();
    await render();
    expect(host.boot()).toBe(true);
  });

  it('switch: works without forms through [(checked)], named by ariaLabel', async () => {
    const sw = q<HTMLButtonElement>('[data-testid="named"] [role="switch"]');
    expect(sw.getAttribute('aria-label')).toBe('Sensitive logging');
    expect(sw.getAttribute('aria-checked')).toBe('true');
    sw.click();
    await render();
    expect(host.plain()).toBe(false);
  });

});
