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

  test('a squeezed tab row neither loops nor gets squeezed: the end content wraps above it', async ({ page }) => {
    // Regression: the selected tab wider than a row squeezed by the History
    // button made reveal() scroll back and forth for ever (NG0103 ~60/s).
    const ng0103: string[] = [];
    page.on('console', (m) => { if (m.text().includes('NG0103')) ng0103.push(m.text()); });
    await page.setViewportSize({ width: 700, height: 800 });
    await configured(page);
    await page.goto('/enhance');
    await page.getByRole('tab', { name: 'Editor' }).click();
    // Attached, not visible: before the fix the row was 0px wide.
    const list = page.getByRole('tablist', { name: 'Text' });
    await expect(list).toBeAttached();

    const work = await list.evaluate(async (el) => {
      let scrolls = 0;
      let classFlips = 0;
      el.addEventListener('scroll', () => scrolls++);
      new MutationObserver((r) => (classFlips += r.length)).observe(el, { attributeFilter: ['class'] });
      await new Promise((r) => setTimeout(r, 800));
      const bar = el.parentElement!.getBoundingClientRect();
      const row = el.getBoundingClientRect();
      const end = el.closest('kl-tabs')!.querySelector('[klTabsEnd]')!.getBoundingClientRect();
      return {
        scrolls, classFlips,
        fits: el.scrollWidth <= el.clientWidth + 1,
        onDivider: Math.abs(row.bottom - bar.bottom) < 1,
        endAbove: end.bottom <= row.top + 1,
      };
    });
    expect(work.scrolls, 'the row is at rest').toBeLessThanOrEqual(1);
    expect(work.classFlips, 'the fades are at rest').toBeLessThanOrEqual(1);
    expect(ng0103).toEqual([]);
    expect(work.fits, 'both tabs fully visible').toBe(true);
    expect(work.onDivider, 'the tabs stay on the divider').toBe(true);
    expect(work.endAbove, 'the History row moved above the tabs').toBe(true);
  });

  test('on a wide window the end content sits beside the tabs', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await configured(page);
    await page.goto('/enhance');
    const list = page.getByRole('tablist', { name: 'Text' });
    await expect(list).toBeVisible();
    const sameLine = await list.evaluate((el) => {
      const row = el.getBoundingClientRect();
      const end = el.closest('kl-tabs')!.querySelector('[klTabsEnd]')!.getBoundingClientRect();
      return end.top < row.bottom && end.bottom > row.top && end.left >= row.right;
    });
    expect(sameLine).toBe(true);
  });

  /**
   * How far the open panel is from sitting against its trigger (6px below,
   * or 6px above when it flipped for room): ~0 while attached, null once it
   * has closed.
   */
  async function detachment(page: Page, triggerSelector: string): Promise<number | null> {
    return page.evaluate((sel) => {
      const panel = document.querySelector('[role="listbox"]');
      if (!panel) return null;
      const p = panel.getBoundingClientRect();
      const t = document.querySelector(sel)!.getBoundingClientRect();
      return Math.round(Math.min(Math.abs(p.top - t.bottom - 6), Math.abs(t.top - p.bottom - 6)));
    }, triggerSelector);
  }

  test('an open select follows its trigger when the page scrolls, and closes once it is out of view', async ({ page }) => {
    await page.setViewportSize({ width: 1000, height: 420 });
    await configured(page);
    await page.goto('/settings?tab=general');
    const trigger = page.locator('#log-level');
    await trigger.evaluate((el) => el.scrollIntoView({ block: 'center' }));
    await trigger.click();
    await expect(page.getByRole('listbox', { name: 'Log level' })).toBeVisible();
    expect(await detachment(page, '#log-level')).toBe(0);

    // A small scroll of the page's own scroll container: the panel moves with the trigger.
    const moved = await page.locator('.layout-main').evaluate((el) => {
      const top = el.scrollTop;
      el.scrollBy(0, -40);
      return top - el.scrollTop;
    });
    expect(moved, 'the page did scroll').toBeGreaterThan(20);
    await expect.poll(() => detachment(page, '#log-level')).toBe(0);

    // Scroll the trigger out of view: the panel goes rather than float detached.
    const below = await page.locator('.layout-main').evaluate((el) => {
      el.scrollTop = 0;
      return document.querySelector('#log-level')!.getBoundingClientRect().top >= el.clientHeight;
    });
    expect(below, 'the trigger is out of view now').toBe(true);
    await expect(page.getByRole('listbox')).toHaveCount(0);
    await expect(trigger).toHaveAttribute('aria-expanded', 'false');
  });

  test('an open select in the Pyramidize options column follows that column’s scroll', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 520 });
    await configured(page);
    await page.goto('/enhance');
    await page.locator('#pz-provider').click();
    await expect(page.getByRole('listbox', { name: 'Provider' })).toBeVisible();
    expect(await detachment(page, '#pz-provider')).toBe(0);
    const moved = await page.locator('app-pyramidize-options').evaluate((el) => {
      el.scrollBy(0, 30);
      return el.scrollTop;
    });
    expect(moved, 'the column did scroll').toBeGreaterThan(20);
    // Give a stale panel the frames it would need to move, then measure.
    await page.waitForTimeout(150);
    await expect.poll(() => detachment(page, '#pz-provider')).toBe(0);
  });
});
