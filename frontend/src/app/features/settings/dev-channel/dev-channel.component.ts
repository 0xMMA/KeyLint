import { Component, OnInit, OnDestroy, ChangeDetectorRef, input, ChangeDetectionStrategy } from '@angular/core';
import { DatePipe } from '@angular/common';
import { ButtonComponent, MessageComponent, TagComponent } from '../../../shared/ui';
import { WailsService, DevBuild, DevChannel, BuildIdentity, InstallResult } from '../../../core/wails.service';
import { versionLabel, isDevVersion } from '../../../core/version-label';

/**
 * Settings › About › Dev channel: installable CI builds of main and of every
 * open pull request, published as GitHub prereleases (see build-linux.yml,
 * job publish-dev-build).
 *
 * Shown only with developer options on, or when the running build is itself
 * from this channel — a dev build's normal update check is silenced (every
 * release would look newer than 0.0.0), so this is its only way off.
 *
 * Nothing installs on its own. When the running PR build's release is gone
 * (the PR was merged or closed) this offers main or the latest release, one
 * click each, and waits.
 */
@Component({
  selector: 'app-dev-channel',
  standalone: true,
  imports: [DatePipe, ButtonComponent, MessageComponent, TagComponent],
  changeDetection: ChangeDetectionStrategy.Eager,
  template: `
    <section class="flex flex-col gap-3" data-testid="dev-channel" aria-labelledby="dev-channel-heading">
      <div>
        <h2 id="dev-channel-heading" class="m-0 text-base font-bold text-fg-strong">Dev channel</h2>
        <p class="mt-1 mb-0 text-sm text-muted">Test builds made by CI from main and from open pull requests. They are not releases: each one is replaced on the next push and removed when its pull request closes.</p>
      </div>

      <p class="m-0 text-sm" data-testid="dev-identity">{{ identityText() }}</p>

      @if (channel?.orphaned) {
        <div class="flex flex-col gap-3" data-testid="return-offer">
          <kl-message
            severity="warn"
            [text]="'Your test build for PR #' + channel!.current.pr + ' is gone (merged or closed). Install the current main build?'"
          />
          <div class="flex flex-wrap gap-2">
            @if (mainBuild?.installable) {
              <kl-button
                data-testid="return-main-btn"
                variant="primary"
                icon="download"
                [loading]="installing === mainBuild!.tag"
                [disabled]="installing !== null"
                (click)="install(mainBuild!.tag)"
              >Install main build</kl-button>
            }
            @if (channel!.latest_release) {
              <kl-button
                data-testid="return-release-btn"
                [loading]="installing === LATEST"
                [disabled]="installing !== null"
                (click)="installLatest()"
              >Install latest release (v{{ channel!.latest_release }})</kl-button>
            }
          </div>
        </div>
      }

      <!-- The way back, whenever a dev build runs: its normal update check is
           silenced, so this is also where it hears about a new release.
           The return offer above already carries it when the PR is gone. -->
      @if (runningDevBuild() && channel && !channel.orphaned && channel.latest_release) {
        <div class="flex flex-col items-start gap-2" data-testid="back-to-release">
          @if (channel.new_release_since_build) {
            <kl-message
              class="self-stretch"
              data-testid="new-release-msg"
              severity="info"
              [text]="'A new release is out: v' + channel.latest_release + releasedOn(' on ') + '.'"
            />
          } @else {
            <p class="m-0 text-sm text-muted" data-testid="latest-release-line">Latest release: v{{ channel.latest_release }}{{ releasedOn(', released ') }}.</p>
          }
          <kl-button
            data-testid="back-to-release-btn"
            icon="rotate-ccw"
            [loading]="installing === LATEST"
            [disabled]="installing !== null"
            (click)="installLatest()"
          >Back to latest release (v{{ channel.latest_release }})</kl-button>
        </div>
      }

      @if (channel?.error) {
        <kl-message data-testid="dev-channel-error" severity="warn" [text]="channel!.error" />
      }

      @if (channel === null) {
        <p class="m-0 text-sm text-muted" data-testid="dev-channel-loading">Looking for test builds…</p>
      } @else {
        <ul class="builds" data-testid="dev-build-list">
          @for (b of channel.builds; track b.tag) {
            <li class="build" [class.installed]="b.installed" [attr.data-testid]="'dev-build-' + b.tag">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2 text-[15px] font-semibold">
                  <span>{{ b.kind === 'main' ? 'main (latest)' : 'PR #' + b.pr }}</span>
                  @if (b.installed) {
                    <kl-tag data-testid="dev-build-installed" value="installed" severity="success" />
                  }
                  @if (b.newer_build) {
                    <kl-tag data-testid="dev-build-newer" value="newer build" severity="info" />
                  }
                </div>
                @if (b.title) {
                  <div class="text-sm [overflow-wrap:anywhere]">{{ b.title }}</div>
                }
                <small class="text-xs text-muted">
                  @if (b.date) { {{ b.date | date: 'd MMM y, HH:mm' }} }
                  @if (b.date && b.commit) { · }
                  @if (b.commit) { <code>{{ b.commit }}</code> }
                </small>
              </div>
              <kl-button
                class="flex-none"
                [attr.data-testid]="'install-' + b.tag"
                [attr.title]="b.installable ? null : 'No build for this platform'"
                size="sm"
                [variant]="b.installed ? 'quiet' : 'primary'"
                [loading]="installing === b.tag"
                [disabled]="!b.installable || installing !== null"
                (click)="install(b.tag)"
              >{{ b.installed ? 'Reinstall' : 'Install' }}</kl-button>
            </li>
          } @empty {
            <li class="build text-sm text-muted" data-testid="dev-build-empty">No test builds published right now.</li>
          }
        </ul>
        <div>
          <kl-button
            data-testid="dev-channel-refresh"
            variant="ghost"
            size="sm"
            icon="refresh-cw"
            [loading]="refreshing"
            [disabled]="installing !== null"
            (click)="load(true)"
          >Refresh</kl-button>
        </div>
      }

      @if (installResult) {
        <kl-message
          data-testid="dev-install-success"
          severity="success"
          [text]="installResult.restart_required
            ? 'Installing — the app will close shortly.'
            : 'Installed. Restart KeyLint to use it.'"
        />
      }
      @if (installError) {
        <kl-message data-testid="dev-install-error" severity="error" [text]="installError" />
      }
    </section>
  `,
  styles: [`
    .builds {
      list-style: none; margin: 0; padding: 0;
      border: 1px solid var(--kl-line); border-radius: var(--kl-radius-sheet); background: var(--kl-sheet); overflow: hidden;
    }
    .build { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 12px 20px; }
    .build + .build { border-top: 1px solid var(--kl-line); }
    .build.installed { background: var(--kl-sheet-alt); box-shadow: inset 3px 0 0 var(--kl-primary); }
    code { font-size: 12px; padding: 1px 4px; border-radius: 3px; background: var(--kl-selected); }
  `],
})
export class DevChannelComponent implements OnInit, OnDestroy {
  /** The running version, for wording a release build's identity. */
  readonly version = input<string>('');

