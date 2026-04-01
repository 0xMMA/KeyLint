/**
 * Shell sidebar — sixth layer.
 *
 * Investigates and guards against the nav-item vertical shift on hover-expand:
 *
 *   When Pyramidize (or any item) is the active route, hovering the collapsed
 *   sidebar triggers hover-expand which renders the text <span> labels. If the
 *   span's line-height is larger than the icon height, align-items:center
 *   re-centers the icon in the taller <a> box, causing a visible 1-2px drop.
 *   The active item (orange background) makes this shift very noticeable.
 *
 * Tests confirm:
 *   1. No vertical shift on any nav item during hover-expand.
 *   2. No height change on <a> elements between collapsed and hover-expanded.
 *   3. Active Pyramidize item specifically does not drop.
 */

import { test, expect, Page } from '@playwright/test';

async function gotoEnhance(page: Page): Promise<void> {
  await page.goto('/enhance');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(200);
}

async function collapse(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await page.waitForTimeout(350);
}

type Rect = { top: number; bottom: number; height: number; left: number; right: number; width: number };

async function getRect(page: Page, selector: string): Promise<Rect> {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { top: r.top, bottom: r.bottom, height: r.height, left: r.left, right: r.right, width: r.width };
  });
}

// ── 1. No vertical position shift during hover-expand ────────────────────────

test.describe('Shell — no nav-item vertical shift on hover-expand', () => {
  test('Pyramidize (active) nav link top position does not change during hover-expand', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const topBefore = (await getRect(page, '.nav-item a[href="/enhance"]')).top;

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const topAfter = (await getRect(page, '.nav-item a[href="/enhance"]')).top;

    expect(Math.abs(topAfter - topBefore), 'Pyramidize link top position shift').toBeLessThanOrEqual(1);
  });

  test('Fix nav link top position does not change during hover-expand', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const topBefore = (await getRect(page, '.nav-item a[href="/fix"]')).top;

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const topAfter = (await getRect(page, '.nav-item a[href="/fix"]')).top;

    expect(Math.abs(topAfter - topBefore), 'Fix link top position shift').toBeLessThanOrEqual(1);
  });

  test('Settings nav link top position does not change during hover-expand', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const topBefore = (await getRect(page, '.nav-item a[href="/settings"]')).top;

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const topAfter = (await getRect(page, '.nav-item a[href="/settings"]')).top;

    expect(Math.abs(topAfter - topBefore), 'Settings link top position shift').toBeLessThanOrEqual(1);
  });
});

// ── 2. <a> height does not change between collapsed and hover-expanded ────────

test.describe('Shell — nav item height stability', () => {
  test('Pyramidize <a> height is the same collapsed vs hover-expanded', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const hCollapsed = (await getRect(page, '.nav-item a[href="/enhance"]')).height;

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const hExpanded = (await getRect(page, '.nav-item a[href="/enhance"]')).height;

    expect(Math.abs(hExpanded - hCollapsed), 'Pyramidize <a> height delta').toBeLessThanOrEqual(1);
  });

  test('all nav item heights are identical collapsed vs hover-expanded', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const heightsBefore = await page.locator('.nav-item a').evaluateAll(
      (els) => els.map((el) => el.getBoundingClientRect().height),
    );

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const heightsAfter = await page.locator('.nav-item a').evaluateAll(
      (els) => els.map((el) => el.getBoundingClientRect().height),
    );

    for (let i = 0; i < heightsBefore.length; i++) {
      expect(
        Math.abs(heightsAfter[i] - heightsBefore[i]),
        `nav item ${i} height delta`,
      ).toBeLessThanOrEqual(1);
    }
  });
});

// ── 3. SVG icon vertical position is stable ───────────────────────────────────

test.describe('Shell — SVG pyramid icon vertical stability', () => {
  test('SVG pyramid icon center Y does not shift during hover-expand', async ({ page }) => {
    await gotoEnhance(page);
    await collapse(page);

    const svgBefore = await getRect(page, '.nav-item a[href="/enhance"] svg');
    const centerYBefore = svgBefore.top + svgBefore.height / 2;

    await page.locator('.layout-sidebar').hover();
    await page.waitForTimeout(350);

    const svgAfter = await getRect(page, '.nav-item a[href="/enhance"] svg');
    const centerYAfter = svgAfter.top + svgAfter.height / 2;

    expect(Math.abs(centerYAfter - centerYBefore), 'SVG center Y shift').toBeLessThanOrEqual(1);
  });
});
