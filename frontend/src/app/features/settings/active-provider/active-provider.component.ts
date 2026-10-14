import { Component, computed, input, output, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { SelectComponent, TagComponent } from '../../../shared/ui';
import { EFFORT_OPTIONS, Feature, ModelOption } from '../model-options';

/** A model or effort the user picked, for the settings page to store. */
export interface FeatureChoice {
  feature: Feature;
  value: string;
}

interface FeatureRow {
  key: Feature;
  name: string;
}

const FEATURES: readonly FeatureRow[] = [
  { key: 'fix', name: 'Fix' },
  { key: 'pyramidize', name: 'Pyramidize' },
];

/**
 * The top of Settings › AI Providers: the provider KeyLint uses right now, and
 * per feature which of its models runs and how hard it may think.
 *
 * Only the active provider is shown. Every provider's choices are still kept
 * in settings, so switching away and back restores them — but a page listing
 * all four providers' pickers at once was the clutter this replaces.
 *
 * The settings page owns the values and saves them with its Save button; this
 * only renders them and reports a change. A warning about the provider goes in
 * with the `status` attribute and shows under the header.
 *
 * The header (the name and what it means) is the focus target of the General
 * tab's pointer, hence `providers-summary`.
 */
@Component({
  selector: 'app-active-provider',
  standalone: true,
  imports: [FormsModule, SelectComponent, TagComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="in-use" data-testid="active-provider" aria-labelledby="in-use-heading">
      <div class="head">
        <div class="summary" data-testid="providers-summary" tabindex="-1">
          <h2 id="in-use-heading" class="name" data-testid="active-provider-name">{{ label() }}</h2>
          <p class="lede">KeyLint sends your text here, for Fix and for Pyramidize.</p>
        </div>
        <kl-tag severity="primary" icon="check">In use</kl-tag>
      </div>

      <ng-content select="[status]" />

      @if (note(); as text) {
        <p class="hint" [attr.data-testid]="'models-note-' + providerId()">{{ text }}</p>
      }

      <div class="grid" [class.with-effort]="showEffort()">
        <span class="col-label"></span>
        <span class="col-label" id="in-use-model-label">Model</span>
        @if (showEffort()) {
          <span class="col-label" id="in-use-effort-label">Effort</span>
        }
        @for (f of features; track f.key) {
          <span class="feature" [id]="'feature-' + f.key">{{ f.name }}</span>

          <div class="model">
            <kl-select
              [attr.data-testid]="'model-' + f.key + '-' + providerId()"
              [ariaLabelledBy]="'feature-' + f.key + ' in-use-model-label'"
              [editable]="editable()"
              [options]="optionsFor(f.key)"
              optionLabel="label"
              optionValue="id"
              optionDisplay="display"
              optionDescription="secondary"
              [optionTestId]="optionTestId"
              [placeholder]="optionsFor(f.key)[0]?.display ?? ''"
              [ngModel]="modelFor(f.key) || null"
              (ngModelChange)="modelChange.emit({ feature: f.key, value: $event ?? '' })"
            />
            @if (effective(f.key); as runs) {
              <small class="effective" [attr.data-testid]="'model-effective-' + f.key">{{ runs }}</small>
            }
          </div>

          @if (showEffort()) {
            <kl-select
              [attr.data-testid]="'effort-' + f.key + '-' + providerId()"
              [ariaLabel]="f.name + ' effort'"
              [options]="effortOptions"
              optionLabel="label"
              optionValue="value"
              [ngModel]="effortFor(f.key)"
              (ngModelChange)="effortChange.emit({ feature: f.key, value: $event ?? '' })"
            />
          }
        }
      </div>

      @if (fixFastNote() || (showEffort() && effortNote())) {
        <div class="notes">
          @if (fixFastNote()) {
            <p class="hint" data-testid="fix-fast-note">
              Fix asks {{ fixModelName() }} to answer without reasoning first, so the hotkey stays quick. An effort lets it reason, but on a long text Fix can then run out of room.
            </p>
          }
          @if (showEffort() && effortNote(); as text) {
            <p class="hint" data-testid="effort-note">{{ text }}</p>
          }
        </div>
      }
    </section>
  `,
  styles: [`
    :host { display: block; }
    /* The one block on the tab that is about now: a sheet of its own, set
       apart from the connections list below it. */
    .in-use {
      display: flex; flex-direction: column; gap: 20px; padding: 24px 28px;
      background: var(--kl-sheet); border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet);
    }
    .head { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; }
    .summary { min-width: 0; border-radius: 4px; }
    .summary:focus { outline: none; }
    .summary:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 4px; }
    .name { margin: 0; font-size: 18px; font-weight: 700; color: var(--kl-text-strong); }
    .lede { margin: 4px 0 0; font-size: 14px; color: var(--kl-muted); }
    .hint { margin: 0; font-size: 13px; line-height: 1.5; color: var(--kl-muted); max-width: 72ch; }
    .notes { display: flex; flex-direction: column; gap: 6px; }

    .grid { display: grid; grid-template-columns: 110px minmax(0, 1fr); gap: 14px 16px; align-items: start; }
    .grid.with-effort { grid-template-columns: 110px minmax(0, 1fr) minmax(0, 200px); }
    .col-label { font-size: 13px; font-weight: 500; color: var(--kl-muted); margin-bottom: -6px; }
    /* Centred on the 40px select, not on the select with its note. */
    .feature { font-size: 15px; font-weight: 600; line-height: 40px; }
    .model { display: flex; flex-direction: column; gap: 4px; min-width: 0; }
    .effective { font-size: 12px; color: var(--kl-muted); overflow-wrap: anywhere; padding-left: 2px; }

    /* Narrow window: the feature name goes above its pickers. */
    @media (max-width: 600px) {
      .in-use { padding: 20px; }
      .grid, .grid.with-effort { grid-template-columns: minmax(0, 1fr) 9rem; }
      .grid:not(.with-effort) { grid-template-columns: minmax(0, 1fr); }
      .col-label { display: none; }
      .feature { grid-column: 1 / -1; line-height: 1.4; margin-bottom: -8px; }
    }
  `],
})
export class ActiveProviderComponent {
  readonly providerId = input.required<string>();
  readonly label = input.required<string>();
  /** Picker contents per feature, the "KeyLint default" entry first. */
  readonly fixOptions = input<ModelOption[]>([]);
  readonly pyramidizeOptions = input<ModelOption[]>([]);
  readonly fixModel = input('');
  readonly pyramidizeModel = input('');
  readonly fixEffort = input('');
  readonly pyramidizeEffort = input('');
  /** Whether a model outside the list can be typed. */
  readonly editable = input(true);
  /** Where the list came from, when that needs saying; see noteForModelSource. */
  readonly note = input('');
  /** Whether this provider does anything with effort. */
  readonly showEffort = input(false);
  readonly effortNote = input<string | null>(null);
  /** Fix will ask its model not to reason; see fixSkipsReasoning. */
  readonly fixFastNote = input(false);

  readonly modelChange = output<FeatureChoice>();
  readonly effortChange = output<FeatureChoice>();

  readonly features = FEATURES;
  readonly effortOptions = [...EFFORT_OPTIONS];
  /** A test id per model option; KeyLint's default is "model-option-default". */
  readonly optionTestId = (option: unknown): string => `model-option-${(option as ModelOption).id || 'default'}`;

  optionsFor(feature: Feature): ModelOption[] {
    return feature === 'fix' ? this.fixOptions() : this.pyramidizeOptions();
  }

  modelFor(feature: Feature): string {
    return feature === 'fix' ? this.fixModel() : this.pyramidizeModel();
  }

  effortFor(feature: Feature): string {
    return feature === 'fix' ? this.fixEffort() : this.pyramidizeEffort();
  }

  /** The Fix model's readable name, for the note that names it. */
  readonly fixModelName = computed(() => {
    const option = this.fixOptions().find(o => o.id === (this.fixModel() || null));
    const name = option?.display ?? this.fixModel();
    return name.replace(/^Default: /, '');
  });

  /**
   * The muted line under a picker: the model an alias — chosen, or standing
   * behind the default — resolves to today, where the listing says. The picker
   * itself already names the alias, so this adds only the ID.
   */
  effective(feature: Feature): string {
    const value = this.modelFor(feature) || null;
    const option = this.optionsFor(feature).find(o => o.id === value);
    return option?.secondary && option.secondary !== value ? `Now ${option.secondary}` : '';
  }
}
