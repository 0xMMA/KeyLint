import { ChangeDetectionStrategy, Component, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent, InputDirective } from '../../../shared/ui';

/**
 * The small bar that appears over a selection in the editor: an instruction
 * for just that passage, and Apply. The page positions it (fixed, at the
 * pointer) through [style.top.px] / [style.left.px] on the host.
 *
 * It sits on the raised overlay surface. Its old --p-surface-overlay
 * background was a PrimeNG token Aura never defined, so it used to float
 * transparent over the text.
 */
@Component({
  selector: 'app-selection-bubble',
  standalone: true,
  imports: [FormsModule, ButtonComponent, InputDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'pz-bubble',
    role: 'group',
    'aria-label': 'Change the selected text',
    'data-testid': 'selection-bubble',
    '(keydown.escape)': '$event.stopPropagation(); closed.emit()',
  },
  template: `
    <input
      klInput
      data-testid="selection-instruction-input"
      aria-label="Instruction for the selected text"
      placeholder="Ask AI…"
      class="pz-bubble-input"
      [(ngModel)]="instruction"
      (keydown.enter)="apply.emit()"
    />
    <kl-button data-testid="selection-apply-btn" variant="primary" size="sm" icon="sparkles"
      [disabled]="!instruction().trim() || disabled()" (click)="apply.emit()">Apply</kl-button>
    <kl-button variant="ghost" size="sm" icon="x" iconOnly ariaLabel="Close" (click)="closed.emit()" />
  `,
  styles: [`
    :host {
      position: fixed; z-index: 1000; display: flex; align-items: center; gap: 6px; padding: 6px;
      background: var(--kl-raised); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-lg);
      box-shadow: var(--kl-shadow-overlay);
    }
    .pz-bubble-input { width: 200px; min-height: 32px; padding: 5px 10px; font-size: 13px; }
  `],
})
export class SelectionBubbleComponent {
  readonly instruction = model('');
  /** Apply is off (no provider can run); the instruction can still be typed. */
  readonly disabled = input(false);

  readonly apply = output<void>();
  readonly closed = output<void>();
}
