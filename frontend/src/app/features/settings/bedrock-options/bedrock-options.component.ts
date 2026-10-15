import { ChangeDetectorRef, Component, computed, effect, inject, input, output, signal } from '@angular/core';
import { FormControl, ReactiveFormsModule } from '@angular/forms';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { InputDirective, MessageComponent, SelectComponent } from '../../../shared/ui';
import type { BedrockRegion, ModelInfo } from '../../../core/wails.service';
import {
  BEDROCK_AUTH_KEY, BEDROCK_AUTH_SIGNIN, BEDROCK_ROUTING_GEO, BEDROCK_ROUTING_GLOBAL, BedrockConnection, bedrockBaseModel,
} from '../../../core/bedrock';

/** A region as the picker lists it: its name, and its code on the second line. */
interface RegionOption {
  id: string;
  label: string;
}

const NO_MODELS: readonly ModelInfo[] = [];

/**
 * How KeyLint reaches AWS Bedrock: the connection (a key, or this computer's
 * AWS sign-in with an optional profile), the region, and where AWS may process
 * the text. Shared by the AWS Bedrock card on Settings › AI providers and the
 * setup wizard; each decides when to save what it emits (the card at once, the
 * wizard on Start).
 *
 * Routing is the user's explicit choice (owner decision 2026-10-15), worded for
 * a user rather than as AWS's "inference profiles": keep the text in the
 * region's geography (preselected), or worldwide — which says plainly that the
 * text can leave that geography, and that it is somewhat cheaper. A region with
 * no geography offers only worldwide, and says so. A model in use that cannot
 * stay in the geography is named, because the backend then refuses rather than
 * quietly sending it worldwide.
 *
 * The key itself is not here: each host has its own key field.
 */
@Component({
  selector: 'app-bedrock-options',
  standalone: true,
  imports: [ReactiveFormsModule, InputDirective, MessageComponent, SelectComponent],
  template: `
    @let c = draft();
    <div class="flex flex-col gap-4" data-testid="bedrock-options">
      <fieldset class="group">
        <legend>How KeyLint connects</legend>
        <label class="choice">
          <input type="radio" [name]="idPrefix() + '-auth'" data-testid="bedrock-auth-key"
            [checked]="c.auth !== signInValue" (change)="patch({ auth: keyValue })" />
          <span class="flex flex-col gap-0.5">
            <span class="choice-title">A Bedrock API key</span>
            <span class="choice-note">Kept in your system keyring, like the other keys.</span>
          </span>
        </label>
        <label class="choice">
          <input type="radio" [name]="idPrefix() + '-auth'" data-testid="bedrock-auth-signin"
            [checked]="c.auth === signInValue" (change)="patch({ auth: signInValue })" />
          <span class="flex flex-col gap-0.5">
            <span class="choice-title">This computer's AWS sign-in</span>
            <span class="choice-note">The profile or SSO session the AWS CLI uses on this computer. KeyLint stores nothing.</span>
          </span>
        </label>
      </fieldset>

      @if (c.auth === signInValue) {
        <div class="flex flex-col gap-1.5">
          <label class="field-label" [for]="idPrefix() + '-profile'">AWS profile <span class="font-normal">(optional)</span></label>
          <input klInput class="max-w-sm" [id]="idPrefix() + '-profile'" data-testid="bedrock-profile"
            autocomplete="off" spellcheck="false" placeholder="default"
            [formControl]="profile" (blur)="commitProfile()" (keydown.enter)="commitProfile()" />
          <small class="hint">Leave empty for your default profile. If your SSO session has expired, run <code>aws sso login</code> in a terminal.</small>
        </div>
        @if (envToken()) {
          <kl-message data-testid="bedrock-env-token" severity="info">
            <code>AWS_BEARER_TOKEN_BEDROCK</code> is set in your environment, so Bedrock calls use that key instead of your
            AWS sign-in, as AWS's own tools do. Remove the variable to use your sign-in.
          </kl-message>
        }
      }

      <div class="flex flex-col gap-1.5">
        <label class="field-label" [for]="idPrefix() + '-region'">Region</label>
        <kl-select class="max-w-sm" data-testid="bedrock-region" [inputId]="idPrefix() + '-region'"
          [options]="regionOptions()" optionLabel="label" optionValue="id" optionDisplay="label" optionDescription="id"
          placeholder="Choose a region" [formControl]="region" />
      </div>

      @if (selectedRegion(); as r) {
        @if (r.geography) {
          <fieldset class="group" data-testid="bedrock-routing">
            <legend>Where AWS may process your text</legend>
            <label class="choice">
              <input type="radio" [name]="idPrefix() + '-routing'" data-testid="bedrock-routing-geo"
                [checked]="c.routing !== globalValue" (change)="patch({ routing: geoValue })" />
              <span class="flex flex-col gap-0.5">
                <span class="choice-title">Keep in {{ r.geography_name }}</span>
                <span class="choice-note">AWS processes your text only in its data centres in {{ r.geography_name }}.</span>
              </span>
            </label>
            <label class="choice">
              <input type="radio" [name]="idPrefix() + '-routing'" data-testid="bedrock-routing-global"
                [checked]="c.routing === globalValue" (change)="patch({ routing: globalValue })" />
              <span class="flex flex-col gap-0.5">
                <span class="choice-title">Worldwide</span>
                <span class="choice-note">AWS may process your text in any of its regions, outside {{ r.geography_name }} too. About 10% cheaper.</span>
              </span>
            </label>
          </fieldset>
          @for (note of cannotStay(); track note) {
            <kl-message data-testid="bedrock-cannot-stay" severity="warn">{{ note }}</kl-message>
          }
        } @else {
          <kl-message data-testid="bedrock-worldwide-only" severity="warn">
            In {{ r.label }}, AWS offers these Claude models only with worldwide routing: your text may be processed in
            any AWS region. To keep it in one place, choose a region in the EU, the US, Canada, Australia or Japan.
          </kl-message>
        }
      }
    </div>
  `,
  styles: [`
    :host { display: block; }
    .group { margin: 0; padding: 0; border: 0; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
    legend, .field-label { padding: 0; margin-bottom: 2px; font-size: 13px; font-weight: 600; color: var(--kl-text-soft); }
    .choice { display: flex; align-items: flex-start; gap: 10px; cursor: pointer; }
    .choice input { flex: none; width: 16px; height: 16px; margin: 2px 0 0; accent-color: var(--kl-primary); }
    .choice input:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 3px; }
    .choice-title { font-size: 14px; color: var(--kl-text-strong); }
    .choice-note, .hint { font-size: 13px; line-height: 1.45; color: var(--kl-muted); }
    code { font-size: 12px; padding: 1px 4px; border-radius: 3px; background: var(--kl-selected); color: var(--kl-text-soft); }
  `],
})
export class BedrockOptionsComponent {
  /** The connection as saved (or, in the wizard, as chosen so far). */
  readonly connection = input.required<BedrockConnection>();
  readonly regions = input<readonly BedrockRegion[]>([]);
  /** The models KeyLint knows on Bedrock, for their names. */
  readonly knownModels = input<readonly ModelInfo[]>(NO_MODELS);
  /** The models Fix and Pyramidize run on, as stored or defaulted. */
  readonly fixModel = input('');
  readonly pyramidizeModel = input('');
  /** AWS_BEARER_TOKEN_BEDROCK is set: in sign-in mode it wins (see llm/bedrock.go). */
  readonly envToken = input(false);
  /** Keeps the radio groups and field IDs apart where two of these could meet. */
  readonly idPrefix = input('bedrock');

