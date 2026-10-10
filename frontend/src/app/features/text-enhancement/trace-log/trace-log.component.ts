import { ChangeDetectionStrategy, Component, input, output } from '@angular/core';
import { ButtonComponent, TooltipDirective } from '../../../shared/ui';
import { formatTraceTime, type TraceEntry } from '../trace-entry';

/**
 * The history column right of the editor (the trace log): every snapshot the
 * session took, oldest first. Hovering or focusing an entry previews it over
 * the editor; clicking pins the preview, where it can be reverted to.
 */
@Component({
  selector: 'app-trace-log',
  standalone: true,
  imports: [ButtonComponent, TooltipDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'pz-trace', role: 'region', 'aria-labelledby': 'pz-trace-title' },
  template: `
    <div class="pz-trace-head">
      <h2 id="pz-trace-title" class="pz-trace-title">History</h2>
      <kl-button data-testid="add-checkpoint-btn" variant="ghost" size="sm" icon="plus" iconOnly
        ariaLabel="Add checkpoint" klTooltip="Add checkpoint" klTooltipPosition="left" (click)="checkpoint.emit()" />
      <kl-button data-testid="trace-log-close" variant="ghost" size="sm" icon="chevron-right" iconOnly
        ariaLabel="Hide history" klTooltip="Hide history" klTooltipPosition="left" (click)="collapse.emit()" />
    </div>
    @if (entries().length) {
      <ol class="pz-trace-list">
        @for (entry of entries(); track entry.id) {
          <li>
            <button
              type="button"
              class="pz-trace-entry"
              data-testid="trace-entry"
              [attr.aria-pressed]="pinnedId() === entry.id"
              (click)="pin.emit(entry)"
              (mouseenter)="hover.emit(entry)"
              (mouseleave)="unhover.emit()"
              (focus)="hover.emit(entry)"
              (blur)="unhover.emit()"
            >
              <span class="pz-trace-label">{{ entry.label }}</span>
              <span class="pz-trace-time">{{ time(entry.timestamp) }}</span>
            </button>
          </li>
        }
      </ol>
    } @else {
      <p class="pz-trace-empty">Snapshots appear here as you pyramidize, refine and add checkpoints.</p>
    }
  `,
  styles: [`
    :host {
      display: flex; flex-direction: column; flex: none; width: 272px; min-height: 0;
      border-left: 1px solid var(--kl-line); background: var(--kl-rail);
    }
    .pz-trace-head {
      display: flex; align-items: center; gap: 2px; flex: none;
      height: 56px; padding: 0 8px 0 16px; border-bottom: 1px solid var(--kl-line);
    }
    .pz-trace-title { flex: 1; margin: 0; font-size: 14px; font-weight: 700; color: var(--kl-text-strong); }
    .pz-trace-list { flex: 1; margin: 0; padding: 8px; overflow-y: auto; list-style: none; }
    .pz-trace-entry {
      display: flex; flex-direction: column; gap: 2px; width: 100%; padding: 8px 10px; text-align: left;
      font: inherit; color: var(--kl-text); background: transparent; border: 0; border-radius: var(--kl-radius-md);
      cursor: pointer; transition: background-color 0.12s;
    }
    .pz-trace-entry:hover { background: var(--kl-raised); }
    .pz-trace-entry[aria-pressed='true'] { background: var(--kl-highlight); box-shadow: inset 2px 0 0 var(--kl-primary); }
    .pz-trace-entry:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: -2px; }
    .pz-trace-label { font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
    .pz-trace-time { font-size: 12px; color: var(--kl-muted); }
    .pz-trace-empty { margin: 0; padding: 16px; font-size: 13px; line-height: 1.5; color: var(--kl-muted); }
  `],
})
export class TraceLogComponent {
  readonly entries = input<readonly TraceEntry[]>([]);
  /** The entry whose preview is pinned, if any. */
  readonly pinnedId = input<string | null>(null);

  readonly pin = output<TraceEntry>();
  readonly hover = output<TraceEntry>();
  readonly unhover = output<void>();
  readonly checkpoint = output<void>();
  readonly collapse = output<void>();

  protected time(d: Date): string { return formatTraceTime(d); }
}
