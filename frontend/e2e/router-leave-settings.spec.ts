/**
 * #38 — navigating away from Settings left it on screen, with the next page
 * appended underneath.
 *
 * jsdom cannot see this: `provideAnimationsAsync()` loads the real animations
 * engine in a browser and a stub under Vitest, and the reported cause is an
 * animation that never finishes. So the repro lives here, where the engine is
 * the real one.
 *
 * The assertion is deliberately about the DOM rather than the URL: the router
 * changed the URL in the bug report too. What failed was the outlet swapping
 * its view.
 */

import { test, expect, Page } from '@playwright/test';

/** Every routed page currently in the document, in DOM order. */
async function routedPages(page: Page): Promise<string[]> {
  return page.evaluate(() =>
    Array.from(document.querySelectorAll('.settings-page, .fix-page, .pyramidize-page'))
      .map(el => el.className.split(' ').find(c => c.endsWith('-page')) ?? '?'));
}

test.describe('leaving Settings', () => {
  test('the version footer, then the tabs, then another page', async ({ page }) => {
    // Application errors only. `ng serve` has no Wails runtime, so the bridge
    // scripts 404 on every page load — noise that has nothing to do with #38.
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(String(e)));
    page.on('console', m => {
      if (m.type() !== 'error') return;
      if (m.text().includes('Failed to load resource')) return;
      errors.push(m.text());
    });

    await page.goto('/fix');
    await expect(page.locator('.layout-sidebar')).toBeVisible();

    // 1. Click the version text in the sidebar footer — goToAbout().
    await page.locator('[data-testid="version-footer"]').click();
    await expect(page.locator('.settings-page')).toBeVisible();
    expect(await routedPages(page)).toEqual(['settings-page']);

    // 2. Click through every Settings tab.
    // toHaveCount retries; locator.count() samples once. The container is
    // visible before PrimeNG has rendered the tabs into it, so counting here
    // returned 0 in roughly one local run in three. The number is pinned rather
    // than lower-bounded because a tab appearing or disappearing should fail
    // this test loudly, not quietly change what it clicks through.
    const tabs = page.locator('.settings-page [role="tab"]');
    await expect(tabs, 'the Settings tabs should be present').toHaveCount(4);
    const count = await tabs.count();
    for (let i = 0; i < count; i++) {
      await tabs.nth(i).click();
      // Let any panel animation start and finish before the next click.
      await page.waitForTimeout(150);
    }

    // 3. Navigate away through the nav, the way a user does.
    await page.locator('a[routerLink="/enhance"], a[href="/enhance"]').first().click();
    await expect(page.locator('.pyramidize-page')).toBeVisible();

    // The bug: Settings stayed and Pyramidize was appended below it.
    expect(await routedPages(page), 'Settings must not survive the navigation')
      .toEqual(['pyramidize-page']);

    // 4. And again, to the third page — the report says this swapped only the
    //    page underneath Settings.
    await page.locator('a[routerLink="/fix"], a[href="/fix"]').first().click();
    await expect(page.locator('.fix-page')).toBeVisible();
    expect(await routedPages(page)).toEqual(['fix-page']);

    expect(errors, 'no application errors during the sequence').toEqual([]);
  });

  // The issue's third hypothesis: goToAbout() navigates to /settings with a
  // query param, which is a same-component navigation when Settings is already
  // open. That is the one path where the router reuses the component instead of
  // recreating it.
  test('the version footer while already on Settings', async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', e => errors.push(String(e)));

    await page.goto('/settings');
    await expect(page.locator('.settings-page')).toBeVisible();

    await page.locator('[data-testid="version-footer"]').click();
    await expect(page).toHaveURL(/tab=about/);
    expect(await routedPages(page), 'reusing the component must not duplicate it')
      .toEqual(['settings-page']);

    await page.locator('a[routerLink="/fix"], a[href="/fix"]').first().click();
    await expect(page.locator('.fix-page')).toBeVisible();
    expect(await routedPages(page)).toEqual(['fix-page']);

    expect(errors).toEqual([]);
  });
});
