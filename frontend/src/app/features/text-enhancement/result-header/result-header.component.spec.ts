import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { ResultHeaderComponent } from './result-header.component';

describe('ResultHeaderComponent', () => {
  let fixture: ComponentFixture<ResultHeaderComponent>;
  let el: HTMLElement;
  let picked: number[];
  let backs: number;

  async function render(inputs: Record<string, unknown>): Promise<void> {
    for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
    fixture.detectChanges();
    await fixture.whenStable();
  }
  const chips = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[data-testid="result-version"]'));

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [ResultHeaderComponent] }).compileComponents();
    fixture = TestBed.createComponent(ResultHeaderComponent);
    el = fixture.nativeElement;
    picked = [];
    backs = 0;
    fixture.componentInstance.picked.subscribe(i => picked.push(i));
    fixture.componentInstance.back.subscribe(() => backs++);
    await render({ forms: ['email'], active: 0 });
  });

  it('titles the result with its form, and shows no versions for one result', () => {
    expect(el.querySelector('h1')?.textContent?.trim()).toBe('Email');
    expect(chips().length).toBe(0);
  });

  it('shows a chip per result next to the title, the shown one pressed, numbering repeats', async () => {
    await render({ forms: ['email', 'chat', 'email'], active: 1 });
    expect(el.querySelector('h1')?.textContent?.trim()).toBe('Chat');
    expect(el.querySelector('[role="group"]')?.getAttribute('aria-label')).toBe('Versions');
    expect(chips().map(c => c.textContent?.trim())).toEqual(['Email', 'Chat', 'Email 2']);
    expect(chips().map(c => c.getAttribute('aria-pressed'))).toEqual(['false', 'true', 'false']);
    chips()[2].click();
    expect(picked).toEqual([2]);
  });

  it('goes back to the form choice', () => {
    el.querySelector<HTMLButtonElement>('[data-testid="try-another-form-btn"] button')!.click();
    expect(backs).toBe(1);
  });
});
