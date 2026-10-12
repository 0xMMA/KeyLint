import { Component, ElementRef, afterNextRender, computed, inject, model, output, viewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent } from '../../../shared/ui';

/**
 * The Text step (mockup "Pyramidize v3 flow", step 1): the text to
 * pyramidize, typed or pasted here. The Pyramidize shortcut brings its text
 * along and skips this step. Mail history in the text is shown as it is: the
 * prompt reads it as context.
 *
 * Enter goes on from anywhere outside the text; in the text it is a new line,
 * and Ctrl+Enter goes on.
 */
@Component({
  selector: 'app-intake-step',
  standalone: true,
  imports: [FormsModule, ButtonComponent],
  host: {
    class: 'pz-intake',
    '(document:keydown)': 'onDocumentKeydown($event)',
  },
  template: `
    <section class="flex min-h-0 flex-1 flex-col gap-4" aria-labelledby="pz-h-text">
      <div>
        <h1 id="pz-h-text" class="pz-heading">Your text</h1>
        <p class="pz-sub">Pasted here, or brought along by the Pyramidize shortcut. The shortcut skips this step and opens the form choice.</p>
      </div>
      <textarea
        #textArea
        data-testid="original-textarea"
        class="pz-sheet"
        aria-label="Your text"
        placeholder="Paste or type the text you want to pyramidize."
        [(ngModel)]="text"
        (keydown.control.enter)="$event.preventDefault(); goOn()"
      ></textarea>
      <div class="flex flex-wrap items-center gap-2.5">
        <kl-button data-testid="paste-from-clipboard-btn" size="lg" icon="clipboard" (click)="paste.emit()">Paste from clipboard</kl-button>
        <kl-button data-testid="choose-form-btn" variant="primary" size="lg" [disabled]="!canGoOn()" (click)="goOn()"
        >Choose the form<span klButtonEnd><kbd>Enter</kbd></span></kl-button>
      </div>
    </section>
  `,
  styles: [`
    :host { display: flex; flex-direction: column; flex: 1; min-height: 0; }
    .pz-heading { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .pz-sub { margin: 6px 0 0; font-size: 13px; color: var(--kl-muted); }
    /* The user's text, on a sheet (design.md: Literata 15.5/1.7). */
    .pz-sheet {
      flex: 1; min-height: 160px; width: 100%; margin: 0; padding: 22px 26px; resize: none; overflow-y: auto;
      font-family: var(--kl-font-text); font-size: 15.5px; line-height: 1.7; color: var(--kl-text-sheet);
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .pz-sheet::placeholder { color: var(--kl-muted); opacity: 1; font-family: var(--kl-font-ui); font-size: 14px; }
    .pz-sheet:focus { outline: none; border-color: var(--kl-line-control); }
    .pz-sheet:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class IntakeStepComponent {
  readonly text = model('');
  readonly paste = output<void>();
  readonly next = output<void>();

  protected readonly canGoOn = computed(() => !!this.text().trim());

  private readonly textArea = viewChild<ElementRef<HTMLTextAreaElement>>('textArea');
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  constructor() {
    // Ready to type or paste into, as the step opens.
    afterNextRender(() => this.textArea()?.nativeElement.focus());
  }

  protected goOn(): void {
    if (this.canGoOn()) this.next.emit();
  }

  /** Enter outside any field or button goes on, as the button's hint says. */
  protected onDocumentKeydown(event: KeyboardEvent): void {
    if (event.key !== 'Enter' || event.defaultPrevented || event.isComposing
      || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
    const target = event.target instanceof Element ? event.target : null;
    if (target?.closest('input, textarea, select, button, a, [contenteditable="true"], [role="button"]')) return;
    event.preventDefault();
    this.goOn();
  }
}
