import { test, expect, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const SCREENSHOT_DIR = path.join(__dirname, 'screenshots');

async function screenshot(page: Page, name: string): Promise<void> {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, `${name}.png`), fullPage: true });
  console.log(`📸 Screenshot: e2e/screenshots/${name}.png`);
}

test.describe('Dark mode — visual verification', () => {
  test('body has app-dark class on enhance page', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const hasAppDark = await page.evaluate(() =>
      document.body.classList.contains('app-dark'),
    );
    expect(hasAppDark, 'body must have .app-dark class').toBe(true);
    await screenshot(page, '01-enhance-dark-mode');
  });

  test('body background is dark (not white)', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const bgColor = await page.evaluate(() =>
      getComputedStyle(document.body).backgroundColor,
    );
    console.log(`Body background: ${bgColor}`);
    // rgb(9,9,11) = zinc-950 (#09090b). Must not be white (255,255,255).
    expect(bgColor, `Body is white: ${bgColor}`).not.toBe('rgb(255, 255, 255)');
    expect(bgColor, `Body is white: ${bgColor}`).not.toBe('rgba(0, 0, 0, 0)');
    await screenshot(page, '02-body-background');
  });

  test('enhance page renders in dark mode — full visual', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(500); // let CDR settle
    await screenshot(page, '03-enhance-full');

    // Sidebar must be visible
    await expect(page.locator('.layout-sidebar')).toBeVisible();
    // Two-tone "KeyLint" logo. The "ey"/"int" spans collapse to max-width: 0
    // when the sidebar is collapsed, so assert on the always-visible K and L.
    await expect(page.locator('.layout-logo')).toBeVisible();
    await expect(page.locator('.layout-logo .logo-key').first()).toBeVisible();
    await expect(page.locator('.layout-logo .logo-lint').first()).toBeVisible();
    await expect(page.locator('.layout-logo')).toHaveText('KeyLint');
  });

  test('settings page renders full content (tabs + form fields)', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000); // CDR + async load settle
    await screenshot(page, '04-settings-full');

    // Tab list and Save button must be present (content rendered)
    await expect(page.locator('p-tablist')).toBeVisible();
    await expect(page.locator('[data-testid="save-btn"] button')).toBeVisible();
    await expect(page.locator('[data-testid="reset-btn"] button')).toBeVisible();
  });

  test('settings page card background is dark (not white)', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    const cardBg = await page.evaluate(() => {
      const card = document.querySelector('.p-card') as HTMLElement | null;
      return card ? getComputedStyle(card).backgroundColor : null;
    });
    console.log(`p-card background: ${cardBg}`);
    expect(cardBg, 'Settings card is white').not.toBe('rgb(255, 255, 255)');
    await screenshot(page, '05-settings-card-bg');
  });

  test('shortcut input shows ctrl+g (settings data loaded)', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    const inputVal = await page.locator('[data-testid="shortcut-input"]').inputValue();
    console.log(`Shortcut input value: ${inputVal}`);
    expect(inputVal).toBe('ctrl+g');
    await screenshot(page, '06-settings-shortcut-value');
  });

  test('select label stays transparent so the rounded corners render', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(1000);

    // Without this the test is vacuous: in light mode the label would be
    // transparent over a white page and every assertion below would pass.
    const isDark = await page.evaluate(() => document.body.classList.contains('app-dark'));
    expect(isDark, 'body must have .app-dark on /settings').toBe(true);

    const colors = await page.evaluate(() => {
      const select = document.querySelector('.p-select') as HTMLElement | null;
      const label = select?.querySelector('.p-select-label') as HTMLElement | null;
      if (!select || !label) return null;
      return {
        select: getComputedStyle(select).backgroundColor,
        label: getComputedStyle(label).backgroundColor,
      };
    });
    console.log(`p-select bg: ${colors?.select} / p-select-label bg: ${colors?.label}`);
    expect(colors, 'no .p-select with .p-select-label found on /settings').not.toBeNull();

    // The label is a square span 1px inside the rounded .p-select box. An opaque
    // background on it paints over the border's corner arcs (#37), so it must stay
    // transparent while the select itself keeps the dark surface background.
    expect(colors!.label, `Select label is not transparent: ${colors!.label}`)
      .toBe('rgba(0, 0, 0, 0)');
    // zinc-950 (#09090b) is the dark surface. Asserting the exact colour, not
    // merely "not transparent", is what makes a light-mode regression fail.
    expect(colors!.select, `Select background: ${colors!.select}`)
      .toBe('rgb(9, 9, 11)');
    await screenshot(page, '07-select-label-transparent');
  });

  // Only the dark theme is styled (#24). A "light" saved by an older version
  // used to strip .app-dark and leave PrimeNG's unstyled light mode; now it is
  // ignored, and there is no theme control to set it with.
  test('a stored light theme still renders dark, with no theme control', async ({ page }) => {
    await serveSettings(page, { theme_preference: 'light' });
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-testid="save-btn"] button')).toBeVisible();

    // Proves the stored value actually reached the app, so the dark assertion
    // below is not vacuous: Log Level comes from the same stubbed response.
    await expect(page.locator('[data-testid="log-level-section"] .p-select-label')).toHaveText('Debug');

    const hasAppDark = await page.evaluate(() => document.body.classList.contains('app-dark'));
    expect(hasAppDark, 'stored "light" must not remove .app-dark').toBe(true);
    const bgColor = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    expect(bgColor).toBe('rgb(9, 9, 11)');

    await expect(page.locator('label', { hasText: /^Theme$/ })).toHaveCount(0);
    await screenshot(page, '08-stored-light-stays-dark');
  });
});

/**
 * ng serve has no Wails backend, so every binding call fails and the app falls
 * back to its browser-mode defaults. This answers Settings.Get only, with the
 * given fields over those defaults, so a spec can put a stored value in front
 * of the UI. The method ID comes from the generated binding
 * (bindings/keylint/internal/features/settings/service.js, Get).
 */
const SETTINGS_GET_METHOD_ID = 2040733582;

async function serveSettings(page: Page, overrides: Record<string, unknown>): Promise<void> {
  await page.route('**/wails/runtime', async (route) => {
    let methodID: unknown;
    try {
      methodID = JSON.parse(route.request().postData() ?? '{}')?.args?.methodID;
    } catch {
      methodID = undefined;
    }
    if (methodID !== SETTINGS_GET_METHOD_ID) {
      await route.fallback();
      return;
    }
    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        active_provider: 'claude',
        models: {},
        providers: { ollama_url: '', aws_region: '' },
        shortcut_key: 'ctrl+g',
        start_on_boot: false,
        theme_preference: 'dark',
        completed_setup: true,
        log_level: 'debug',
        sensitive_logging: false,
        update_channel: '',
        app_presets: [],
        pyramidize_quality_threshold: 0.65,
        ...overrides,
      }),
    });
  });
}
