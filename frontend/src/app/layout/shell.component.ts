import { Component, OnInit, OnDestroy, ChangeDetectorRef } from '@angular/core';
import { Router, RouterOutlet, RouterLink, RouterLinkActive } from '@angular/router';
import { isDevMode } from '@angular/core';
import { Subscription } from 'rxjs';
import { TooltipModule } from 'primeng/tooltip';
import { WailsService } from '../core/wails.service';
import { LogService } from '../core/log.service';

// Persists across navigation
let sidebarCollapsed = false;
let sidebarHovered   = false;

@Component({
  selector: 'app-shell',
  standalone: true,
  imports: [RouterOutlet, RouterLink, RouterLinkActive, TooltipModule],
  styleUrls: ['./shell.component.scss'],
  template: `
    <div class="layout-wrapper" [class.sidebar-collapsed]="collapsedView">
      <aside class="layout-sidebar"
        [class.collapsed]="collapsedView"
        [class.hover-expanded]="hoverExpanded"
        (mouseenter)="onSidebarEnter()"
        (mouseleave)="onSidebarLeave()">
        <div class="layout-logo">
          <span class="logo-k logo-key">K</span><span class="logo-reveal logo-ey logo-key">ey</span><span class="logo-l logo-lint">L</span><span class="logo-reveal logo-int logo-lint">int</span>
        </div>
        <nav class="sidebar-nav">
          <ul>
            <li class="nav-item">
              <a routerLink="/fix" routerLinkActive="active-route"
                [pTooltip]="collapsedView && !hoverExpanded ? 'Fix' : ''"
                tooltipPosition="right"
                appendTo="body">
                <i class="pi pi-sparkles"></i>
                @if (!collapsedView || hoverExpanded) { <span>Fix</span> }
              </a>
            </li>
            <li class="nav-item">
              <a routerLink="/enhance" routerLinkActive="active-route"
                [pTooltip]="collapsedView && !hoverExpanded ? 'Pyramidize' : ''"
                tooltipPosition="right"
                appendTo="body">
                <svg class="nav-icon-svg" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linejoin="round" width="1rem" height="1rem">
                  <polygon points="12,3 22,21 2,21"/>
                </svg>
                @if (!collapsedView || hoverExpanded) { <span>Pyramidize</span> }
              </a>
            </li>
            <li class="nav-item">
              <a routerLink="/settings" routerLinkActive="active-route"
                [pTooltip]="collapsedView && !hoverExpanded ? 'Settings' : ''"
                tooltipPosition="right"
                appendTo="body">
                <i class="pi pi-cog"></i>
                @if (!collapsedView || hoverExpanded) { <span>Settings</span> }
              </a>
            </li>
            @if (dev) {
              <li class="nav-item">
                <a routerLink="/dev-tools" routerLinkActive="active-route"
                  [pTooltip]="collapsedView && !hoverExpanded ? 'Dev Tools' : ''"
                  tooltipPosition="right"
                  appendTo="body">
                  <i class="pi pi-wrench"></i>
                  @if (!collapsedView || hoverExpanded) { <span>Dev Tools</span> }
                </a>
              </li>
            }
          </ul>
        </nav>
        <div class="sidebar-footer">
          <div class="version-row" data-testid="version-footer" (click)="goToAbout()">
            @if (!collapsedView || hoverExpanded) {
              <span class="version-text">v{{ appVersion || '…' }}</span>
              @if (updateAvailable) {
                <i class="pi pi-arrow-circle-up update-indicator" data-testid="update-indicator" title="Update available"></i>
              }
            } @else {
              @if (updateAvailable) {
                <i class="pi pi-arrow-circle-up update-indicator" data-testid="update-indicator" title="Update available"></i>
              }
            }
          </div>
          <button class="collapse-btn" (click)="toggleSidebar()" type="button"
            [pTooltip]="collapsedView ? 'Expand sidebar' : 'Collapse sidebar'"
            tooltipPosition="right"
            appendTo="body">
            <i class="pi" [class.pi-chevron-left]="!collapsedView" [class.pi-chevron-right]="collapsedView"></i>
          </button>
        </div>
      </aside>
      <div class="layout-main">
        <router-outlet />
      </div>
    </div>
  `,
})
export class ShellComponent implements OnInit, OnDestroy {
  readonly dev = isDevMode();
  appVersion = '';
  updateAvailable = false;
  private subs: Subscription[] = [];

  get collapsedView(): boolean  { return sidebarCollapsed; }
  get hoverExpanded(): boolean  { return sidebarCollapsed && sidebarHovered; }

  constructor(
    private readonly wails: WailsService,
    private readonly router: Router,
    private readonly cdr: ChangeDetectorRef,
    private readonly log: LogService,
  ) {}

  ngOnInit(): void {
    void this.applyTheme();
    void this.loadVersionInfo();
    this.subs.push(
      this.wails.settingsChanged$.subscribe(() => void this.applyTheme()),
      this.wails.shortcutFix$.subscribe(() => {
        void this.silentFix();
      }),
      this.wails.shortcutPyramidize$.subscribe(() => {
        void this.router.navigate(['/enhance']);
      }),
    );
  }

  goToAbout(): void {
    void this.router.navigate(['/settings'], { queryParams: { tab: 'about' } });
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
      const info = await this.wails.checkForUpdate();
      this.updateAvailable = info.is_available;
    } catch {
      // Silently ignore — update check is best-effort.
    }
    this.cdr.detectChanges();
  }

  ngOnDestroy(): void {
    this.subs.forEach(s => s.unsubscribe());
  }

  private async silentFix(): Promise<void> {
    this.log.info('shell: silent fix started');
    try {
      const text = await this.wails.readClipboard();
      if (!text.trim()) return;
      const result = await this.wails.enhance(text);
      await this.wails.writeClipboard(result);
      await this.wails.pasteToForeground();
      this.log.info('shell: silent fix done');
    } catch (e: unknown) {
      this.log.error('shell: silent fix failed: ' + (e instanceof Error ? e.message : String(e)));
    }
  }

  private async applyTheme(): Promise<void> {
    const settings = await this.wails.loadSettings();
    const dark = settings.theme_preference !== 'light';
    document.body.classList.toggle('app-dark', dark);
  }
}
