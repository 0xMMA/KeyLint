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

// AWS Bedrock in use (roadmap E6): the banner says what is missing — a region,
// or a key in key mode — and nothing at all with this computer's AWS sign-in,
// whose working can only be known by trying. Pyramidize is never held back.
describe('TextEnhancementComponent — AWS Bedrock in use', () => {
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

  async function render(providers: Record<string, string>): Promise<void> {
    resetPyramidizeSession();
    TestBed.resetTestingModule();
    wailsMock = createWailsMock();
    wailsMock.loadSettings.mockResolvedValue({
      ...defaultSettings, active_provider: 'bedrock', providers: { ...defaultSettings.providers, ...providers },
    });
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
  }

  afterEach(() => {
    fixture.destroy();
    resetPyramidizeSession();
  });

  it('names AWS Bedrock and its model on the line under the form choice', async () => {
    await render({ aws_region: 'eu-central-1', bedrock_auth: 'aws-signin' });
    expect(el.querySelector('[data-testid="run-settings"]')?.textContent).toContain('anthropic.claude-sonnet-5-5 · AWS Bedrock');
    expect(el.querySelector('[data-testid="provider-unavailable"]')).toBeNull();
  });

  it('says when no region is set', async () => {
    await render({ bedrock_auth: 'aws-signin' });
    expect(el.querySelector('[data-testid="api-key-banner-message"]')?.textContent).toContain('No AWS region is set for Bedrock');
  });

  it('asks for the key in key mode', async () => {
    await render({ aws_region: 'eu-central-1' });
    expect(wailsMock.getKeyStatus).toHaveBeenCalledWith('bedrock');
    expect(el.querySelector('[data-testid="api-key-banner-message"]')?.textContent).toContain('No AI API key configured');
  });

  it("asks for nothing with this computer's AWS sign-in, and runs", async () => {
    await render({ aws_region: 'eu-central-1', bedrock_auth: 'aws-signin' });
    expect(el.querySelector('[data-testid="api-key-banner"]')).toBeNull();
    expect(wailsMock.getKeyStatus).not.toHaveBeenCalledWith('bedrock');
    expect(el.querySelector<HTMLButtonElement>('[data-testid="pyramidize-btn"] button')!.disabled).toBe(false);
    await fixture.componentInstance.pyramidize();
    expect(svcMock.pyramidize).toHaveBeenCalled();
  });
});
