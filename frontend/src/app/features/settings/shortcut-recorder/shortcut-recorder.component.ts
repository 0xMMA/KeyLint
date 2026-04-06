import { ChangeDetectionStrategy, Component, Input, Output, EventEmitter, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ButtonModule } from 'primeng/button';
import { WailsService } from '../../../core/wails.service';

const MODIFIER_KEYS = new Set(['Control', 'Shift', 'Alt', 'Meta']);

const KEY_TO_NAME: Record<string, string> = {
  ' ': 'space', 'Enter': 'enter', 'Tab': 'tab', 'Escape': 'escape',
  'Backspace': 'backspace', 'Delete': 'delete', 'Insert': 'insert',
  'Home': 'home', 'End': 'end', 'PageUp': 'pageup', 'PageDown': 'pagedown',
  'ArrowUp': 'up', 'ArrowDown': 'down', 'ArrowLeft': 'left', 'ArrowRight': 'right',
};

function formatCombo(combo: string): string {
  if (!combo) return '';
  const parts = combo.split('+');
  return parts.map(p => {
    if (p === 'ctrl') return 'Ctrl';
    if (p === 'shift') return 'Shift';
    if (p === 'alt') return 'Alt';
    if (p === 'win') return 'Win';
    return p.length === 1 ? p.toUpperCase() : p.charAt(0).toUpperCase() + p.slice(1).toUpperCase();
  }).join(' + ');
}

@Component({
  selector: 'app-shortcut-recorder',
  standalone: true,
  imports: [CommonModule, ButtonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="recorder-wrapper" [class.recording]="recording" data-testid="recorder-field"
         tabindex="0" (keydown)="onKeyDown($event)">
      @if (recording) {
        <span class="recording-text" data-testid="recording-indicator">Press a key combo...</span>
      } @else {
        <span class="combo-text" data-testid="combo-display">{{ displayValue }}</span>
      }
      <button
        data-testid="record-btn"
        class="p-button p-button-sm"
        [class.p-button-danger]="recording"
        [class.p-button-secondary]="!recording"
        type="button"
        (click)="toggleRecording()"
      >{{ recording ? 'Cancel' : 'Record...' }}</button>
    </div>
  `,
  styles: [`
    .recorder-wrapper {
      display: flex;
      align-items: center;
      justify-content: space-between;
      border: 1px solid var(--p-content-border-color);
      border-radius: 6px;
      padding: 0.5rem 0.75rem;
      background: var(--p-content-hover-background);
      min-height: 2.5rem;
      outline: none;
    }
    .recorder-wrapper.recording {
      border-color: var(--p-primary-color);
      box-shadow: 0 0 0 1px var(--p-primary-color);
    }
    .combo-text {
      font-family: monospace;
      font-size: 0.9rem;
      color: var(--p-text-color);
    }
    .recording-text {
      font-size: 0.85rem;
      color: var(--p-text-muted-color);
      animation: pulse 1.5s ease-in-out infinite;
    }
    @keyframes pulse {
      0%, 100% { opacity: 1; }
      50% { opacity: 0.5; }
    }
  `],
})
export class ShortcutRecorderComponent {
  @Input() value = '';
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
