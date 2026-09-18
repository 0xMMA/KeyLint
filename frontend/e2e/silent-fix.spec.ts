import { test, expect, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const SCREENSHOT_DIR = path.join(__dirname, 'screenshots');

async function screenshot(page: Page, name: string): Promise<void> {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, `${name}.png`), fullPage: true });
  console.log(`📸 Screenshot: e2e/screenshots/${name}.png`);
}

// SKIPPED, AND REMOVING `.skip` WILL NOT MAKE IT WORK.
//
// These tests were written against a browser-mode fallback in the Angular app:
// the app would call the Anthropic API itself, the spec would proxy that call
// through `page.route()` to get around CORS, and the key reached the app via
// `localStorage.setItem('_e2e_apikey_claude', ...)`. That fallback is gone —
// `TextEnhancementService` is now a pass-through to `WailsService`, nothing in
// `frontend/src` reads `_e2e_apikey_claude`, and no code under `src/` calls a
// provider at all. So the second test would fill the input, click, and wait
// 30s for an output element that nothing populates.
//
// Left in place rather than deleted because the proxy pattern below is the
// hard-won part and would have to be rewritten from scratch. Reviving this
// needs a decision first: either give the app a test-only provider path again,
// or drive the real Go backend (which means a Wails binary, not `ng serve`).
// The first and third tests are independent of all that and would pass today.
//
// Last verified working 2026-03-05, before the fallback was removed.
test.describe.skip('Silent Fix — live API verification (skipped)', () => {
  test('Fix page renders correctly', async ({ page }) => {
    await page.goto('/fix');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);
    await screenshot(page, '10-fix-page');

    await expect(page.locator('[data-testid="fix-input"]')).toBeVisible();
    await expect(page.locator('[data-testid="fix-btn"] button')).toBeVisible();
    await expect(page.locator('[data-testid="fix-clipboard-btn"] button')).toBeVisible();
  });

  test('Fix button enhances text via Claude API and shows result', async ({ page }) => {
    const apiKey = process.env['ANTHROPIC_API_KEY'] ?? '';

    // Proxy Anthropic API calls via Playwright (Node.js has no CORS restrictions;
    // Chromium's browser sandbox would otherwise block cross-origin API requests).
    await page.route('https://api.anthropic.com/**', async (route) => {
      const req = route.request();
      // Strip browser-only headers that break server-side fetch
      const skip = new Set(['host', 'connection', 'origin', 'referer', 'sec-fetch-dest', 'sec-fetch-mode', 'sec-fetch-site']);
      const headers: Record<string, string> = {};
      for (const [k, v] of Object.entries(req.headers())) {
        if (!skip.has(k.toLowerCase())) headers[k] = v;
      }
      const upstream = await fetch(req.url(), {
        method: req.method(),
        headers,
        body: req.postData() ?? undefined,
      });
      const body = Buffer.from(await upstream.arrayBuffer());
      const respHeaders: Record<string, string> = {};
      upstream.headers.forEach((v, k) => { respHeaders[k] = v; });
      await route.fulfill({ status: upstream.status, headers: respHeaders, body });
    });

    await page.goto('/fix');
    await page.waitForLoadState('networkidle');

    // Inject the API key for browser-mode operation (no Wails keyring available in Playwright).
    await page.evaluate((key) => localStorage.setItem('_e2e_apikey_claude', key), apiKey);

    const badText = 'i has meny grammer misstaek in dis sentance and it need fixin';
    await page.locator('[data-testid="fix-input"]').fill(badText);
    await screenshot(page, '11-fix-before');

    await page.locator('[data-testid="fix-btn"] button').click();

    // Wait for the API response — allow up to 30s
    await page.waitForSelector('[data-testid="fix-output"]', { timeout: 30_000 });
    await screenshot(page, '12-fix-after');

    const output = await page.locator('[data-testid="fix-output"]').inputValue();
    console.log(`Input:  ${badText}`);
    console.log(`Output: ${output}`);

    expect(output).toBeTruthy();
    expect(output.length).toBeGreaterThan(5);
    expect(output.toLowerCase()).not.toBe(badText.toLowerCase());
  });

  test('Fix navigates as default route (/)', async ({ page }) => {
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500);

    // Should redirect to /fix
    expect(page.url()).toContain('/fix');
    await expect(page.locator('[data-testid="fix-input"]')).toBeVisible();
    await screenshot(page, '13-fix-default-route');
  });
});
