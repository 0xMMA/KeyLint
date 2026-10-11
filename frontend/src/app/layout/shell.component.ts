import { Component, OnInit, OnDestroy, ChangeDetectorRef } from '@angular/core';
import { Router, RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { CdkScrollable } from '@angular/cdk/scrolling';
import { isDevMode } from '@angular/core';
import { Subscription } from 'rxjs';
import { IconComponent, TooltipDirective, type IconName } from '../shared/ui';
import { versionLabel } from '../core/version-label';
import { WailsService, type SilentFixNotice } from '../core/wails.service';
import { LogService } from '../core/log.service';
import { SilentFixNoticeService } from '../features/fix/silent-fix-notice.service';

// Persists across navigation
let sidebarCollapsed = false;
let sidebarHovered   = false;

@Component({
  selector: 'app-shell',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, CdkScrollable, IconComponent, TooltipDirective],
  styleUrls: ['./shell.component.scss'],
  template: `
    <div class="layout-wrapper" [class.sidebar-collapsed]="collapsedView">
      <aside class="layout-sidebar" data-testid="sidebar"
        [class.collapsed]="collapsedView"
        [class.hover-expanded]="hoverExpanded"
        (mouseenter)="onSidebarEnter()"
        (mouseleave)="onSidebarLeave()">
        <div class="layout-logo" aria-label="KeyLint" role="img">
          <span class="logo-k logo-key" aria-hidden="true">K</span><span class="logo-reveal logo-ey logo-key" aria-hidden="true">ey</span><span class="logo-l logo-lint" aria-hidden="true">L</span><span class="logo-reveal logo-int logo-lint" aria-hidden="true">int</span>
        </div>
        <nav class="sidebar-nav" aria-label="Main" cdkScrollable>
          <ul>
            @for (item of navItems; track item.path) {
              <li class="nav-item">
                <a [routerLink]="item.path" routerLinkActive="active-route" ariaCurrentWhenActive="page"
                  [attr.data-testid]="'nav-' + item.id"
                  [attr.aria-label]="labelsShown ? null : item.label"
                  [klTooltip]="labelsShown ? '' : item.label"
                  klTooltipPosition="right">
                  <kl-icon class="nav-icon" [name]="item.icon" [size]="18" />
                  @if (labelsShown) { <span>{{ item.label }}</span> }
                </a>
              </li>
            }
          </ul>
        </nav>
        <div class="sidebar-footer">
          <div class="version-row" data-testid="version-footer" (click)="goToAbout()">
            @if (labelsShown || updateAvailable) {
              <button type="button" class="version-link"
                [attr.aria-label]="labelsShown ? null : updateTitle + ' — open About'"
                [title]="labelsShown ? 'About KeyLint' : ''">
                @if (labelsShown) {
                  <span class="version-text" data-testid="version-text">{{ versionLabel(appVersion) }}</span>
                }
                @if (updateAvailable) {
                  <kl-icon class="update-indicator" data-testid="update-indicator" name="circle-arrow-up"
                    [size]="15" [attr.title]="updateTitle" [label]="updateTitle" />
                }
              </button>
            }
          </div>
          <button class="collapse-btn" data-testid="collapse-btn" (click)="toggleSidebar()" type="button"
            [attr.aria-label]="collapsedView ? 'Expand sidebar' : 'Collapse sidebar'"
            [attr.aria-expanded]="!collapsedView"
            [klTooltip]="collapsedView ? 'Expand sidebar' : 'Collapse sidebar'"
            klTooltipPosition="right">
            <kl-icon [name]="collapsedView ? 'chevron-right' : 'chevron-left'" [size]="16" />
          </button>
        </div>
      </aside>
      <!-- cdkScrollable: open selects and tooltips follow this area's scroll. -->
      <div class="layout-main" cdkScrollable>
        <router-outlet />
      </div>
    </div>
  `,
})
export class ShellComponent implements OnInit, OnDestroy {
  readonly dev = isDevMode();
  readonly navItems: ReadonlyArray<{ id: string; path: string; label: string; icon: IconName }> = [
    { id: 'fix', path: '/fix', label: 'Fix', icon: 'sparkle' },
    { id: 'enhance', path: '/enhance', label: 'Pyramidize', icon: 'pyramid' },
    { id: 'settings', path: '/settings', label: 'Settings', icon: 'sliders-horizontal' },
    ...(isDevMode() ? [{ id: 'dev-tools', path: '/dev-tools', label: 'Dev Tools', icon: 'wrench' as IconName }] : []),
  ];
  readonly versionLabel = versionLabel;
  appVersion = '';
  updateAvailable = false;
  updateTitle = 'Update available';
  private subs: Subscription[] = [];

