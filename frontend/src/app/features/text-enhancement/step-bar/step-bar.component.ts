import { Component, input, output } from '@angular/core';
import { IconComponent } from '../../../shared/ui';

/** Pyramidize's three steps: the text, the form choice and the result. */
export type PyramidizeStep = 'intake' | 'choice' | 'result';

const STEPS: ReadonlyArray<{ id: PyramidizeStep; label: string }> = [
  { id: 'intake', label: 'Text' },
  { id: 'choice', label: 'Form' },
  { id: 'result', label: 'Result' },
];

/**
 * The small step bar over Pyramidize (Text → Form → Result). Every step
 * reached is a button back to it; going back keeps what was there. On the
 * right, where the text came from.
 */
@Component({
  selector: 'app-step-bar',
  standalone: true,
  imports: [IconComponent],
  host: { class: 'pz-step-bar' },
  template: `
    <ol aria-label="Steps" class="pz-steps">
      @for (step of steps; track step.id; let i = $index; let last = $last) {
        <li class="flex items-center gap-1.5">
          <button
            type="button"
            class="pz-step"
            [attr.data-testid]="'step-' + step.id"
            [attr.aria-current]="step.id === current() ? 'step' : null"
            [disabled]="disabled() || !reachable(step.id)"
            (click)="go.emit(step.id)"
          >
            <span class="pz-step-dot">{{ i + 1 }}</span>
            <span>{{ step.label }}</span>
          </button>
          @if (!last) {
            <kl-icon name="chevron-right" [size]="14" class="text-faint" />
          }
        </li>
      }
    </ol>
    @if (source()) {
      <span class="pz-source" data-testid="source-chip">
        <kl-icon name="corner-down-left" [size]="13" class="text-muted" />{{ source() }}
      </span>
    }
  `,
  styles: [`
    :host { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
    .pz-steps { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; margin: 0; padding: 0; list-style: none; }
    .pz-step {
      display: flex; align-items: center; gap: 8px; padding: 6px 10px; border: 0; border-radius: var(--kl-radius-md);
      font: inherit; font-size: 14px; color: var(--kl-text-soft); background: transparent; cursor: pointer;
    }
    .pz-step:not(:disabled):hover { background: var(--kl-raised); }
    .pz-step:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .pz-step:disabled { color: var(--kl-faint); cursor: default; }
    .pz-step[aria-current='step'] { color: var(--kl-primary-text); font-weight: 600; }
    .pz-step-dot {
      display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px;
      border-radius: 999px; font-size: 11px; font-weight: 700; background: var(--kl-selected); color: var(--kl-text);
    }
    .pz-step:disabled .pz-step-dot { background: var(--kl-sheet); color: var(--kl-faint); box-shadow: inset 0 0 0 1px var(--kl-line-control); }
    .pz-step[aria-current='step'] .pz-step-dot { background: var(--kl-primary); color: var(--kl-on-primary); box-shadow: none; }
    .pz-source {
      display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 999px; font-size: 12px;
      color: var(--kl-text-soft); background: var(--kl-sheet); border: 1px solid var(--kl-line-control);
      max-width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
    }
  `],
})
export class StepBarComponent {
  readonly current = input.required<PyramidizeStep>();
  /** There is text, so the Form step can be reached. */
  readonly canChoose = input(false);
  /** There is a result to go back to. */
  readonly canSeeResult = input(false);
  /** A step is running: nothing can be clicked. */
  readonly disabled = input(false);
  /** "From Outlook" / "Pasted into KeyLint"; '' shows nothing. */
  readonly source = input('');

  readonly go = output<PyramidizeStep>();

  protected readonly steps = STEPS;

  protected reachable(step: PyramidizeStep): boolean {
    return step === 'intake' || (step === 'choice' ? this.canChoose() : this.canSeeResult());
  }
}
