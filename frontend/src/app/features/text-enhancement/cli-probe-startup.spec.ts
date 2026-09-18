import { describe, it, expect, beforeEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

// PrimeNG TabList uses ResizeObserver which is not available in jsdom.
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

// Its own file on purpose: text-enhancement.component.ts keeps the selected
// provider in a module-level variable that survives between tests, so a spec
// sharing a file with others cannot choose which provider ngOnInit sees. This
// one needs claude-code, which is the only provider that probes the CLI.

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockResolvedValue({
      documentType: 'EMAIL', language: 'en', fullDocument: 'doc', headers: [],
      qualityScore: 0.9, qualityFlags: [], appliedRefinement: false,
      refinementWarning: '', detectedType: 'EMAIL', detectedLang: 'en', detectedConfidence: 1,
    }),
    refineGlobal: vi.fn(), splice: vi.fn(), sendBack: vi.fn(),
  };
}

// #55: the Claude Code probe spawns processes, and ngOnInit used to await it.
//
// Angular does not delay rendering for an async ngOnInit, so the template
// painted either way — what the await actually held up was everything after it:
// the model list, the quality threshold, and the hotkey subscription. With that
// provider selected and a slow CLI, the page was on screen but inert.
//
// So this asserts the startup that follows the probe, not the paint. An earlier
// version of this test checked that the page rendered, which passed with the
// await still in place and proved nothing.
describe('TextEnhancementComponent — startup does not wait on the CLI probe', () => {
  let wailsMock: ReturnType<typeof createWailsMock>;

  beforeEach(() => {
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, active_provider: 'claude-code' });
  });

  it('finishes starting up while the probe is still running', async () => {
    // A probe that never answers. Anything sequenced behind it never happens.
    let release: (v: unknown) => void = () => {};
    wailsMock.getClaudeCodeStatus.mockReturnValue(new Promise(resolve => { release = resolve; }));

    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideAnimationsAsync(),
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();

    const fixture = TestBed.createComponent(TextEnhancementComponent);
    const component = fixture.componentInstance;
    const el: HTMLElement = fixture.nativeElement;
    // Set before the first change detection, so ngOnInit's "adopt the active
    // provider if none is chosen" branch leaves it alone. The variable behind
    // this setter is module-level and shared across spec files, so it cannot be
    // steered through the settings mock.
    component.providerView = 'claude-code';
    fixture.detectChanges();
    // ngOnInit awaits several Wails calls before it reaches the work asserted
    // below. Drain them; with the probe awaited, draining never gets past it,
    // which is the difference this test exists to see.
    for (let i = 0; i < 10; i++) {
      await fixture.whenStable();
    }
    fixture.detectChanges();

    expect(el.querySelector('.pyramidize-page')).not.toBeNull();
    // The work sequenced after the probe in ngOnInit. These are what a hanging
    // CLI used to swallow.
    expect(wailsMock.getQualityThreshold, 'startup stopped at the probe').toHaveBeenCalled();
    expect(wailsMock.listModels, 'the model list never loaded').toHaveBeenCalled();

    release({ installed: true, path: '/usr/bin/claude', version: '1.0', loggedIn: true });
    await fixture.whenStable();
  });

  it('asks the backend for a cached answer, not a fresh probe', async () => {
    wailsMock.getClaudeCodeStatus.mockResolvedValue({
      installed: true, path: '/usr/bin/claude', version: '1.0', loggedIn: true,
    });

    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideAnimationsAsync(),
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: makeEnhancementServiceMock() },
      ],
    }).compileComponents();

    const fixture = TestBed.createComponent(TextEnhancementComponent);
    fixture.componentInstance.providerView = 'claude-code';
    fixture.detectChanges();
    await fixture.whenStable();
    await fixture.whenStable();

    // force is the re-check button's business; this page just wants the answer.
    for (const call of wailsMock.getClaudeCodeStatus.mock.calls) {
      expect(call[0] ?? false, 'the Pyramidize page must not force a fresh probe').toBe(false);
    }
  });
});
