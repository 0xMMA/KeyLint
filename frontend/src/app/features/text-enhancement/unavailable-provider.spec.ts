import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { ChangeDetectorRef } from '@angular/core';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent, resetPyramidizeSession } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockResolvedValue({ documentType: 'EMAIL', language: 'en', fullDocument: 'doc' }),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: '' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: '' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
  };
}

// A settings file that still names AWS Bedrock (#22): Settings offers no way to
// pick it, so the page says why nothing can run instead of failing later with a
// raw backend error. The value is not switched for the user: provider, model
// and effort are Settings' business only.
describe('TextEnhancementComponent — Settings name a provider that is not available yet', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  async function settle(): Promise<void> {
    for (let i = 0; i < 3; i++) {
      await new Promise(r => setTimeout(r));
      fixture.debugElement.injector.get(ChangeDetectorRef).markForCheck();
      fixture.detectChanges();
      await fixture.whenStable();
    }
  }

  beforeEach(async () => {
    resetPyramidizeSession();
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, active_provider: 'bedrock' });
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });
    svcMock = makeEnhancementServiceMock();
    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(TextEnhancementComponent);
    el = fixture.nativeElement;
    fixture.componentInstance.originalTextView = 'Some text to pyramidize';
    fixture.componentInstance.goToStep('choice');
    fixture.detectChanges();
    await settle();
  });

  afterEach(() => {
    fixture.destroy();
    resetPyramidizeSession();
  });

  it('says why nothing can run, and shows no key banner', () => {
    expect(el.querySelector('[data-testid="provider-unavailable"]')?.textContent).toContain('AWS Bedrock is not available yet');
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeNull();
    expect(el.querySelector('[data-testid="run-settings"]')?.textContent).toContain('AWS Bedrock');
  });

  it('does not ask the backend about a provider it cannot use', () => {
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalled();
  });

  it('keeps Pyramidize off, by button and by Enter', async () => {
    expect(el.querySelector<HTMLButtonElement>('[data-testid="pyramidize-btn"] button')!.disabled).toBe(true);
    document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }));
    await fixture.componentInstance.pyramidize();
    await settle();
    expect(svcMock.pyramidize).not.toHaveBeenCalled();
  });
});
