import { ChangeDetectionStrategy, Component, ElementRef, effect, input, output, viewChild } from '@angular/core';
import { ButtonComponent } from '../../../shared/ui';
import { formatTraceTime, type TraceEntry } from '../trace-entry';

/**
 * A history snapshot laid over the editor: a preview while an entry is
 * hovered, pinned (with Revert and Close) once it is clicked. A pinned
 * preview takes the focus (to Close, not to the destructive Revert) and
 * closes on Escape.
 */
@Component({
  selector: 'app-trace-peek',
  standalone: true,
  imports: [ButtonComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    class: 'pz-peek',
    '(document:keydown.escape)': 'pinned() && closed.emit()',
  },
  template: `
    <section class="pz-peek-panel" role="region" [attr.aria-label]="'Snapshot: ' + entry().label">
      <header class="pz-peek-head">
        <span class="pz-peek-title">{{ entry().label }}</span>
        <span class="pz-peek-meta">{{ time() }}</span>
        @if (pinned()) {
          <kl-button data-testid="trace-peek-close" variant="ghost" size="sm" icon="x" iconOnly ariaLabel="Close snapshot"
            (click)="closed.emit()" />
        } @else {
          <span class="pz-peek-meta">Click to pin</span>
        }
      </header>
      <pre class="pz-peek-content" data-testid="trace-peek-content">{{ entry().snapshot }}</pre>
      <footer class="pz-peek-foot">
        <kl-button data-testid="trace-revert-btn" variant="danger" icon="rotate-ccw" (click)="revert.emit()">
          Revert to here
        </kl-button>
        @if (pinned()) {
          <kl-button #closeBtn data-testid="trace-peek-done" variant="quiet" (click)="closed.emit()">Close</kl-button>
        }
      </footer>
    </section>
  `,
  styles: [`
    :host {
      position: absolute; inset: 0; z-index: 50; display: flex; padding: 16px;
      background: color-mix(in srgb, var(--kl-surface-950), transparent 35%);
    }
    .pz-peek-panel {
      flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden;
      background: var(--kl-sheet); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-sheet);
      box-shadow: var(--kl-shadow-overlay);
    }
    .pz-peek-head, .pz-peek-foot { display: flex; align-items: center; gap: 12px; flex: none; padding: 10px 12px 10px 20px; }
    .pz-peek-head { border-bottom: 1px solid var(--kl-line); }
    .pz-peek-foot { border-top: 1px solid var(--kl-line); padding: 12px 20px; }
    .pz-peek-title { flex: 1; min-width: 0; font-size: 14px; font-weight: 600; color: var(--kl-text-strong); overflow-wrap: anywhere; }
    .pz-peek-meta { font-size: 12px; color: var(--kl-muted); }
    .pz-peek-content {
      flex: 1; margin: 0; padding: 24px 32px; overflow-y: auto; white-space: pre-wrap; overflow-wrap: anywhere;
      font-family: var(--kl-font-text); font-size: 15.5px; line-height: 1.7; color: var(--kl-surface-200);
    }
  `],
})
export class TracePeekComponent {
  readonly entry = input.required<TraceEntry>();
  /** Clicked rather than hovered: stays, and can be closed. */
  readonly pinned = input(false);

  readonly revert = output<void>();
  readonly closed = output<void>();

  private readonly closeBtn = viewChild('closeBtn', { read: ElementRef });

  constructor() {
    effect(() => {
      if (this.pinned()) this.closeBtn()?.nativeElement.querySelector('button')?.focus();
    });
  }

  protected time(): string { return formatTraceTime(this.entry().timestamp); }
}
