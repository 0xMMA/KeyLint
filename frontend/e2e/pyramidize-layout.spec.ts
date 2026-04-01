/**
 * Pyramidize layout tests — run in real Chromium, no API calls.
 *
 * These catch CSS layout regressions (flex height, tab visibility, overflow)
 * that jsdom/Vitest cannot detect because jsdom does not apply stylesheets.
 *
 * Convention: assert rendered dimensions with getBoundingClientRect() and
 * getComputedStyle() rather than snapshotting pixels — snapshots are brittle
 * across font hinting / sub-pixel differences between environments.
 */

import { test, expect, Page } from '@playwright/test';

// ── helpers ──────────────────────────────────────────────────────────────────

async function gotoEnhance(page: Page): Promise<void> {
  await page.goto('/enhance');
  await page.waitForLoadState('networkidle');
  // Wait for the component's initial RPC calls (getSourceApp, loadSettings…) to settle.
  await page.waitForTimeout(300);
}

/** Returns getBoundingClientRect() for a selector. */
async function rect(page: Page, selector: string) {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { width: r.width, height: r.height, top: r.top, bottom: r.bottom };
  });
}

/** Returns getComputedStyle property. */
async function style(page: Page, selector: string, prop: string): Promise<string> {
  return page.locator(selector).evaluate(
    (el, p) => getComputedStyle(el).getPropertyValue(p),
    prop,
  );
}

// ── Tab switching ─────────────────────────────────────────────────────────────

test.describe('Pyramidize — tab switching', () => {
  test('only the active tab panel is visible', async ({ page }) => {
    await gotoEnhance(page);

    // Original tab is active by default.
    const originalPanel = page.locator('p-tabpanel[value="original"]');
    const canvasPanel   = page.locator('p-tabpanel[value="canvas"]');

    await expect(originalPanel).toBeVisible();

    // The inactive canvas panel must not be visible.
    await expect(canvasPanel).toBeHidden();

    // Switch to canvas tab.
    await page.locator('p-tab[value="canvas"]').click();

    await expect(canvasPanel).toBeVisible();
    await expect(originalPanel).toBeHidden();
  });

  test('both panels are never simultaneously visible', async ({ page }) => {
    await gotoEnhance(page);

    const panels = page.locator('p-tabpanel');
    const count  = await panels.count();
    expect(count).toBe(2);

    // Measure how many panels have height > 0 on the initial render.
    const heights = await panels.evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().height),
    );
    const visible = heights.filter((h) => h > 0);
    expect(visible.length).toBe(1);

    // Switch tab, re-check.
    await page.locator('p-tab[value="canvas"]').click();
    const heights2 = await panels.evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().height),
    );
    const visible2 = heights2.filter((h) => h > 0);
    expect(visible2.length).toBe(1);
  });
});

// ── Textarea height ───────────────────────────────────────────────────────────

test.describe('Pyramidize — textarea fills available space', () => {
  test('original textarea is taller than 200px', async ({ page }) => {
    await gotoEnhance(page);

    const r = await rect(page, '[data-testid="original-textarea"]');
    expect(r.height).toBeGreaterThan(200);
  });

  test('canvas textarea is taller than 200px after switching to canvas tab', async ({ page }) => {
    await gotoEnhance(page);
    await page.locator('p-tab[value="canvas"]').click();

    const r = await rect(page, '[data-testid="canvas-textarea"]');
    expect(r.height).toBeGreaterThan(200);
  });

  test('textareas do not overflow below the viewport', async ({ page }) => {
    await gotoEnhance(page);

    const viewportHeight = page.viewportSize()!.height;
    const r = await rect(page, '[data-testid="original-textarea"]');
    expect(r.bottom).toBeLessThanOrEqual(viewportHeight + 1); // +1px tolerance
  });
});

// ── Bottom controls always visible ───────────────────────────────────────────

