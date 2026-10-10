/**
 * KeyLint's token layer and Tailwind, next to PrimeNG 21 (roadmap E11).
 *
 * App code colours itself from --kl-* tokens (src/theme.css) while PrimeNG
 * still renders most controls from its own --p-* tokens. Until the new look
 * lands, the two must agree, or KeyLint's own blocks drift away from the
 * PrimeNG controls next to them. When the design.md palette is switched on,
 * the first test is meant to fail: replace it then, don't loosen it.
 *
 * The second test guards the layering. PrimeNG injects its theme unlayered
 * and Tailwind's utilities live in @layer utilities, so a utility loses to
 * any PrimeNG rule that sets the same property on the same element. The
 * utilities below sit on PrimeNG hosts (styleClass); if one stops applying,
 * a PrimeNG update has started setting that property, and the class needs
 * the important modifier (see the comment in theme.css).
 */
import { test, expect, Page } from '@playwright/test';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS } from './support/fake-backend';

/** Each KeyLint token next to the PrimeNG token it mirrors today. */
const MIRRORS: Array<[string, string]> = [
  ['--kl-text', '--p-text-color'],
  ['--kl-muted', '--p-text-muted-color'],
  ['--kl-primary', '--p-primary-color'],
  ['--kl-primary-text', '--p-primary-color'],
  ['--kl-line', '--p-content-border-color'],
  ['--kl-hover', '--p-content-hover-background'],
  ['--kl-sheet', '--p-content-hover-background'],
  ['--kl-highlight', '--p-highlight-background'],
  ['--kl-error', '--p-red-400'],
  ['--kl-warn', '--p-amber-400'],
  ['--kl-radius-md', '--p-border-radius-md'],
  ...['0', '50', '100', '200', '300', '400', '500', '600', '700', '800', '900', '950']
    .map((n): [string, string] => [`--kl-surface-${n}`, `--p-surface-${n}`]),
];

/** What a token computes to as a colour and as a radius, so #fff and rgb(255,255,255) compare equal. */
async function resolve(page: Page, token: string): Promise<string> {
  return page.evaluate((t) => {
    const probe = document.createElement('div');
    probe.style.cssText = `color: var(${t}); border-top-left-radius: var(${t});`;
    document.body.appendChild(probe);
    const cs = getComputedStyle(probe);
    const value = `${cs.color} / ${cs.borderTopLeftRadius}`;
    probe.remove();
    return value;
  }, token);
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
  test('every KeyLint token resolves to what PrimeNG renders today', async ({ page }) => {
    await configured(page);
    await page.goto('/settings');
    await expect(page.getByTestId('save-btn')).toBeVisible();

    for (const [kl, p] of MIRRORS) {
      const defined = await page.evaluate((t) => getComputedStyle(document.documentElement).getPropertyValue(t).trim(), kl);
      expect(defined, `${kl} is not defined`).not.toBe('');
      expect(await resolve(page, kl), `${kl} vs ${p}`).toBe(await resolve(page, p));
    }
    // The page itself sits on the ground token.
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(9, 9, 11)');
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
