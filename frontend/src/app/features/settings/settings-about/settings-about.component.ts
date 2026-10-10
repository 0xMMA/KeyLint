import { ChangeDetectorRef, Component, OnDestroy, ViewChild, input } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { ButtonComponent, MessageComponent, SelectComponent, SwitchComponent } from '../../../shared/ui';
import { WailsService, type BuildIdentity, type Settings as AppSettings, type UpdateInfo } from '../../../core/wails.service';
import { versionLabel } from '../../../core/version-label';
import { LogService } from '../../../core/log.service';
import { SettingsSectionComponent } from '../settings-section/settings-section.component';
import { SettingRowComponent } from '../setting-row/setting-row.component';
import { DevChannelComponent } from '../dev-channel/dev-channel.component';

/** Taps on the version that unlock the developer options, as on Android. */
const UNLOCK_TAPS = 7;

/**
 * Settings › About: the version, updates, and the developer options with the
 * dev channel they unlock.
 *
 * The update channel is part of the settings object and is saved with the
 * page's Save button; the developer options switch saves itself at once, as
 * does the unlock — there is no Save to press for those.
 */
@Component({
  selector: 'app-settings-about',
  standalone: true,
  imports: [
    FormsModule, ButtonComponent, MessageComponent, SelectComponent, SwitchComponent,
    SettingsSectionComponent, SettingRowComponent, DevChannelComponent,
  ],
  template: `
    @let s = settings();
    <div class="flex flex-col gap-7">
      <section class="flex flex-col gap-1" aria-label="KeyLint">
        <p class="m-0 text-lg font-bold text-fg-strong"><span>Key</span><span class="text-primary">Lint</span></p>
        <!-- Tapping the version seven times unlocks the developer options,
             as on Android. A button, so it also works from the keyboard. -->
        <p class="m-0 text-sm text-soft" data-testid="app-version">Version: <button type="button" class="version-tap" data-testid="version-tap" (click)="onVersionTap()">{{ versionLabel(version()) }}</button></p>
        <p class="m-0 text-sm text-muted">Built with Go, Wails and Angular.</p>
        <p class="m-0 min-h-5 text-sm text-muted" data-testid="unlock-hint" aria-live="polite">{{ unlockHint }}</p>
      </section>

      <app-settings-section heading="Updates">
        <!-- The channel stays settable in a dev build: it decides which
             release the dev channel's "latest release" offer installs. -->
        <app-setting-row data-testid="update-channel-section" label="Update channel" forId="update-channel"
          hint="Auto detects from your current version: pre-release versions check for pre-releases, stable versions check for stable only. Test builds count as pre-release.">
          <kl-select
            class="w-56"
            inputId="update-channel"
            ariaLabel="Update channel"
            [options]="updateChannels"
            optionLabel="label"
            optionValue="value"
            [(ngModel)]="s.update_channel"
          />
        </app-setting-row>

        <!-- A dev build is 0.0.0, so every release would read as an
             update: the dev channel replaces the normal check there. -->
        @if (!buildIdentity().is_dev_build) {
          <app-setting-row label="Check for updates" hint="Asks GitHub for a newer release on your channel.">
            <kl-button
              data-testid="check-update-btn"
              icon="refresh-cw"
              loadingLabel="Checking…"
              [loading]="updateChecking"
              (click)="checkForUpdate()"
            >Check for updates</kl-button>
            @if (updateInfo?.is_available) {
              <div below class="flex basis-full flex-col items-start gap-3">
                <kl-message
                  class="self-stretch"
                  data-testid="update-available-msg"
                  severity="info"
                  [text]="'Update available: v' + updateInfo!.latest_version + (updateInfo!.notes ? ' — ' + updateInfo!.notes : '')"
                />
                <kl-button
                  data-testid="install-update-btn"
                  variant="primary"
                  icon="download"
                  loadingLabel="Installing…"
                  [loading]="updateInstalling"
                  (click)="installUpdate()"
                >Download and install</kl-button>
              </div>
            }
            @if (updateSuccess) {
              <kl-message
                below
                class="basis-full"
                data-testid="update-success-msg"
                severity="success"
                [text]="updateRestartRequired
                  ? 'Update is installing — the app will close shortly.'
                  : 'Update installed! Restart the app to use the new version.'"
              />
            }
            <!-- "Already on the latest version" arrives here too; it is news, not a failure. -->
            @if (updateError) {
              <kl-message below class="basis-full" data-testid="update-error-msg" [text]="updateError"
                [severity]="updateInfo && !updateInfo.is_available ? 'success' : 'error'" />
            }
          </app-setting-row>
        }
      </app-settings-section>

      @if (s.developer_options) {
        <app-settings-section>
          <app-setting-row data-testid="developer-options-section" label="Developer options" forId="developer-options-toggle"
            hint="Shows the dev channel below. Turn off to hide it again; tapping the version seven times brings it back.">
            <kl-switch
              #developerOptionsToggle
              inputId="developer-options-toggle"
              ariaLabel="Developer options"
              data-testid="developer-options-toggle"
              [ngModel]="s.developer_options"
              (ngModelChange)="setDeveloperOptions($event)"
            />
          </app-setting-row>
        </app-settings-section>
      }
      @if (devOptionsError) {
        <kl-message data-testid="developer-options-error" severity="error" [text]="devOptionsError" />
      }

      @if (s.developer_options || buildIdentity().is_dev_build) {
        <app-dev-channel [version]="version()" />
      }
    </div>
  `,
  styles: [`
    .version-tap {
      font: inherit; color: var(--kl-text); background: none; border: 0; padding: 0; cursor: default;
      border-radius: 2px;
    }
    .version-tap:focus-visible { outline: 2px solid var(--kl-focus); outline-offset: 2px; }
  `],
})
export class SettingsAboutComponent implements OnDestroy {
  /** Edited in place: the update channel is saved with the page's Save button. */
  readonly settings = input.required<AppSettings>();
  readonly version = input('');
  /** What the running build says about itself; a release build until asked. */
  readonly buildIdentity = input<BuildIdentity>({ is_dev_build: false, kind: '', pr: 0, commit: '', tag: '' });

