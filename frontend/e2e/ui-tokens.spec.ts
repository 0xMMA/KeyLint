/**
 * KeyLint's token layer and Tailwind, next to PrimeNG 21 (roadmap E11).
 *
 * The first test pins the palette to docs/ui/design.md: if a token moves, the
 * design moved, and that should be a decision rather than a side effect.
 *
 * The second checks the role wiring that keeps the screens still on PrimeNG
 * in step: styles.scss points PrimeNG's semantic tokens at the --kl-* roles,
 * so a PrimeNG button is KeyLint orange with dark text, a PrimeNG field sits
 * on the field surface, and so on. If PrimeNG renames a token, this fails
 * instead of a screen quietly drifting back to Aura.
 *
 * The third checks that the bundled fonts load (no network at runtime).
 *
 * The last guards the layering. PrimeNG injects its theme unlayered and
 * Tailwind's utilities live in @layer utilities, so a utility loses to any
 * PrimeNG rule that sets the same property on the same element. The
 * utilities below sit on PrimeNG hosts (styleClass); if one stops applying,
 * a PrimeNG update has started setting that property, and the class needs
 * the important modifier (see the comment in theme.css).
 */
import { test, expect, Page } from '@playwright/test';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS } from './support/fake-backend';

/** design.md's table, as theme.css names it. */
const DESIGN: Array<[string, string]> = [
  ['--kl-rail', '#18181b'],
  ['--kl-ground', '#1f1f23'],
  ['--kl-sheet', '#27272a'],
  ['--kl-sheet-alt', '#2b2b30'],
  ['--kl-line', '#36363c'],
  ['--kl-line-control', '#3f3f46'],
  ['--kl-text', '#f4f4f5'],
  ['--kl-muted', '#a1a1aa'],
  ['--kl-primary', '#f97316'],
  ['--kl-primary-text', '#fdba74'],
  ['--kl-link', '#fb923c'],
  ['--kl-focus', '#fb923c'],
  ['--kl-on-primary', '#1c1209'],
  ['--kl-info', '#60a5fa'],
  ['--kl-success', '#2dd4bf'],
  ['--kl-warn', '#fbbf24'],
  ['--kl-error', '#f87171'],
  ['--kl-radius-md', '6px'],
  ['--kl-radius-lg', '8px'],
  ['--kl-radius-sheet', '10px'],
];

/** PrimeNG token → the KeyLint role styles.scss points it at. */
const WIRED: Array<[string, string]> = [
  ['--p-primary-color', '--kl-primary'],
  ['--p-primary-contrast-color', '--kl-on-primary'],
  ['--p-text-color', '--kl-text'],
  ['--p-text-muted-color', '--kl-muted'],
  ['--p-content-border-color', '--kl-line'],
  ['--p-form-field-background', '--kl-field'],
  ['--p-form-field-border-color', '--kl-line-control'],
  ['--p-overlay-select-background', '--kl-raised'],
  ['--p-focus-ring-color', '--kl-focus'],
  // Component tokens are :root aliases of the semantic ones: they follow too.
  ['--p-button-primary-background', '--kl-primary'],
  ['--p-button-primary-color', '--kl-on-primary'],
];

/** What a token or literal computes to as a colour and as a radius, so #fff and rgb(255,255,255) compare equal. */
async function resolve(page: Page, value: string): Promise<string> {
  return page.evaluate((v) => {
    const css = v.startsWith('--') ? `var(${v})` : v;
    const probe = document.createElement('div');
    probe.style.cssText = `color: ${css}; border-top-left-radius: ${css};`;
    document.body.appendChild(probe);
    const cs = getComputedStyle(probe);
    const out = `${cs.color} / ${cs.borderTopLeftRadius}`;
    probe.remove();
    return out;
  }, value);
}

async function defined(page: Page, token: string): Promise<string> {
  return page.evaluate((t) => getComputedStyle(document.documentElement).getPropertyValue(t).trim(), token);
}

