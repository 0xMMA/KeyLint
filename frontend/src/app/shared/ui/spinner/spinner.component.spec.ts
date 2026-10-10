import { describe, it, expect, beforeEach } from 'vitest';
import { Component } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { SpinnerComponent } from './spinner.component';

@Component({
  standalone: true,
  imports: [SpinnerComponent],
  template: `
    <kl-spinner data-testid="spin" />
    <kl-spinner data-testid="spin-named" label="Loading models" [size]="24" />
  `,
})
class Host {}

describe('kl-spinner', () => {
  let el: HTMLElement;
  const q = (id: string) => el.querySelector<HTMLElement>(`[data-testid="${id}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    const fixture: ComponentFixture<Host> = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  it('is hidden from assistive tech unless it is the only sign of a wait', () => {
    expect(q('spin').getAttribute('aria-hidden')).toBe('true');
    expect(q('spin').querySelector('svg')).toBeTruthy();
    expect(q('spin-named').getAttribute('role')).toBe('status');
    expect(q('spin-named').getAttribute('aria-label')).toBe('Loading models');
    expect(q('spin-named').style.width).toBe('24px');
  });
});