test.describe('Pyramidize — bottom controls are not clipped', () => {
  test('global instruction input is visible and within viewport', async ({ page }) => {
    await gotoEnhance(page);

    const viewportHeight = page.viewportSize()!.height;
    const r = await rect(page, '[data-testid="global-instruction-input"]');

    expect(r.height).toBeGreaterThan(0);
    expect(r.bottom).toBeLessThanOrEqual(viewportHeight + 1);
  });

  test('Copy Markdown button is visible and within viewport', async ({ page }) => {
    await gotoEnhance(page);

    const viewportHeight = page.viewportSize()!.height;
    const r = await rect(page, '[data-testid="copy-markdown-btn"]');

    expect(r.height).toBeGreaterThan(0);
    expect(r.bottom).toBeLessThanOrEqual(viewportHeight + 1);
  });
});

// ── Left panel controls ───────────────────────────────────────────────────────

test.describe('Pyramidize — left panel', () => {
  test('provider and model selectors are visible', async ({ page }) => {
    await gotoEnhance(page);

    await expect(page.locator('[data-testid="provider-select"]')).toBeVisible();
    await expect(page.locator('[data-testid="model-select"]')).toBeVisible();
  });

  test('doc-type selector is visible', async ({ page }) => {
    await gotoEnhance(page);
    await expect(page.locator('[data-testid="doc-type-select"]')).toBeVisible();
  });

  test('pyramidize button is visible', async ({ page }) => {
    await gotoEnhance(page);
    await expect(page.locator('[data-testid="pyramidize-btn"]')).toBeVisible();
  });
});

// ── Trace log panel ───────────────────────────────────────────────────────────

test.describe('Pyramidize — trace log panel', () => {
  test('trace panel starts collapsed and can be expanded', async ({ page }) => {
    await gotoEnhance(page);

    const panel = page.locator('[data-testid="trace-log-panel"]');
    await expect(panel).toBeVisible();

    // In collapsed state the width should be narrow (≤ 50px).
    const r = await rect(page, '[data-testid="trace-log-panel"]');
    expect(r.width).toBeLessThanOrEqual(50);

    // Click the history icon to expand.
    await page.locator('[data-testid="trace-log-panel"] button').first().click();
    await page.waitForTimeout(250); // transition

    const r2 = await rect(page, '[data-testid="trace-log-panel"]');
    expect(r2.width).toBeGreaterThan(50);
  });
});

// ── Sidebar collapse ─────────────────────────────────────────────────────────

test.describe('Shell — sidebar collapse', () => {
  test('sidebar collapses to icon-only strip on button click', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');

    const sidebar = page.locator('.layout-sidebar');
    const initialWidth = await sidebar.evaluate((el) => el.getBoundingClientRect().width);
    expect(initialWidth).toBeGreaterThan(100); // expanded

    await page.locator('.collapse-btn').click();
    await page.waitForTimeout(300); // CSS transition

    const collapsedWidth = await sidebar.evaluate((el) => el.getBoundingClientRect().width);
    expect(collapsedWidth).toBeLessThan(70); // collapsed to ~48px

    // Expand again.
    await page.locator('.collapse-btn').click();
    await page.waitForTimeout(300);

    const expandedWidth = await sidebar.evaluate((el) => el.getBoundingClientRect().width);
    expect(expandedWidth).toBeGreaterThan(100);
  });
});

// ── Canvas preview mode ───────────────────────────────────────────────────────

test.describe('Pyramidize — canvas preview mode', () => {
  test('preview div has meaningful height and does not overflow viewport', async ({ page }) => {
    await gotoEnhance(page);

    // Switch to canvas tab and click Preview.
    await page.locator('p-tab[value="canvas"]').click();
    await page.locator('button:has-text("Preview")').click();

    const viewportHeight = page.viewportSize()!.height;
    const previewEl = page.locator('.canvas-preview');

    await expect(previewEl).toBeVisible();

    const r = await rect(page, '.canvas-preview');
    expect(r.height).toBeGreaterThan(100);
    expect(r.bottom).toBeLessThanOrEqual(viewportHeight + 1);
  });
});
