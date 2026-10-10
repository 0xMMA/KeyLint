import { ChangeDetectionStrategy, Component, input } from '@angular/core';

let nextId = 0;

/**
 * A sheet-coloured block with the divider border and the sheet radius
 * (docs/ui/design.md), for a settings section or a dev-tools panel.
 *
 *   <kl-card heading="Claude Code" subheading="KeyLint sends your text here.">…</kl-card>
 *
 * The heading is an <h2>; content that needs its own heading level can leave
 * it empty and bring one. A section with a heading is labelled by it.
 */
@Component({
  selector: 'kl-card',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'kl-card',
    '[attr.role]': 'heading() ? "region" : null',
    '[attr.aria-labelledby]': 'heading() ? headingId : null',
  },
  template: `
    @if (heading()) {
      <header class="kl-card-header">
        <h2 class="kl-card-heading" [id]="headingId">{{ heading() }}</h2>
        @if (subheading()) {
          <p class="kl-card-subheading">{{ subheading() }}</p>
        }
      </header>
    }
    <ng-content />
  `,
  styles: [`
    :host {
      display: flex; flex-direction: column; gap: 20px;
      padding: 24px 28px;
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .kl-card-heading { margin: 0; font-size: 18px; font-weight: 700; color: var(--kl-text-strong); }
    .kl-card-subheading { margin: 4px 0 0; font-size: 14px; color: var(--kl-muted); }
  `],
})
export class CardComponent {
  readonly heading = input('');
  readonly subheading = input('');
  protected readonly headingId = `kl-card-${nextId++}`;
}
