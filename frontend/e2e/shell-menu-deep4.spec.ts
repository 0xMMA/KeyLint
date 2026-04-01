/**
 * Shell sidebar — fifth layer.
 *
 * Covers the five fixes made in this session:
 *   1. No scrollbar on hover-expand (layout-main does not resize)
 *   2. Active Pyramidize icon is white (not orange-on-orange)
 *   3. Collapsed logo shows "KL" with K white / L orange
 *   4. Collapse button is at least 40px tall
 *   5. Hover-expand: sidebar expands to full width on hover; collapses on leave
 */

import { test, expect, Page } from '@playwright/test';

async function gotoFix(page: Page): Promise<void> {
  await page.goto('/fix');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(200);
}

async function collapse(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await page.waitForTimeout(350); // CSS transition
}

type Rect = { width: number; height: number; left: number; right: number; top: number; bottom: number };
async function getRect(page: Page, selector: string): Promise<Rect> {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { width: r.width, height: r.height, left: r.left, right: r.right, top: r.top, bottom: r.bottom };
  });
}

// ── 1. No layout shift / scrollbar on hover-expand ────────────────────────────

test.describe('Shell — hover-expand does not shift layout', () => {
  test('layout-main width does NOT change when hovering the collapsed sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const mainBefore = await getRect(page, '.layout-main');

    // Hover the sidebar — triggers hover-expand
    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350); // transition complete

    const mainAfter = await getRect(page, '.layout-main');

    // Main content must not resize (sidebar is overlay, not push)
    expect(Math.abs(mainAfter.width - mainBefore.width), 'main width shift on hover').toBeLessThanOrEqual(2);
    expect(Math.abs(mainAfter.left  - mainBefore.left),  'main left shift on hover').toBeLessThanOrEqual(2);
  });

  test('layout-main has no horizontal scrollbar after hover-expand', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const overflow = await page.locator('.layout-main').evaluate(
      (el) => getComputedStyle(el).overflowX,
    );
    // Should be auto (or hidden) but must not have an actual scrollbar
    const scrollWidth = await page.locator('.layout-main').evaluate(
      (el) => (el as HTMLElement).scrollWidth,
    );
    const clientWidth = await page.locator('.layout-main').evaluate(
      (el) => (el as HTMLElement).clientWidth,
    );
    expect(scrollWidth, 'scrollWidth <= clientWidth (no horiz scroll)').toBeLessThanOrEqual(clientWidth + 2);
  });
});

// ── 2. Active Pyramidize icon is white (not orange-on-orange) ─────────────────

test.describe('Shell — active Pyramidize SVG icon colour', () => {
  test('SVG pyramid icon is white (not orange) when Pyramidize is the active route', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(200);

    const svgColor = await page.locator('.nav-item a[href="/enhance"] svg').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // Must be white, not orange
    expect(svgColor, 'SVG color on active Pyramidize link').toBe('rgb(255, 255, 255)');
  });

  test('SVG pyramid icon is NOT orange when Pyramidize is the active route', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(200);

    const svgColor = await page.locator('.nav-item a[href="/enhance"] svg').evaluate(
      (el) => getComputedStyle(el).color,
    );
    const isOrange = svgColor.includes('249') || svgColor.includes('251');
    expect(isOrange, 'SVG should not be orange on active background').toBe(false);
  });
});

// ── 3. Collapsed logo shows "KL" ──────────────────────────────────────────────

