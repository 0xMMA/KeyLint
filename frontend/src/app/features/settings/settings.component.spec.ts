import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { ActivatedRoute, convertToParamMap } from '@angular/router';
import { SettingsComponent } from './settings.component';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings, defaultKeyStatus, defaultUpdateInfo } from '../../../testing/wails-mock';

function makeActivatedRoute(tab?: string): Partial<ActivatedRoute> {
  return {
    snapshot: {
      queryParamMap: convertToParamMap(tab ? { tab } : {}),
    } as ActivatedRoute['snapshot'],
  };
}

// PrimeNG TabList uses ResizeObserver which is not available in jsdom
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

describe('SettingsComponent', () => {
  let fixture: ComponentFixture<SettingsComponent>;
  let component: SettingsComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings });
    wailsMock.getKeyStatus.mockResolvedValue({ ...defaultKeyStatus });

    await TestBed.configureTestingModule({
      imports: [SettingsComponent],
      providers: [
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: ActivatedRoute, useValue: makeActivatedRoute() },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(SettingsComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;

    // Pre-initialize to avoid @if(settings) NG0100 from async ngOnInit
    component.settings = { ...defaultSettings };

    fixture.detectChanges();
    await fixture.whenStable();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  // --- DOM tests ---

  it('renders tab list after settings load', () => {
    expect(el.querySelector('p-tablist')).toBeTruthy();
  });

  it('log-level section contains select and hint text', () => {
    const section = el.querySelector('[data-testid="log-level-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-select')).toBeTruthy();
    expect(section!.querySelector('small')).toBeTruthy();
  });

  it('sensitive-logging section contains both toggle and hint text', () => {
    const section = el.querySelector('[data-testid="sensitive-logging-section"]');
    expect(section).toBeTruthy();
    expect(section!.querySelector('p-toggle-switch')).toBeTruthy();
    expect(section!.querySelector('small')).toBeTruthy();
  });

  it('shortcut key input is present with correct initial value', () => {
    const input = el.querySelector<HTMLInputElement>('[data-testid="shortcut-input"]');
    expect(input).toBeTruthy();
    expect(input?.value).toBe('ctrl+g');
  });

  it('Save button is present', () => {
    expect(el.querySelector('[data-testid="save-btn"]')).toBeTruthy();
  });

  it('Reset to Defaults button is present', () => {
    expect(el.querySelector('[data-testid="reset-btn"]')).toBeTruthy();
  });

  it('saved banner appears in DOM after save', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="saved-banner"]')).toBeTruthy();
  });

  // --- Logic tests ---

  it('creates successfully', () => {
    expect(component).toBeTruthy();
  });

  it('loads settings on init', async () => {
    await component.ngOnInit();
    expect(wailsMock.loadSettings).toHaveBeenCalled();
    expect(component.settings).toMatchObject({ active_provider: 'openai' });
  });

  it('ngOnInit loads key statuses for all providers', async () => {
    await component.ngOnInit();
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('openai');
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('claude');
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('bedrock');
  });

  it('save() calls saveSettings with current settings', async () => {
    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ active_provider: 'openai' }),
    );
  });

  it('save() shows saved banner', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    expect(component.saved).toBe(true);
  });

  it('save() hides saved banner after 3 seconds', async () => {
    vi.useFakeTimers();
    void component.save();
    await Promise.resolve();
    vi.advanceTimersByTime(3000);
    expect(component.saved).toBe(false);
  });

  it('save sends log_level to backend', async () => {
    component.settings!.log_level = 'warning';
    await component.save();
    expect(wailsMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ log_level: 'warning' }),
    );
  });

  it('save() does nothing when settings is null', async () => {
    component.settings = null;
    await component.save();
    expect(wailsMock.saveSettings).not.toHaveBeenCalled();
  });

  it('resetToDefaults() calls resetSettings and reloads settings', async () => {
    await component.resetToDefaults();
    expect(wailsMock.resetSettings).toHaveBeenCalled();
    expect(wailsMock.loadSettings).toHaveBeenCalled();
  });

  it('saveKey() calls wails.setKey and refreshes status', async () => {
    wailsMock.setKey.mockResolvedValue(undefined);
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' });

    const pk = component.providerKeys[0]; // openai
    pk.draftKey = 'sk-test';
    await component.saveKey(pk);

    expect(wailsMock.setKey).toHaveBeenCalledWith('openai', 'sk-test');
    expect(pk.status?.is_set).toBe(true);
    expect(pk.editing).toBe(false);
  });

  it('clearKey() calls wails.deleteKey and refreshes status', async () => {
    wailsMock.deleteKey.mockResolvedValue(undefined);
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });

    const pk = component.providerKeys[0];
    await component.clearKey(pk);

    expect(wailsMock.deleteKey).toHaveBeenCalledWith('openai');
    expect(pk.status?.is_set).toBe(false);
  });

  describe('activeTab from query param', () => {
    it('defaults to "general" when no tab param is present', async () => {
      // Outer beforeEach provides makeActivatedRoute() with no tab param
      await component.ngOnInit();
      expect(component.activeTab).toBe('general');
    });

    it('sets activeTab to "about" when tab=about query param is present', async () => {
      const route = TestBed.inject(ActivatedRoute);
      vi.spyOn(route.snapshot.queryParamMap, 'get').mockReturnValue('about');
      await component.ngOnInit();
      expect(component.activeTab).toBe('about');
    });
  });

  describe('About tab', () => {
    it('displays app version after init', async () => {
      wailsMock.getVersion.mockResolvedValue('3.6.0');
      await component.ngOnInit();
      fixture.detectChanges();
      expect(component.appVersion).toBe('3.6.0');
    });

    it('checkForUpdate() sets updateInfo when update is available', async () => {
      wailsMock.checkForUpdate.mockResolvedValue({
        ...defaultUpdateInfo,
        is_available: true,
        latest_version: '3.7.0',
        notes: 'Bug fixes',
      });
      await component.checkForUpdate();
      expect(component.updateInfo?.is_available).toBe(true);
      expect(component.updateInfo?.latest_version).toBe('3.7.0');
      expect(component.updateError).toBe('');
    });

    it('checkForUpdate() sets updateError when check throws', async () => {
      wailsMock.checkForUpdate.mockRejectedValue(new Error('network error'));
      await component.checkForUpdate();
      expect(component.updateError).toContain('network error');
      expect(component.updateInfo).toBeNull();
    });

    it('update channel selector renders in About tab', async () => {
      // Click on the About tab to activate it.
      const aboutTab = el.querySelector('p-tab[value="about"]') as HTMLElement;
      aboutTab?.click();
      fixture.detectChanges();
      await fixture.whenStable();
      fixture.detectChanges();

      const section = el.querySelector('[data-testid="update-channel-section"]');
      expect(section).toBeTruthy();
      expect(section!.querySelector('p-select')).toBeTruthy();
    });

    it('installUpdate() sets updateSuccess on success', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: false });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(wailsMock.downloadAndInstall).toHaveBeenCalled();
    });

    it('installUpdate() shows restart message when restart_required is true', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: true });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(true);
    });

    it('installUpdate() shows standard success when restart_required is false', async () => {
      wailsMock.downloadAndInstall.mockResolvedValue({ restart_required: false });
      component.updateInfo = { ...defaultUpdateInfo, is_available: true, latest_version: '3.7.0' };
      await component.installUpdate();
      expect(component.updateSuccess).toBe(true);
      expect(component.updateRestartRequired).toBe(false);
    });
  });
});
