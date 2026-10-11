import { ChangeDetectionStrategy, Component, input } from '@angular/core';

/**
 * A small rotating arc, in the current text colour.
 *
 * Decorative by default: whatever shows it says in words what is happening
 * (a button's "Fixing…", aria-busy). Give it a `label` when it stands alone
 * and is the only sign that something is loading. With reduced motion the
 * global rule in styles.scss stops the rotation and the arc stays put.
 */
@Component({
  selector: 'kl-spinner',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'kl-spinner',
    '[attr.role]': 'label() ? "status" : null',
    '[attr.aria-label]': 'label() || null',
    '[attr.aria-hidden]': 'label() ? null : "true"',
    '[style.width.px]': 'size()',
    '[style.height.px]': 'size()',
  },
  template: `
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" focusable="false">
      <path d="M21 12a9 9 0 1 1-6.2-8.6" />
    </svg>
  `,
  styles: [`
    :host { display: inline-flex; flex: none; line-height: 0; }
    svg { width: 100%; height: 100%; animation: kl-spin 0.8s linear infinite; }
    @keyframes kl-spin { to { transform: rotate(360deg); } }
  `],
})
export class SpinnerComponent {
  readonly size = input(16);
  /** Accessible name, only when nothing else announces the wait. */
  readonly label = input('');
}
