import { Injectable } from '@angular/core';
import { WailsService, PyramidizeRequest, PyramidizeResult, RefineGlobalRequest, RefineGlobalResult, SpliceRequest, SpliceResult, AppPreset } from '../../core/wails.service';

export type { PyramidizeRequest, PyramidizeResult, RefineGlobalRequest, RefineGlobalResult, SpliceRequest, SpliceResult, AppPreset };

@Injectable({ providedIn: 'root' })
export class TextEnhancementService {
  constructor(private readonly wails: WailsService) {}

  pyramidize(req: PyramidizeRequest): Promise<PyramidizeResult> {
    return this.wails.pyramidize(req);
  }

  refineGlobal(req: RefineGlobalRequest): Promise<RefineGlobalResult> {
    return this.wails.refineGlobal(req);
  }

  splice(req: SpliceRequest): Promise<SpliceResult> {
    return this.wails.splice(req);
  }

  cancelOperation(): Promise<void> {
    return this.wails.cancelOperation();
  }

  sendBack(text: string): Promise<void> {
    return this.wails.sendBack(text);
  }

  getSourceApp(): Promise<string> {
    return this.wails.getSourceApp();
  }

  getAppPresets(): Promise<AppPreset[]> {
    return this.wails.getAppPresets();
  }

  setAppPreset(preset: AppPreset): Promise<void> {
    return this.wails.setAppPreset(preset);
  }

  deleteAppPreset(sourceApp: string): Promise<void> {
    return this.wails.deleteAppPreset(sourceApp);
  }

  getQualityThreshold(): Promise<number> {
    return this.wails.getQualityThreshold();
  }

  setQualityThreshold(v: number): Promise<void> {
    return this.wails.setQualityThreshold(v);
  }

  // Keep enhance() for backward compatibility with remaining callers.
  enhance(text: string): Promise<string> {
    return this.wails.enhance(text);
  }
}
