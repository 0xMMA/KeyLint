import { ChangeDetectionStrategy, Component, booleanAttribute, computed, input } from '@angular/core';
import { IconComponent } from '../icon/icon.component';
import type { IconName } from '../icon/icons';
import { SpinnerComponent } from '../spinner/spinner.component';

export type ButtonVariant = 'primary' | 'quiet' | 'ghost' | 'danger';
export type ButtonSize = 'sm' | 'md' | 'lg';

/**
 * KeyLint's button (docs/ui/design.md, "Buttons"): primary (orange, dark
 * text), quiet (grey fill, border), ghost (text only), danger (red outline).
 *
 *   <kl-button data-testid="fix-btn" icon="sparkles" [loading]="busy"
 *              loadingLabel="Fixing…" (click)="fix()">Fix</kl-button>
 *
 * The host wraps one real <button>, so `getByTestId('fix-btn').getByRole('button')`
 * reaches it and (click) on the host fires for it. A disabled or loading
 * button is a disabled <button>, which dispatches no click at all.
 *
 * Loading: a spinner takes the icon's place, the label says what is
 * happening (`loadingLabel`, if given) and the button carries aria-busy.
 * Anything marked `klButtonEnd` is projected after the label, e.g. a
 * keyboard hint: `<span klButtonEnd><kbd>Ctrl</kbd> <kbd>↵</kbd></span>`.
 */
@Component({
  selector: 'kl-button',
  standalone: true,
  imports: [IconComponent, SpinnerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'kl-button-host',
    '[class.kl-button-block]': 'block()',
  },
  template: `
    <button
      class="kl-button"
      [class]="classes()"
      [attr.type]="type()"
      [disabled]="disabled() || loading()"
      [attr.aria-busy]="loading() ? 'true' : null"
      [attr.aria-label]="ariaLabel() || null"
      [attr.aria-pressed]="pressed() === null ? null : pressed()"
    >
      <span class="kl-button-main">
        @if (loading()) {
          <kl-spinner [size]="iconSize()" data-testid="kl-button-spinner" />
        } @else if (icon(); as name) {
          <kl-icon [name]="name" [size]="iconSize()" />
        }
        @if (loading() && loadingLabel()) {
          <span class="kl-button-label">{{ loadingLabel() }}</span>
        } @else {
          <span class="kl-button-label" [class.kl-empty]="iconOnly()"><ng-content /></span>
        }
      </span>
      <span class="kl-button-end"><ng-content select="[klButtonEnd]" /></span>
    </button>
  `,
  styles: [`
    :host { display: inline-flex; vertical-align: middle; }
    :host(.kl-button-block) { display: flex; width: 100%; }
    .kl-button {
      font: inherit;
      font-size: 14px;
      font-weight: 600;
      line-height: 1;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      width: 100%;
      white-space: nowrap;
      border: 1px solid transparent;
      border-radius: var(--kl-radius-md);
      cursor: pointer;
      transition: background-color 0.12s, color 0.12s, border-color 0.12s;
    }
    .kl-button:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .kl-button:disabled { cursor: not-allowed; opacity: 0.5; }
    .kl-button[aria-busy='true'] { cursor: progress; opacity: 0.85; }
    .kl-button-main { display: inline-flex; align-items: center; gap: 8px; min-width: 0; }
    .kl-button-label:empty, .kl-empty, .kl-button-end:empty { display: none; }
    .kl-button-end { display: inline-flex; align-items: center; gap: 4px; font-weight: 500; }
    :host(.kl-button-block) .kl-button:has(.kl-button-end:not(:empty)) { justify-content: space-between; }

    .size-sm { height: 32px; padding: 0 10px; font-size: 13px; }
    .size-md { height: 36px; padding: 0 14px; }
    .size-lg { height: 40px; padding: 0 16px; }
    .icon-only.size-sm { width: 32px; padding: 0; }
    .icon-only.size-md { width: 36px; padding: 0; }
    .icon-only.size-lg { width: 40px; padding: 0; }

    .primary { background: var(--kl-primary); color: var(--kl-on-primary); }
    .primary:not(:disabled):hover { background: var(--kl-primary-hover); }
    .quiet { background: var(--kl-raised); color: var(--kl-surface-200); border-color: var(--kl-line-control); }
    .quiet:not(:disabled):hover { background: var(--kl-raised-hover); }
    .quiet[aria-pressed='true'] { background: var(--kl-selected); color: var(--kl-text-strong); }
    .ghost { background: transparent; color: var(--kl-muted); }
    .ghost:not(:disabled):hover { background: var(--kl-sheet); color: var(--kl-text); }
    .danger { background: transparent; color: var(--kl-danger-text); border-color: color-mix(in srgb, var(--kl-error), transparent 55%); }
    .danger:not(:disabled):hover { background: color-mix(in srgb, var(--kl-error), transparent 90%); }
  `],
})
export class ButtonComponent {
  readonly variant = input<ButtonVariant>('quiet');
  readonly size = input<ButtonSize>('md');
  readonly icon = input<IconName | null>(null);
  readonly type = input<'button' | 'submit' | 'reset'>('button');
  readonly disabled = input(false, { transform: booleanAttribute });
  readonly loading = input(false, { transform: booleanAttribute });
  /** What the label says while loading, e.g. "Fixing…". Empty keeps the label. */
  readonly loadingLabel = input('');
  /** Accessible name; required for an icon-only button. */
  readonly ariaLabel = input('');
  /** Square, no label: the projected content is ignored. Give it an ariaLabel. */
  readonly iconOnly = input(false, { transform: booleanAttribute });
  /** Fills its container's width. */
  readonly block = input(false, { transform: booleanAttribute });
  /** For a toggle button: sets aria-pressed. null (the default) is no toggle. */
  readonly pressed = input<boolean | null>(null);

  protected readonly iconSize = computed(() => (this.size() === 'sm' ? 14 : 16));
  protected readonly classes = computed(() =>
    `kl-button ${this.variant()} size-${this.size()}${this.iconOnly() ? ' icon-only' : ''}`);
}