async function configured(page: Page): Promise<void> {
  await installFakeBackend(page, fakeState({
    activeProvider: 'claude',
    keys: { claude: 'keyring' },
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
}

test.describe('UI tokens and Tailwind next to PrimeNG', () => {
  test('every KeyLint token has its design.md value', async ({ page }) => {
    await configured(page);
    await page.goto('/fix');
    await expect(page.getByTestId('fix-btn')).toBeVisible();

    for (const [token, value] of DESIGN) {
      expect(await defined(page, token), `${token} is not defined`).not.toBe('');
      expect(await resolve(page, token), token).toBe(await resolve(page, value));
    }
    // The page itself sits on the ground token, the rail on its own.
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(31, 31, 35)');
    expect(await page.locator('.layout-sidebar').evaluate((el) => getComputedStyle(el).backgroundColor))
      .toBe('rgb(24, 24, 27)');
  });

  test('PrimeNG’s tokens follow the KeyLint roles', async ({ page }) => {
    await configured(page);
    await page.goto('/settings');
    await expect(page.getByTestId('save-btn')).toBeVisible();

    for (const [p, kl] of WIRED) {
      // Both sides: an undefined token would make the probe inherit, and the
      // comparison could then pass for the wrong reason.
      expect(await defined(page, p), `${p} is not defined`).not.toBe('');
      expect(await defined(page, kl), `${kl} is not defined`).not.toBe('');
      expect(await resolve(page, p), `${p} vs ${kl}`).toBe(await resolve(page, kl));
    }
    // And the paint: a primary PrimeNG button is orange with dark text.
    const save = page.getByTestId('save-btn').getByRole('button');
    expect(await save.evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(249, 115, 22)');
    expect(await save.evaluate((el) => getComputedStyle(el).color)).toBe('rgb(28, 18, 9)');
  });

  test('the bundled fonts load and are used', async ({ page }) => {
    const external: string[] = [];
    page.on('request', (r) => {
      const url = new URL(r.url());
      if (url.hostname !== 'localhost' && url.hostname !== '127.0.0.1') external.push(r.url());
    });
    await configured(page);
    await page.goto('/fix');
    await expect(page.getByTestId('fix-btn')).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    // check() is also true for a family with no @font-face at all, so ask the
    // face itself whether it loaded.
    const loaded = await page.evaluate(() =>
      Array.from(document.fonts).filter((f) => f.status === 'loaded').map((f) => f.family.replace(/"/g, '')));
    expect(loaded).toContain('Schibsted Grotesk');
    expect(loaded).toContain('Literata');
    expect(await page.evaluate(() => getComputedStyle(document.body).fontFamily)).toContain('Schibsted Grotesk');
    expect(await page.getByTestId('fix-input').evaluate((el) => getComputedStyle(el).fontFamily)).toContain('Literata');
    expect(external, 'no request leaves the machine').toEqual([]);
  });

  test('Tailwind utilities apply on elements PrimeNG styles', async ({ page }) => {
    await configured(page);
    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('model-fix-claude')).toBeVisible();
    // (The w-full on the model and effort p-selects is not asserted: their
    // grid cell stretches them to the same width with or without it.)

    // mt-4 on a p-message host.
    await page.getByTestId('save-btn').getByRole('button').click();
    const banner = page.getByTestId('saved-banner');
    await expect(banner).toBeVisible();
    expect(await banner.evaluate((el) => getComputedStyle(el).marginTop)).toBe('16px');

    // flex + gap-4 + mt-6 on KeyLint's own button row, for completeness.
    await page.getByTestId('reset-btn').getByRole('button').click();
    const row = page.getByTestId('reset-confirm-btn').locator('xpath=..');
    expect(await row.evaluate((el) => {
      const cs = getComputedStyle(el);
      return `${cs.display} ${cs.columnGap}`;
    })).toBe('flex 16px');
    expect(await page.getByTestId('reset-confirm').evaluate((el) => getComputedStyle(el).marginTop)).toBe('24px');
  });
});
