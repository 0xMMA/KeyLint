import { describe, it, expect, beforeEach } from 'vitest';
import { Component, ChangeDetectionStrategy } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { CardComponent } from './card.component';

@Component({
  standalone: true,
  imports: [CardComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <kl-card data-testid="titled" heading="Claude Code" subheading="KeyLint sends your text here.">
      <p data-testid="body">Body</p>
    </kl-card>
    <kl-card data-testid="bare"><p>Only content</p></kl-card>
  `,
})
class Host {}

describe('kl-card', () => {
  let el: HTMLElement;
  const q = (id: string) => el.querySelector<HTMLElement>(`[data-testid="${id}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [Host] }).compileComponents();
    const fixture: ComponentFixture<Host> = TestBed.createComponent(Host);
    el = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
  });

  it('is a region labelled by its heading, with the subheading and content', () => {
    const card = q('titled');
    const h2 = card.querySelector('h2')!;
    expect(h2.textContent).toBe('Claude Code');
    expect(card.getAttribute('role')).toBe('region');
    expect(card.getAttribute('aria-labelledby')).toBe(h2.id);
    expect(card.textContent).toContain('KeyLint sends your text here.');
    expect(q('body')).toBeTruthy();
  });

  it('without a heading it is just a block', () => {
    expect(q('bare').getAttribute('role')).toBeNull();
    expect(q('bare').querySelector('h2')).toBeNull();
  });
});
