/**
 * KeyLint's token layer, fonts and Tailwind setup (roadmap E11).
 *
 * The first test pins the palette to docs/ui/design.md: if a token moves, the
 * design moved, and that should be a decision rather than a side effect.
 *
 * The second checks that the bundled fonts load and that nothing — no font,
 * no icon set, no stylesheet — is fetched from the network at runtime.
 *
 * The third guards the layering. Tailwind's utilities live in @layer
 * utilities and every component style is unlayered, so a utility loses to any
 * component rule that sets the same property on the same element (see the
 * comment in theme.css). The utilities below sit on KeyLint components'
 * hosts; if one stops applying, a component rule has started setting that
 * property.
 *
 * The last makes sure PrimeNG stays gone: it was removed in E11 phase D, and
 * its token names, class names and icon font must not creep back in.
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

test.describe('UI tokens, fonts and Tailwind', () => {
  test('every KeyLint token has its design.md value', async ({ page }) => {
    await configured(page);
    await page.goto('/fix');
    await expect(page.getByTestId('fix-btn')).toBeVisible();

    for (const [token, value] of DESIGN) {
      expect(await defined(page, token), `${token} is not defined`).not.toBe('');
      expect(await resolve(page, token), token).toBe(await resolve(page, value));
    }
    // Native controls (radios, scrollbars) draw their dark variants.
    expect(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme)).toBe('dark');
    // The page itself sits on the ground token, the rail on its own.
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(31, 31, 35)');
    expect(await page.locator('.layout-sidebar').evaluate((el) => getComputedStyle(el).backgroundColor))
      .toBe('rgb(24, 24, 27)');
    // And the paint: a primary button is orange with dark text on it.
    await page.goto('/settings');
    const save = page.getByTestId('save-btn').getByRole('button');
    await expect(save).toBeVisible();
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
    // Only KeyLint's two families are bundled; icons are inline SVG, not a font.
    expect([...new Set(loaded)].sort()).toEqual(['Literata', 'Schibsted Grotesk']);
    expect(external, 'no request leaves the machine').toEqual([]);
  });

  test('Tailwind utilities apply on KeyLint components', async ({ page }) => {
    await configured(page);
    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('model-fix-claude')).toBeVisible();

    // mt-4 on a kl-message host.
    await page.getByTestId('save-btn').getByRole('button').click();
    const banner = page.getByTestId('saved-banner');
    await expect(banner).toBeVisible();
    expect(await banner.evaluate((el) => getComputedStyle(el).marginTop)).toBe('16px');

    // flex + gap-4 + mt-6 on a button row.
    await page.getByTestId('reset-btn').getByRole('button').click();
    const row = page.getByTestId('reset-confirm-btn').locator('xpath=..');
    expect(await row.evaluate((el) => {
      const cs = getComputedStyle(el);
      return `${cs.display} ${cs.columnGap}`;
    })).toBe('flex 16px');
    expect(await page.getByTestId('reset-confirm').evaluate((el) => getComputedStyle(el).marginTop)).toBe('24px');
  });

  test('no PrimeNG token, class or icon font is left on any screen', async ({ page }) => {
    await configured(page);
    for (const url of ['/fix', '/enhance', '/settings', '/settings?tab=providers', '/settings?tab=about', '/dev-tools/ui']) {
      await page.goto(url);
      await page.waitForLoadState('networkidle');
      const found = await page.evaluate(() => ({
        tokens: Array.from(getComputedStyle(document.documentElement)).filter((n) => n.startsWith('--p-')),
        classes: Array.from(document.querySelectorAll('[class]'))
          .flatMap((el) => Array.from(el.classList))
          .filter((c) => /^(p-|pi$|pi-)/.test(c)),
        tags: Array.from(document.querySelectorAll('*'))
          .map((el) => el.tagName.toLowerCase())
          .filter((t) => t.startsWith('p-')),
        fonts: Array.from(document.fonts).map((f) => f.family.replace(/"/g, '')).filter((f) => /prime/i.test(f)),
      }));
      expect(found, url).toEqual({ tokens: [], classes: [], tags: [], fonts: [] });
    }
  });
});
