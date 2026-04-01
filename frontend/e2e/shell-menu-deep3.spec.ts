/**
 * Shell sidebar — fourth layer.
 *
 * Probes: footer dead-space, version-row click target in collapsed state,
 * nav item left-edge alignment, sidebar total height, and logo top/bottom
 * padding asymmetry when collapsed.
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

// ── Version-row dead-space ─────────────────────────────────────────────────────

test.describe('Shell — version-row dead-space when collapsed', () => {
  test('version-row has no visible / hoverable area when collapsed with no content', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    // When collapsed with no update: version-row should not present an
    // accidental hover target. Height > 0 means there is dead clickable space.
    const r = await getRect(page, '[data-testid="version-footer"]');
    expect(r.height, 'version-row should have zero height when empty-collapsed').toBe(0);
  });
});

// ── Nav item left-edge alignment ───────────────────────────────────────────────

test.describe('Shell — nav item alignment', () => {
  test('all nav items have the same left edge (consistent margin) when expanded', async ({ page }) => {
    await gotoFix(page);

    const lefts = await page.locator('.nav-item a').evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().left),
    );
    const min = Math.min(...lefts);
    const max = Math.max(...lefts);
    expect(max - min, 'nav item left-edge variance').toBeLessThanOrEqual(2);
  });

  test('all nav items have the same left edge when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const lefts = await page.locator('.nav-item a').evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().left),
    );
    const min = Math.min(...lefts);
    const max = Math.max(...lefts);
    expect(max - min, 'collapsed nav item left-edge variance').toBeLessThanOrEqual(2);
  });

  test('all nav icons have the same horizontal center when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const iconSelectors = [
      '.nav-item a[href="/fix"] i',
      '.nav-item a[href="/enhance"] svg',
      '.nav-item a[href="/settings"] i',
    ];

    const centers = await Promise.all(
      iconSelectors.map((sel) =>
        page.locator(sel).evaluate((el) => {
          const r = el.getBoundingClientRect();
          return r.left + r.width / 2;
        }),
      ),
    );

    const min = Math.min(...centers);
    const max = Math.max(...centers);
    expect(max - min, 'icon horizontal center variance when collapsed').toBeLessThanOrEqual(4);
  });
});

// ── Sidebar total height ───────────────────────────────────────────────────────

test.describe('Shell — sidebar fills viewport height', () => {
  test('sidebar height equals the viewport height', async ({ page }) => {
    await gotoFix(page);
    const viewportHeight = page.viewportSize()!.height;
    const r = await getRect(page, '.layout-sidebar');
    expect(r.height, 'sidebar height').toBeGreaterThanOrEqual(viewportHeight - 2);
  });

  test('sidebar height equals the viewport height when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const viewportHeight = page.viewportSize()!.height;
    const r = await getRect(page, '.layout-sidebar');
    expect(r.height, 'collapsed sidebar height').toBeGreaterThanOrEqual(viewportHeight - 2);
  });
});

// ── Layout-main fills remaining space ─────────────────────────────────────────

test.describe('Shell — layout-main dimensions', () => {
  test('layout-main right edge touches viewport right edge', async ({ page }) => {
    await gotoFix(page);
    const viewportWidth = page.viewportSize()!.width;
    const r = await getRect(page, '.layout-main');
    expect(Math.abs(r.right - viewportWidth), 'layout-main right edge').toBeLessThanOrEqual(2);
  });

  test('sidebar and main together fill the full viewport width', async ({ page }) => {
    await gotoFix(page);
    const viewportWidth = page.viewportSize()!.width;
    const sidebar = await getRect(page, '.layout-sidebar');
    const main    = await getRect(page, '.layout-main');
    expect(Math.abs((sidebar.width + main.width) - viewportWidth), 'sidebar + main width').toBeLessThanOrEqual(4);
  });

  test('layout-main expands when sidebar is collapsed', async ({ page }) => {
    await gotoFix(page);
    const expandedMainWidth = (await getRect(page, '.layout-main')).width;

    await collapse(page);
    const collapsedMainWidth = (await getRect(page, '.layout-main')).width;

    expect(collapsedMainWidth).toBeGreaterThan(expandedMainWidth);
  });
});

// ── Logo vertical padding when collapsed ──────────────────────────────────────

test.describe('Shell — logo area padding in collapsed state', () => {
  test('"K" is vertically centered within the logo area (top ≈ bottom gap)', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350);

    const logoArea = await getRect(page, '.layout-logo');
    const kRect    = await getRect(page, '.layout-logo .logo-k');

    const topGap    = kRect.top - logoArea.top;
    const bottomGap = logoArea.bottom - kRect.bottom;

    // With padding: 1.5rem 1rem 1rem, top gap is larger than bottom gap.
    // Top gap should not be more than 2× the bottom gap (otherwise "K" looks dropped).
    expect(topGap / bottomGap, '"K" vertical position ratio (top/bottom gap)').toBeLessThanOrEqual(2.5);
  });
});
