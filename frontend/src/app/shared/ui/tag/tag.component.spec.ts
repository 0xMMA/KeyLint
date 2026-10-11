import { describe, it, expect, beforeEach } from 'vitest';
import { Component, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { TagComponent } from './tag.component';

@Component({
  standalone: true,
  imports: [TagComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-tag data-testid="in-use" severity="primary" icon="check">In use</kl-tag>
    <kl-tag data-testid="value" value="● key set" severity="success" />
    <kl-tag data-testid="plain">Haiku 5.5</kl-tag>
  `,
})
class Host {}

describe('kl-tag', () => {
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

  it('tag: text, severity and optional icon', () => {
    expect(q('in-use').textContent?.trim()).toBe('In use');
    expect(q('in-use').getAttribute('data-severity')).toBe('primary');
    expect(q('in-use').querySelector('kl-icon[data-icon="check"]')).toBeTruthy();
    expect(q('value').textContent?.trim()).toBe('● key set');
    expect(q('plain').getAttribute('data-severity')).toBe('neutral');
  });

});
