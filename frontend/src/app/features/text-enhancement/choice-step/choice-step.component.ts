import { Component, ElementRef, afterNextRender, computed, inject, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterLink } from '@angular/router';
import { ButtonComponent, CheckboxComponent, IconComponent, InputDirective, MessageComponent } from '../../../shared/ui';
import { FORMS, formLabel, DEFAULT_FORM, type PyramidizeForm } from '../forms';

/**
 * The Form step (mockup "Pyramidize v3 flow", step 2): what should the text
 * become? Five large buttons in a radio group, keys 1–5, Enter runs. The
 * choice is never skipped; the page preselects email or the app's remembered
 * form.
 *
 * Text that came from an app gets an unticked "Remember <form> for <app>"
 * box; the page writes the preset when the user runs with it ticked. When the
 * picked form already is the app's remembered one, a note with Forget takes
 * its place. Pasted text gets neither.
 *
 * One visible field: the user's own instructions, which go to the prompt as
 * they are typed. No tone, style or audience control (roadmap E3, 2026-10-12).
 *
 * The keys listen on the document only while this component exists — the
 * page shows it on the Form step and nowhere else — and never take a digit
 * typed into a text field.
 */
@Component({
  selector: 'app-choice-step',
  standalone: true,
  imports: [FormsModule, RouterLink, ButtonComponent, CheckboxComponent, IconComponent, InputDirective, MessageComponent],
  host: {
    class: 'pz-choice',
    '(document:keydown)': 'onDocumentKeydown($event)',
  },
  template: `
    <section class="flex flex-col gap-[22px]" aria-labelledby="pz-h-form">
      <div>
        <h1 id="pz-h-form" class="pz-heading">What should it become?</h1>
        <p class="pz-sub">
          <span data-testid="text-excerpt">“{{ excerpt() }}”</span> ·
          <button type="button" class="pz-link" data-testid="edit-text-btn" (click)="editText.emit()">Edit text</button>
        </p>
      </div>

      <div class="pz-forms" role="radiogroup" aria-label="Form" (keydown)="onGroupKeydown($event)">
        @for (form of forms; track form.value; let i = $index) {
          <button
            type="button"
            role="radio"
            class="pz-form"
            [attr.data-testid]="'form-' + form.value"
            [attr.data-form]="form.value"
            [attr.aria-checked]="form.value === docType()"
            [tabIndex]="form.value === docType() ? 0 : -1"
            (click)="pick(form)"
          >
            <kbd class="pz-form-key">{{ i + 1 }}</kbd>
            <kl-icon class="pz-form-icon" [name]="form.icon" [size]="32" [strokeWidth]="1.7" />
            <span class="pz-form-label">{{ form.label }}</span>
            <span class="pz-form-desc">{{ form.description }}</span>
            @if (tagFor(form); as tag) {
              <span class="pz-form-tag">{{ tag }}</span>
            }
          </button>
        }
      </div>

      @if (app()) {
        @if (pickedIsRemembered()) {
          <p class="pz-remember text-muted" data-testid="remembered-note">
            {{ pickedLabel() }} is remembered for {{ app() }}.
            <button type="button" class="pz-link" data-testid="forget-preset-btn" (click)="forget.emit()">Forget</button>
          </p>
        } @else {
          <kl-checkbox class="pz-remember" data-testid="remember-checkbox" [(checked)]="remember">
            Remember {{ pickedLabel() }} for {{ app() }}
          </kl-checkbox>
        }
      }

      <div class="flex max-w-[720px] min-w-0 flex-col gap-2">
        <label for="pz-instructions" class="pz-field-label">Instructions <span class="pz-optional">(optional)</span></label>
        <input
          klInput
          id="pz-instructions"
          data-testid="custom-instructions"
          autocomplete="off"
          placeholder="e.g. more formal · in English · for the whole department"
          [(ngModel)]="instructions"
          (keydown.enter)="onInstructionsEnter($event)"
        />
      </div>

      <ng-content />

      <div class="pz-footer">
        <span class="text-[13px] text-muted" data-testid="run-settings">
          @if (runOn()) { {{ runOn() }} · }
          <a routerLink="/settings" [queryParams]="{ tab: 'providers' }" data-testid="change-in-settings">Change in Settings</a>
        </span>
        <kl-button data-testid="pyramidize-btn" variant="primary" size="lg" icon="pyramid"
          [disabled]="!canRun()" (click)="runIfPossible()"
        >Pyramidize as {{ pickedLabel().toLowerCase() }}<span klButtonEnd><kbd>Enter</kbd></span></kl-button>
      </div>
    </section>
  `,
  styles: [`
    :host { display: block; }
    .pz-heading { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .pz-sub { margin: 6px 0 0; max-width: 640px; font-size: 13px; color: var(--kl-muted); overflow-wrap: anywhere; }
    .pz-link {
      font: inherit; padding: 0; border: 0; background: none; color: var(--kl-link);
      cursor: pointer; text-decoration: underline; text-underline-offset: 2px;
    }
    .pz-link:hover { color: var(--kl-primary-text); }
    .pz-link:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; border-radius: 2px; }

    .pz-forms { display: grid; grid-template-columns: repeat(auto-fit, minmax(150px, 1fr)); gap: 14px; }
    .pz-form {
      position: relative; display: flex; flex-direction: column; gap: 10px; min-height: 168px;
      padding: 18px 18px 16px; text-align: left; color: var(--kl-text); cursor: pointer;
      background: var(--kl-sheet); border: 2px solid var(--kl-line-control); border-radius: var(--kl-radius-sheet);
      transition: background-color 0.12s, border-color 0.12s;
    }
    .pz-form:hover { background: var(--kl-raised-hover); }
    .pz-form[aria-checked='true'] {
      border-color: var(--kl-primary); background: color-mix(in srgb, var(--kl-primary), var(--kl-sheet) 88%);
    }
    .pz-form:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .pz-form-key { position: absolute; top: 14px; right: 14px; }
    .pz-form-icon { color: var(--kl-muted); }
    .pz-form[aria-checked='true'] .pz-form-icon { color: var(--kl-link); }
    .pz-form-label { font-size: 18px; font-weight: 700; color: var(--kl-text-strong); }
    .pz-form-desc { font-size: 13px; line-height: 1.45; color: var(--kl-muted); }
    .pz-form-tag {
      margin-top: auto; align-self: flex-start; padding: 2px 8px; border-radius: 999px;
      font-size: 11px; font-weight: 600; background: var(--kl-selected); color: var(--kl-text-soft);
    }

    .pz-remember { align-self: flex-start; min-height: 44px; display: flex; align-items: center; gap: 8px; margin: 0; }
    .pz-field-label { font-size: 13px; font-weight: 500; color: var(--kl-text-soft); }
    .pz-optional { font-weight: 400; color: var(--kl-muted); }
    .pz-footer {
      display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;
      padding-top: 16px; border-top: 1px solid var(--kl-line);
    }
  `],
})
export class ChoiceStepComponent {
  /** The first line of the text, already shortened by the page. */
  readonly excerpt = input('');
  /** The app the text came from; '' for text pasted or typed into KeyLint. */
  readonly app = input('');
  /** The form remembered for `app`, or null when none is. */
  readonly rememberedDocType = input<string | null>(null);
  /** "<model> · <provider> · effort <level>", from Settings. */
  readonly runOn = input('');
  readonly canRun = input(true);

