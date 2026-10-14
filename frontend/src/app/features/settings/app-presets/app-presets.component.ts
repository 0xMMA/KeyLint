import { ChangeDetectorRef, Component, effect, input, ChangeDetectionStrategy } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent, InputDirective, SelectComponent } from '../../../shared/ui';
import { WailsService, type AppPreset, type Settings as AppSettings } from '../../../core/wails.service';
import { DOCUMENT_TYPE_OPTIONS } from '../../../core/constants';
import { SettingsSectionComponent } from '../settings-section/settings-section.component';

/**
 * Settings › App Defaults: which document type Pyramidize assumes for text
 * that comes from a given app.
 *
 * Presets are saved by the backend the moment they change, outside the
 * settings page's Save. The page's settings object must follow, or the next
 * save — the Save button or a provider switch — writes back the list as it was
 * when the page opened and undoes the change; see syncPresets.
 */
@Component({
  selector: 'app-app-presets',
  standalone: true,
  imports: [FormsModule, ButtonComponent, InputDirective, SelectComponent, SettingsSectionComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <app-settings-section heading="App presets" description="Pyramidize preselects the document type for text the shortcut copies from these apps; you can always pick another. Presets are saved as soon as you change them.">
      @if (presets.length === 0 && !adding) {
        <p class="row m-0 text-sm text-muted" data-testid="presets-empty">No app presets yet. When the shortcut opens Pyramidize, the window title of the app you copied from is matched against the names here (whole words, the app's part of the title first) to preselect a document type. Add one below.</p>
      }
      @for (preset of presets; track preset.sourceApp) {
        @if (editing?.sourceApp === preset.sourceApp) {
          <div class="row" data-testid="preset-edit-row">
            <input klInput class="flex-1" aria-label="App name" [(ngModel)]="editDraft.sourceApp" />
            <kl-select class="w-40" ariaLabel="Document type" [options]="docTypeOptions" optionLabel="label" optionValue="value" [(ngModel)]="editDraft.documentType" />
            <kl-button variant="primary" size="sm" icon="check" iconOnly ariaLabel="Save preset" [disabled]="!editDraft.sourceApp.trim()" (click)="saveEdit()" />
            <kl-button size="sm" icon="x" iconOnly ariaLabel="Cancel" (click)="cancelEdit()" />
          </div>
        } @else {
          <div class="row" [attr.data-testid]="'preset-' + preset.sourceApp">
            <span class="flex-1 min-w-0 text-sm font-semibold [overflow-wrap:anywhere]">{{ preset.sourceApp }}</span>
            <span class="w-28 text-sm text-muted">{{ typeLabel(preset.documentType) }}</span>
            <kl-button variant="ghost" size="sm" icon="pencil" iconOnly [ariaLabel]="'Edit ' + preset.sourceApp" (click)="startEdit(preset)" />
            <kl-button variant="ghost" size="sm" icon="trash" iconOnly [ariaLabel]="'Delete ' + preset.sourceApp" (click)="remove(preset.sourceApp)" />
          </div>
        }
      }
      @if (adding) {
        <div class="row" data-testid="preset-add-row">
          <input klInput class="flex-1" aria-label="App name" placeholder="App name" [(ngModel)]="addDraft.sourceApp" />
          <kl-select class="w-40" ariaLabel="Document type" [options]="docTypeOptions" optionLabel="label" optionValue="value" [(ngModel)]="addDraft.documentType" />
          <kl-button variant="primary" size="sm" icon="check" iconOnly ariaLabel="Add preset" [disabled]="!addDraft.sourceApp.trim()" (click)="saveAdd()" />
          <kl-button size="sm" icon="x" iconOnly ariaLabel="Cancel" (click)="cancelAdd()" />
        </div>
      }
      @if (!adding) {
        <div sectionFooter>
          <kl-button data-testid="preset-add-btn" size="sm" icon="plus" (click)="startAdd()">Add manually</kl-button>
        </div>
      }
    </app-settings-section>
  `,
  styles: [`
    .row { display: flex; align-items: center; gap: 8px 12px; padding: 12px 20px; }
    .row + .row { border-top: 1px solid var(--kl-line); }
  `],
})
export class AppPresetsComponent {
  /** The settings page's object, kept in step with what the backend saved. */
  readonly settings = input<AppSettings | null>(null);

  presets: AppPreset[] = [];
  editing: AppPreset | null = null;
  editDraft: AppPreset = { sourceApp: '', documentType: 'email' };
  adding = false;
  addDraft: AppPreset = { sourceApp: '', documentType: 'email' };

  readonly docTypeOptions = DOCUMENT_TYPE_OPTIONS;

  constructor(
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
  ) {
    // Loaded when the page hands over its settings, and again when it hands
    // over new ones: Reset to Defaults clears the presets too.
    effect(() => {
      if (this.settings()) void this.load();
    });
  }

  private async load(): Promise<void> {
    this.presets = await this.wails.getAppPresets();
    this.cdr.markForCheck();
  }

  /** "Email" for "email"; an unknown type as it is stored. */
  typeLabel(type: string): string {
    return this.docTypeOptions.find(o => o.value === type)?.label ?? type;
  }

  private syncPresets(presets: AppPreset[]): void {
    this.presets = presets;
    const settings = this.settings();
    if (settings) {
      settings.app_presets = presets.map(p => ({ ...p }));
    }
  }

  startEdit(preset: AppPreset): void {
    this.editing = preset;
    this.editDraft = { ...preset };
  }

  cancelEdit(): void {
    this.editing = null;
  }

  async saveEdit(): Promise<void> {
    // Renamed to nothing, the delete below would lose the preset.
    // Names are stored trimmed and compared trimmed, ignoring case, as the
    // backend and the Pyramidize page's Remember do.
    const draft = { ...this.editDraft, sourceApp: this.editDraft.sourceApp.trim() };
    if (!draft.sourceApp) return;
    const before = (this.editing?.sourceApp ?? '').trim();
    await this.wails.setAppPreset(draft);
    // The backend matches presets by name, so a new name is a new preset: the
    // old one goes, or both would match. A change of case or padding is the
    // same preset and was just updated in place.
    if (before && before.toLowerCase() !== draft.sourceApp.toLowerCase()) {
      await this.wails.deleteAppPreset(before);
    }
    this.syncPresets(await this.wails.getAppPresets());
    this.editing = null;
    this.cdr.markForCheck();
  }

  async remove(sourceApp: string): Promise<void> {
    await this.wails.deleteAppPreset(sourceApp);
    this.syncPresets(await this.wails.getAppPresets());
    this.cdr.markForCheck();
  }

  startAdd(): void {
    this.adding = true;
    this.addDraft = { sourceApp: '', documentType: 'email' };
  }

  cancelAdd(): void {
    this.adding = false;
  }

  async saveAdd(): Promise<void> {
    const draft = { ...this.addDraft, sourceApp: this.addDraft.sourceApp.trim() };
    if (!draft.sourceApp) return;
    await this.wails.setAppPreset(draft);
    this.syncPresets(await this.wails.getAppPresets());
    this.adding = false;
    this.cdr.markForCheck();
  }
}
