import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { ShellComponent } from './shell.component';
import { WailsService } from '../core/wails.service';
import { createWailsMock, defaultSettings, defaultUpdateInfo, defaultDevChannel } from '../../testing/wails-mock';

describe('ShellComponent — theme / body class', () => {
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    document.documentElement.classList.remove('app-dark');

    wailsMock = createWailsMock();

    await TestBed.configureTestingModule({
      imports: [ShellComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();
  });

  afterEach(() => {
    document.documentElement.classList.remove('app-dark');
  });

  async function createAndWait(theme_preference: string): Promise<ComponentFixture<ShellComponent>> {
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, theme_preference: theme_preference as never });
    const fixture = TestBed.createComponent(ShellComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    return fixture;
  }

  it('adds app-dark to the root element for dark theme', async () => {
    await createAndWait('dark');
    expect(document.documentElement.classList.contains('app-dark')).toBe(true);
  });

  // Only the dark theme is styled (#24). A value saved by an older version, or
  // one a light theme (#25) will honour later, must not unstyle the app today.
  for (const stored of ['light', 'system', '', 'something-else']) {
    it(`stays dark when theme_preference is "${stored}"`, async () => {
      await createAndWait(stored);
      expect(document.documentElement.classList.contains('app-dark')).toBe(true);
    });
  }

  it('stays dark when settingsChanged$ brings a light preference', async () => {
    const fixture = await createAndWait('dark');
    expect(document.documentElement.classList.contains('app-dark')).toBe(true);

    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, theme_preference: 'light' });
    wailsMock._settingsChanged$.next();
    await fixture.whenStable();

    expect(document.documentElement.classList.contains('app-dark')).toBe(true);
  });

  it('renders the sidebar nav', async () => {
    const fixture = await createAndWait('dark');
    const el: HTMLElement = fixture.nativeElement;
    expect(el.querySelector('.layout-sidebar')).toBeTruthy();
    expect(el.querySelector('nav.sidebar-nav')).toBeTruthy();
  });

  it('displays version in sidebar footer', async () => {
    wailsMock.getVersion.mockResolvedValue('4.1.7');
    const fixture = await createAndWait('dark');
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    const footer = el.querySelector('[data-testid="version-footer"]');
    expect(footer).toBeTruthy();
    expect(footer!.textContent).toContain('v4.1.7');
  });

  // Release builds get their version from the git tag, which already carries
  // the "v" (#21); local builds may not, and dev builds say "dev".
  for (const [raw, shown] of [
    ['v3.6.0', 'v3.6.0'],
    ['v3.7.0-alpha.2-5-gabc1234', 'v3.7.0-alpha.2-5-gabc1234'],
    ['3.6.0', 'v3.6.0'],
    ['dev', 'dev'],
    // `git describe --always` without a reachable tag: a bare commit hash.
    ['0a1b2c3', '0a1b2c3'],
  ] as const) {
    it(`shows version "${raw}" as "${shown}"`, async () => {
      wailsMock.getVersion.mockResolvedValue(raw);
      const fixture = await createAndWait('dark');
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      const text = fixture.nativeElement.querySelector('[data-testid="version-text"]')?.textContent?.trim();
      expect(text).toBe(shown);
    });
  }

  it('shows update indicator when update is available', async () => {
    wailsMock.getVersion.mockResolvedValue('4.1.7');
    wailsMock.checkForUpdate.mockResolvedValue({ ...defaultUpdateInfo, is_available: true, latest_version: '4.1.8' });
    const fixture = await createAndWait('dark');
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="update-indicator"]')).toBeTruthy();
  });

  it('hides update indicator when no update is available', async () => {
    wailsMock.getVersion.mockResolvedValue('4.1.8');
    wailsMock.checkForUpdate.mockResolvedValue({ ...defaultUpdateInfo, is_available: false });
    const fixture = await createAndWait('dark');
    const el: HTMLElement = fixture.nativeElement;
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="update-indicator"]')).toBeFalsy();
  });

  describe('in a dev build', () => {
    const identity = { is_dev_build: true, kind: 'pr', pr: 12, commit: 'abc1234', tag: 'v0.0.0-pr.12' };

    async function renderDevBuild(channel: Partial<typeof defaultDevChannel>): Promise<HTMLElement> {
      wailsMock.getVersion.mockResolvedValue('0.0.0-pr.12+abc1234');
      wailsMock.getBuildIdentity.mockResolvedValue({ ...identity });
      wailsMock.listDevBuilds.mockResolvedValue({ ...defaultDevChannel, current: { ...identity }, ...channel });
      const fixture = await createAndWait('dark');
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();
      return fixture.nativeElement;
    }

    it('names the build in the footer and never runs the normal update check', async () => {
      const el = await renderDevBuild({});
      expect(el.querySelector('[data-testid="version-text"]')?.textContent?.trim()).toBe('PR #12 · abc1234');
      expect(wailsMock.checkForUpdate).not.toHaveBeenCalled();
      expect(el.querySelector('[data-testid="update-indicator"]')).toBeFalsy();
    });

    it('points to About when the PR build is orphaned', async () => {
      const el = await renderDevBuild({ orphaned: true });
      const indicator = el.querySelector('[data-testid="update-indicator"]');
      expect(indicator?.getAttribute('title')).toContain('This test build is gone');
    });

    it('points to About when a new release is out', async () => {
      const el = await renderDevBuild({ new_release_since_build: true, latest_release: '4.5.0-beta' });
      expect(el.querySelector('[data-testid="update-indicator"]')?.getAttribute('title')).toContain('A new release is out (v4.5.0-beta)');
    });

    it('points to About when a newer build of it is up', async () => {
      const el = await renderDevBuild({
        builds: [{ tag: 'v0.0.0-pr.12', kind: 'pr', pr: 12, title: '', pr_url: '', commit: 'fffffff', date: '', installable: true, installed: false, newer_build: true }],
      });
      expect(el.querySelector('[data-testid="update-indicator"]')?.getAttribute('title')).toBe('A newer test build is available');
    });
  });

  it('goToAbout navigates to /settings with tab=about', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);
    fixture.componentInstance.goToAbout();
    expect(navigateSpy).toHaveBeenCalledWith(['/settings'], { queryParams: { tab: 'about' } });
  });

  it('navigates to /enhance on shortcutPyramidize$', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    wailsMock._shortcutPyramidize$.next('hotkey');
    await fixture.whenStable();

    expect(navigateSpy).toHaveBeenCalledWith(['/enhance']);
  });

  // The silent fix runs in Go (internal/features/silentfix); the shell only
  // routes a click on its notification. It must never run a fix itself — a
  // second, frontend copy of the pipeline is what #93's double paste was.
  it('runs no fix of its own: nothing reads, enhances or pastes', async () => {
    const fixture = await createAndWait('dark');
    vi.spyOn(TestBed.inject(Router), 'navigate').mockResolvedValue(true);
    wailsMock._silentFixOpen$.next({
      id: 'silentfix-1', title: "Fix didn't run", body: 'No API key for Anthropic.',
      target: 'providers', detail: '', input: 'their going', output: '',
    });
    await fixture.whenStable();

    expect(wailsMock.readClipboard).not.toHaveBeenCalled();
    expect(wailsMock.enhance).not.toHaveBeenCalled();
    expect(wailsMock.writeClipboard).not.toHaveBeenCalled();
    expect(wailsMock.pasteToForeground).not.toHaveBeenCalled();
  });

  it('sends a key or sign-in problem to the AI Providers tab', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    wailsMock._silentFixOpen$.next({
      id: 'silentfix-1', title: "Fix didn't run", body: "Claude Code CLI isn't signed in.",
      target: 'providers', detail: '', input: '', output: '',
    });
    await fixture.whenStable();

    expect(navigateSpy).toHaveBeenCalledWith(['/settings'], { queryParams: { tab: 'providers' } });
  });

  it('sends anything else to the Fix page', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    wailsMock._silentFixOpen$.next({
      id: 'silentfix-2', title: 'Fix took too long', body: 'The model took too long — try a shorter selection or a faster model.',
      target: 'fix', detail: 'Claude took too long to answer', input: 'their going', output: '',
    });
    await fixture.whenStable();

    expect(navigateSpy).toHaveBeenCalledWith(['/fix']);
  });
});
