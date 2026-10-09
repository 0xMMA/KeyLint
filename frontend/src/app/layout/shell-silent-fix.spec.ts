import { describe, it, expect, beforeEach } from 'vitest';
import { Component } from '@angular/core';
import { By } from '@angular/platform-browser';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router, ActivatedRoute } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { ShellComponent } from './shell.component';
import { SettingsComponent } from '../features/settings/settings.component';
import { FixComponent } from '../features/fix/fix.component';
import { WailsService, type SilentFixNotice } from '../core/wails.service';
import { createWailsMock } from '../../testing/wails-mock';
import { SilentFixNoticeService } from '../features/fix/silent-fix-notice.service';

// jsdom gaps that PrimeNG reaches for (the Settings tabs). Set here, not
// borrowed from another file: see .claude/rules/testing.md.
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false, media: query, onchange: null,
    addListener() {}, removeListener() {},
    addEventListener() {}, removeEventListener() {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

// A click on a hotkey-fix notification, through the real shell and router:
// what the user sees is the page that can fix the problem, already showing it.
describe('ShellComponent — a clicked hotkey-fix notification', () => {
  let wailsMock: ReturnType<typeof createWailsMock>;
  let router: Router;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    await TestBed.configureTestingModule({
      imports: [ShellComponent],
      providers: [
        provideAnimationsAsync(),
        provideRouter([
          {
            path: '',
            component: ShellComponent,
            children: [
              { path: 'fix', component: FixComponent },
              { path: 'settings', component: SettingsComponent },
              { path: '', redirectTo: 'fix', pathMatch: 'full' },
            ],
          },
        ]),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();
    router = TestBed.inject(Router);
  });

  function notice(overrides: Partial<SilentFixNotice>): SilentFixNotice {
    return {
      id: 'silentfix-1', title: "Fix didn't run", body: '', target: 'fix',
      detail: '', input: '', output: '', ...overrides,
    };
  }

  async function settle(harness: RouterTestingHarness): Promise<void> {
    // The navigation, then the routed page's own async start-up.
    for (let i = 0; i < 3; i++) {
      harness.fixture.detectChanges();
      await harness.fixture.whenStable();
    }
  }

  function selectedTab(el: HTMLElement): string | undefined {
    return el.querySelector('[role="tab"][aria-selected="true"]')?.textContent?.trim();
  }

  it('switches to AI Providers when Settings is already open on another tab', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/settings']);
    await settle(harness);
    const el: HTMLElement = harness.fixture.nativeElement;
    expect(selectedTab(el)).toBe('General');

    wailsMock._silentFixOpen$.next(notice({ body: 'No API key for Anthropic.', target: 'providers' }));
    await settle(harness);

    expect(selectedTab(el)).toBe('AI Providers');
    expect(el.querySelector('[data-testid="silent-fix-note"]')?.textContent?.trim())
      .toBe("Fix didn't run: No API key for Anthropic.");
  });

  // The router ignores a navigation to the URL it is already on, so a second
  // click while Settings sits on ?tab=providers must still bring the tab back.
  it('brings AI Providers back on a second click with the URL unchanged', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/settings']);
    await settle(harness);
    const el: HTMLElement = harness.fixture.nativeElement;

    wailsMock._silentFixOpen$.next(notice({ body: "Claude Code CLI isn't signed in.", target: 'providers' }));
    await settle(harness);
    expect(router.url).toBe('/settings?tab=providers');

    // The user wanders off to another tab; the URL stays as it was.
    const general = Array.from(el.querySelectorAll<HTMLElement>('[role="tab"]')).find(t => t.textContent?.trim() === 'General');
    general!.click();
    await settle(harness);
    expect(selectedTab(el)).toBe('General');

    wailsMock._silentFixOpen$.next(notice({ id: 'silentfix-2', body: "Claude Code CLI isn't signed in.", target: 'providers' }));
    await settle(harness);
    expect(selectedTab(el)).toBe('AI Providers');
  });

  it('lets the user dismiss the note', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/settings']);
    await settle(harness);
    const el: HTMLElement = harness.fixture.nativeElement;

    wailsMock._silentFixOpen$.next(notice({ body: 'No API key for Anthropic.', target: 'providers' }));
    await settle(harness);
    el.querySelector<HTMLElement>('[data-testid="silent-fix-note"] button')!.click();
    await settle(harness);

    expect(el.querySelector('[data-testid="silent-fix-note"]')).toBeFalsy();
  });

  it('drops the note once the user saves a key', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/settings']);
    await settle(harness);
    const settings: SettingsComponent = harness.fixture.debugElement.query(By.directive(SettingsComponent)).componentInstance;
    const el: HTMLElement = harness.fixture.nativeElement;

    wailsMock._silentFixOpen$.next(notice({ body: 'No API key for Anthropic.', target: 'providers' }));
    await settle(harness);
    expect(el.querySelector('[data-testid="silent-fix-note"]')).toBeTruthy();

    wailsMock.getKeyStatus.mockResolvedValue({ is_set: true, source: 'keyring' } as never);
    const anthropic = settings.providerKeys.find(k => k.id === 'claude')!;
    anthropic.draftKey = 'sk-ant-new';
    await settings.saveKey(anthropic);
    await settle(harness);

    expect(el.querySelector('[data-testid="silent-fix-note"]')).toBeFalsy();
  });

  it('does not show a notice meant for AI Providers on the Fix page', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/fix']);
    await settle(harness);

    // Settings never takes it in this test — the stand-in case of a click
    // whose page was not reached.
    TestBed.inject(SilentFixNoticeService).present(notice({ body: 'No API key for Anthropic.', target: 'providers' }));
    await settle(harness);

    expect(harness.fixture.nativeElement.querySelector('[data-testid="silent-fix-notice"]')).toBeFalsy();
  });

  it('opens the Fix page with the reason and the text for anything else', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/settings']);
    await settle(harness);

    wailsMock._silentFixOpen$.next(notice({
      title: 'Fix took too long',
      body: 'The model took too long — try a shorter selection or a faster model.',
      detail: 'Claude took too long to answer — try a shorter selection or a faster model',
      input: 'a long selection their going',
    }));
    await settle(harness);

    const el: HTMLElement = harness.fixture.nativeElement;
    expect(el.querySelector('.fix-page')).toBeTruthy();
    expect(el.querySelector('[data-testid="silent-fix-notice-title"]')?.textContent).toContain('Fix took too long');
    expect(el.querySelector('[data-testid="silent-fix-notice-body"]')?.textContent).toContain('try a shorter selection');
    expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-input"]')?.value).toBe('a long selection their going');
  });

  it('updates the Fix page in place when it is already open', async () => {
    const harness = await RouterTestingHarness.create();
    await router.navigate(['/fix']);
    await settle(harness);

    wailsMock._silentFixOpen$.next(notice({
      title: 'Fixed, not pasted', body: 'You switched windows — the fix is on the clipboard.',
      input: 'their going home', output: "They're going home.",
    }));
    await settle(harness);

    const el: HTMLElement = harness.fixture.nativeElement;
    expect(el.querySelectorAll('[data-testid="silent-fix-notice"]')).toHaveLength(1);
    expect(el.querySelector('[data-testid="silent-fix-notice-body"]')?.textContent).toContain('You switched windows');
    expect(el.querySelector<HTMLTextAreaElement>('[data-testid="fix-output"]')?.value).toBe("They're going home.");
  });
});

