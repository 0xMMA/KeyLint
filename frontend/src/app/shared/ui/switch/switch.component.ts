import { ChangeDetectionStrategy, Component, booleanAttribute, forwardRef, input, model, signal } from '@angular/core';
import { ControlValueAccessor, NG_VALUE_ACCESSOR } from '@angular/forms';

let nextId = 0;

/**
 * An on/off switch: a real <button role="switch"> with aria-checked, so it is
 * reachable by Tab and flipped by Space or Enter.
 *
 *   <kl-switch data-testid="start-on-boot" [(ngModel)]="startOnBoot">Start on boot</kl-switch>
 *   <kl-switch [(checked)]="on" ariaLabel="Sensitive logging" />
 *
 * Works with ngModel / reactive forms (ControlValueAccessor) or with the
 * `checked` model on its own. The projected text is the switch's accessible
 * name and is clickable; `ariaLabel` replaces it, `inputId` lets an outside
 * <label for> point at it.
 */
@Component({
  selector: 'kl-switch',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  providers: [{ provide: NG_VALUE_ACCESSOR, useExisting: forwardRef(() => SwitchComponent), multi: true }],
  host: { class: 'kl-switch' },
  template: `
    <button
      type="button"
      role="switch"
      class="kl-switch-track"
      [id]="inputId() || null"
      [attr.aria-checked]="checked()"
      [attr.aria-label]="ariaLabel() || null"
      [attr.aria-labelledby]="ariaLabel() ? null : labelId"
      [disabled]="isDisabled()"
      (click)="toggle()"
      (blur)="onTouched()"
    ><span class="kl-switch-knob"></span></button>
    <span class="kl-switch-label" [id]="labelId" (click)="toggle()"><ng-content /></span>
  `,
  styles: [`
    :host { display: inline-flex; align-items: center; gap: 10px; font-size: 14px; }
    .kl-switch-track {
      position: relative; flex: none; width: 40px; height: 22px; padding: 0;
      border-radius: 999px; border: 1px solid var(--kl-line-strong); background: var(--kl-sheet);
      cursor: pointer; transition: background-color 0.15s, border-color 0.15s;
    }
    .kl-switch-knob {
      position: absolute; top: 3px; left: 3px; width: 14px; height: 14px; border-radius: 50%;
      background: var(--kl-muted); transition: transform 0.15s, background-color 0.15s;
    }
    .kl-switch-track[aria-checked='true'] { background: var(--kl-primary); border-color: var(--kl-primary); }
    .kl-switch-track[aria-checked='true'] .kl-switch-knob { transform: translateX(18px); background: var(--kl-on-primary); }
    .kl-switch-track:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .kl-switch-track:disabled { opacity: 0.5; cursor: not-allowed; }
    .kl-switch-label { cursor: pointer; }
    .kl-switch-label:empty { display: none; }
    .kl-switch-track:disabled + .kl-switch-label { cursor: not-allowed; color: var(--kl-muted); }
  `],
})
export class SwitchComponent implements ControlValueAccessor {
  readonly checked = model(false);
  readonly disabled = input(false, { transform: booleanAttribute });
  readonly ariaLabel = input('');
  readonly inputId = input('');

  protected readonly labelId = `kl-switch-label-${nextId++}`;
  private readonly formDisabled = signal(false);
  protected readonly isDisabled = () => this.disabled() || this.formDisabled();

  private onChange: (value: boolean) => void = () => {};
  protected onTouched: () => void = () => {};

  toggle(): void {
    if (this.isDisabled()) return;
    const next = !this.checked();
    this.checked.set(next);
    this.onChange(next);
  }

  writeValue(value: unknown): void { this.checked.set(!!value); }
  registerOnChange(fn: (value: boolean) => void): void { this.onChange = fn; }
  registerOnTouched(fn: () => void): void { this.onTouched = fn; }
  setDisabledState(disabled: boolean): void { this.formDisabled.set(disabled); }
}
