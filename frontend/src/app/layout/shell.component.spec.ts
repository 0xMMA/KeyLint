import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { ShellComponent } from './shell.component';
import { WailsService } from '../core/wails.service';
import { createWailsMock, defaultSettings, defaultUpdateInfo } from '../../testing/wails-mock';

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

  it('goToAbout navigates to /settings with tab=about', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);
    fixture.componentInstance.goToAbout();
    expect(navigateSpy).toHaveBeenCalledWith(['/settings'], { queryParams: { tab: 'about' } });
  });
});