  readonly connectionChange = output<BedrockConnection>();

  readonly keyValue = BEDROCK_AUTH_KEY;
  readonly signInValue = BEDROCK_AUTH_SIGNIN;
  readonly geoValue = BEDROCK_ROUTING_GEO;
  readonly globalValue = BEDROCK_ROUTING_GLOBAL;

  /**
   * What the controls show: the connection, with the user's last change on
   * top until the host answers. A host that could not save hands the old
   * connection back as a new object, and this follows it — a radio would
   * otherwise keep showing the click that did not stick.
   */
  readonly draft = signal<BedrockConnection>({ region: '', auth: BEDROCK_AUTH_KEY, profile: '', routing: BEDROCK_ROUTING_GEO });

  private readonly cdr = inject(ChangeDetectorRef);

  readonly region = new FormControl<string | null>(null);
  readonly profile = new FormControl('', { nonNullable: true });

  readonly regionOptions = computed<RegionOption[]>(() => {
    const listed = this.regions().map(r => ({ id: r.id, label: r.label }));
    const saved = this.draft().region;
    // A region typed into a settings file that this build does not list is
    // still shown, by its code, rather than as an empty field.
    return saved && !listed.some(r => r.id === saved) ? [...listed, { id: saved, label: saved }] : listed;
  });

  readonly selectedRegion = computed(() => this.regions().find(r => r.id === this.draft().region) ?? null);

  /**
   * The models in use that cannot stay in the geography from this region. Only
   * models KeyLint knows (an unknown one gets the prefix and Bedrock judges
   * it), and only where the routing applies: an ID the user routed themselves
   * (eu.…, global.…, an ARN) is sent as it is.
   */
  readonly cannotStay = computed(() => {
    const r = this.selectedRegion();
    if (!r?.geography || this.draft().routing === BEDROCK_ROUTING_GLOBAL) return [];
    const notes: string[] = [];
    for (const [feature, id] of [['Fix', this.fixModel()], ['Pyramidize', this.pyramidizeModel()]] as const) {
      const known = this.knownModels().find(m => m.id === id);
      if (!known || bedrockBaseModel(id) !== id || r.geo_models.includes(id)) continue;
      notes.push(`${known.label || id} (${feature}) cannot stay in ${r.geography_name}, so ${feature} will not run on it here. Choose Worldwide, or another model for ${feature}.`);
    }
    return notes;
  });

  constructor() {
    effect(() => {
      const c = this.connection();
      this.draft.set({ ...c });
      this.region.setValue(c.region || null, { emitEvent: false });
      this.profile.setValue(c.profile, { emitEvent: false });
    });
    this.region.valueChanges.pipe(takeUntilDestroyed()).subscribe(id => this.patch({ region: id ?? '' }));
  }

  /**
   * Applies a change and tells the host. The view is brought in step first:
   * a host whose save fails at once hands the old connection back before any
   * other check has run, and a radio binding that never saw the click would
   * then find nothing changed and leave the click on screen.
   */
  patch(change: Partial<BedrockConnection>): void {
    const next = { ...this.draft(), ...change };
    this.draft.set(next);
    this.cdr.detectChanges();
    this.connectionChange.emit(next);
  }

  /** The profile field is saved when it is left, not on every keystroke. */
  commitProfile(): void {
    const value = this.profile.value.trim();
    if (value !== this.profile.value) this.profile.setValue(value, { emitEvent: false });
    if (value !== this.draft().profile) this.patch({ profile: value });
  }
}
