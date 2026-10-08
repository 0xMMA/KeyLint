import { Component, OnInit, OnDestroy, ChangeDetectorRef, input } from '@angular/core';
import { DatePipe } from '@angular/common';
import { Button } from 'primeng/button';
import { Tag } from 'primeng/tag';
import { Message } from 'primeng/message';
import { WailsService, DevBuild, DevChannel, BuildIdentity, InstallResult } from '../../../core/wails.service';
import { versionLabel } from '../../../core/version-label';

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
  imports: [DatePipe, Button, Tag, Message],
  template: `
    <section class="dev-channel" data-testid="dev-channel" aria-labelledby="dev-channel-heading">
      <h4 id="dev-channel-heading" class="dev-channel-heading">Dev channel</h4>
      <small class="hint-text">Test builds made by CI from main and from open pull requests. They are not releases: each one is replaced on the next push and removed when its pull request closes.</small>

      <p class="dev-identity" data-testid="dev-identity">{{ identityText() }}</p>

      @if (channel?.orphaned) {
        <div class="return-offer" data-testid="return-offer">
          <p-message
            severity="warn"
            [text]="'Your test build for PR #' + channel!.current.pr + ' is gone (merged or closed). Install the current main build?'"
          />
          <div class="return-actions">
            @if (mainBuild?.installable) {
              <p-button
                data-testid="return-main-btn"
                label="Install main build"
                icon="pi pi-download"
                [loading]="installing === mainBuild!.tag"
                [disabled]="installing !== null"
                (onClick)="install(mainBuild!.tag)"
              />
            }
            @if (channel!.latest_release) {
              <p-button
                data-testid="return-release-btn"
                [label]="'Install latest release (v' + channel!.latest_release + ')'"
                severity="secondary"
                [loading]="installing === LATEST"
                [disabled]="installing !== null"
                (onClick)="installLatest()"
              />
            }
          </div>
        </div>
      }

      @if (channel?.error) {
        <p-message data-testid="dev-channel-error" severity="warn" [text]="channel!.error" styleClass="mb-2" />
      }

      @if (channel === null) {
        <p class="hint-text" data-testid="dev-channel-loading">Looking for test builds…</p>
      } @else {
        <ul class="dev-build-list" data-testid="dev-build-list">
          @for (b of channel.builds; track b.tag) {
            <li class="dev-build" [class.installed]="b.installed" [attr.data-testid]="'dev-build-' + b.tag">
              <div class="dev-build-text">
                <div class="dev-build-name">
                  <span>{{ b.kind === 'main' ? 'main (latest)' : 'PR #' + b.pr }}</span>
                  @if (b.installed) {
                    <p-tag data-testid="dev-build-installed" value="installed" severity="success" />
                  }
                  @if (b.newer_build) {
                    <p-tag data-testid="dev-build-newer" value="newer build" severity="info" />
                  }
                </div>
                @if (b.title) {
                  <div class="dev-build-title">{{ b.title }}</div>
                }
                <small class="dev-build-meta">
                  @if (b.date) { {{ b.date | date: 'd MMM y, HH:mm' }} }
                  @if (b.date && b.commit) { · }
                  @if (b.commit) { <code>{{ b.commit }}</code> }
                </small>
              </div>
              <p-button
                [attr.data-testid]="'install-' + b.tag"
                [label]="b.installed ? 'Reinstall' : 'Install'"
                [title]="b.installable ? '' : 'No build for this platform'"
                size="small"
                [severity]="b.installed ? 'secondary' : 'primary'"
                [loading]="installing === b.tag"
                [disabled]="!b.installable || installing !== null"
                (onClick)="install(b.tag)"
              />
            </li>
          } @empty {
            <li class="hint-text" data-testid="dev-build-empty">No test builds published right now.</li>
          }
        </ul>
        <p-button
          data-testid="dev-channel-refresh"
          label="Refresh"
          icon="pi pi-refresh"
          severity="secondary"
          text
          size="small"
          [loading]="refreshing"
          [disabled]="installing !== null"
          (onClick)="load(true)"
        />
      }

      @if (installResult) {
        <p-message
          data-testid="dev-install-success"
          severity="success"
          [text]="installResult.restart_required
            ? 'Installing — the app will close shortly.'
            : 'Installed. Restart KeyLint to use it.'"
          styleClass="mt-2"
        />
      }
      @if (installError) {
        <p-message data-testid="dev-install-error" severity="error" [text]="installError" styleClass="mt-2" />
      }
    </section>
  `,
  styles: [`
    .dev-channel {
      margin-top: 1.5rem;
      padding-top: 1rem;
      border-top: 1px solid var(--p-content-border-color);
    }
    .dev-channel-heading { margin: 0 0 0.25rem; font-size: 1rem; }
    .hint-text { display: block; font-size: 0.8rem; color: var(--p-text-muted-color); margin-bottom: 0.75rem; }
    .dev-identity { margin: 0 0 0.75rem; font-size: 0.9rem; }
    .return-offer { margin-bottom: 1rem; }
    .return-actions { display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem; }
    .dev-build-list { list-style: none; margin: 0 0 0.5rem; padding: 0; }
    .dev-build {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 1rem;
      padding: 0.6rem 0.75rem;
      border: 1px solid var(--p-content-border-color);
      border-radius: var(--p-border-radius-md, 6px);
      margin-bottom: 0.5rem;
    }
    .dev-build.installed { border-color: var(--p-primary-color); }
    .dev-build-text { min-width: 0; }
    .dev-build-name { display: flex; align-items: center; gap: 0.5rem; font-weight: 600; font-size: 0.9rem; }
    .dev-build-title { font-size: 0.875rem; overflow-wrap: anywhere; }
    .dev-build-meta { color: var(--p-text-muted-color); font-size: 0.75rem; }
    .dev-build p-button { flex-shrink: 0; }
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

  identityText(): string {
    const cur: BuildIdentity | undefined = this.channel?.current;
    if (cur?.is_dev_build) {
      const what = cur.kind === 'pr' ? `the test build of PR #${cur.pr}` : 'the latest main build';
      return `Running ${what}${cur.commit ? ` (commit ${cur.commit})` : ''}.`;
    }
    return `Running ${this.version() ? versionLabel(this.version()) : 'a release build'}, not a test build.`;
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
