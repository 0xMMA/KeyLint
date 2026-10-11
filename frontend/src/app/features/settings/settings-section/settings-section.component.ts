import { ChangeDetectionStrategy, Component, booleanAttribute, input } from '@angular/core';

let nextId = 0;

/**
 * A group on a Settings tab: a heading, a line saying what the group is for,
 * and its rows in one bordered list (the mockup's "calm list", see the
 * Connections list in docs/ui/design.md's Settings mockup).
 *
 *   <app-settings-section heading="Shortcuts" description="…">
 *     <app-setting-row …>…</app-setting-row>
 *   </app-settings-section>
 *
 * Rows are projected; each row draws the divider above itself, so any row
 * component can go in. `list` gives the body list semantics, for rows that
 * are items of one kind (the connections), not a form.
 */
@Component({
  selector: 'app-settings-section',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'flex flex-col gap-3',
    '[attr.role]': 'heading() ? "region" : null',
    '[attr.aria-labelledby]': 'heading() ? headingId : null',
  },
  template: `
    @if (heading()) {
      <div>
        <h2 class="m-0 text-base font-bold text-fg-strong" [id]="headingId">{{ heading() }}</h2>
        @if (description()) {
          <p class="mt-1 mb-0 text-sm text-muted">{{ description() }}</p>
        }
      </div>
    }
    <div class="body" [attr.role]="list() ? 'list' : null"><ng-content /></div>
    <ng-content select="[sectionFooter]" />
  `,
  styles: [`
    .body {
      border: 1px solid var(--kl-line);
      border-radius: var(--kl-radius-sheet);
      background: var(--kl-sheet);
      overflow: hidden;
    }
    .body:empty { display: none; }
  `],
})
export class SettingsSectionComponent {
  readonly heading = input('');
  readonly description = input('');
  readonly list = input(false, { transform: booleanAttribute });
  protected readonly headingId = `settings-section-${nextId++}`;
}
