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
  /** What GetDefaultDocType answers: the form preselected for a capture. */
  defaultDocType: string;
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
    defaultDocType: 'email',
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
      case 'GetDefaultDocType':
        return json(state.defaultDocType);
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
