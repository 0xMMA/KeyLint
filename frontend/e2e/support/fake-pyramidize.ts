/**
 * A stand-in for the Go Pyramidize service (and the clipboard read), for specs
 * that drive Pyramidize past its empty state without reaching a provider.
 *
 * Install it after `installFakeBackend`: Playwright runs the route handler
 * registered last first, so this answers the Pyramidize calls and falls back
 * to the settings fake (and from there to `ng serve`) for everything else.
 * Method IDs come from the generated bindings, as in fake-backend.ts.
 */
import { Page } from '@playwright/test';
import { methodIDs } from './fake-backend';

export interface FakePyramidizeState {
  /** What GetSourceApp answers; '' hides Send back. */
  sourceApp: string;
  /** What the clipboard holds for "Paste from clipboard". */
  clipboard: string;
  /** The document Pyramidize returns. */
  fullDocument: string;
  /**
   * What GetSourcePreset answers for a capture: the app's name, the form to
   * preselect, and whether that form is remembered. SetAppPreset and
   * DeleteAppPreset change it as the backend would.
   */
  sourcePreset: { app: string; documentType: string; remembered: boolean };
  /** What SetAppPreset received, oldest first. */
  presetsSet: { sourceApp: string; documentType: string }[];
  /** What DeleteAppPreset received, oldest first. */
  presetsDeleted: string[];
  /** When set, Pyramidize fails with this message, as a Go error would. */
  failPyramidize?: string;
  /** When set, Pyramidize answers only once this resolves (to show the step). */
  hold?: Promise<void>;
  /** The requests Pyramidize received, oldest first. */
  requests: Record<string, unknown>[];
  /** What SendBack received. */
  sentBack: string[];
}

export function fakePyramidize(overrides: Partial<FakePyramidizeState> = {}): FakePyramidizeState {
  return {
    sourceApp: '',
    clipboard: '',
    fullDocument: '',
    sourcePreset: { app: '', documentType: 'email', remembered: false },
    presetsSet: [],
    presetsDeleted: [],
    requests: [],
    sentBack: [],
    ...overrides,
  };
}

export async function installFakePyramidize(page: Page, state: FakePyramidizeState): Promise<void> {
  const ids = methodIDs('pyramidize');
  const clipboardIDs = methodIDs('clipboard');

  await page.route('**/wails/runtime', async (route) => {
    let body: { args?: { methodID?: number; args?: unknown[] } } = {};
    try {
      body = route.request().postDataJSON() ?? {};
    } catch {
      // Not a JSON binding call.
    }
    const id = body.args?.methodID ?? -1;
    const args = body.args?.args ?? [];
    const json = (value: unknown) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });
    const empty = () => route.fulfill({ status: 200, contentType: 'text/plain', body: '' });

    if (clipboardIDs.get(id) === 'Read') return json(state.clipboard);

    switch (ids.get(id)) {
      case 'GetSourceApp':
        return json(state.sourceApp);
      case 'GetSourcePreset':
        return json(state.sourcePreset);
      case 'SetAppPreset': {
        const preset = args[0] as { sourceApp: string; documentType: string };
        state.presetsSet.push(preset);
        state.sourcePreset = { app: preset.sourceApp, documentType: preset.documentType, remembered: true };
        return empty();
      }
      case 'DeleteAppPreset':
        state.presetsDeleted.push(args[0] as string);
        state.sourcePreset = { ...state.sourcePreset, documentType: 'email', remembered: false };
        return empty();
      case 'CancelOperation':
        return empty();
      case 'Pyramidize': {
        state.requests.push(args[0] as Record<string, unknown>);
        if (state.hold) await state.hold;
        if (state.failPyramidize) {
          return route.fulfill({ status: 500, contentType: 'text/plain', body: state.failPyramidize });
        }
        // PyramidizeResult since ADR-003: the form written, and the text.
        const requested = String((args[0] as { documentType?: string })?.documentType ?? 'email');
        return json({
          documentType: requested.toUpperCase(),
          language: 'en',
          fullDocument: state.fullDocument,
        });
      }
      case 'SendBack':
        state.sentBack.push(args[0] as string);
        return empty();
      default:
        return route.fallback();
    }
  });
}

/**
 * Presses the Pyramidize shortcut, as far as the page can tell: the event the
 * Go side emits after it captured the source app and copied the selection.
 * The Wails runtime in the page dispatches it to the app's listeners; the
 * fake above answers what the page then asks (clipboard, source app, preset).
 */
export async function pressPyramidizeShortcut(page: Page): Promise<void> {
  await page.evaluate(() => {
    const wails = (window as unknown as { _wails?: { dispatchWailsEvent?: (e: { name: string; data: unknown }) => void } })._wails;
    if (!wails?.dispatchWailsEvent) throw new Error('the Wails runtime is not loaded in the page');
    wails.dispatchWailsEvent({ name: 'shortcut:pyramidize', data: '' });
  });
}
