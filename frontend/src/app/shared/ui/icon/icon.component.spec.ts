import { describe, it, expect, beforeEach } from 'vitest';
import { Component } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { IconComponent } from './icon.component';
import { ICONS } from './icons';

@Component({
  standalone: true,
  imports: [IconComponent],
  template: `
    <kl-icon data-testid="deco" name="copy" />
    <kl-icon data-testid="named" name="check" label="Signed in" [size]="20" />
  `,
})
class Host {}

describe('kl-icon', () => {
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

  it('draws inline SVG in the current colour, decorative unless labelled', () => {
    const svg = q('deco').querySelector('svg')!;
    expect(svg.getAttribute('stroke')).toBe('currentColor');
    expect(svg.querySelector('path, rect')).toBeTruthy();
    expect(q('deco').getAttribute('aria-hidden')).toBe('true');
    expect(q('deco').getAttribute('role')).toBeNull();
    expect(q('named').getAttribute('role')).toBe('img');
    expect(q('named').getAttribute('aria-label')).toBe('Signed in');
    expect(q('named').style.width).toBe('20px');
  });

  it('carries every icon the old primeicons set covered, under its Lucide name', () => {
    for (const name of ['x', 'refresh-cw', 'check', 'sparkles', 'trash', 'download', 'copy', 'circle-arrow-up',
      'wrench', 'loader-circle', 'send', 'rotate-ccw', 'plus', 'play', 'pencil', 'key-round', 'rotate-ccw-clock',
      'file-text', 'settings', 'clipboard', 'chevron-left', 'chevron-right', 'chevron-down', 'zap', 'pyramid']) {
      expect(ICONS[name as keyof typeof ICONS], name).toBeTruthy();
    }
  });
});
