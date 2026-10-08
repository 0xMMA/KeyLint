import { Component, computed, input, output } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Select } from 'primeng/select';
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
 * only renders them and reports a change.
 */
@Component({
  selector: 'app-active-provider',
  standalone: true,
  imports: [FormsModule, Select],
  template: `
    <section class="in-use" data-testid="active-provider" aria-labelledby="in-use-heading">
      <div class="in-use-header">
        <h2 id="in-use-heading" class="in-use-name" data-testid="active-provider-name">{{ label() }}</h2>
        <ng-content select="[status]" />
        <span class="in-use-mark"><i class="pi pi-check" aria-hidden="true"></i> In use</span>
      </div>

      @if (note(); as text) {
        <p class="in-use-note" [attr.data-testid]="'models-note-' + providerId()">{{ text }}</p>
      }

      <div class="feature-grid" [class.with-effort]="showEffort()">
        @for (f of features; track f.key) {
          <span class="feature-name" [id]="'feature-' + f.key">{{ f.name }}</span>

          <div class="feature-model">
            <p-select
              [attr.data-testid]="'model-' + f.key + '-' + providerId()"
              [ariaLabelledBy]="'feature-' + f.key"
              [editable]="editable()"
              [options]="optionsFor(f.key)"
              optionLabel="label"
              optionValue="id"
              styleClass="w-full"
              [placeholder]="optionsFor(f.key)[0]?.display ?? ''"
              [ngModel]="modelFor(f.key) || null"
              (ngModelChange)="modelChange.emit({ feature: f.key, value: $event ?? '' })"
            >
              <ng-template #item let-option>
                <!-- PrimeNG's option row is a flex row; this column keeps the
                     name and the ID on two lines instead of jammed together. -->
                <span class="model-option" [attr.data-testid]="'model-option-' + (option.id || 'default')">
                  <span class="model-option-name">{{ option.display }}</span>
                  @if (option.secondary) {
                    <span class="model-option-id">{{ option.secondary }}</span>
                  }
                </span>
              </ng-template>
              <ng-template #selectedItem let-option>
                <span class="model-selected">{{ option?.display || option?.id }}</span>
              </ng-template>
            </p-select>
            @if (effective(f.key); as runs) {
              <small class="model-effective" [attr.data-testid]="'model-effective-' + f.key">{{ runs }}</small>
            }
          </div>

          @if (showEffort()) {
            <p-select
              [attr.data-testid]="'effort-' + f.key + '-' + providerId()"
              [ariaLabel]="f.name + ' effort'"
              [options]="effortOptions"
              optionLabel="label"
              optionValue="value"
              styleClass="w-full"
              [ngModel]="effortFor(f.key)"
              (ngModelChange)="effortChange.emit({ feature: f.key, value: $event ?? '' })"
            />
          }
        }
      </div>

      @if (fixFastNote()) {
        <p class="in-use-hint" data-testid="fix-fast-note">
          Fix asks {{ fixModelName() }} to answer without reasoning first, so the hotkey stays quick. Choose an effort to let it reason.
        </p>
      }
      @if (showEffort() && effortNote(); as text) {
        <p class="in-use-hint" data-testid="effort-note">{{ text }}</p>
      }
    </section>
  `,
  styles: [`
    :host { display: block; }

    /* The one block on the tab that is about now: a raised surface and the
       accent rule set it apart from the connection cards below it. */
    .in-use {
      background: var(--p-content-hover-background);
      border: 1px solid var(--p-content-border-color);
      border-left: 3px solid var(--p-primary-color);
      border-radius: var(--p-border-radius-md, 6px);
      padding: 1rem 1.125rem 0.875rem;
      margin-bottom: 1.75rem;
    }
    .in-use-header {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      gap: 0.625rem;
      margin-bottom: 0.875rem;
    }
    .in-use-name {
      margin: 0;
      font-size: 1.15rem;
      font-weight: 600;
      letter-spacing: -0.005em;
    }
    .in-use-mark {
      margin-left: auto;
      color: var(--p-primary-color);
      font-size: 0.85rem;
      font-weight: 500;
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
    }

    .feature-grid {
      display: grid;
      grid-template-columns: 6.5rem minmax(0, 1fr);
      column-gap: 0.75rem;
      row-gap: 0.75rem;
      align-items: start;
    }
    .feature-grid.with-effort {
      grid-template-columns: 6.5rem minmax(0, 1fr) 10.5rem;
    }
    .feature-name {
      font-size: 0.9rem;
      font-weight: 500;
      /* Centred on the select's first line, not on the row with its note. */
      line-height: 2.5rem;
    }
    .feature-model {
      display: flex;
      flex-direction: column;
      gap: 0.25rem;
      min-width: 0;
    }
    .model-effective {
      font-size: 0.75rem;
      color: var(--p-text-muted-color);
      overflow-wrap: anywhere;
    }

    .in-use-note,
    .in-use-hint {
      margin: 0 0 0.75rem;
      font-size: 0.8rem;
      color: var(--p-text-muted-color);
      max-width: 70ch;
    }
    .in-use-hint { margin: 0.75rem 0 0; }
    .in-use-hint + .in-use-hint { margin-top: 0.35rem; }

    /* Narrow window: the feature name goes above its pickers. */
    @media (max-width: 560px) {
      .feature-grid,
      .feature-grid.with-effort {
        grid-template-columns: minmax(0, 1fr) 8.5rem;
      }
      .feature-grid:not(.with-effort) { grid-template-columns: minmax(0, 1fr); }
      .feature-name { grid-column: 1 / -1; line-height: 1.4; margin-bottom: -0.4rem; }
    }

    /* Declared in this template, so encapsulation reaches them inside the
       select's overlay too. */
    .model-option {
      display: flex;
      flex-direction: column;
      gap: 0.1rem;
      min-width: 0;
    }
    .model-option-id {
      font-size: 0.75rem;
      color: var(--p-text-muted-color);
    }
    .model-selected {
      display: block;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
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