test.describe('Shell — collapsed logo "KL"', () => {
  test('collapsed: "ey" and "int" collapse to zero width (only KL visible)', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350); // let max-width transition complete
    const eyW  = (await page.locator('.layout-logo .logo-ey').boundingBox())?.width ?? 0;
    const intW = (await page.locator('.layout-logo .logo-int').boundingBox())?.width ?? 0;
    expect(eyW,  '"ey" width when collapsed').toBeLessThanOrEqual(2);
    expect(intW, '"int" width when collapsed').toBeLessThanOrEqual(2);
  });

  test('"K" in collapsed logo is white', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const color = await page.locator('.layout-logo .logo-k').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // --p-surface-50 resolves to #fafafa (rgb(250,250,250)) in this theme
    expect(color, '"K" should be near-white').toMatch(/^rgb\(2[45]\d, 2[45]\d, 2[45]\d\)$/);
  });

  test('"L" in collapsed logo is orange', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const color = await page.locator('.layout-logo .logo-l').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // Primary orange: rgb(249, 115, 22) or similar
    const isOrange = color.includes('249') || color.includes('251') || color.includes('247');
    expect(isOrange, 'L should be orange').toBe(true);
  });

  test('expanded: "ey" and "int" are fully visible (non-zero width)', async ({ page }) => {
    await gotoFix(page);
    await page.waitForTimeout(300);
    const eyW  = (await page.locator('.layout-logo .logo-ey').boundingBox())?.width ?? 0;
    const intW = (await page.locator('.layout-logo .logo-int').boundingBox())?.width ?? 0;
    expect(eyW,  '"ey" width when expanded').toBeGreaterThan(5);
    expect(intW, '"int" width when expanded').toBeGreaterThan(5);
  });
});

// ── 4. Collapse button minimum height ─────────────────────────────────────────

test.describe('Shell — collapse button height', () => {
  test('collapse button is at least 40px tall when expanded', async ({ page }) => {
    await gotoFix(page);
    const r = await getRect(page, '.collapse-btn');
    expect(r.height, 'collapse-btn height when expanded').toBeGreaterThanOrEqual(40);
  });

  test('collapse button is at least 40px tall when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const r = await getRect(page, '.collapse-btn');
    expect(r.height, 'collapse-btn height when collapsed').toBeGreaterThanOrEqual(40);
  });
});

// ── 5. Hover-expand popover ───────────────────────────────────────────────────

test.describe('Shell — hover-expand popover', () => {
  test('sidebar expands to full width when hovering while collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    // Confirm collapsed
    const collapsedWidth = (await getRect(page, '.layout-sidebar')).width;
    expect(collapsedWidth).toBeLessThanOrEqual(60);

    // Hover to trigger expand
    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const hoveredWidth = (await getRect(page, '.layout-sidebar')).width;
    expect(hoveredWidth, 'sidebar width on hover').toBeGreaterThan(200);
  });

  test('nav labels are visible during hover-expand', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    for (const href of ['/fix', '/enhance', '/settings']) {
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText().catch(() => '');
      expect(text.trim(), `${href} label during hover-expand`).not.toBe('');
    }
  });

  test('sidebar collapses back when mouse leaves', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    // Hover to expand
    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    // Move mouse away (to layout-main)
    await page.locator('.layout-main').hover();
    await page.waitForTimeout(350);

    const width = (await getRect(page, '.layout-sidebar')).width;
    expect(width, 'sidebar should collapse back after mouse leave').toBeLessThanOrEqual(60);
  });

  test('hover-expand: "ey" and "int" are visible (non-zero width)', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const eyW  = (await page.locator('.layout-logo .logo-ey').boundingBox())?.width ?? 0;
    const intW = (await page.locator('.layout-logo .logo-int').boundingBox())?.width ?? 0;
    expect(eyW,  '"ey" width during hover-expand').toBeGreaterThan(5);
    expect(intW, '"int" width during hover-expand').toBeGreaterThan(5);
  });

  test('sidebar does NOT hover-expand when already fully expanded', async ({ page }) => {
    await gotoFix(page);

    const widthBefore = (await getRect(page, '.layout-sidebar')).width;
    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);
    const widthAfter = (await getRect(page, '.layout-sidebar')).width;

    // Width should not change (already expanded)
    expect(Math.abs(widthAfter - widthBefore)).toBeLessThanOrEqual(2);
  });
});
