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
    document.body.classList.remove('app-dark');

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
    document.body.classList.remove('app-dark');
  });

  async function createAndWait(theme_preference: string): Promise<ComponentFixture<ShellComponent>> {
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, theme_preference: theme_preference as never });
    const fixture = TestBed.createComponent(ShellComponent);
    fixture.detectChanges();
    await fixture.whenStable();
    return fixture;
  }

  it('adds app-dark to body for dark theme', async () => {
    await createAndWait('dark');
    expect(document.body.classList.contains('app-dark')).toBe(true);
  });

  it('removes app-dark from body for light theme', async () => {
    document.body.classList.add('app-dark');
    await createAndWait('light');
    expect(document.body.classList.contains('app-dark')).toBe(false);
  });

  it('keeps app-dark for system theme (dark-first app)', async () => {
    await createAndWait('system');
    expect(document.body.classList.contains('app-dark')).toBe(true);
  });

  it('re-applies theme when settingsChanged$ emits', async () => {
    const fixture = await createAndWait('dark');
    expect(document.body.classList.contains('app-dark')).toBe(true);

    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, theme_preference: 'light' });
    wailsMock._settingsChanged$.next();
    await fixture.whenStable();

    expect(document.body.classList.contains('app-dark')).toBe(false);
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

  it('navigates to /enhance on shortcutPyramidize$', async () => {
    const fixture = await createAndWait('dark');
    const router = TestBed.inject(Router);
    const navigateSpy = vi.spyOn(router, 'navigate').mockResolvedValue(true);

    wailsMock._shortcutPyramidize$.next('hotkey');
    await fixture.whenStable();

    expect(navigateSpy).toHaveBeenCalledWith(['/enhance']);
  });

  it('shortcutFix$ triggers silent fix (enhance + paste)', async () => {
    wailsMock.readClipboard.mockResolvedValue('bad grammer');
    wailsMock.enhance.mockResolvedValue('bad grammar');
    await createAndWait('dark');

    wailsMock._shortcutFix$.next('hotkey');
    await new Promise(r => setTimeout(r, 0));

    expect(wailsMock.readClipboard).toHaveBeenCalled();
    expect(wailsMock.enhance).toHaveBeenCalledWith('bad grammer');
    expect(wailsMock.writeClipboard).toHaveBeenCalledWith('bad grammar');
    expect(wailsMock.pasteToForeground).toHaveBeenCalled();
  });
});
