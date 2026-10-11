import { Directive, booleanAttribute, input } from '@angular/core';

/**
 * KeyLint's text field look for a native <input> or <textarea>:
 *
 *   <input klInput data-testid="api-key" [(ngModel)]="key" />
 *   <textarea klInput rows="3"></textarea>
 *
 * Only a class: the styles are global (src/ui.css, `.kl-input`), in
 * @layer components, so a Tailwind utility on the same element (w-full, mt-2)
 * still wins. The element stays native — labels, ngModel, autofill and
 * spell-check behave as the browser makes them.
 */
@Directive({
  selector: 'input[klInput], textarea[klInput]',
  standalone: true,
  host: {
    class: 'kl-input',
    '[class.kl-input-invalid]': 'invalid()',
    '[attr.aria-invalid]': 'invalid() ? "true" : null',
  },
})
export class InputDirective {
  /** Marks the field as wrong: red border, aria-invalid. */
  readonly invalid = input(false, { transform: booleanAttribute });
}
