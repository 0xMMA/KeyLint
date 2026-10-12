import { Component, computed, input, output } from '@angular/core';
import { ButtonComponent } from '../../../shared/ui';
import { formLabel } from '../forms';

/**
 * The Result step's heading: the form of the result shown, and — once there
 * are several — a chip per result to switch between them. "Try another form"
 * goes back to the form choice; whatever runs from there is added as a new
 * result, so every earlier one stays here, whole. Projected content
 * (`[resultTools]`) sits before it: the editor's view switch and History.
 */
@Component({
  selector: 'app-result-header',
  standalone: true,
  imports: [ButtonComponent],
  host: { class: 'pz-result-header' },
  template: `
    <div class="flex min-w-0 flex-wrap items-center gap-3">
      <h1 class="pz-heading" data-testid="result-title">{{ title() }}</h1>
      @if (labels().length > 1) {
        <div role="group" aria-label="Versions" class="flex flex-wrap gap-1.5">
          @for (label of labels(); track $index) {
            <button type="button" class="pz-version" data-testid="result-version"
              [attr.aria-pressed]="$index === active()" (click)="picked.emit($index)">{{ label }}</button>
          }
        </div>
      }
    </div>
    <div class="flex flex-wrap items-center justify-end gap-2">
      <ng-content select="[resultTools]" />
      <kl-button data-testid="try-another-form-btn" variant="ghost" size="sm" icon="chevron-left" (click)="back.emit()">Try another form</kl-button>
    </div>
  `,
  styles: [`
    :host { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
    .pz-heading { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .pz-version {
      height: 32px; padding: 0 12px; border-radius: 999px; font: inherit; font-size: 12px; cursor: pointer;
      color: var(--kl-text-soft); background: var(--kl-sheet); border: 1px solid var(--kl-line-control);
    }
    .pz-version:hover { background: var(--kl-raised-hover); }
    .pz-version[aria-pressed='true'] {
      color: var(--kl-primary-text); border-color: var(--kl-primary);
      background: color-mix(in srgb, var(--kl-primary), var(--kl-sheet) 88%);
    }
    .pz-version:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class ResultHeaderComponent {
  /** The form of each result, oldest first ("email", "CHAT", …). */
  readonly forms = input<readonly string[]>([]);
  /** The index of the result shown. */
  readonly active = input(0);

  readonly picked = output<number>();
  readonly back = output<void>();

  /** "Email", "Chat", "Email 2": a repeated form is numbered, so two emails can be told apart. */
  protected readonly labels = computed(() => {
    const seen = new Map<string, number>();
    return this.forms().map(form => {
      const label = formLabel(form);
      const n = (seen.get(label) ?? 0) + 1;
      seen.set(label, n);
      return n > 1 ? `${label} ${n}` : label;
    });
  });

  protected readonly title = computed(() => this.labels()[this.active()]?.replace(/ \d+$/, '') ?? '');
}
