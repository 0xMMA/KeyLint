import { Component, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent, MessageComponent, SelectComponent, SwitchComponent } from '../../../shared/ui';
import type { Settings as AppSettings } from '../../../core/wails.service';
import { SettingsSectionComponent } from '../settings-section/settings-section.component';
import { SettingRowComponent } from '../setting-row/setting-row.component';
import { ShortcutRecorderComponent } from '../shortcut-recorder/shortcut-recorder.component';

/**
 * Settings › General: where the provider is chosen, the shortcuts, start on
 * boot and logging.
 *
 * Edits the settings object it is given in place; the settings page saves it
 * with its Save button. The provider is only named here — it is chosen on the
 * AI Providers tab, next to the keys and the CLI status that decide whether it
 * works — so this reports the pointer's click and the page switches the tab.
 */
@Component({
  selector: 'app-settings-general',
  standalone: true,
  imports: [
    FormsModule, ButtonComponent, MessageComponent, SelectComponent, SwitchComponent,
    SettingsSectionComponent, SettingRowComponent, ShortcutRecorderComponent,
  ],
  template: `
    @let s = settings();
    <div class="flex flex-col gap-7">
      <app-settings-section>
        <app-setting-row data-testid="provider-pointer" label="AI provider">
          <span hint data-testid="provider-pointer-text">
            @if (unavailableProvider()) {
              No provider in use
            } @else if (providerLabel(); as label) {
              Using: <strong class="font-semibold text-fg">{{ label }}</strong>
            } @else {
              No provider chosen yet
            }
          </span>
          <!-- An action, not a link: it switches the tab, so it is a button. -->
          <kl-button data-testid="provider-pointer-link" size="sm" (click)="openProviders.emit()">Change under AI providers</kl-button>
          @if (unavailableProvider(); as name) {
            <kl-message below class="basis-full" data-testid="provider-unavailable" severity="warn">
              {{ name }} is saved but not available yet, so KeyLint has no provider in use. Pick another one under AI Providers.
            </kl-message>
          } @else if (providerProblem(); as why) {
            <kl-message below class="basis-full" data-testid="provider-pointer-problem" severity="warn">{{ why }}</kl-message>
          }
        </app-setting-row>
      </app-settings-section>

      <app-settings-section heading="Shortcuts" description="The global hotkey works in any app: select text, press it, and the fixed text goes back in place.">
        <app-setting-row data-testid="shortcut-mode-section" label="Double-tap mode" forId="double-tap-mode"
          [hint]="s.shortcut_mode === 'double_tap'
            ? 'Hold your modifier keys, tap the trigger key once for Fix, twice for Pyramidize.'
            : 'Assign separate shortcuts for each action.'">
          <kl-switch
            inputId="double-tap-mode"
            ariaLabel="Double-tap mode"
            [ngModel]="s.shortcut_mode === 'double_tap'"
            (ngModelChange)="s.shortcut_mode = $event ? 'double_tap' : 'independent'"
          />
        </app-setting-row>

        @if (s.shortcut_mode === 'double_tap') {
          <app-setting-row data-testid="shortcut-fix-section" label="Shortcut" hint="Single tap → Fix · Double tap → Pyramidize">
            <app-shortcut-recorder ariaLabel="Shortcut" [value]="s.shortcut_fix" (valueChange)="s.shortcut_fix = $event" />
          </app-setting-row>
          <app-setting-row data-testid="shortcut-delay-section" label="Double-tap delay" forId="double-tap-delay"
            hint="How long to wait for a second tap. Lower is faster but harder to hit.">
            <input
              id="double-tap-delay"
              type="range"
              class="delay"
              min="100"
              max="500"
              step="25"
              [attr.aria-valuetext]="s.shortcut_double_tap_delay + ' milliseconds'"
              [(ngModel)]="s.shortcut_double_tap_delay"
            />
            <output for="double-tap-delay" class="w-16 text-right text-sm font-semibold tabular-nums" data-testid="double-tap-delay-value">{{ s.shortcut_double_tap_delay }} ms</output>
          </app-setting-row>
        } @else {
          <app-setting-row data-testid="shortcut-fix-section" label="Fix shortcut" hint="Silently fixes clipboard text.">
            <app-shortcut-recorder ariaLabel="Fix shortcut" [value]="s.shortcut_fix" (valueChange)="s.shortcut_fix = $event" />
          </app-setting-row>
          <app-setting-row data-testid="shortcut-pyramidize-section" label="Pyramidize shortcut" hint="Opens the Pyramidize editor with clipboard text.">
            <app-shortcut-recorder ariaLabel="Pyramidize shortcut" [value]="s.shortcut_pyramidize" (valueChange)="s.shortcut_pyramidize = $event" />
          </app-setting-row>
        }
      </app-settings-section>

      <app-settings-section heading="Startup and logging">
        <app-setting-row data-testid="start-on-boot-section" label="Start on boot" forId="start-on-boot">
          <kl-switch inputId="start-on-boot" ariaLabel="Start on boot" [(ngModel)]="s.start_on_boot" />
        </app-setting-row>
        <app-setting-row data-testid="log-level-section" label="Log level" forId="log-level">
          <span hint>Writes to <code>~/.config/KeyLint/debug.log</code> (Linux) or <code>%AppData%/KeyLint/debug.log</code> (Windows). Takes effect on next launch.</span>
          <kl-select
            class="w-44"
            inputId="log-level"
            ariaLabel="Log level"
            [options]="logLevels"
            optionLabel="label"
            optionValue="value"
            [(ngModel)]="s.log_level"
          />
        </app-setting-row>
        <app-setting-row data-testid="sensitive-logging-section" label="Sensitive logging" forId="sensitive-logging">
          <span hint>Logs full API request payloads and responses. <strong class="text-soft">Do not share the log file while this is enabled.</strong> Takes effect on next launch.</span>
          <kl-switch inputId="sensitive-logging" ariaLabel="Sensitive logging" [(ngModel)]="s.sensitive_logging" [disabled]="s.log_level === 'off'" />
        </app-setting-row>
      </app-settings-section>
    </div>
  `,
  styles: [`
    .delay { width: 200px; accent-color: var(--kl-primary); cursor: pointer; }
    .delay:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 4px; border-radius: 4px; }
    code { font-size: 12px; padding: 1px 4px; border-radius: 3px; background: var(--kl-selected); color: var(--kl-text-soft); }
  `],
})
export class SettingsGeneralComponent {
  /** Edited in place; saved by the settings page. */
  readonly settings = input.required<AppSettings>();
  /** The provider in use, as the AI Providers tab names it; null when none of the offered ones is. */
  readonly providerLabel = input<string | null>(null);
  /** A saved provider that is not offered yet (Bedrock), by name. */
  readonly unavailableProvider = input<string | null>(null);
  /** Why the provider in use cannot work right now. */
  readonly providerProblem = input<string | null>(null);

  /** The pointer was pressed: show the AI Providers tab. */
  readonly openProviders = output<void>();

  readonly logLevels = [
    { label: 'Off', value: 'off' },
    { label: 'Trace', value: 'trace' },
    { label: 'Debug', value: 'debug' },
    { label: 'Info', value: 'info' },
    { label: 'Warning', value: 'warning' },
    { label: 'Error', value: 'error' },
  ];
}