  readonly versionLabel = versionLabel;
  @ViewChild('developerOptionsToggle') private developerOptionsToggle?: SwitchComponent;

  /** Taps on the version so far, towards UNLOCK_TAPS. */
  private versionTaps = 0;
  unlockHint = '';
  devOptionsError = '';
  updateInfo: UpdateInfo | null = null;
  updateChecking = false;
  updateInstalling = false;
  updateError = '';
  updateSuccess = false;
  updateRestartRequired = false;
  private destroyed = false;

  readonly updateChannels = [
    { label: 'Auto (detect from version)', value: '' },
    { label: 'Stable', value: 'stable' },
    { label: 'Pre-release', value: 'pre-release' },
  ];

  constructor(
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
  ) {}

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  /**
   * One tap on the version. The last few taps count down, Android-style, and
   * the seventh turns the developer options on and saves that at once.
   */
  async onVersionTap(): Promise<void> {
    const settings = this.settings();
    if (settings.developer_options) {
      this.unlockHint = 'Developer options are already on.';
      return;
    }
    this.versionTaps++;
    const left = UNLOCK_TAPS - this.versionTaps;
    if (left > 0) {
      if (left <= UNLOCK_TAPS - 3) {
        this.unlockHint = `${left} more ${left === 1 ? 'tap' : 'taps'} to turn on the developer options.`;
      }
      return;
    }
    this.versionTaps = 0;
    if (await this.setDeveloperOptions(true)) {
      this.unlockHint = 'Developer options are on.';
    }
  }

  /** Saves the developer options switch on its own; reports whether it stuck. */
  async setDeveloperOptions(enabled: boolean): Promise<boolean> {
    const settings = this.settings();
    this.devOptionsError = '';
    try {
      await this.wails.setDeveloperOptions(enabled);
      settings.developer_options = enabled;
      if (!enabled) this.unlockHint = '';
      this.log.info(`settings: developer options ${enabled ? 'on' : 'off'}`);
      return true;
    } catch (e) {
      // The switch has already moved, and its [ngModel] input has not
      // changed, so Angular will not move it back: do it here, so it shows
      // what is saved rather than what was asked for.
      this.developerOptionsToggle?.writeValue(settings.developer_options);
      this.devOptionsError = `Could not change the developer options: ${e instanceof Error ? e.message : String(e)}`;
      return false;
    } finally {
      if (!this.destroyed) this.cdr.detectChanges();
    }
  }

  async checkForUpdate(): Promise<void> {
    this.updateChecking = true;
    this.updateError = '';
    this.updateInfo = null;
    try {
      this.updateInfo = await this.wails.checkForUpdate();
      if (!this.updateInfo.is_available) {
        this.updateError = 'You are already on the latest version.';
      }
    } catch (e) {
      this.updateError = `Update check failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.updateChecking = false;
      if (!this.destroyed) this.cdr.detectChanges();
    }
  }

  async installUpdate(): Promise<void> {
    this.updateInstalling = true;
    this.updateError = '';
    this.updateSuccess = false;
    this.updateRestartRequired = false;
    try {
      const result = await this.wails.downloadAndInstall();
      this.updateSuccess = true;
      this.updateRestartRequired = result.restart_required;
      this.updateInfo = null;
    } catch (e) {
      this.updateError = `Install failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.updateInstalling = false;
      if (!this.destroyed) this.cdr.detectChanges();
    }
  }
}
