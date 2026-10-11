import { ChangeDetectionStrategy, Component, input, model, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { CdkScrollable } from '@angular/cdk/scrolling';
import {
  ButtonComponent, IconComponent, InputDirective, MessageComponent, SelectComponent, TooltipDirective,
} from '../../../shared/ui';
import type { EffortOption } from '../../settings/model-options';

export interface LabelledOption { label: string; value: string }
export interface ModelOption { id: string; label: string }

/**
 * The Pyramidize options column (mockup "Pyramidize", left): provider,
 * model and effort, document type, style, relationship, a one-off
 * instruction and the Pyramidize button.
 *
 * Presentational: the page owns the state (module-level, so it survives
 * navigation) and every decision; this only shows it and reports what the
 * user did. Values bound with [(…)] are written back as the user changes
 * them; `providerPicked` and `docTypePicked` fire after, on user action only.
 */
@Component({
  selector: 'app-pyramidize-options',
  standalone: true,
  imports: [FormsModule, ButtonComponent, IconComponent, InputDirective, MessageComponent, SelectComponent, TooltipDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  // The column scrolls on its own; cdkScrollable lets an open select follow it.
  hostDirectives: [CdkScrollable],
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

    <!-- Only where the provider does something with effort (not Ollama). -->
    @if (effortNote(); as note) {
      <div class="pz-field">
        <!-- What effort does sits in a tooltip: as a line of text under the
             picker it pushed the column past the default window height. -->
        <div class="pz-label-row">
          <label class="pz-label" for="pz-effort">Effort</label>
          <button type="button" class="pz-info" data-testid="effort-info" aria-label="What effort does"
            [klTooltip]="note" klTooltipPosition="right" klTooltipOnClick><kl-icon name="info" [size]="14" /></button>
        </div>
        <!-- The same text for a screen reader, read where the user is: on the picker. -->
        <span id="pz-effort-note" class="sr-only" data-testid="effort-note">{{ note }}</span>
        <kl-select
          data-testid="effort-select"
          inputId="pz-effort"
          ariaDescribedBy="pz-effort-note"
          [options]="effortOptions()"
          optionLabel="label"
          optionValue="value"
          [(ngModel)]="effort"
        />
      </div>
    }

    <div class="pz-field">
      <!-- What detection found sits on the label's line: a line of its own
           pushed the column past the default window height after a run. -->
      <div class="pz-label-row">
        <label class="pz-label" for="pz-doc-type">Document type</label>
        @if (detectedType()) {
          <small class="pz-hint pz-detected-hint" data-testid="detected-type">Detected: <span class="pz-detected">{{ detectedType() }}</span></small>
        }
      </div>
      <kl-select
        data-testid="doc-type-select"
        inputId="pz-doc-type"
        [options]="docTypeOptions()"
        optionLabel="label"
        optionValue="value"
        [(ngModel)]="docType"
        (changed)="docTypePicked.emit()"
      />
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

    <div class="pz-field pz-field-grow">
      <label class="pz-label" for="pz-instructions">Instructions for this text</label>
      <textarea
        klInput
        id="pz-instructions"
        data-testid="custom-instructions"
        rows="1"
        class="pz-instructions resize-none"
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
  `,
  styles: [`
    :host {
      /* Fits the default window (1280×800) without scrolling with every
         provider: e2e/pyramidize-options-fit.spec.ts. The instruction field
         takes what height is left: usually a few lines, one when a hint or
         two take the room. */
      display: flex; flex-direction: column; gap: 14px;
      width: 300px; flex: none; padding: 24px; overflow-y: auto;
      border-right: 1px solid var(--kl-line);
    }
    .pz-title { margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.015em; color: var(--kl-text-strong); }
    .pz-field { display: flex; flex-direction: column; gap: 6px; min-width: 0; }
    .pz-field-grow { flex: 1 0 auto; }
    /* Fills the height left, up to about ten lines; the button stays at the bottom. */
    .pz-instructions { flex: 1; max-height: 14rem; }
    .pz-detected-hint { margin-left: auto; }
    .pz-label-row { display: flex; align-items: center; gap: 6px; }
    .pz-info {
      display: inline-flex; padding: 2px; border: 0; border-radius: 4px; background: none;
      color: var(--kl-muted); cursor: help;
    }
    .pz-info:hover { color: var(--kl-text); }
    .pz-info:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 1px; }
    .pz-label { font-size: 13px; font-weight: 500; color: var(--kl-muted); }
    .pz-hint { font-size: 12px; line-height: 1.45; color: var(--kl-muted); }
    .pz-detected { color: var(--kl-primary-text); font-weight: 600; }
    .pz-link {
      font: inherit; padding: 0; margin-left: 4px; border: 0; background: none;
      color: var(--kl-link); cursor: pointer; text-decoration: underline; text-underline-offset: 2px;
    }
    .pz-link:hover { color: var(--kl-primary-text); }
    .pz-link:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; border-radius: 2px; }
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
  readonly effortOptions = input<readonly EffortOption[]>([]);
  readonly effort = model('');
  /** What effort does on this provider; null hides the picker (it does nothing there). */
  readonly effortNote = input<string | null>(null);

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

  readonly providerPicked = output<string>();
  readonly followSettings = output<void>();
  readonly docTypePicked = output<void>();
  readonly pyramidize = output<void>();
  readonly dismissBanner = output<void>();

  /** The button and Ctrl+Enter in the instruction field: only when the button could be pressed. */
  protected run(): void {
    if (this.canPyramidize()) this.pyramidize.emit();
  }

  /**
   * A switch resets the model to the default. Picking the provider already in
   * use is no switch, and kl-select does not report it (as p-select did not).
   */
  protected onProviderPicked(value: unknown): void {
    if (typeof value === 'string') this.providerPicked.emit(value);
  }
}