  get collapsedView(): boolean  { return sidebarCollapsed; }
  get hoverExpanded(): boolean  { return sidebarCollapsed && sidebarHovered; }
  /** Labels (and the version) show unless the rail is collapsed and not hovered. */
  get labelsShown(): boolean    { return !sidebarCollapsed || sidebarHovered; }

  constructor(
    private readonly wails: WailsService,
    private readonly router: Router,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
    private readonly silentNotices: SilentFixNoticeService,
  ) {}

  ngOnInit(): void {
    void this.applyTheme();
    void this.loadVersionInfo();
    this.subs.push(
      this.wails.settingsChanged$.subscribe(() => void this.applyTheme()),
      // The fix shortcut itself never reaches the frontend: the silent fix
      // runs in Go with the window hidden. A click on its notification does.
      this.wails.silentFixOpen$.subscribe(notice => this.openSilentFixNotice(notice)),
      this.wails.shortcutPyramidize$.subscribe(() => {
        void this.router.navigate(['/enhance']);
      }),
    );
  }

  goToAbout(): void {
    // 'reload': the URL may already be /settings?tab=about with another tab
    // picked since; Settings reads the tab from every finished navigation.
    void this.router.navigate(['/settings'], { queryParams: { tab: 'about' }, onSameUrlNavigation: 'reload' });
  }

  toggleSidebar(): void {
    sidebarCollapsed = !sidebarCollapsed;
    sidebarHovered = false; // Don't immediately re-expand on manual toggle
    this.cdr.detectChanges();
  }

  onSidebarEnter(): void {
    if (sidebarCollapsed) {
      sidebarHovered = true;
      this.cdr.detectChanges();
    }
  }

  onSidebarLeave(): void {
    sidebarHovered = false;
    this.cdr.detectChanges();
  }

  private async loadVersionInfo(): Promise<void> {
    this.appVersion = await this.wails.getVersion();
    this.cdr.detectChanges();
    try {
      // A dev build's normal check is silenced (it is 0.0.0, so every release
      // would be newer); its news comes from the dev channel instead: its PR
      // is gone, or a newer build of the same PR or of main is up.
      const identity = await this.wails.getBuildIdentity();
      if (identity.is_dev_build) {
        const channel = await this.wails.listDevBuilds();
        if (channel.orphaned) {
          this.updateAvailable = true;
          this.updateTitle = 'This test build is gone — see Settings › About';
        } else if (channel.new_release_since_build) {
          this.updateAvailable = true;
          this.updateTitle = `A new release is out (v${channel.latest_release}) — see Settings › About`;
        } else if (channel.builds.some(b => b.newer_build)) {
          this.updateAvailable = true;
          this.updateTitle = 'A newer test build is available';
        }
      } else {
        const info = await this.wails.checkForUpdate();
        this.updateAvailable = info.is_available;
      }
    } catch {
      // Silently ignore — update check is best-effort.
    }
    this.cdr.detectChanges();
  }

  ngOnDestroy(): void {
    this.subs.forEach(s => s.unsubscribe());
  }

  /**
   * The user clicked a hotkey-fix notification: take them where it can be
   * fixed. A missing key, a signed-out CLI or the wrong model is on AI
   * Providers; everything else is the Fix page, which shows what happened.
   */
  private openSilentFixNotice(notice: SilentFixNotice): void {
    this.log.info(`shell: silent fix notification opened (${notice.target})`);
    // Presented first: a page already on screen switches on the notice
    // itself, because the router ignores a navigation to the URL it is on.
    this.silentNotices.present(notice);
    if (notice.target === 'providers') {
      void this.router.navigate(['/settings'], { queryParams: { tab: 'providers' }, onSameUrlNavigation: 'reload' });
      return;
    }
    void this.router.navigate(['/fix']);
  }

  private applyTheme(): void {
    // Only the dark theme is styled (#24). theme_preference is deliberately
    // not read: a "light" or "system" saved by an older version would ask for
    // a theme that does not exist yet. The light theme (#25) reads it again.
    // On <html>, not <body>: see index.html.
    document.documentElement.classList.add('app-dark');
  }
}
