/**
 * Shell sidebar — third layer of visual regression tests.
 *
 * Probes icon width parity, hover colour consistency, logo area height,
 * padding geometry, and sidebar scroll behaviour.
 */

import { test, expect, Page } from '@playwright/test';

async function gotoFix(page: Page): Promise<void> {
  await page.goto('/fix');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(200);
}

async function collapse(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await page.waitForTimeout(300);
}

type Rect = { x: number; y: number; width: number; height: number; top: number; right: number; bottom: number; left: number };

async function getRect(page: Page, selector: string): Promise<Rect> {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y, width: r.width, height: r.height, top: r.top, right: r.right, bottom: r.bottom, left: r.left };
  });
}

// ── Icon width parity ─────────────────────────────────────────────────────────

test.describe('Shell — icon width parity', () => {
  test('all nav icons have the same rendered width when expanded', async ({ page }) => {
    await gotoFix(page);

    const fixW      = (await getRect(page, '.nav-item a[href="/fix"] i')).width;
    const settingsW = (await getRect(page, '.nav-item a[href="/settings"] i')).width;
    const pyramidW  = (await getRect(page, '.nav-item a[href="/enhance"] svg')).width;

    expect(Math.abs(fixW - settingsW), 'fix vs settings icon width').toBeLessThanOrEqual(4);
    expect(Math.abs(fixW - pyramidW),  'fix vs pyramid icon width').toBeLessThanOrEqual(4);
  });

  test('all nav icons have the same rendered width when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const fixW      = (await getRect(page, '.nav-item a[href="/fix"] i')).width;
    const settingsW = (await getRect(page, '.nav-item a[href="/settings"] i')).width;
    const pyramidW  = (await getRect(page, '.nav-item a[href="/enhance"] svg')).width;

    expect(Math.abs(fixW - settingsW), 'fix vs settings icon width collapsed').toBeLessThanOrEqual(4);
    expect(Math.abs(fixW - pyramidW),  'fix vs pyramid icon width collapsed').toBeLessThanOrEqual(4);
  });
});

// ── Hover colour parity ───────────────────────────────────────────────────────

test.describe('Shell — hover colour parity', () => {
  /**
   * On hover the <i> icons turn orange (explicit rule).
   * The SVG pyramid uses stroke="currentColor" and should also turn orange.
   * Compare two *non-active* links so active-route white doesn't pollute results.
   */
  test('<i> icons and SVG pyramid icon have the same effective color on hover', async ({ page }) => {
    // Navigate to settings so that neither Fix nor Pyramidize is active.
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(200);

    // Hover the Fix link (non-active), wait for transition to settle, then capture.
    await page.locator('.nav-item a[href="/fix"]').hover();
    await page.waitForTimeout(200); // let the 150ms color transition finish
    const fixIconColor = await page.locator('.nav-item a[href="/fix"] i').evaluate(
      (el) => getComputedStyle(el).color,
    );

    // Hover the Pyramidize link (non-active), wait, then capture SVG color.
    await page.locator('.nav-item a[href="/enhance"]').hover();
    await page.waitForTimeout(200);
    const pyramidIconColor = await page.locator('.nav-item a[href="/enhance"] svg').evaluate(
      (el) => getComputedStyle(el).color,
    );

    // Both should be the same orange accent colour when hovered.
    expect(pyramidIconColor, 'SVG colour on hover should match <i> colour on hover')
      .toBe(fixIconColor);
  });

  test('<i> icon color is the same before and after hovering a different nav item', async ({ page }) => {
    await gotoFix(page);
    const beforeColor = await page.locator('.nav-item a[href="/fix"] i').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // Hover settings — should not change Fix icon color
    await page.locator('.nav-item a[href="/settings"]').hover();
    const afterColor = await page.locator('.nav-item a[href="/fix"] i').evaluate(
      (el) => getComputedStyle(el).color,
    );
    expect(afterColor).toBe(beforeColor);
  });
});

// ── Logo area height ──────────────────────────────────────────────────────────

test.describe('Shell — logo area dimensions', () => {
  test('logo area height is the same in expanded and collapsed state (padding drives height)', async ({ page }) => {
    await gotoFix(page);
    const expandedH = (await getRect(page, '.layout-logo')).height;

    await collapse(page);
    const collapsedH = (await getRect(page, '.layout-logo')).height;

    // Height should not change significantly (same padding, same font-size)
    expect(Math.abs(expandedH - collapsedH), 'logo area height diff').toBeLessThanOrEqual(4);
  });

  test('"K" is fully visible (not clipped) in collapsed state', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350);

    const sidebar = await getRect(page, '.layout-sidebar');
    const kRect   = await getRect(page, '.layout-logo .logo-k');

    expect(kRect.left).toBeGreaterThanOrEqual(sidebar.left - 1);
    expect(kRect.right).toBeLessThanOrEqual(sidebar.right + 1);
    expect(kRect.width).toBeGreaterThan(0);
  });

  test('"K" and "L" combined width does not exceed the collapsed sidebar width', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350);

    const sidebar = await getRect(page, '.layout-sidebar');
    const kRect   = await getRect(page, '.layout-logo .logo-k');
    const lRect   = await getRect(page, '.layout-logo .logo-l');
    const klWidth = lRect.right - kRect.left;

    expect(klWidth).toBeLessThanOrEqual(sidebar.width);
  });
});

// ── Sidebar scroll ────────────────────────────────────────────────────────────

test.describe('Shell — no unexpected scroll', () => {
  test('sidebar has no vertical scrollbar overflow in expanded state', async ({ page }) => {
    await gotoFix(page);
    const overflow = await page.locator('.layout-sidebar').evaluate(
      (el) => getComputedStyle(el).overflowY,
    );
    // Should be 'hidden' (not 'scroll' or 'auto' with content exceeding height)
    expect(overflow).toBe('hidden');
  });

  test('sidebar nav does not overflow the sidebar vertically', async ({ page }) => {
    await gotoFix(page);
    const sidebar  = await getRect(page, '.layout-sidebar');
    const nav      = await getRect(page, '.sidebar-nav');

    expect(nav.top).toBeGreaterThanOrEqual(sidebar.top - 1);
    expect(nav.bottom).toBeLessThanOrEqual(sidebar.bottom + 1);
  });
});

// ── Active route - background width ──────────────────────────────────────────

test.describe('Shell — active route background', () => {
  test('active link background spans the full nav-item width (not just icon)', async ({ page }) => {
    await gotoFix(page);

    // Get the width of the active <a> element
    const activeRect = await getRect(page, '.nav-item a[href="/fix"]');
    expect(activeRect.width).toBeGreaterThan(0);

    // Verify the background is actually painted (non-transparent)
    const bg = await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    expect(bg).not.toBe('rgba(0, 0, 0, 0)');
  });

  test('active link background width in collapsed state is appropriate (not zero)', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const activeRect = await getRect(page, '.nav-item a[href="/fix"]');
    expect(activeRect.width).toBeGreaterThan(20);

    const bg = await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    expect(bg).not.toBe('rgba(0, 0, 0, 0)');
  });
});

// ── Version row padding ───────────────────────────────────────────────────────

test.describe('Shell — version row padding in collapsed state', () => {
  test('version-row has zero or minimal height when collapsed with no content', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const r = await getRect(page, '[data-testid="version-footer"]');
    // When collapsed and no update available, the row has no content.
    // It should collapse to near-zero rather than maintaining full expanded padding.
    // This test will fail if the row keeps its full padding (wasted visual space).
    expect(r.height, 'version-row should not have large height when empty').toBeLessThanOrEqual(16);
  });
});