  readonly docType = model(DEFAULT_FORM);
  /** "Remember <form> for <app>": unticked until the user ticks it, again after every other pick. */
  readonly remember = model(false);
  readonly instructions = model('');

  readonly run = output<void>();
  readonly editText = output<void>();
  readonly forget = output<void>();

  protected readonly forms = FORMS;
  protected readonly pickedLabel = computed(() => formLabel(this.docType()));
  protected readonly pickedIsRemembered = computed(() => this.rememberedDocType() === this.docType());

  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  constructor() {
    // Where the keyboard starts on this step: the form that is checked.
    afterNextRender(() => this.focusCard(this.docType()));
  }

  /** Enter in the field runs — unless an IME is composing, whose Enter commits the text. */
  protected onInstructionsEnter(event: Event): void {
    const key = event as KeyboardEvent;
    if (key.isComposing || key.keyCode === 229) return;
    key.preventDefault();
    this.runIfPossible();
  }

  /** The small tag on the card the page preselected from the app: never for pasted text. */
  protected tagFor(form: PyramidizeForm): string | null {
    if (!this.app()) return null;
    const remembered = this.rememberedDocType();
    if (remembered) return form.value === remembered ? `Remembered for ${this.app()}` : null;
    return form.value === DEFAULT_FORM ? 'Default' : null;
  }

