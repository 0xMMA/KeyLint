/**
 * KeyLint's own primitives where jsdom cannot judge them: label activation and
 * layout belong to the browser. Unit specs cover the rest
 * (src/app/shared/ui/*.spec.ts).
 */
import { test, expect, Page } from '@playwright/test';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS } from './support/fake-backend';

async function configured(page: Page): Promise<void> {
  await installFakeBackend(page, fakeState({
    activeProvider: 'claude',
    keys: { claude: 'keyring' },
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
}

test.describe('UI primitives in the browser', () => {
  test('a select’s <label for> focuses the select without opening it', async ({ page }) => {
    await configured(page);
    await page.goto('/enhance');
    await expect(page.getByTestId('model-select')).toContainText('KeyLint default');

    await page.locator('label[for="pz-provider"]').click();
    await expect(page.locator('#pz-provider')).toBeFocused();
    await expect(page.locator('#pz-provider')).toHaveAttribute('aria-expanded', 'false');
    await expect(page.getByRole('listbox')).toHaveCount(0);

    // The trigger itself still opens, and the list is named after the label.
    await page.locator('#pz-provider').click();
    await expect(page.getByRole('listbox', { name: 'Provider' })).toBeVisible();
  });

  test('the tab row scrolls sideways on a narrow window instead of wrapping', async ({ page }) => {
    await page.setViewportSize({ width: 520, height: 800 });
    await configured(page);
    await page.goto('/settings');
    const list = page.getByRole('tablist', { name: 'Settings sections' });
    await expect(list).toBeVisible();

    const box = await list.evaluate((el) => ({
      height: el.getBoundingClientRect().height,
      scrolls: el.scrollWidth > el.clientWidth,
      tops: Array.from(el.children).map((t) => Math.round(t.getBoundingClientRect().top)),
    }));
    expect(new Set(box.tops).size, 'all tabs on one line').toBe(1);
    expect(box.height).toBeLessThan(60);
    expect(box.scrolls, 'the row is wider than the window here').toBe(true);
    await expect(list, 'the hidden end fades out').toHaveClass(/more-end/);

    // Keyboard selection brings the last tab into view.
    await page.getByRole('tab', { name: 'General' }).focus();
    await page.keyboard.press('End');
    const about = page.getByRole('tab', { name: 'About' });
    await expect(about).toBeFocused();
    await expect(about).toHaveAttribute('aria-selected', 'true');
    const inView = await about.evaluate((tab) => {
      const t = tab.getBoundingClientRect();
      const l = tab.parentElement!.getBoundingClientRect();
      return t.left >= l.left - 1 && t.right <= l.right + 1;
    });
    expect(inView).toBe(true);
    await expect(list).toHaveClass(/more-start/);
    await expect(list).not.toHaveClass(/more-end/);
  });

  test('a page opened on its last tab starts with that tab in view', async ({ page }) => {
    await page.setViewportSize({ width: 520, height: 800 });
    await configured(page);
    await page.goto('/settings?tab=about');
    const about = page.getByRole('tab', { name: 'About' });
    await expect(about).toHaveAttribute('aria-selected', 'true');
    const inView = await about.evaluate((tab) => {
      const t = tab.getBoundingClientRect();
      const l = tab.parentElement!.getBoundingClientRect();
      return t.left >= l.left - 1 && t.right <= l.right + 1;
    });
    expect(inView).toBe(true);
    // The page itself did not scroll sideways to get there.
    expect(await page.evaluate(() => window.scrollX)).toBe(0);
  });
});
