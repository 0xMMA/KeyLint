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
  test('<html> has the app-dark class on enhance page', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const hasAppDark = await page.evaluate(() =>
      document.documentElement.classList.contains('app-dark'),
    );
    expect(hasAppDark, '<html> must have .app-dark class').toBe(true);
    await screenshot(page, '01-enhance-dark-mode');
  });

  test('body background is dark (not white)', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const bgColor = await page.evaluate(() =>
      getComputedStyle(document.body).backgroundColor,
    );
    console.log(`Body background: ${bgColor}`);
    // The ground token (#1f1f23). Must not be white (255,255,255).
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
    await expect(page.getByRole('tablist', { name: 'Settings sections' })).toBeVisible();
    await expect(page.locator('[data-testid="save-btn"] button')).toBeVisible();
    await expect(page.locator('[data-testid="reset-btn"] button')).toBeVisible();
  });

  test('settings sections sit on the dark sheet (not white)', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // The list the Shortcuts rows sit in: the sheet token (#27272a).
    const list = page.getByTestId('shortcut-mode-section').locator('xpath=..');
    await expect(list).toBeVisible();
    const bg = await list.evaluate((el) => getComputedStyle(el).backgroundColor);
    console.log(`settings list background: ${bg}`);
    expect(bg, `Settings list background: ${bg}`).toBe('rgb(39, 39, 42)');
    await screenshot(page, '05-settings-card-bg');
  });

  test('the shortcut recorder shows ctrl+g (settings data loaded)', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // What this test is really for: settings reached the UI at all. It used to
    // read a plain text input that no longer exists — #30 replaced it with the
    // mode toggle and the recorder component — so it sat on inputValue() for
    // the full 60s timeout.
    await expect(page.locator('[data-testid="shortcut-mode-section"]')).toBeVisible();

    // Browser mode defaults to double_tap (wails.service.ts BROWSER_MODE_DEFAULTS).
    // Asserted rather than assumed, because the next assertion is vacuous if the
    // mode is something else: the delay slider exists only in double-tap mode.
    await expect(page.locator('[data-testid="shortcut-delay-section"]')).toBeVisible();

    // ctrl+g, as the recorder formats it for display.
    const fixCombo = page.locator('[data-testid="shortcut-fix-section"] [data-testid="combo-display"]');
    await expect(fixCombo).toHaveText('Ctrl + G');

    // One shortcut in double-tap mode: single tap fixes, double tap pyramidizes.
    await expect(page.locator('[data-testid="shortcut-pyramidize-section"]')).toHaveCount(0);

    await screenshot(page, '06-settings-shortcut-value');
  });

  // #37 was PrimeNG's square label painting over the select's rounded
  // corners. kl-select is one rounded box with nothing inside it painting a
  // background, so what is left to guard is the dark field surface itself.
  test('a select sits on the dark field surface with rounded corners', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');

    // Without this the test is vacuous: in light mode a transparent field
    // over a white page would read as "not dark" for the wrong reason.
    const isDark = await page.evaluate(() => document.documentElement.classList.contains('app-dark'));
    expect(isDark, '<html> must have .app-dark on /settings').toBe(true);

    const trigger = page.getByTestId('log-level-section').getByRole('combobox');
    await expect(trigger).toBeVisible();
    const style = await trigger.evaluate((el) => {
      const cs = getComputedStyle(el);
      return { bg: cs.backgroundColor, radius: cs.borderTopLeftRadius };
    });
    console.log(`log level select: ${style.bg} / ${style.radius}`);
    // The field surface (--kl-field, #232327). Asserting the exact colour, not
    // merely "not transparent", is what makes a light-mode regression fail.
    expect(style.bg, `Select background: ${style.bg}`).toBe('rgb(35, 35, 39)');
    expect(style.radius).toBe('6px');
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
    await expect(page.getByTestId('log-level-section').getByRole('combobox')).toHaveText('Debug');

    const hasAppDark = await page.evaluate(() => document.documentElement.classList.contains('app-dark'));
    expect(hasAppDark, 'stored "light" must not remove .app-dark').toBe(true);
    const bgColor = await page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    expect(bgColor).toBe('rgb(31, 31, 35)');

    await expect(page.locator('label', { hasText: /^Theme$/ })).toHaveCount(0);
    await screenshot(page, '08-stored-light-stays-dark');
  });

  // With .app-dark on <body>, PrimeNG's :root component tokens stayed light, and
  // a disabled input came out slate-200 on the near-black page (#28). The
  // Pyramidize instruction input is disabled until the canvas has text.
  test('a disabled input keeps a dark surface', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const input = page.locator('[data-testid="global-instruction-input"]');
    await expect(input).toBeDisabled();
    const bg = await input.evaluate((el) => getComputedStyle(el).backgroundColor);
    console.log(`disabled instruction input bg: ${bg}`);
    // The rail surface (--kl-rail, #18181b): darker than an enabled field.
    expect(bg, `Disabled input background: ${bg}`).toBe('rgb(24, 24, 27)');

    // What regressed was PrimeNG's token, which only resolved dark with the
    // class on <html>. PrimeNG is gone (E11), but the rule outlives it: tokens
    // that alias other tokens resolve where they are declared, so the theme
    // class has to sit on <html>, where KeyLint's tokens are (index.html).
    const onRoot = await page.evaluate(() => document.documentElement.classList.contains('app-dark'));
    expect(onRoot, '.app-dark must sit on <html>').toBe(true);
    await screenshot(page, '09-disabled-input-dark');
  });

  // Same root cause as above: PrimeNG's dropdown panel background was a :root
  // alias of the overlay token and used to resolve to white. Pyramidize's
  // selects are kl-select now (E11), whose panel takes the raised token.
  test('an open select panel is dark', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    await page.locator('[data-testid="provider-select"]').click();
    const panel = page.getByRole('listbox');
    await expect(panel).toBeVisible();
    const bg = await panel.evaluate((el) => getComputedStyle(el).backgroundColor);
    console.log(`select panel bg: ${bg}`);
    // The raised surface (--kl-raised, #2a2a2f) that every overlay uses.
    expect(bg, `Select panel background: ${bg}`).toBe('rgb(42, 42, 47)');
    await screenshot(page, '10-select-panel-dark');
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
        shortcut_mode: 'double_tap',
        shortcut_fix: 'ctrl+g',
        shortcut_pyramidize: 'ctrl+shift+g',
        shortcut_double_tap_delay: 200,
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
