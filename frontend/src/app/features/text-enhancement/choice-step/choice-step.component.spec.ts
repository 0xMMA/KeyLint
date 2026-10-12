import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { ChoiceStepComponent } from './choice-step.component';

describe('ChoiceStepComponent (the Form step)', () => {
  let fixture: ComponentFixture<ChoiceStepComponent>;
  let el: HTMLElement;
  let runs: number;
  let edits: number;
  let forgets: number;

  async function render(inputs: Record<string, unknown> = {}): Promise<void> {
    for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
    fixture.detectChanges();
    await fixture.whenStable();
  }

  const q = <T extends Element = HTMLElement>(testid: string) => el.querySelector<T>(`[data-testid="${testid}"]`);
  const radios = () => Array.from(el.querySelectorAll<HTMLButtonElement>('[role="radio"]'));
  const checked = () => radios().find(r => r.getAttribute('aria-checked') === 'true')?.dataset['form'];

  function press(key: string, target: EventTarget = document.body, init: KeyboardEventInit = {}): KeyboardEvent {
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
    target.dispatchEvent(event);
    fixture.detectChanges();
    return event;
  }

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [ChoiceStepComponent],
      providers: [provideRouter([])],
    }).compileComponents();
    fixture = TestBed.createComponent(ChoiceStepComponent);
    el = fixture.nativeElement;
    document.body.appendChild(el);
    runs = edits = forgets = 0;
    fixture.componentInstance.run.subscribe(() => runs++);
    fixture.componentInstance.editText.subscribe(() => edits++);
    fixture.componentInstance.forget.subscribe(() => forgets++);
    await render({ excerpt: 'Hi Tom, kurzes Update zum Deployment', runOn: 'Sonnet (latest) · Claude Code (installed CLI) · effort high' });
  });

  afterEach(() => {
    fixture.destroy();
    el.remove();
  });

  it('asks what the text should become, with an excerpt and a way back to the text', () => {
    expect(el.querySelector('h1')?.textContent).toContain('What should it become?');
    expect(q('text-excerpt')?.textContent).toContain('Hi Tom, kurzes Update zum Deployment');
    q<HTMLButtonElement>('edit-text-btn')!.click();
    expect(edits).toBe(1);
  });

  it('offers the five forms as one radio group, each with its key, label and description', () => {
    const group = el.querySelector('[role="radiogroup"]');
    expect(group?.getAttribute('aria-label')).toBe('Form');
    expect(radios().map(r => r.querySelector('.pz-form-label')?.textContent?.trim()))
      .toEqual(['Email', 'Chat', 'Memo', 'Wiki', 'Presentation']);
    expect(radios().map(r => r.querySelector('kbd')?.textContent?.trim())).toEqual(['1', '2', '3', '4', '5']);
    for (const r of radios()) expect(r.querySelector('.pz-form-desc')?.textContent?.trim()).toBeTruthy();
    expect(checked()).toBe('email');
  });

  it('picks a form on click, and names it on the run button', async () => {
    radios()[3].click();
    fixture.detectChanges();
    expect(checked()).toBe('wiki');
    expect(fixture.componentInstance.docType()).toBe('wiki');
    expect(q('pyramidize-btn')?.textContent).toContain('Pyramidize as wiki');
  });

  it('keeps one tab stop in the group and moves the choice with the arrow keys', () => {
    expect(radios().map(r => r.tabIndex)).toEqual([0, -1, -1, -1, -1]);
    press('ArrowRight', radios()[0]);
    expect(checked()).toBe('chat');
    expect(document.activeElement).toBe(radios()[1]);
    press('ArrowLeft', radios()[1]);
    press('ArrowLeft', radios()[0]);
    expect(checked()).toBe('powerpoint');
  });

  it('picks a form with 1–5 and runs with Enter', () => {
    press('3');
    expect(checked()).toBe('memo');
    press('5');
    expect(checked()).toBe('powerpoint');
    press('Enter');
    expect(runs).toBe(1);
  });

  it('runs with Enter on a focused form card, picking that card', () => {
    radios()[1].focus();
    const event = press('Enter', radios()[1]);
    expect(checked()).toBe('chat');
    expect(runs).toBe(1);
    expect(event.defaultPrevented).toBe(true);
  });

  it('leaves Enter on another button to that button', () => {
    press('Enter', q('edit-text-btn')!.querySelector('button') ?? q('edit-text-btn')!);
    expect(runs).toBe(0);
  });

  it('takes digits typed into the instructions as text, and runs on Enter there', () => {
    const field = q<HTMLInputElement>('custom-instructions')!;
    const digit = press('2', field);
    expect(checked()).toBe('email');
    expect(digit.defaultPrevented).toBe(false);
    press('Enter', field);
    expect(runs).toBe(1);
  });

  it('ignores keys with a modifier', () => {
    press('2', document.body, { ctrlKey: true });
    press('Enter', document.body, { altKey: true });
    expect(checked()).toBe('email');
    expect(runs).toBe(0);
  });

  it('does not run on Enter while running is not possible', async () => {
    await render({ canRun: false });
    press('Enter');
    q<HTMLInputElement>('custom-instructions')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    expect(runs).toBe(0);
    expect(q('pyramidize-btn')?.querySelector('button')?.disabled).toBe(true);
  });

  it('stops listening once it is gone, so no other page gets the keys', () => {
    fixture.destroy();
    press('2');
    press('Enter');
    expect(runs).toBe(0);
  });

  it('has one visible field, Instructions (optional), with examples as its placeholder', () => {
    const field = q<HTMLInputElement>('custom-instructions')!;
    const label = el.querySelector(`label[for="${field.id}"]`);
    expect(label?.textContent?.replace(/\s+/g, ' ').trim()).toBe('Instructions (optional)');
    expect(field.placeholder).toBe('e.g. more formal · in English · for the whole department');
    expect(el.querySelectorAll('input:not([type="checkbox"]), textarea, kl-select').length).toBe(1);
  });

  it('says what it runs on, with a link to Settings', () => {
    expect(q('run-settings')?.textContent).toContain('Sonnet (latest) · Claude Code (installed CLI) · effort high');
    const link = q<HTMLAnchorElement>('change-in-settings')!;
    expect(link.textContent?.trim()).toBe('Change in Settings');
    expect(link.getAttribute('href')).toBe('/settings?tab=providers');
  });

  it('explains a provider that cannot run', async () => {
    await render({ unavailableProvider: 'AWS Bedrock', canRun: false });
    expect(q('provider-unavailable')?.textContent).toContain('AWS Bedrock is not available yet');
  });

  describe('text pasted into KeyLint', () => {
    it('offers nothing to remember, and tags no card', () => {
      expect(q('remember-checkbox')).toBeNull();
      expect(q('remembered-note')).toBeNull();
      expect(el.querySelector('.pz-form-tag')).toBeNull();
    });
  });

  describe('text from an app', () => {
    beforeEach(async () => render({ app: 'Outlook' }));

    it('tags the default on email and offers an unticked box to remember the form', () => {
      expect(radios()[0].querySelector('.pz-form-tag')?.textContent?.trim()).toBe('Default');
      expect(el.querySelectorAll('.pz-form-tag').length).toBe(1);
      const box = q('remember-checkbox')!;
      expect(box.textContent?.trim()).toBe('Remember Email for Outlook');
      expect(box.querySelector<HTMLInputElement>('input')!.checked).toBe(false);
    });

    it('names the picked form on the box, and unticks it when the form changes', () => {
      const input = q('remember-checkbox')!.querySelector<HTMLInputElement>('input')!;
      input.click();
      fixture.detectChanges();
      expect(fixture.componentInstance.remember()).toBe(true);
      press('3');
      expect(q('remember-checkbox')!.textContent?.trim()).toBe('Remember Memo for Outlook');
      expect(fixture.componentInstance.remember()).toBe(false);
    });
  });

  describe('text from an app with a remembered form', () => {
    beforeEach(async () => render({ app: 'Teams', rememberedDocType: 'chat', docType: 'chat' }));

    it('tags the remembered card and says it is remembered, with Forget instead of the box', () => {
      expect(radios()[1].querySelector('.pz-form-tag')?.textContent?.trim()).toBe('Remembered for Teams');
      expect(q('remember-checkbox')).toBeNull();
      expect(q('remembered-note')?.textContent).toContain('Chat is remembered for Teams.');
      q<HTMLButtonElement>('forget-preset-btn')!.click();
      expect(forgets).toBe(1);
    });

    it('offers the box again for another form', () => {
      press('1');
      expect(q('remembered-note')).toBeNull();
      expect(q('remember-checkbox')?.textContent?.trim()).toBe('Remember Email for Teams');
      // The tag stays on what is remembered, not on what is picked.
      expect(radios()[1].querySelector('.pz-form-tag')?.textContent?.trim()).toBe('Remembered for Teams');
    });
  });
});