// From any other page to AI Providers, through the real shell and router. The
// Settings page is a stand-in here that shows which tab it was asked for:
// creating the real one on its AI Providers tab inside RouterTestingHarness
// trips Angular's dev-mode NG0100 check on main as well (its @if (settings)
// block, with or without this change). The real page switching tabs on the
// same request is covered above.
describe('ShellComponent — a key or sign-in notification from another page', () => {
  @Component({ standalone: true, template: `<div class="stub-settings" data-testid="stub-tab">{{ tab }}</div>` })
  class StubSettings {
    tab = '';
    constructor(route: ActivatedRoute) {
      route.queryParamMap.subscribe(p => { this.tab = p.get('tab') ?? ''; });
    }
  }

  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(async () => {
    wailsMock = createWailsMock();
    await TestBed.configureTestingModule({
      imports: [ShellComponent],
      providers: [
        provideAnimationsAsync(),
        provideRouter([
          {
            path: '',
            component: ShellComponent,
            children: [
              { path: 'fix', component: FixComponent },
              { path: 'settings', component: StubSettings },
              { path: '', redirectTo: 'fix', pathMatch: 'full' },
            ],
          },
        ]),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();
  });

  for (const body of ["Claude Code CLI isn't signed in.", 'No API key for OpenAI.', "Claude didn't accept the API key."]) {
    it(`opens AI Providers for "${body}"`, async () => {
      const harness = await RouterTestingHarness.create();
      await TestBed.inject(Router).navigate(['/fix']);
      harness.fixture.detectChanges();
      await harness.fixture.whenStable();

      wailsMock._silentFixOpen$.next({
        id: 'silentfix-1', title: "Fix didn't run", body, target: 'providers', detail: '', input: '', output: '',
      });
      await harness.fixture.whenStable();
      harness.fixture.detectChanges();

      const el: HTMLElement = harness.fixture.nativeElement;
      expect(el.querySelector('.fix-page')).toBeFalsy();
      expect(el.querySelector('[data-testid="stub-tab"]')?.textContent).toBe('providers');
    });
  }
});

