import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { IntakeStepComponent } from './intake-step.component';

describe('IntakeStepComponent (the Text step)', () => {
  let fixture: ComponentFixture<IntakeStepComponent>;
  let el: HTMLElement;
  let nexts: number;
  let pastes: number;

  async function render(inputs: Record<string, unknown>): Promise<void> {
    for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
    fixture.detectChanges();
    await fixture.whenStable();
  }
  const q = <T extends Element = HTMLElement>(testid: string) => el.querySelector<T>(`[data-testid="${testid}"]`);
  function press(key: string, target: EventTarget, init: KeyboardEventInit = {}): KeyboardEvent {
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
    target.dispatchEvent(event);
    return event;
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({ imports: [IntakeStepComponent] }).compileComponents();
    fixture = TestBed.createComponent(IntakeStepComponent);
    el = fixture.nativeElement;
    document.body.appendChild(el);
    nexts = pastes = 0;
    fixture.componentInstance.next.subscribe(() => nexts++);
    fixture.componentInstance.paste.subscribe(() => pastes++);
    await render({ text: '' });
  });

  afterEach(() => {
    fixture.destroy();
    el.remove();
  });

  it('takes the text typed into it, and goes on only once there is some', async () => {
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    const go = q('choose-form-btn')!.querySelector('button')!;
    expect(go.disabled).toBe(true);
    area.value = 'Hi Tom';
    area.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await fixture.whenStable();
    expect(fixture.componentInstance.text()).toBe('Hi Tom');
    expect(go.disabled).toBe(false);
    go.click();
    expect(nexts).toBe(1);
  });

  it('offers to paste from the clipboard', () => {
    q('paste-from-clipboard-btn')!.querySelector('button')!.click();
    expect(pastes).toBe(1);
  });

  it('keeps Enter in the text a new line, and goes on with Ctrl+Enter there', async () => {
    await render({ text: 'Hi Tom' });
    const area = q<HTMLTextAreaElement>('original-textarea')!;
    expect(press('Enter', area).defaultPrevented).toBe(false);
    expect(nexts).toBe(0);
    press('Enter', area, { ctrlKey: true });
    expect(nexts).toBe(1);
  });

  it('goes on with Enter outside the text, and not while there is none', async () => {
    press('Enter', document.body);
    expect(nexts).toBe(0);
    await render({ text: 'Hi Tom' });
    press('Enter', document.body);
    expect(nexts).toBe(1);
    // A digit here is nobody's: the form keys belong to the Form step.
    press('2', document.body);
    expect(nexts).toBe(1);
  });

  it('stops listening once it is gone', async () => {
    await render({ text: 'Hi Tom' });
    fixture.destroy();
    press('Enter', document.body);
    expect(nexts).toBe(0);
  });
});
