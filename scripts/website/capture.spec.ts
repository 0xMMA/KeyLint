/**
 * Stills and demo clips for keylint.io (website/img/).
 *
 * Rerun after a UI change, from the repo root, with one command:
 *
 *   scripts/website/capture.sh                  # stills, fix-clip, pyramidize-clip
 *   scripts/website/capture.sh stills fix-clip  # only these
 *
 * When a sample text or a prompt changed, first record the real outputs:
 *   scripts/website/refresh-samples.sh [fix chat pyramidize]  (real model calls)
 *
 * capture.spec.ts takes the app screenshots, demos.spec.ts records the two
 * staged clips (scenes/). Both serve the production build on 127.0.0.1:47813
 * (support.ts), so they never collide with an `ng serve` on :4200, and answer
 * the Wails binding calls with the e2e fake backend. Raw frames land in
 * scripts/website/out/ (gitignored); encode.py (Pillow, plus the ffmpeg that
 * ships with Playwright) turns them into the WebP and WebM files the page uses.
 *
 * Not part of the e2e suite on purpose: CI runs every spec in frontend/e2e/.
 */
import { test, expect } from '@playwright/test';
import * as http from 'http';
import * as path from 'path';
import { installFakeBackend } from '../../frontend/e2e/support/fake-backend';
import { fakePyramidize, installFakePyramidize } from '../../frontend/e2e/support/fake-pyramidize';
import {
  BASE_URL, FIX_INPUT, OUT, PYRAMIDIZE_INPUT, PYRAMIDIZE_OUTPUT, VIEWPORT, configured, installFakeFix, installFakeUpdater,
  settle, startServer, stopServer,
} from './support';

let server: http.Server;

test.use({ viewport: VIEWPORT, deviceScaleFactor: 2, baseURL: BASE_URL });
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await stopServer(server); });

test('still: Fix, with the corrections marked', async ({ page }) => {
  await installFakeBackend(page, configured('claude-code'));
  await installFakeFix(page);
  await installFakeUpdater(page);
  await page.goto('/fix');
  await page.getByTestId('fix-input').fill(FIX_INPUT);
  await page.getByTestId('fix-btn').getByRole('button').click();
  await expect(page.locator('ins.fx').first()).toBeVisible();
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'fix.png') });
});

test('still: Settings › AI providers, with the installed CLI signed in', async ({ page }) => {
  const state = configured('claude-code');
  // A local Ollama that answers, so its row reads as set up rather than broken.
  state.modelSources['ollama'] = 'live';
  state.modelLists['ollama'] = [{ id: 'llama3.2', label: 'llama3.2', resolved: '' }];
  await installFakeBackend(page, state);
  await installFakeUpdater(page);
  await page.goto('/settings?tab=providers');
  await expect(page.getByTestId('active-provider-name')).toBeVisible();
  await settle(page);
  // Taller than the default window, so every connection is in the picture.
  const content = await page.locator('.layout-main').evaluate((el) => el.scrollHeight);
  await page.setViewportSize({ width: VIEWPORT.width, height: Math.max(VIEWPORT.height, content) });
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'providers.png') });
});

// The hero's faded backdrop shows this next to the Fix page: an email written
// from the sample notes, in Preview.
test('still: Pyramidize, an email in Preview', async ({ page }) => {
  await installFakeBackend(page, configured('claude-code'));
  await installFakePyramidize(page, fakePyramidize({ fullDocument: PYRAMIDIZE_OUTPUT }));
  await installFakeUpdater(page);
  await page.goto('/enhance');
  await page.getByTestId('original-textarea').fill(PYRAMIDIZE_INPUT);
  await page.getByTestId('choose-form-btn').getByRole('button').click();
  await page.getByTestId('form-email').click();
  await page.getByTestId('pyramidize-btn').getByRole('button').click();
  await expect(page.getByTestId('canvas-textarea')).not.toHaveValue('');
  await page.getByTestId('canvas-mode').getByText('Preview', { exact: true }).click();
  await expect(page.getByTestId('canvas-preview')).toBeVisible();
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'pyramidize.png') });
});
