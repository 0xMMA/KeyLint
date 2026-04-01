import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TextEnhancementService } from './text-enhancement.service';
import { WailsService } from '../../core/wails.service';
import { createWailsMock } from '../../../testing/wails-mock';

describe('TextEnhancementService', () => {
  let svc: TextEnhancementService;
  let wailsMock: ReturnType<typeof createWailsMock>;

  const mockPyramidizeResult = {
    documentType: 'EMAIL',
    language: 'en',
    fullDocument: 'Body text',
    headers: [],
    qualityScore: 0.9,
    qualityFlags: [],
    appliedRefinement: false,
    refinementWarning: '',
    detectedType: 'EMAIL',
    detectedLang: 'en',
    detectedConfidence: 0.95,
  };

  beforeEach(() => {
    wailsMock = createWailsMock();

    TestBed.configureTestingModule({
      providers: [
        TextEnhancementService,
        { provide: WailsService, useValue: wailsMock },
      ],
    });
    svc = TestBed.inject(TextEnhancementService);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // ── Legacy enhance() path ──

  it('enhance() delegates to wails.enhance() and returns the result', async () => {
    wailsMock.enhance.mockResolvedValue('Backend enhanced text.');
    const result = await svc.enhance('bad grammer');
    expect(wailsMock.enhance).toHaveBeenCalledWith('bad grammer');
    expect(result).toBe('Backend enhanced text.');
  });

  // ── Pyramidize delegation ──

  it('pyramidize() delegates to wails.pyramidize()', async () => {
    wailsMock.pyramidize.mockResolvedValue(mockPyramidizeResult);
    const req = { text: 'hello', documentType: 'auto', communicationStyle: 'professional', relationshipLevel: 'professional', customInstructions: '', provider: 'claude', model: 'claude-sonnet-4-6', promptVariant: 0 };
    const result = await svc.pyramidize(req);
    expect(wailsMock.pyramidize).toHaveBeenCalledWith(req);
    expect(result).toEqual(mockPyramidizeResult);
  });

  // ── RefineGlobal delegation ──

  it('refineGlobal() delegates to wails.refineGlobal()', async () => {
    const mockResult = { newCanvas: 'Refined text' };
    wailsMock.refineGlobal.mockResolvedValue(mockResult);
    const req = { fullCanvas: 'canvas', originalText: 'orig', instruction: 'shorter', documentType: 'email', communicationStyle: 'professional', relationshipLevel: 'professional', provider: 'claude', model: 'claude-sonnet-4-6' };
    const result = await svc.refineGlobal(req);
    expect(wailsMock.refineGlobal).toHaveBeenCalledWith(req);
    expect(result).toEqual(mockResult);
  });

  // ── Splice delegation ──

  it('splice() delegates to wails.splice()', async () => {
    const mockResult = { rewrittenSection: 'New section' };
    wailsMock.splice.mockResolvedValue(mockResult);
    const req = { fullCanvas: 'canvas', originalText: 'orig', selectedText: 'selected', instruction: 'rewrite', provider: 'claude', model: 'claude-sonnet-4-6' };
    const result = await svc.splice(req);
    expect(wailsMock.splice).toHaveBeenCalledWith(req);
    expect(result).toEqual(mockResult);
  });

  // ── CancelOperation delegation ──

  it('cancelOperation() delegates to wails.cancelOperation()', async () => {
    await svc.cancelOperation();
    expect(wailsMock.cancelOperation).toHaveBeenCalled();
  });

  // ── AppPresets delegation ──

  it('getAppPresets() delegates to wails.getAppPresets()', async () => {
    const mockPresets = [{ sourceApp: 'Outlook', documentType: 'email' }];
    wailsMock.getAppPresets.mockResolvedValue(mockPresets);
    const result = await svc.getAppPresets();
    expect(wailsMock.getAppPresets).toHaveBeenCalled();
    expect(result).toEqual(mockPresets);
  });

  it('setAppPreset() delegates to wails.setAppPreset()', async () => {
    const preset = { sourceApp: 'Outlook', documentType: 'email' };
    await svc.setAppPreset(preset);
    expect(wailsMock.setAppPreset).toHaveBeenCalledWith(preset);
  });

  it('deleteAppPreset() delegates to wails.deleteAppPreset()', async () => {
    await svc.deleteAppPreset('Outlook');
    expect(wailsMock.deleteAppPreset).toHaveBeenCalledWith('Outlook');
  });

  // ── QualityThreshold delegation ──

  it('getQualityThreshold() delegates to wails.getQualityThreshold()', async () => {
    wailsMock.getQualityThreshold.mockResolvedValue(0.75);
    const result = await svc.getQualityThreshold();
    expect(wailsMock.getQualityThreshold).toHaveBeenCalled();
    expect(result).toBe(0.75);
  });

  it('setQualityThreshold() delegates to wails.setQualityThreshold()', async () => {
    await svc.setQualityThreshold(0.8);
    expect(wailsMock.setQualityThreshold).toHaveBeenCalledWith(0.8);
  });

  // ── enhance() propagates errors ──

  it('enhance() propagates backend errors to caller', async () => {
    wailsMock.enhance.mockRejectedValue(new Error('API key not configured'));
    await expect(svc.enhance('text')).rejects.toThrow('API key not configured');
  });
});
