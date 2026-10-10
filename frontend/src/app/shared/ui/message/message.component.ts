import { ChangeDetectionStrategy, Component, booleanAttribute, computed, input, output, signal } from '@angular/core';
import { IconComponent } from '../icon/icon.component';
import type { IconName } from '../icon/icons';

export type MessageSeverity = 'info' | 'success' | 'warn' | 'error' | 'secondary';

const ICON: Record<MessageSeverity, IconName> = {
  info: 'info',
  success: 'check',
  warn: 'triangle-alert',
  error: 'circle-x',
  secondary: 'info',
};

/**
 * An inline message (docs/ui/design.md, "Messages"): tinted background, a
 * border and an icon in the severity's colour, light text.
 *
 *   <kl-message severity="error" data-testid="fix-error">{{ error }}</kl-message>
 *   <kl-message severity="warn" title="Fix took too long" closable>…</kl-message>
 *
 * Errors are announced at once (role="alert"); everything else politely
 * (role="status"). `text` is a shorthand for a one-line body. A closable
 * message hides itself and emits `closed`; the owner may also drop it.
 */
@Component({
  selector: 'kl-message',
  standalone: true,
  imports: [IconComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: {
    '[attr.role]': 'severity() === "error" ? "alert" : "status"',
    '[attr.data-severity]': 'severity()',
    class: 'kl-message',
    '[hidden]': 'dismissed()',
  },
  template: `
    <kl-icon class="kl-message-icon" [name]="icon()" [size]="18" />
    <div class="kl-message-body">
      @if (title()) {
        <strong class="kl-message-title" data-testid="kl-message-title">{{ title() }}</strong>
      }
      @if (text()) {
        <span class="kl-message-text">{{ text() }}</span>
      }
      <ng-content />
    </div>
    @if (closable()) {
      <button type="button" class="kl-message-close" aria-label="Dismiss" data-testid="kl-message-close" (click)="close()">
        <kl-icon name="x" [size]="16" />
      </button>
    }
  `,
  styles: [`
    :host {
      --tone: var(--kl-info);
      display: flex;
      gap: 10px;
      align-items: flex-start;
      padding: 10px 14px;
      border-radius: var(--kl-radius-lg);
      border: 1px solid color-mix(in srgb, var(--tone), transparent 62%);
      background: color-mix(in srgb, var(--tone), transparent 92%);
      color: color-mix(in srgb, var(--tone), white 80%);
      font-size: 14px;
      line-height: 1.45;
    }
    :host([hidden]) { display: none; }
    :host([data-severity='success']) { --tone: var(--kl-success); }
    :host([data-severity='warn']) { --tone: var(--kl-warn); }
    :host([data-severity='error']) { --tone: var(--kl-error); }
    :host([data-severity='secondary']) { --tone: var(--kl-muted); color: var(--kl-text-soft); }
    .kl-message-icon { color: var(--tone); margin-top: 1px; }
    .kl-message-body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; overflow-wrap: anywhere; }
    .kl-message-title { font-weight: 600; color: var(--kl-text-strong); }
    .kl-message-close {
      flex: none; display: inline-flex; align-items: center; justify-content: center;
      width: 24px; height: 24px; margin: -2px -6px -2px 0; padding: 0;
      border: 0; border-radius: var(--kl-radius-sm); background: transparent; color: inherit; cursor: pointer;
    }
    .kl-message-close:hover { background: color-mix(in srgb, var(--tone), transparent 85%); }
    .kl-message-close:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class MessageComponent {
  readonly severity = input<MessageSeverity>('info');
  readonly title = input('');
  /** A one-line body; projected content works too. */
  readonly text = input('');
  readonly closable = input(false, { transform: booleanAttribute });
  readonly closed = output<void>();

  protected readonly icon = computed(() => ICON[this.severity()]);
  protected readonly dismissed = signal(false);

  close(): void {
    this.dismissed.set(true);
    this.closed.emit();
  }
}
