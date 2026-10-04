import { Component, input, output } from '@angular/core';
import { ButtonModule } from 'primeng/button';
import { TagModule } from 'primeng/tag';
import { MessageModule } from 'primeng/message';

/**
 * One provider on Settings › AI Providers: its name, whether KeyLint is using
 * it, and the one-click switch to it.
 *
 * Every provider gets the same frame so "which one is in use" reads the same
 * way on all four; what goes inside (a key editor, the CLI's detection result,
 * the Ollama URL) is projected by the settings page.
 *
 * A provider that cannot work yet can still be chosen. The choice stays the
 * user's; the card says why it will not work rather than refusing the click,
 * because a button that silently does nothing is the confusion this replaces.
 */
@Component({
  selector: 'app-provider-card',
  standalone: true,
  imports: [ButtonModule, TagModule, MessageModule],
  template: `
    <div
      class="provider-card"
      [class.in-use]="inUse()"
      [attr.data-testid]="'provider-card-' + providerId()"
      [attr.aria-current]="inUse() ? 'true' : null"
      tabindex="-1"
    >
      <div class="provider-card-header">
        <span class="provider-card-label">{{ label() }}</span>
        <ng-content select="[cardStatus]" />
        <span class="provider-card-spacer"></span>
        @if (inUse()) {
          <p-tag
            [attr.data-testid]="'provider-in-use-' + providerId()"
            value="In use"
            icon="pi pi-check"
          />
        } @else {
          <p-button
            [attr.data-testid]="'provider-use-' + providerId()"
            label="Use this"
            size="small"
            severity="secondary"
            [outlined]="true"
            [disabled]="locked()"
            (onClick)="use.emit()"
          />
        }
      </div>

      @if (inUse() && problem(); as why) {
        <p-message
          [attr.data-testid]="'provider-not-ready-' + providerId()"
          severity="warn"
          size="small"
          styleClass="provider-card-warning"
        >{{ why }}</p-message>
      }

      <ng-content />
    </div>
  `,
  styles: [`
    .provider-card {
      border: 1px solid var(--p-content-border-color);
      border-radius: var(--p-border-radius-md, 6px);
      padding: 0.75rem 1rem;
      margin-bottom: 0.75rem;
    }
    /* The one KeyLint uses: the theme's accent, so it is found at a glance
       without reading every card's tag. */
    .provider-card.in-use {
      border-color: var(--p-primary-color);
      box-shadow: inset 3px 0 0 var(--p-primary-color);
    }
    .provider-card-header {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 0.75rem;
      margin-bottom: 0.5rem;
    }
    .provider-card-label {
      font-weight: 600;
      font-size: 0.9rem;
    }
    .provider-card-spacer { flex: 1; }
    .provider-card:focus { outline: none; }
    .provider-card:focus-visible { outline: 2px solid var(--p-primary-color); outline-offset: 2px; }
    :host ::ng-deep .provider-card-warning { margin-bottom: 0.75rem; }
  `],
})
export class ProviderCardComponent {
  readonly providerId = input.required<string>();
  readonly label = input.required<string>();
  /** Whether this is the provider saved as active. */
  readonly inUse = input(false);
  /** Why this provider cannot work right now, or null when nothing is known to be wrong. */
  readonly problem = input<string | null>(null);
  /** Another switch is in flight; one at a time keeps the saves in order. */
  readonly locked = input(false);

  /** The user asked to make this the active provider. */
  readonly use = output<void>();
}
