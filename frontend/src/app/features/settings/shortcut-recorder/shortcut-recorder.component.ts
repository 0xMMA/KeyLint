import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, OnDestroy, inject } from '@angular/core';
import { ButtonComponent } from '../../../shared/ui';
import { WailsService } from '../../../core/wails.service';
import { formatCombo } from '../../../core/shortcut-format';

const MODIFIER_KEYS = new Set(['Control', 'Shift', 'Alt', 'Meta']);

const KEY_TO_NAME: Record<string, string> = {
  ' ': 'space', 'Enter': 'enter', 'Tab': 'tab', 'Escape': 'escape',
  'Backspace': 'backspace', 'Delete': 'delete', 'Insert': 'insert',
  'Home': 'home', 'End': 'end', 'PageUp': 'pageup', 'PageDown': 'pagedown',
  'ArrowUp': 'up', 'ArrowDown': 'down', 'ArrowLeft': 'left', 'ArrowRight': 'right',
};

@Component({
  selector: 'app-shortcut-recorder',
  standalone: true,
  imports: [ButtonComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- The field itself takes the keys while recording, so it is focusable;
         it names what it holds for a screen reader. -->
    <div class="recorder" [class.recording]="recording" data-testid="recorder-field"
         tabindex="0" role="group" [attr.aria-label]="ariaLabel || null" (keydown)="onKeyDown($event)">
      @if (recording) {
        <span class="recording-text" data-testid="recording-indicator" aria-live="polite">Press a key combo…</span>
      } @else {
        <span class="combo-text" data-testid="combo-display">{{ displayValue }}</span>
      }
      <kl-button
        data-testid="record-btn"
        size="sm"
        [variant]="recording ? 'danger' : 'quiet'"
        (click)="toggleRecording()"
      >{{ recording ? 'Cancel' : 'Record…' }}</kl-button>
    </div>
  `,
  styles: [`
    .recorder {
      display: flex; align-items: center; justify-content: space-between; gap: 12px;
      min-width: 260px; height: 44px; padding: 0 6px 0 12px;
      background: var(--kl-field); border: 1px solid var(--kl-line-control); border-radius: var(--kl-radius-md);
    }
    .recorder:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
    .recorder.recording { border-color: var(--kl-primary); }
    .combo-text { font-size: 14px; font-weight: 600; color: var(--kl-text); letter-spacing: 0.01em; }
    .recording-text { font-size: 13px; color: var(--kl-primary-text); animation: pulse 1.5s ease-in-out infinite; }
    @keyframes pulse { 50% { opacity: 0.55; } }
    @media (prefers-reduced-motion: reduce) { .recording-text { animation: none; } }
  `],
})
export class ShortcutRecorderComponent implements OnDestroy {
  @Input() value = '';
  /** What the shortcut is for, e.g. "Fix shortcut". */
  @Input() ariaLabel = '';
  @Output() valueChange = new EventEmitter<string>();

  private readonly wails = inject(WailsService);
  recording = false;

  get displayValue(): string {
    return formatCombo(this.value);
  }

  toggleRecording(): void {
    this.recording = !this.recording;
    // Pause/resume the global shortcut hook so it doesn't intercept keypresses during recording.
    void this.wails.setShortcutPaused(this.recording);
  }

  ngOnDestroy(): void {
    // Removed while waiting for a combo (the page was left, or the mode switch
    // swapped the recorders): without this the hook stays paused until restart.
    if (this.recording) {
      this.recording = false;
      void this.wails.setShortcutPaused(false);
    }
  }

  onKeyDown(event: KeyboardEvent): void {
    if (!this.recording) return;

    event.preventDefault();
    event.stopPropagation();

    if (event.key === 'Escape') {
      this.recording = false;
      void this.wails.setShortcutPaused(false);
      return;
    }

    // Ignore modifier-only presses — wait for a trigger key.
    if (MODIFIER_KEYS.has(event.key)) return;

    const parts: string[] = [];
    if (event.ctrlKey) parts.push('ctrl');
    if (event.shiftKey) parts.push('shift');
    if (event.altKey) parts.push('alt');
    if (event.metaKey) parts.push('win');

    // Map the key to our canonical name.
    let keyName = KEY_TO_NAME[event.key] ?? event.key.toLowerCase();
    // Shift+digit reports the symbol ("!" on a US layout, "\"" on a German
    // one); the hook triggers on the digit key, so take it from the code.
    const digit = /^Digit(\d)$/.exec(event.code ?? '');
    if (digit) {
      keyName = digit[1];
    }
    // Function keys come as "F1", "F12" etc.
    if (/^f\d{1,2}$/i.test(event.key)) {
      keyName = event.key.toLowerCase();
    }

    parts.push(keyName);
    const combo = parts.join('+');

    this.value = combo;
    this.valueChange.emit(combo);
    this.recording = false;
    void this.wails.setShortcutPaused(false);
  }
}
