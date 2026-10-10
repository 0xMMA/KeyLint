import { ChangeDetectionStrategy, Component, input, model } from '@angular/core';

export interface SegmentOption {
  value: string;
  label: string;
}

/**
 * A small segmented toggle: one of a few views, e.g. the Fix result's
 * "Show changes / Clean text".
 *
 *   <kl-segmented ariaLabel="Result view" [options]="views" [(value)]="view" />
 *
 * A labelled group of toggle buttons (aria-pressed), each its own Tab stop —
 * plain enough for two or three views, and what the mockup specifies. Each
 * button carries data-testid="segment-<value>".
 */
@Component({
  selector: 'kl-segmented',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'group', class: 'kl-segmented', '[attr.aria-label]': 'ariaLabel() || null' },
  template: `
    @for (option of options(); track option.value) {
      <button type="button" class="kl-segment" [attr.data-testid]="'segment-' + option.value"
        [attr.aria-pressed]="option.value === value()" (click)="value.set(option.value)">{{ option.label }}</button>
    }
  `,
  styles: [`
    :host {
      display: inline-flex; gap: 2px; padding: 2px;
      background: var(--kl-field); border: 1px solid var(--kl-line); border-radius: 7px;
    }
    .kl-segment {
      font: inherit; font-size: 13px; font-weight: 500; line-height: 1.2;
      padding: 5px 12px; border: 0; border-radius: 5px; cursor: pointer;
      background: transparent; color: var(--kl-muted); transition: background-color 0.12s, color 0.12s;
    }
    .kl-segment:hover { color: var(--kl-text); }
    .kl-segment[aria-pressed='true'] { background: var(--kl-selected); color: var(--kl-text-strong); }
    .kl-segment:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class SegmentedComponent {
  readonly options = input<readonly SegmentOption[]>([]);
  readonly value = model('');
  readonly ariaLabel = input('');
}
