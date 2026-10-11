import { ChangeDetectionStrategy, Component, booleanAttribute, forwardRef, input, model, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';
import { IconComponent } from '../icon/icon.component';

let nextId = 0;

/**
 * A checkbox: the native <input type="checkbox">, restyled, with its label.
 *
 *   <kl-checkbox data-testid="auto-copy-checkbox" [(ngModel)]="autoCopy">Copy the result</kl-checkbox>
 *
 * ControlValueAccessor for ngModel / reactive forms, or the `checked` model on
 * its own. The projected text sits in a <label> for the input; `inputId`
 * names the input for an outside <label for> instead.
 */
@Component({
  selector: 'kl-checkbox',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [{ provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => CheckboxComponent), multi: true }],
  host: { class: 'kl-checkbox' },
  template: `
    <span class="kl-checkbox-box">
      <input
        type="checkbox"
        [id]="id()"
        [checked]="checked()"
        [disabled]="isDisabled()"
        [attr.aria-label]="ariaLabel() || null"
        (change)="onInput($event)"
        (blur)="onTouched()"
      />
      <kl-icon class="kl-checkbox-mark" name="check" [size]="12" [strokeWidth]="3" />
    </span>
    <label class="kl-checkbox-label" [for]="id()"><ng-content /></label>
  `,
  styles: [`
    :host { display: inline-flex; align-items: center; gap: 8px; font-size: 14px; }
    .kl-checkbox-box { position: relative; display: inline-flex; flex: none; width: 18px; height: 18px; }
    input {
      appearance: none; margin: 0; width: 18px; height: 18px; cursor: pointer;
      border: 1px solid var(--kl-line-strong); border-radius: var(--kl-radius-sm); background: var(--kl-field);
      transition: background-color 0.12s, border-color 0.12s;
    }
    input:hover:not(:disabled) { border-color: var(--kl-muted); }
    input:checked { background: var(--kl-primary); border-color: var(--kl-primary); }
    input:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    input:disabled { opacity: 0.5; cursor: not-allowed; }
    .kl-checkbox-mark {
      position: absolute; inset: 3px; pointer-events: none; color: var(--kl-on-primary);
      opacity: 0; transition: opacity 0.12s;
    }
    input:checked + .kl-checkbox-mark { opacity: 1; }
    .kl-checkbox-label { cursor: pointer; }
    .kl-checkbox-label:empty { display: none; }
  `],
})
export class CheckboxComponent implements ControlValueAccessor {
  readonly checked = model(false);
  readonly disabled = input(false, { transform: booleanAttribute });
  readonly ariaLabel = input('');
  readonly inputId = input('');

  private readonly fallbackId = `kl-checkbox-${nextId++}`;
  protected readonly id = () => this.inputId() || this.fallbackId;
  private readonly formDisabled = signal(false);
  protected readonly isDisabled = () => this.disabled() || this.formDisabled();

  private onChange: (value: boolean) => void = () => {};
  protected onTouched: () => void = () => {};

  protected onInput(event: Event): void {
    const value = (event.target as HTMLInputElement).checked;
    this.checked.set(value);
    this.onChange(value);
  }

  writeValue(value: unknown): void { this.checked.set(!!value); }
  registerOnChange(fn: (value: boolean) => void): void { this.onChange = fn; }
  registerOnTouched(fn: () => void): void { this.onTouched = fn; }
  setDisabledState(disabled: boolean): void { this.formDisabled.set(disabled); }
}
