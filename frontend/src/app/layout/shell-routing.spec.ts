import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideRouter, Router } from '@angular/router';
import { RouterTestingHarness } from '@angular/router/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { Component, OnDestroy } from '@angular/core';
import { ShellComponent } from './shell.component';
import { SettingsComponent } from '../features/settings/settings.component';
import { FixComponent } from '../features/fix/fix.component';
import { TextEnhancementComponent } from '../features/text-enhancement/text-enhancement.component';
import { WailsService } from '../core/wails.service';
import { createWailsMock, defaultSettings } from '../../testing/wails-mock';

// jsdom gaps that PrimeNG reaches for. See .claude/rules/testing.md.
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

// #38: after clicking the version footer (which navigates to Settings with a
// query param) and touching the tabs, navigating away left Settings on screen
// with the next page appended underneath it.
//
// These drive the real router through the real shell, so "the page was
// replaced" means what it means in the app: exactly one routed component in the
// outlet.
describe('ShellComponent — routing away from Settings', () => {
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
              { path: 'enhance', component: TextEnhancementComponent },
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

  /** How many routed feature components are currently in the DOM. */
  function routedPages(el: HTMLElement): string[] {
    const found: string[] = [];
    if (el.querySelector('.settings-page')) found.push('settings');
    if (el.querySelector('.fix-page')) found.push('fix');
    if (el.querySelector('.pyramidize-page')) found.push('enhance');
    return found;
  }

  it('replaces Settings when navigating away after the version click', async () => {
    const harness = await RouterTestingHarness.create();
    const fixture = harness.fixture;
    const el: HTMLElement = fixture.nativeElement;

    // 1. The version footer: Settings with the About tab selected.
    await router.navigate(['/settings'], { queryParams: { tab: 'about' } });
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(routedPages(el), 'settings should be the only page').toEqual(['settings']);

    // 2. Click through the tabs, the way the repro does.
    const tabs = Array.from(el.querySelectorAll<HTMLElement>('[role="tab"]'));
    for (const tab of tabs) {
      tab.click();
      fixture.detectChanges();
      await fixture.whenStable();
    }

    // 3. Navigate away.
    await router.navigate(['/enhance']);
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(routedPages(el), 'Settings must not survive the navigation').toEqual(['enhance']);
  });
});

// What a throwing destroy actually does, measured rather than assumed.
//
// #38 lists "an exception thrown during destroy" as a candidate cause, and the
// obvious guess is that RouterOutlet.deactivate() aborts before clearing its
// reference, leaving the old DOM attached for the next activation to land under.
// That guess is wrong on this Angular version, and the test below says so: the
// throw takes the whole navigation down and the outlet ends up EMPTY.
//
// Worth keeping, because it rules a hypothesis out. A blank content area and a
// stuck page are different bugs with different causes.
describe('what a throwing ngOnDestroy does to a router outlet', () => {
  @Component({ standalone: true, template: `<div class="page-a">A</div>` })
  class PageA implements OnDestroy {
    static throwOnDestroy = false;
    ngOnDestroy(): void {
      if (PageA.throwOnDestroy) {
        throw new Error('destroy blew up');
      }
    }
  }

  @Component({ standalone: true, template: `<div class="page-b">B</div>` })
  class PageB {}

  beforeEach(async () => {
    PageA.throwOnDestroy = false;
    await TestBed.configureTestingModule({
      providers: [
        provideRouter([
          { path: 'a', component: PageA },
          { path: 'b', component: PageB },
        ]),
      ],
    }).compileComponents();
  });

  it('replaces the page when destroy is quiet', async () => {
    const harness = await RouterTestingHarness.create();
    const el: HTMLElement = harness.fixture.nativeElement;

    await TestBed.inject(Router).navigate(['/a']);
    harness.fixture.detectChanges();
    expect(el.querySelectorAll('.page-a, .page-b')).toHaveLength(1);

    await TestBed.inject(Router).navigate(['/b']);
    harness.fixture.detectChanges();

    expect(Array.from(el.querySelectorAll('.page-a, .page-b')).map(n => n.className))
      .toEqual(['page-b']);
  });

  it('takes the whole navigation down when destroy throws, rather than stacking pages', async () => {
    const harness = await RouterTestingHarness.create();
    const el: HTMLElement = harness.fixture.nativeElement;
    const router = TestBed.inject(Router);

    await router.navigate(['/a']);
    harness.fixture.detectChanges();

    PageA.throwOnDestroy = true;
    await router.navigate(['/b']).catch(() => undefined);
    harness.fixture.detectChanges();

    const pages = Array.from(el.querySelectorAll('.page-a, .page-b')).map(n => n.className);
    expect(pages, 'a throwing destroy empties the outlet; it does not stack pages — so it is not the cause of #38')
      .toEqual([]);
  });
});

// The three unguarded refreshes this fix removes, as a behaviour rather than a
// diff: leaving Settings while its startup is still in flight must not throw.
//
// ngOnInit awaits five Wails calls before touching the view. Over the desktop
// bridge each is an IPC round trip, so a user who opens Settings and clicks
// away immediately gets that view refresh on a destroyed component — and the
// test above shows what an exception around navigation costs.
describe('SettingsComponent — leaving while it is still starting up', () => {
  it('does not refresh a view it no longer has', async () => {
    const wailsMock = createWailsMock();
    let releaseSettings: (v: unknown) => void = () => {};
    wailsMock.loadSettings.mockReturnValue(new Promise(resolve => { releaseSettings = resolve; }));

    await TestBed.configureTestingModule({
      providers: [
        provideAnimationsAsync(),
        provideRouter([
          { path: 'settings', component: SettingsComponent },
          { path: 'fix', component: FixComponent },
        ]),
        { provide: WailsService, useValue: wailsMock },
      ],
    }).compileComponents();

    const harness = await RouterTestingHarness.create();
    const router = TestBed.inject(Router);

    await router.navigate(['/settings']);
    harness.fixture.detectChanges();

    // Away again before the first Wails call has answered.
    await router.navigate(['/fix']);
    harness.fixture.detectChanges();

    const errors: unknown[] = [];
    const onError = (e: ErrorEvent) => errors.push(e.error ?? e.message);
    const onRejection = (e: PromiseRejectionEvent) => errors.push(e.reason);
    window.addEventListener('error', onError);
    window.addEventListener('unhandledrejection', onRejection);

    // Now let the startup finish into a component that is gone.
    releaseSettings({ ...defaultSettings });
    await new Promise(resolve => setTimeout(resolve, 0));
    harness.fixture.detectChanges();

    window.removeEventListener('error', onError);
    window.removeEventListener('unhandledrejection', onRejection);

    expect(errors, 'the late startup must not blow up on a destroyed view').toEqual([]);
  });
});