  protected pick(form: PyramidizeForm): void {
    if (form.value !== this.docType()) this.remember.set(false);
    this.docType.set(form.value);
  }

  protected runIfPossible(): void {
    if (this.canRun()) this.run.emit();
  }

  /** The radio group's own keys: arrows, Home and End move the choice and the focus with it. */
  protected onGroupKeydown(event: KeyboardEvent): void {
    const index = FORMS.findIndex(f => f.value === this.docType());
    const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
    let target: number;
    if (step) target = (index + step + FORMS.length) % FORMS.length;
    else if (event.key === 'Home') target = 0;
    else if (event.key === 'End') target = FORMS.length - 1;
    else return;
    event.preventDefault();
    const next = FORMS[target];
    this.pick(next);
    this.focusCard(next.value);
  }

  /**
   * 1–5 pick a form and Enter runs, from anywhere on the page while this step
   * is shown. A digit typed into a text field stays text; Enter on a button
   * or a link stays that button's (Edit text, Forget, a nav link), and the
   * Remember box takes none; on a form card it picks the card and runs.
   */
  protected onDocumentKeydown(event: KeyboardEvent): void {
    if (event.defaultPrevented || event.isComposing || event.ctrlKey || event.metaKey || event.altKey) return;
    const target = event.target instanceof Element ? event.target : null;
    if (/^[1-5]$/.test(event.key)) {
      if (isTextEntry(target)) return;
      event.preventDefault();
      const form = FORMS[Number(event.key) - 1];
      this.pick(form);
      // With the focus: Enter on the focused card runs that card.
      if (!target?.closest('input[type="checkbox"]')) this.focusCard(form.value);
      return;
    }
    if (event.key !== 'Enter' || isTextEntry(target)) return;
    // The Remember box: Enter there is no request to run.
    if (target?.closest('input[type="checkbox"], kl-checkbox')) return;
    const card = target?.closest<HTMLElement>('[role="radio"][data-form]');
    if (card && this.host.nativeElement.contains(card)) {
      event.preventDefault();
      const form = FORMS.find(f => f.value === card.dataset['form']);
      if (form) this.pick(form);
      this.runIfPossible();
      return;
    }
    if (target?.closest('button, a, [role="button"], [role="link"]')) return;
    event.preventDefault();
    this.runIfPossible();
  }

  private focusCard(value: string): void {
    this.host.nativeElement.querySelector<HTMLElement>(`[data-form="${value}"]`)?.focus();
  }
}

/** Whether keys typed at `el` are text: a text input, a textarea, a select or an editable element. */
function isTextEntry(el: Element | null): boolean {
  if (!el) return false;
  if (el instanceof HTMLInputElement) {
    return !['checkbox', 'radio', 'button', 'submit', 'reset', 'range', 'color', 'file'].includes(el.type);
  }
  return el instanceof HTMLTextAreaElement || el instanceof HTMLSelectElement || (el as HTMLElement).isContentEditable;
}
