import { ChangeDetectionStrategy, Component, input } from '@angular/core';
import { IconComponent } from '../icon/icon.component';
import type { IconName } from '../icon/icons';

/**
 * - primary: orange tint, the "In use" mark
 * - neutral: grey chip, a plain value ("Haiku 5.5")
 * - muted: dashed outline, something absent ("Not set up")
 * - success / warn / info / error: tinted like kl-message
 * - secondary: same as muted, the name PrimeNG's p-tag used
 */
export type TagSeverity = 'primary' | 'neutral' | 'muted' | 'secondary' | 'success' | 'warn' | 'info' | 'error';

/**
 * A pill-shaped status or value (docs/ui/design.md: tags are pills).
 *
 *   <kl-tag severity="primary" icon="check">In use</kl-tag>
 *   <kl-tag [value]="tag.value" [severity]="tag.severity" />
 */
@Component({
  selector: 'kl-tag',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'kl-tag',
    '[attr.data-severity]': 'severity()',
  },
  template: `
    @if (icon(); as name) {
      <kl-icon [name]="name" [size]="14" [strokeWidth]="2.5" />
    }
    @if (value()) {
      {{ value() }}
    }
    <ng-content />
  `,
  styles: [`
    :host {
      --tone: var(--kl-muted);
      display: inline-flex;
      align-items: center;
      gap: 6px;
      font-size: 13px;
      font-weight: 500;
      line-height: 1.3;
      white-space: nowrap;
      padding: 2px 10px;
      border-radius: 999px;
      border: 1px solid var(--kl-line-control);
      background: var(--kl-sheet);
      color: var(--kl-text-control);
    }
    :host([data-severity='primary']) {
      font-weight: 600;
      color: var(--kl-primary-text);
      background: color-mix(in srgb, var(--kl-primary), transparent 88%);
      border-color: color-mix(in srgb, var(--kl-primary), transparent 65%);
    }
    :host([data-severity='muted']),
    :host([data-severity='secondary']) {
      color: var(--kl-muted);
      background: transparent;
      border: 1px dashed var(--kl-line-strong);
    }
    :host([data-severity='success']) { --tone: var(--kl-success); }
    :host([data-severity='warn']) { --tone: var(--kl-warn); }
    :host([data-severity='info']) { --tone: var(--kl-info); }
    :host([data-severity='error']) { --tone: var(--kl-error); }
    :host([data-severity='success']),
    :host([data-severity='warn']),
    :host([data-severity='info']),
    :host([data-severity='error']) {
      color: color-mix(in srgb, var(--tone), var(--kl-tone-text-mix) 55%);
      background: color-mix(in srgb, var(--tone), transparent 90%);
      border-color: color-mix(in srgb, var(--tone), transparent 62%);
    }
  `],
})
export class TagComponent {
  readonly severity = input<TagSeverity>('neutral');
  /** Shorthand for the text; projected content works too. */
  readonly value = input('');
  readonly icon = input<IconName | null>(null);
}