  /** Marks the latest-release install in `installing`; never a tag. */
  readonly LATEST = '__latest__';

  channel: DevChannel | null = null;
  refreshing = false;
  /** The tag being installed, LATEST, or null. One install at a time. */
  installing: string | null = null;
  installResult: InstallResult | null = null;
  installError = '';
  private destroyed = false;

  constructor(
    private readonly wails: WailsService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  async ngOnInit(): Promise<void> {
    await this.load(false);
  }

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  get mainBuild(): DevBuild | undefined {
    return this.channel?.builds.find(b => b.kind === 'main');
  }

  /** Whether the running build is from this channel, by the backend or by its version. */
  runningDevBuild(): boolean {
    return !!this.channel?.current.is_dev_build || isDevVersion(this.version());
  }

  identityText(): string {
    if (this.channel === null) return 'Checking which build is running…';
    const cur: BuildIdentity = this.channel.current;
    if (cur.is_dev_build) {
      const what = cur.kind === 'pr' ? `the test build of PR #${cur.pr}` : 'the latest main build';
      return `Running ${what}${cur.commit ? ` (commit ${cur.commit})` : ''}.`;
    }
    const version = this.version();
    if (isDevVersion(version)) return `Running the test build ${versionLabel(version)}.`;
    // Only claim "not a test build" on a version that says so.
    if (!version || version === 'dev') return 'Could not tell which build is running.';
    return `Running ${versionLabel(version)}, not a test build.`;
  }

  /** "<prefix>5 Oct 2026" for the latest release, or "" without a date. */
  releasedOn(prefix: string): string {
    const raw = this.channel?.latest_release_date;
    if (!raw) return '';
    const d = new Date(raw);
    if (isNaN(d.getTime())) return '';
    return prefix + d.toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' });
  }

  async load(force: boolean): Promise<void> {
    this.refreshing = force;
    try {
      // listDevBuilds never rejects; a failure arrives as channel.error.
      this.channel = await this.wails.listDevBuilds(force);
    } finally {
      this.refreshing = false;
      if (!this.destroyed) this.cdr.detectChanges();
    }
  }

  async install(tag: string): Promise<void> {
    await this.run(tag, () => this.wails.installDevBuild(tag));
  }

  async installLatest(): Promise<void> {
    await this.run(this.LATEST, () => this.wails.installLatestRelease());
  }

  private async run(what: string, call: () => Promise<InstallResult>): Promise<void> {
    if (this.installing !== null) return;
    this.installing = what;
    this.installResult = null;
    this.installError = '';
    this.cdr.detectChanges();
    try {
      this.installResult = await call();
    } catch (e) {
      this.installError = `Install failed: ${e instanceof Error ? e.message : String(e)}`;
    } finally {
      this.installing = null;
      if (!this.destroyed) this.cdr.detectChanges();
    }
  }
}
