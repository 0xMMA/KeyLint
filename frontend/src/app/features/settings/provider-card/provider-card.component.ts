import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { ButtonComponent, MessageComponent } from '../../../shared/ui';

/**
 * One provider in the Connections list on Settings › AI Providers: a dot for
 * its state, its name and status, and the one-click switch to it.
 *
 * Every provider gets the same row so "which one is in use" reads the same
 * way on all four; what goes inside is projected by the settings page:
 *
 * - `[cardSummary]`: the status in words under the name (where the CLI was found, …)
 * - `[cardActions]`: buttons before "Use this" (Add key, Check again)
 * - the rest: full-width details under the row (a key editor, the Ollama URL)
 *
 * A provider that cannot work yet can still be chosen. The choice stays the
 * user's; the row says why it will not work rather than refusing the click,
 * because a button that silently does nothing is the confusion this replaces.
 */
@Component({
  selector: 'app-provider-card',
  standalone: true,
  imports: [ButtonComponent, MessageComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { role: 'listitem', '[class.in-use]': 'inUse()' },
  template: `
    <div class="row" [attr.data-testid]="'provider-card-' + providerId()">
      <span class="dot" [class.ready]="ready()" aria-hidden="true"></span>
      <div class="main">
        <!-- A heading, so a screen reader can jump between providers, and
             the focus target after a switch: it carries the provider's name. -->
        <h3 class="name" tabindex="-1" [attr.data-testid]="'provider-heading-' + providerId()"
          >{{ label() }}@if (inUse()) {<span class="sr-only"> (in use)</span>}</h3>
        <div class="summary"><ng-content select="[cardSummary]" /></div>
      </div>
      <div class="actions">
        <ng-content select="[cardActions]" />
        @if (inUse()) {
          <span class="in-use-mark" [attr.data-testid]="'provider-in-use-' + providerId()">In use</span>
        } @else {
          <kl-button
            [attr.data-testid]="'provider-use-' + providerId()"
            size="sm"
            [ariaLabel]="'Use this: ' + label()"
            [loading]="switching()"
            loadingLabel="Switching…"
            [disabled]="locked()"
            (click)="use.emit()"
          >Use this</kl-button>
        }
      </div>
      <div class="details">
        @if (inUse() && problem(); as why) {
          <kl-message [attr.data-testid]="'provider-not-ready-' + providerId()" severity="warn">{{ why }}</kl-message>
        }
        <ng-content />
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; }
    :host(:not(:first-child)) { border-top: 1px solid var(--kl-line); }
    :host(.in-use) { background: var(--kl-sheet-alt); }
    .row {
      display: grid; grid-template-columns: 9px minmax(0, 1fr) auto;
      align-items: center; gap: 4px 16px; padding: 16px 20px;
    }
    .dot { width: 9px; height: 9px; border-radius: 50%; border: 1.5px solid var(--kl-faint); }
    .dot.ready { background: var(--kl-muted); border-color: var(--kl-muted); }
    :host(.in-use) .dot { background: var(--kl-primary); border-color: var(--kl-primary); }
    .main { min-width: 0; }
    .name { margin: 0; font-size: 15px; font-weight: 600; color: var(--kl-text-strong); }
    .name:focus { outline: none; }
    .name:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; border-radius: 2px; }
    .summary { margin-top: 2px; font-size: 13px; line-height: 1.45; color: var(--kl-muted); overflow-wrap: anywhere; }
    .summary:empty { display: none; }
    .actions { display: flex; flex-wrap: wrap; justify-content: flex-end; align-items: center; gap: 8px; }
    .in-use-mark { font-size: 13px; font-weight: 600; color: var(--kl-primary-text); padding: 0 4px; }
    .details { grid-column: 2 / -1; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
    .details:empty { display: none; }
    .details:not(:empty) { margin-top: 10px; }
    @media (max-width: 560px) {
      .row { grid-template-columns: 9px minmax(0, 1fr); }
      .actions { grid-column: 2; justify-content: flex-start; }
    }
  `],
})
export class ProviderCardComponent {
  readonly providerId = input.required<string>();
  readonly label = input.required<string>();
  /** Whether this is the provider saved as active. */
  readonly inUse = input(false);
  /** Set up and answering (key set, CLI signed in, daemon up): a filled dot. */
  readonly ready = input(false);
  /** Why this provider cannot work right now, or null when nothing is known to be wrong. */
  readonly problem = input<string | null>(null);
  /** This card's switch is being saved. */
  readonly switching = input(false);
  /** Another switch, a save or a reset is in flight; one at a time keeps them in order. */
  readonly locked = input(false);

  /** The user asked to make this the active provider. */
  readonly use = output<void>();
}
