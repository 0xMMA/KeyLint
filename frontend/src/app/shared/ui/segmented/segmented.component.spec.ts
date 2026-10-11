import { describe, it, expect, beforeEach } from 'vitest';
import { Component, signal } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { SegmentedComponent } from './segmented.component';

@Component({
  standalone: true,
  imports: [SegmentedComponent],
  template: `<kl-segmented data-testid="view" ariaLabel="Result view" [options]="views" [(value)]="view" />`,
})
class Host {
  readonly views = [{ value: 'changes', label: 'Show changes' }, { value: 'clean', label: 'Clean text' }];
  readonly view = signal('changes');
}

describe('kl-segmented', () => {
  let fixture: ComponentFixture<Host>;
  let el: HTMLElement;
  const segment = (v: string) => el.querySelector<HTMLButtonElement>(`[data-testid="segment-${v}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    fixture = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  it('is a labelled group of toggle buttons with the current one pressed', () => {
    const group = el.querySelector('[data-testid="view"]')!;
    expect(group.getAttribute('role')).toBe('group');
    expect(group.getAttribute('aria-label')).toBe('Result view');
    expect(segment('changes').getAttribute('aria-pressed')).toBe('true');
    expect(segment('clean').getAttribute('aria-pressed')).toBe('false');
  });

  it('switches on click and writes the value back', async () => {
    segment('clean').click();
    fixture.detectChanges();
    await fixture.whenStable();
    expect(fixture.componentInstance.view()).toBe('clean');
    expect(segment('clean').getAttribute('aria-pressed')).toBe('true');
    expect(segment('changes').getAttribute('aria-pressed')).toBe('false');
  });
});
