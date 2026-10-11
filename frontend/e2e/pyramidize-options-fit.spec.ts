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
 * taller title bar, and against every provider whose options differ in what
 * they show under the pickers.
 */

import { test, expect, Page } from '@playwright/test';
import { ANTHROPIC_MODELS, fakeState, installFakeBackend } from './support/fake-backend';

const DEFAULT_WINDOW_PAGE = { width: 1264, height: 740 };

async function open(page: Page, activeProvider: string): Promise<void> {
  await page.setViewportSize(DEFAULT_WINDOW_PAGE);
  await installFakeBackend(page, fakeState({
    activeProvider,
    keys: { claude: 'keyring', openai: 'keyring' },
    cliSignedIn: true,
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
  await page.goto('/enhance');
  await expect(page.getByTestId('pyramidize-btn')).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
}

/** How far the column's content reaches past its visible height, in px. */
function overflow(page: Page): Promise<number> {
  return page.locator('.pz-options').evaluate((el) => el.scrollHeight - el.clientHeight);
}

test.describe('Pyramidize — options fit the default window', () => {
  for (const provider of ['claude', 'claude-code', 'openai', 'ollama']) {
    test(`no scrollbar with ${provider}`, async ({ page }) => {
      await open(page, provider);
      expect(await overflow(page), 'options column scrolls at the default window size').toBeLessThanOrEqual(0);
    });
  }
});
