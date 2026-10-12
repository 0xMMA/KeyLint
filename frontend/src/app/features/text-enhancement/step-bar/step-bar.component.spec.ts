import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { StepBarComponent, type PyramidizeStep } from './step-bar.component';

describe('StepBarComponent', () => {
  let fixture: ComponentFixture<StepBarComponent>;
  let el: HTMLElement;
  let went: PyramidizeStep[];

  async function render(inputs: Record<string, unknown>): Promise<void> {
    for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
    fixture.detectChanges();
    await fixture.whenStable();
  }
  const step = (id: string) => el.querySelector<HTMLButtonElement>(`[data-testid="step-${id}"]`)!;

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [StepBarComponent] }).compileComponents();
    fixture = TestBed.createComponent(StepBarComponent);
    el = fixture.nativeElement;
    went = [];
    fixture.componentInstance.go.subscribe(s => went.push(s));
    await render({ current: 'intake' });
  });

  it('shows Text, Form and Result in order, marking the current one', async () => {
    await render({ current: 'choice', canChoose: true });
    expect(Array.from(el.querySelectorAll('li')).map(li =>
      Array.from(li.querySelectorAll('.pz-step span')).map(s => s.textContent?.trim()).join(' ')))
      .toEqual(['1 Text', '2 Form', '3 Result']);
    expect(step('choice').getAttribute('aria-current')).toBe('step');
    expect(step('intake').getAttribute('aria-current')).toBeNull();
  });

  it('lets every reached step be clicked, and no other', async () => {
    expect(step('intake').disabled).toBe(false);
    expect(step('choice').disabled).toBe(true);
    expect(step('result').disabled).toBe(true);
    await render({ canChoose: true, canSeeResult: true });
    step('result').click();
    step('choice').click();
    expect(went).toEqual(['result', 'choice']);
  });

  it('holds still while a step runs', async () => {
    await render({ canChoose: true, canSeeResult: true, disabled: true });
    expect([step('intake'), step('choice'), step('result')].every(b => b.disabled)).toBe(true);
  });

  it('says where the text came from', async () => {
    expect(el.querySelector('[data-testid="source-chip"]')).toBeNull();
    await render({ source: 'From Outlook' });
    expect(el.querySelector('[data-testid="source-chip"]')?.textContent?.trim()).toBe('From Outlook');
  });
});
