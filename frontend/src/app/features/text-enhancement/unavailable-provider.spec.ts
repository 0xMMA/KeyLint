import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { TestBed, ComponentFixture } from '@angular/core/testing';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideRouter } from '@angular/router';
import { TextEnhancementComponent } from './text-enhancement.component';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock, defaultSettings } from '../../../testing/wails-mock';

// PrimeNG TabList uses ResizeObserver, which jsdom lacks. Set here rather than
// relied on from another spec: files share globals only by scheduling.
(globalThis as Record<string, unknown>)['ResizeObserver'] = class {
  observe() {}
  unobserve() {}
  disconnect() {}
};

function makeEnhancementServiceMock() {
  return {
    pyramidize: vi.fn().mockResolvedValue({}),
    refineGlobal: vi.fn().mockResolvedValue({ newCanvas: '' }),
    splice: vi.fn().mockResolvedValue({ rewrittenSection: '' }),
    cancelOperation: vi.fn().mockResolvedValue(undefined),
    sendBack: vi.fn().mockResolvedValue(undefined),
    getSourceApp: vi.fn().mockResolvedValue(''),
    getAppPresets: vi.fn().mockResolvedValue([]),
    setAppPreset: vi.fn().mockResolvedValue(undefined),
    deleteAppPreset: vi.fn().mockResolvedValue(undefined),
    getQualityThreshold: vi.fn().mockResolvedValue(0.65),
    setQualityThreshold: vi.fn().mockResolvedValue(undefined),
    enhance: vi.fn().mockResolvedValue(''),
  };
}

// A settings file that still names AWS Bedrock (#22): the provider dropdown no
// longer offers it, so the page says why the field is empty instead of failing
// later with a raw backend error. The value is not switched for the user.
describe('TextEnhancementComponent — saved provider that is not available yet', () => {
  let fixture: ComponentFixture<TextEnhancementComponent>;
  let component: TextEnhancementComponent;
  let el: HTMLElement;
  let wailsMock: ReturnType<typeof createWailsMock>;
  let svcMock: ReturnType<typeof makeEnhancementServiceMock>;

  beforeEach(async () => {
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({ ...defaultSettings, active_provider: 'bedrock' });
    wailsMock.getKeyStatus.mockResolvedValue({ is_set: false, source: 'none' });
    svcMock = makeEnhancementServiceMock();

    await TestBed.configureTestingModule({
      imports: [TextEnhancementComponent],
      providers: [
        provideRouter([]),
        provideAnimationsAsync(),
        { provide: WailsService, useValue: wailsMock },
        { provide: TextEnhancementService, useValue: svcMock },
      ],
    }).compileComponents();

    fixture = TestBed.createComponent(TextEnhancementComponent);
    component = fixture.componentInstance;
    el = fixture.nativeElement;
    // Provider choice is module-level; clear it so ngOnInit reads settings.
    component.providerView = '';
    component.originalTextView = 'Some text to pyramidize';
    component.canvasTextView = '';
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
  });

  afterEach(() => {
    // Module-level state outlives the fixture; put it back to a fresh page.
    component.originalTextView = '';
    component.canvasTextView = '';
    component.providerView = '';
  });

  function pyramidizeButton(): HTMLButtonElement {
    return el.querySelector<HTMLButtonElement>('[data-testid="pyramidize-btn"] button')!;
  }

  it('says why the provider field is empty, and shows no key banner', () => {
    const note = el.querySelector('[data-testid="provider-unavailable"]');
    expect(note?.textContent).toContain('AWS Bedrock is not available yet');
    // A choice here lasts for the session only; Settings is where it sticks.
    expect(note?.textContent).toContain('Settings');
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeNull();
  });

  it('does not ask the backend about a provider it cannot use', () => {
    expect(wailsMock.listModels).not.toHaveBeenCalledWith('bedrock');
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalledWith('bedrock');
  });

  it('keeps Pyramidize disabled until a provider is chosen', async () => {
    expect(pyramidizeButton().disabled).toBe(true);

    await component.pyramidize();
    expect(svcMock.pyramidize).not.toHaveBeenCalled();
  });

  // The canvas is editable before any Pyramidize, so the refine paths need the
  // same guard: otherwise they end in a raw "unsupported provider" error.
  it('keeps the canvas refinements from reaching the backend', async () => {
    component.canvasTextView = 'Some canvas text';
    fixture.detectChanges();
    const input = el.querySelector<HTMLInputElement>('[data-testid="global-instruction-input"]')!;
    input.value = 'make it shorter';
    input.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    const apply = el.querySelector<HTMLButtonElement>('[data-testid="apply-instruction-btn"] button')!;
    expect(apply.disabled).toBe(true);
    expect(apply.classList).toContain('p-button-secondary');

    await component.applyGlobalInstruction();
    component.selectionInstruction = 'rephrase';
    await component.applySelectionInstruction();
    expect(svcMock.refineGlobal).not.toHaveBeenCalled();
    expect(svcMock.splice).not.toHaveBeenCalled();
    expect(component.errorMessage).toBe('');
  });

  it('clears the note and enables Pyramidize once the user picks a provider', async () => {
    component.providerView = 'openai';
    await component.onProviderChange();
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();

    expect(el.querySelector('[data-testid="provider-unavailable"]')).toBeNull();
    expect(pyramidizeButton().disabled).toBe(false);
  });
});
