import { Component, signal, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import {
  ButtonComponent, CardComponent, CheckboxComponent, ICONS, IconComponent, InputDirective, MessageComponent,
  SegmentedComponent, SelectComponent, SpinnerComponent, SwitchComponent, TabComponent, TabPanelComponent,
  TabsComponent, TagComponent, TooltipDirective, type IconName,
} from '../../../shared/ui';
import { defaultOption, EFFORT_OPTIONS, toModelOption, type ModelOption } from '../../settings/model-options';

const LISTED: ModelOption[] = [
  toModelOption({ id: 'haiku', label: 'Haiku (latest)', resolved: 'claude-haiku-5-5' }),
  toModelOption({ id: 'sonnet', label: 'Sonnet (latest)', resolved: 'claude-sonnet-5-5' }),
  toModelOption({ id: 'claude-sonnet-4-6', label: 'Claude Sonnet 4.6', resolved: '' }),
];

/**
 * Every shared UI primitive in its states, on one page (/dev-tools/ui), for
 * checking the look against docs/ui/design.md and for screenshots. Not linked
 * from the main navigation.
 */
@Component({
  selector: 'app-ui-gallery',
  standalone: true,
  imports: [
    FormsModule, ButtonComponent, CardComponent, CheckboxComponent, IconComponent, InputDirective, MessageComponent,
    SegmentedComponent, SelectComponent, SpinnerComponent, SwitchComponent, TabComponent, TabPanelComponent,
    TabsComponent, TagComponent, TooltipDirective,
  ],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <div class="gallery" data-testid="ui-gallery">
      <h1>UI primitives</h1>

      <section>
        <h2>Buttons</h2>
        <div class="row">
          <kl-button variant="primary" icon="copy">Copy result</kl-button>
          <kl-button variant="quiet" icon="refresh-cw">Check again</kl-button>
          <kl-button variant="ghost">Edit key</kl-button>
          <kl-button variant="danger" icon="trash">Reset to defaults</kl-button>
          <kl-button variant="primary" icon="sparkles" loading loadingLabel="Fixing…">Fix</kl-button>
          <kl-button variant="quiet" disabled>Disabled</kl-button>
          <kl-button variant="ghost" icon="x" iconOnly ariaLabel="Close" klTooltip="Close" klTooltipPosition="top" />
        </div>
        <div class="row">
          <kl-button variant="primary" size="lg" icon="sparkles">Large</kl-button>
          <kl-button variant="quiet" size="sm" icon="plus">Small</kl-button>
          <div class="w-72">
            <kl-button variant="primary" size="lg" icon="pyramid" block>Pyramidize
              <span klButtonEnd><kbd>Ctrl</kbd> <kbd>Enter</kbd></span>
            </kl-button>
          </div>
        </div>
      </section>

      <section>
        <h2>Messages</h2>
        <div class="stack">
          <kl-message severity="info" text="An info message: one line of context, nothing to do." />
          <kl-message severity="success" text="Fixed and copied to the clipboard." />
          <kl-message severity="warn" title="Fix took too long" closable>
            The hotkey fix ran out of time. Your text is unchanged on the clipboard.
          </kl-message>
          <kl-message severity="error" text="OpenAI rejected the key. Check it under Connections, then try again." />
          <kl-message severity="secondary" text="No models listed; type one." />
        </div>
      </section>

      <section>
        <h2>Tags, switches, checkbox</h2>
        <div class="row">
          <kl-tag severity="primary" icon="check">In use</kl-tag>
          <kl-tag>Haiku 5.5</kl-tag>
          <kl-tag severity="muted">Not set up</kl-tag>
          <kl-tag severity="success" value="signed in" />
          <kl-tag severity="warn" value="not signed in" />
          <kl-tag severity="info" value="key from env var" />
        </div>
        <div class="row">
          <kl-switch [(checked)]="on">Start on boot</kl-switch>
          <kl-switch [(checked)]="off">Sensitive logging</kl-switch>
          <kl-switch [checked]="true" disabled>Locked</kl-switch>
          <kl-checkbox [(checked)]="checked">Copy the result to the clipboard</kl-checkbox>
          <kl-spinner [size]="18" label="Loading" />
        </div>
      </section>

      <section>
        <h2>Fields</h2>
        <div class="grid">
          <label class="field"><span>Text</span><input klInput placeholder="sk-ant-…" /></label>
          <label class="field"><span>Invalid</span><input klInput invalid value="not-a-key" /></label>
          <label class="field"><span>Disabled</span><input klInput disabled value="Disabled" /></label>
          <label class="field"><span>Textarea</span><textarea klInput rows="3" placeholder="Optional, e.g. keep it under 100 words"></textarea></label>
          <div class="field">
            <span id="g-level">Select</span>
            <kl-select data-testid="gallery-select" ariaLabelledBy="g-level" [options]="effort" optionLabel="label" optionValue="value" [(ngModel)]="effortValue" />
          </div>
          <div class="field">
            <span id="g-model">Editable select (model picker)</span>
            <kl-select data-testid="gallery-editable" ariaLabelledBy="g-model" editable [options]="models"
              optionLabel="label" optionValue="id" optionDisplay="display" optionDescription="secondary"
              [placeholder]="models[0].display" [ngModel]="model() || null" (ngModelChange)="model.set($event ?? '')" />
          </div>
        </div>
      </section>

      <section>
        <h2>Tabs and segmented</h2>
        <kl-tabs [(value)]="tab" ariaLabel="Gallery tabs">
          <kl-tab value="general">General</kl-tab>
          <kl-tab value="providers">AI providers</kl-tab>
          <kl-tab value="about">About</kl-tab>
          <kl-button klTabsEnd variant="ghost" size="sm" icon="rotate-ccw-clock">History, 3 steps</kl-button>
          <kl-tab-panel value="general"><p class="muted">General panel</p></kl-tab-panel>
          <kl-tab-panel value="providers"><p class="muted">Providers panel</p></kl-tab-panel>
          <kl-tab-panel value="about"><p class="muted">About panel</p></kl-tab-panel>
        </kl-tabs>
        <div class="row">
          <kl-segmented ariaLabel="Result view" [options]="views" [(value)]="view" />
        </div>
      </section>

      <section>
        <h2>Card</h2>
        <kl-card heading="Claude Code" subheading="KeyLint sends your text here, for Fix and for Pyramidize.">
          <div class="row"><kl-tag severity="primary" icon="check">In use</kl-tag></div>
        </kl-card>
      </section>

      <section>
        <h2>Icons</h2>
        <div class="icons">
          @for (name of icons; track name) {
            <span class="icon-cell" [klTooltip]="name" klTooltipPosition="top" tabindex="0"><kl-icon [name]="name" [size]="20" /></span>
          }
        </div>
      </section>
    </div>
  `,
  styles: [`
    .gallery { padding: 36px 44px; display: flex; flex-direction: column; gap: 32px; max-width: 1100px; }
    h1 { margin: 0; font-size: 24px; font-weight: 700; }
    h2 { margin: 0 0 14px; font-size: 15px; font-weight: 700; color: var(--kl-text-strong); }
    .row { display: flex; flex-wrap: wrap; align-items: center; gap: 10px 12px; margin-top: 12px; }
    .stack { display: flex; flex-direction: column; gap: 10px; max-width: 640px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 16px; }
    .field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 500; color: var(--kl-muted); }
    .muted { color: var(--kl-muted); margin: 16px 0 0; }
    .icons { display: flex; flex-wrap: wrap; gap: 8px; }
    .icon-cell { display: inline-flex; padding: 8px; border-radius: 6px; background: var(--kl-sheet); color: var(--kl-text-soft); }
  `],
})
export class UiGalleryComponent {
  readonly on = signal(true);
  readonly off = signal(false);
  readonly checked = signal(true);
  readonly tab = signal('providers');
  readonly view = signal('changes');
  readonly model = signal('');
  effortValue = 'high';
  readonly effort = EFFORT_OPTIONS;
  readonly models: ModelOption[] = [defaultOption('claude', 'fix', LISTED), ...LISTED];
  readonly views = [{ value: 'changes', label: 'Show changes' }, { value: 'clean', label: 'Clean text' }];
  readonly icons = Object.keys(ICONS) as IconName[];
}
