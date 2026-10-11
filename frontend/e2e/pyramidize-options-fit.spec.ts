/**
 * The Pyramidize options column fits the default window without scrolling.
 *
 * The column scrolls on its own (overflow-y: auto) so a small window still
 * reaches the button, but at the size KeyLint opens with a scrollbar there is
 * a layout bug: v4.7.0-beta shipped one, after the Effort picker and its
 * help text went in under Model.
 *
 * The window is 1280×800 (main.go), and that is the outer size: on Windows 11
 * at 100 % the title bar and frame take about 40 px of it, so the page gets
 * roughly 1264×760. The check runs at 740 to leave room for a theme with a
 * taller title bar. A screen too small for the 800 px window (a 1080p laptop
 * at 150 %) scrolls, and is not what this guards.
 *
 * Covered, one at a time: every provider set up, a model list that could not
 * be fetched, the state after a run ("Pyramidize again"), and a provider
 * picked for this session only. Two of those hints at once may scroll. Not covered, on
 * purpose: the missing-key banner. It names a problem
 * that stops Pyramidize running at all, it can be closed, and fitting it
 * would mean squeezing the column for a state nobody works in.
 */

import { test, expect, Page } from '@playwright/test';
import { ANTHROPIC_MODELS, fakeState, installFakeBackend, type FakeBackendState } from './support/fake-backend';
import { fakePyramidize, installFakePyramidize } from './support/fake-pyramidize';

const DEFAULT_WINDOW_PAGE = { width: 1264, height: 740 };

const OPENAI_MODELS = [
  { id: 'gpt-6-astra', label: 'GPT-6 Astra', resolved: '' },
  { id: 'gpt-6-luna', label: 'GPT-6 Luna', resolved: '' },
];

/** Keys for both API providers and a signed-in CLI: nothing on the page asks for setup. */
function configured(activeProvider: string, reachable = true): FakeBackendState {
  return fakeState({
    activeProvider,
    keys: { claude: 'keyring', openai: 'keyring' },
    cliSignedIn: true,
    modelSources: reachable ? { claude: 'live', openai: 'live' } : {},
    modelLists: reachable ? { claude: ANTHROPIC_MODELS, openai: OPENAI_MODELS } : {},
  });
}

async function open(page: Page, state: FakeBackendState): Promise<void> {
  await page.setViewportSize(DEFAULT_WINDOW_PAGE);
  await installFakeBackend(page, state);
  await installFakePyramidize(page, fakePyramidize({ fullDocument: 'Two things stand out | Numbers are in\n\nHi,\n\n**Two things stand out**\n- The numbers are in' }));
  await page.goto('/enhance');
  // Settled: settings, key status and the model list have all answered.
  await expect(page.getByTestId('model-select')).toContainText('KeyLint default');
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
}

/** How far the column's content reaches past its visible height, in px. */
function overflow(page: Page): Promise<number> {
  return page.locator('.pz-options').evaluate((el) => el.scrollHeight - el.clientHeight);
}

async function expectNoScrollbar(page: Page): Promise<void> {
  expect(await overflow(page), 'options column scrolls at the default window size').toBeLessThanOrEqual(0);
}

test.describe('Pyramidize — options fit the default window', () => {
  for (const provider of ['claude', 'claude-code', 'openai', 'ollama']) {
    test(`with ${provider} set up`, async ({ page }) => {
      await open(page, configured(provider));
      await expectNoScrollbar(page);
    });
  }

  test('when the model list could not be fetched', async ({ page }) => {
    await open(page, configured('openai', false));
    await expect(page.getByTestId('model-list-note')).toBeVisible();
    await expectNoScrollbar(page);
  });

  test('after a run', async ({ page }) => {
    await open(page, configured('openai'));
    await page.getByTestId('original-textarea').fill('Hi, the numbers are in. Two things stand out.');
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('pyramidize-btn')).toContainText('Pyramidize again');
    await expectNoScrollbar(page);
  });

  test('with a provider picked for this session only', async ({ page }) => {
    await open(page, configured('claude'));
    await page.getByTestId('provider-select').click();
    await page.getByRole('option', { name: 'OpenAI' }).click();
    await expect(page.getByTestId('provider-session-override')).toBeVisible();
    await expectNoScrollbar(page);
  });
});
