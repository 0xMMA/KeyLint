import { ChangeDetectionStrategy, Component, input, model, output, signal } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent, IconComponent, InputDirective, MessageComponent, SelectComponent } from '../../../shared/ui';

export interface LabelledOption { label: string; value: string }
export interface ModelOption { id: string; label: string }

/**
 * The Pyramidize options column (mockup "Pyramidize", left): provider and
 * model, document type, style, relationship, a one-off instruction, the
 * Pyramidize button and the Advanced section.
 *
 * Presentational: the page owns the state (module-level, so it survives
 * navigation) and every decision; this only shows it and reports what the
 * user did. Values bound with [(…)] are written back as the user changes
 * them; `providerPicked` and `docTypePicked` fire after, on user action only.
 */
@Component({
  selector: 'app-pyramidize-options',
  standalone: true,
  imports: [FormsModule, ButtonComponent, IconComponent, InputDirective, MessageComponent, SelectComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  host: { class: 'pz-options', role: 'region', 'aria-label': 'Pyramidize options' },
  template: `
    <h1 class="pz-title">Pyramidize</h1>

    @if (showBanner()) {
      <kl-message data-testid="api-key-banner" severity="warn" closable (closed)="dismissBanner.emit()">
        <span data-testid="api-key-banner-message">{{ bannerMessage() }}</span>
      </kl-message>
    }

    <div class="pz-field">
      <label class="pz-label" for="pz-provider">Provider</label>
      <kl-select
        data-testid="provider-select"
        inputId="pz-provider"
        [options]="providerOptions()"
        optionLabel="label"
        optionValue="value"
        placeholder="Choose a provider"
        [ngModel]="provider()"
        (changed)="onProviderPicked($event)"
      />
      @if (sessionOverrideOf(); as settingsLabel) {
        <small class="pz-hint" data-testid="provider-session-override">
          Session override — Settings uses {{ settingsLabel }}.
          <button type="button" class="pz-link" data-testid="provider-follow-settings" (click)="followSettings.emit()">Use {{ settingsLabel }}</button>
        </small>
      }
      @if (settingsProviderUnusable(); as name) {
        <small class="pz-hint" data-testid="provider-forced-note">
          Settings names {{ name }}, which is not available yet, so Pyramidize uses this provider for now.
        </small>
      }
      @if (unavailableProvider(); as name) {
        <kl-message data-testid="provider-unavailable" severity="warn">
          {{ name }} is not available yet. Choose another provider here, or in Settings to keep it.
        </kl-message>
      }
    </div>

    <div class="pz-field">
      <label class="pz-label" for="pz-model">Model</label>
      <kl-select
        data-testid="model-select"
        inputId="pz-model"
        [options]="modelOptions()"
        optionLabel="label"
        optionValue="id"
        [(ngModel)]="model"
      />
      @if (modelListNote()) {
        <small class="pz-hint" data-testid="model-list-note">{{ modelListNote() }}</small>
      }
    </div>

    <div class="pz-field">
      <label class="pz-label" for="pz-doc-type">Document type</label>
      <kl-select
        data-testid="doc-type-select"
        inputId="pz-doc-type"
        [options]="docTypeOptions()"
        optionLabel="label"
        optionValue="value"
        [(ngModel)]="docType"
        (changed)="docTypePicked.emit()"
      />
      @if (detectedType()) {
        <small class="pz-hint" data-testid="detected-type">Detected: <span class="pz-detected">{{ detectedType() }}</span></small>
      }
    </div>

    <div class="pz-field">
      <label class="pz-label" for="pz-style">Style</label>
      <kl-select data-testid="comm-style-select" inputId="pz-style" [options]="commStyleOptions()"
        optionLabel="label" optionValue="value" [(ngModel)]="commStyle" />
    </div>
    <div class="pz-field">
      <label class="pz-label" for="pz-relationship">Relationship</label>
      <kl-select data-testid="rel-level-select" inputId="pz-relationship" [options]="relLevelOptions()"
        optionLabel="label" optionValue="value" [(ngModel)]="relLevel" />
    </div>

    <div class="pz-field">
      <label class="pz-label" for="pz-instructions">Instructions for this text</label>
      <textarea
        klInput
        id="pz-instructions"
        data-testid="custom-instructions"
        rows="3"
        class="resize-none"
        placeholder="Optional, e.g. keep it under 100 words"
        [(ngModel)]="customInstructions"
        (keydown.control.enter)="$event.preventDefault(); run()"
      ></textarea>
    </div>

    <kl-button
      data-testid="pyramidize-btn"
      class="mt-1"
      variant="primary"
      size="lg"
      icon="pyramid"
      block
      [loading]="loading()"
      [disabled]="!canPyramidize()"
      (click)="run()"
    >{{ hasResult() ? 'Pyramidize again' : 'Pyramidize' }}<span klButtonEnd><kbd>Ctrl</kbd> <kbd>Enter</kbd></span></kl-button>

    <div class="pz-advanced">
      <button type="button" class="pz-advanced-toggle" data-testid="advanced-toggle"
        [attr.aria-controls]="advancedOpen() ? 'pz-advanced-body' : null" [attr.aria-expanded]="advancedOpen()" (click)="toggleAdvanced.emit()">
        <kl-icon [name]="advancedOpen() ? 'chevron-down' : 'chevron-right'" />
        Advanced
      </button>
      @if (advancedOpen()) {
        <div id="pz-advanced-body" class="pz-field pt-3" data-testid="advanced-body">
          <label class="pz-label" for="pz-threshold">Quality threshold</label>
          <div class="flex items-center gap-2">
            <input
              klInput
              id="pz-threshold"
              data-testid="quality-threshold-input"
              type="number"
              min="0"
              max="1"
              step="0.05"
              class="w-24"
              aria-describedby="pz-threshold-hint"
              [invalid]="thresholdInvalid()"
              [value]="qualityThreshold()"
              (input)="checkThreshold($event)"
              (change)="saveThreshold($event)"
            />
            <span class="pz-hint">0–1</span>
          </div>
          <small id="pz-threshold-hint" class="pz-hint">Scores below this trigger a refinement pass (default 0.65).</small>
        </div>
      }
    </div>
  `,
  styles: [`
    :host {
      display: flex; flex-direction: column; gap: 18px;
      width: 300px; flex: none; padding: 32px 24px; overflow-y: auto;
      border-right: 1px solid var(--kl-line);
    }
    .pz-title { margin: 0 0 4px; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .pz-field { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
    .pz-label { font-size: 13px; font-weight: 500; color: var(--kl-muted); }
    .pz-hint { font-size: 12px; line-height: 1.45; color: var(--kl-muted); }
    .pz-detected { color: var(--kl-primary-text); font-weight: 600; }
    .pz-link {
      font: inherit; padding: 0; margin-left: 4px; border: 0; background: none;
      color: var(--kl-link); cursor: pointer; text-decoration: underline; text-underline-offset: 2px;
    }
    .pz-link:hover { color: var(--kl-primary-text); }
    .pz-link:focus-visible, .pz-advanced-toggle:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; border-radius: 2px; }
    .pz-advanced { border-top: 1px solid var(--kl-line); padding-top: 12px; }
    .pz-advanced-toggle {
      display: flex; align-items: center; gap: 6px; width: 100%; padding: 4px 0;
      font: inherit; font-size: 13px; font-weight: 500; color: var(--kl-muted);
      background: none; border: 0; cursor: pointer;
    }
    .pz-advanced-toggle:hover { color: var(--kl-text); }
  `],
})
export class OptionsPanelComponent {
  readonly providerOptions = input<readonly LabelledOption[]>([]);
  /** The provider in use; a pick goes out through `providerPicked`. */
  readonly provider = input('');
  readonly sessionOverrideOf = input<string | null>(null);
  readonly settingsProviderUnusable = input<string | null>(null);
  readonly unavailableProvider = input<string | null>(null);

  readonly modelOptions = input<readonly ModelOption[]>([]);
  readonly model = model('');
  readonly modelListNote = input('');

  readonly docTypeOptions = input<readonly LabelledOption[]>([]);
  readonly docType = model('auto');
  /** The type an automatic detection found, as a label; '' for none. */
  readonly detectedType = input('');
  readonly commStyleOptions = input<readonly LabelledOption[]>([]);
  readonly commStyle = model('professional');
  readonly relLevelOptions = input<readonly LabelledOption[]>([]);
  readonly relLevel = model('professional');
  readonly customInstructions = model('');

  readonly canPyramidize = input(false);
  readonly loading = input(false);
  /** The editor already holds a result: the button offers to run again. */
  readonly hasResult = input(false);

  readonly showBanner = input(false);
  readonly bannerMessage = input('');

  readonly advancedOpen = input(false);
  readonly qualityThreshold = model(0.65);

  readonly providerPicked = output<string>();
  readonly followSettings = output<void>();
  readonly docTypePicked = output<void>();
  readonly pyramidize = output<void>();
  readonly dismissBanner = output<void>();
  readonly toggleAdvanced = output<void>();
  /** A valid threshold (0–1) was committed (blur, Enter, spinner); it is in `qualityThreshold` already. */
  readonly thresholdChanged = output<number>();

  /** The threshold field holds something that is not a number from 0 to 1. */
  protected readonly thresholdInvalid = signal(false);

  /** The button and Ctrl+Enter in the instruction field: only when the button could be pressed. */
  protected run(): void {
    if (this.canPyramidize()) this.pyramidize.emit();
  }

  /**
   * Picking the provider already in use is no switch: kl-select reports every
   * pick, and a switch resets the model to the default.
   */
  protected onProviderPicked(value: unknown): void {
    if (typeof value === 'string' && value !== this.provider()) this.providerPicked.emit(value);
  }

  /** Marks the field while typed; nothing is saved until the value is committed. */
  protected checkThreshold(event: Event): void {
    this.thresholdInvalid.set(parseThreshold((event.target as HTMLInputElement).value) === null);
  }

  protected saveThreshold(event: Event): void {
    const value = parseThreshold((event.target as HTMLInputElement).value);
    this.thresholdInvalid.set(value === null);
    if (value === null || value === this.qualityThreshold()) return;
    this.qualityThreshold.set(value);
    this.thresholdChanged.emit(value);
  }
}

/** A threshold from 0 to 1, or null for anything else (empty, half typed, out of range). */
function parseThreshold(raw: string): number | null {
  if (raw.trim() === '') return null;
  const value = Number(raw);
  return Number.isFinite(value) && value >= 0 && value <= 1 ? value : null;
}
