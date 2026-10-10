import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { InputDirective } from './input.directive';

@Component({
  standalone: true,
  imports: [InputDirective],
  template: `<input data-testid="field" klInput [invalid]="bad()" /><textarea data-testid="area" klInput></textarea>`,
})
class Host {
  readonly bad = signal(false);
}

describe('klInput', () => {
  let fixture: ComponentFixture<Host>;
  let host: Host;
  let el: HTMLElement;
  const q = <T extends HTMLElement = HTMLElement>(sel: string) => el.querySelector<T>(sel)!;

  async function render(): Promise<void> {
    fixture.detectChanges();
    await fixture.whenStable();
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    host = fixture.componentInstance;
    el = fixture.nativeElement;
    await render();
  });

  it('puts the field class on inputs and textareas', () => {
    expect(q('[data-testid="area"]').classList).toContain('kl-input');
  });

  it('the field class, and aria-invalid when marked wrong', async () => {
    const input = q<HTMLInputElement>('[data-testid="field"]');
    expect(input.classList).toContain('kl-input');
    expect(input.getAttribute('aria-invalid')).toBeNull();
    host.bad.set(true);
    await render();
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(input.classList).toContain('kl-input-invalid');
  });
});
