import { ChangeDetectionStrategy, Component, booleanAttribute, input } from '@angular/core';

/**
 * One setting in an app-settings-section: its name and hint on the left, the
 * control on the right, wrapping under each other in a narrow window.
 *
 *   <app-setting-row data-testid="start-on-boot-section" label="Start on boot" forId="start-on-boot">
 *     <kl-switch inputId="start-on-boot" ariaLabel="Start on boot" [(ngModel)]="…" />
 *   </app-setting-row>
 *
 * `forId` makes the name a <label for> (clicking it reaches the control);
 * without it the name is plain text. A hint with markup is projected with the
 * `hint` attribute, a plain one goes in `hint`. Content marked `below` takes
 * the row's full width under both (a message, a second field). `stacked` puts
 * the control under the text instead of beside it, for wide controls.
 */
@Component({
  selector: 'app-setting-row',
  standalone: true,
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'setting-row', '[class.stacked]': 'stacked()' },
  template: `
    <div class="text">
      @if (forId()) {
        <label class="name" [for]="forId()">{{ label() }}</label>
      } @else {
        <span class="name" [id]="labelId() || null">{{ label() }}</span>
      }
      <!-- Comment anchors only when empty, so :empty hides it. -->
      <small class="hint">@if (hint()) {{{ hint() }}}<ng-content select="[hint]" /></small>
    </div>
    <div class="control"><ng-content /></div>
    <ng-content select="[below]" />
  `,
  styles: [`
    :host {
      display: flex; flex-wrap: wrap; align-items: center; gap: 12px 24px;
      padding: 16px 20px;
    }
    :host(:not(:first-child)) { border-top: 1px solid var(--kl-line); }
    .text { flex: 1 1 260px; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
    .name { font-size: 14px; font-weight: 600; color: var(--kl-text); }
    .hint { font-size: 13px; line-height: 1.45; color: var(--kl-muted); }
    .hint:empty { display: none; }
    .control { flex: 0 1 auto; min-width: 0; display: flex; align-items: center; gap: 8px; }
    :host(.stacked) .control { flex-basis: 100%; }
    .control:empty { display: none; }
  `],
})
export class SettingRowComponent {
  readonly label = input('');
  readonly hint = input('');
  /** id of the control, so the name is its <label>. */
  readonly forId = input('');
  /** id for the plain-text name, for a control to point aria-labelledby at. */
  readonly labelId = input('');
  readonly stacked = input(false, { transform: booleanAttribute });
}
