/**
 * Pyramidize fits the default window without a scrollbar, on every step.
 *
 * The page scrolls on its own (.canvas-area, overflow-y: auto) so a small
 * window still reaches every control, but at the size KeyLint opens with a
 * scrollbar is a layout bug: v4.7.0-beta shipped one in the old options column
 * (the spec this one replaces, pyramidize-options-fit, guarded that column).
 *
 * The window is 1280×800 (main.go), and that is the outer size: on Windows 11
 * at 100 % the title bar and frame take about 40 px of it, so the page gets
 * roughly 1264×760. The check runs at 740 to leave room for a theme with a
 * taller title bar. A screen too small for the 800 px window (a 1080p laptop
 * at 150 %) scrolls, and is not what this guards.
 *
 * Covered: Text; Form for every provider (the line under the forms differs),
 * from an app with the Remember box, with a remembered form, after a failed
 * run; the working state; Result with one result, and with two versions and
 * the history open. Not covered, on purpose: the missing-key banner, which
 * names a problem that stops Pyramidize running at all and can be closed.
 */
import { test, expect, Page } from '@playwright/test';
import { ANTHROPIC_MODELS, fakeState, installFakeBackend, type FakeBackendState } from './support/fake-backend';
import { fakePyramidize, installFakePyramidize, pressPyramidizeShortcut, type FakePyramidizeState } from './support/fake-pyramidize';

const DEFAULT_WINDOW_PAGE = { width: 1264, height: 740 };

const TEXT = 'Hi Tom, quick update on the deployment: it failed last night because the certificate had expired. '
  + 'I have requested a new one, it should arrive by tomorrow noon. I will deploy right after. '
  + 'Could you let the customer know until then? Thanks!';

const DOCUMENT = 'Deployment moved: certificate expired | new one by tomorrow noon | @Tom please tell the customer\n\nHi Tom,\n\n'
  + '**Deployment failed last night: the certificate had expired**\n- New certificate requested, due tomorrow noon\n- Deployment right after\n\n'
  + '**Please tell the customer until then**\n\nThanks!';

/** Keys for both API providers and a signed-in CLI: nothing on the page asks for setup. */
function configured(activeProvider: string): FakeBackendState {
  return fakeState({
    activeProvider,
    keys: { claude: 'keyring', openai: 'keyring' },
    cliSignedIn: true,
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  });
}

async function open(page: Page, activeProvider = 'claude', pyramidize: Partial<FakePyramidizeState> = {}): Promise<FakePyramidizeState> {
  await page.setViewportSize(DEFAULT_WINDOW_PAGE);
  await installFakeBackend(page, configured(activeProvider));
  const state = fakePyramidize({ fullDocument: DOCUMENT, clipboard: TEXT, ...pyramidize });
  await installFakePyramidize(page, state);
  await page.goto('/enhance');
  await expect(page.getByTestId('original-textarea')).toBeVisible();
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  return state;
}

async function toForm(page: Page): Promise<void> {
  await page.getByTestId('original-textarea').fill(TEXT);
  await page.getByTestId('choose-form-btn').getByRole('button').click();
  await expect(page.getByRole('radiogroup', { name: 'Form' })).toBeVisible();
}

async function run(page: Page): Promise<void> {
  await page.getByTestId('pyramidize-btn').getByRole('button').click();
  await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
}

/** How far the page and its scroll areas reach past what is visible, in px, either way. */
function overflow(page: Page): Promise<Record<string, number>> {
  return page.evaluate(() => {
    const out: Record<string, number> = {};
    for (const selector of ['.canvas-area', '.layout-main']) {
      const el = document.querySelector(selector);
      if (el) out[selector] = Math.max(el.scrollHeight - el.clientHeight, el.scrollWidth - el.clientWidth);
    }
    const doc = document.scrollingElement!;
    out['document'] = Math.max(doc.scrollHeight - doc.clientHeight, doc.scrollWidth - doc.clientWidth);
    return out;
  });
}

async function expectNoScrollbar(page: Page): Promise<void> {
  const measured = await overflow(page);
  for (const [where, px] of Object.entries(measured)) {
    expect(px, `${where} scrolls at the default window size`).toBeLessThanOrEqual(0);
  }
}

test.describe('Pyramidize — fits the default window', () => {
  test('the Text step', async ({ page }) => {
    await open(page);
    await expectNoScrollbar(page);
  });

  for (const provider of ['claude', 'claude-code', 'openai', 'ollama']) {
    test(`the Form step with ${provider} set up`, async ({ page }) => {
      await open(page, provider);
      await toForm(page);
      await expectNoScrollbar(page);
    });
  }

  test('the Form step for text from an app, with the Remember box', async ({ page }) => {
    await open(page, 'claude', { sourceApp: 'Inbox - Outlook', sourcePreset: { app: 'Outlook', documentType: 'email', remembered: false } });
    await pressPyramidizeShortcut(page);
    await expect(page.getByTestId('remember-checkbox')).toBeVisible();
    await expectNoScrollbar(page);
  });

  test('the Form step with a remembered form', async ({ page }) => {
    await open(page, 'claude', { sourceApp: 'general | Acme - Microsoft Teams', sourcePreset: { app: 'Teams', documentType: 'chat', remembered: true } });
    await pressPyramidizeShortcut(page);
    await expect(page.getByTestId('remembered-note')).toBeVisible();
    await expectNoScrollbar(page);
  });

  test('the Form step after a failed run', async ({ page }) => {
    await open(page, 'claude', { failPyramidize: 'Anthropic rejected the key. Check it in Settings, then try again.' });
    await toForm(page);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('error-row')).toBeVisible();
    await expectNoScrollbar(page);
  });

  test('while it works', async ({ page }) => {
    let release!: () => void;
    await open(page, 'claude', { hold: new Promise<void>(r => (release = r)) });
    await toForm(page);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('step-label')).toBeVisible();
    await expectNoScrollbar(page);
    release();
  });

  test('the Result step', async ({ page }) => {
    await open(page);
    await toForm(page);
    await run(page);
    await expectNoScrollbar(page);
  });

  test('the Result step with two versions and the history open', async ({ page }) => {
    await open(page, 'claude', { sourceApp: 'Inbox - Outlook', sourcePreset: { app: 'Outlook', documentType: 'email', remembered: false } });
    await pressPyramidizeShortcut(page);
    await run(page);
    await page.getByTestId('try-another-form-btn').getByRole('button').click();
    await expect(page.getByRole('radiogroup', { name: 'Form' })).toBeVisible();
    await page.keyboard.press('2');
    await expect(page.getByRole('radio', { name: /Chat/ })).toHaveAttribute('aria-checked', 'true');
    await page.keyboard.press('Enter');
    await expect(page.getByTestId('result-version')).toHaveCount(2);
    await page.getByTestId('trace-log-toggle').getByRole('button').click();
    await expect(page.getByTestId('trace-log-panel')).toBeVisible();
    await expect(page.getByTestId('send-back-btn')).toBeVisible();
    await expectNoScrollbar(page);
  });
});
