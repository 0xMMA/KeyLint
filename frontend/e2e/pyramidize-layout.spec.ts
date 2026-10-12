/**
 * Pyramidize layout tests — run in real Chromium, no API calls.
 *
 * These catch CSS layout regressions (flex height, overflow, clipped
 * controls) that jsdom/Vitest cannot detect because jsdom does not apply
 * stylesheets. The Result step needs a result, so the Pyramidize service is
 * faked (support/fake-pyramidize.ts).
 *
 * Convention: assert rendered dimensions with getBoundingClientRect() and
 * getComputedStyle() rather than snapshotting pixels — snapshots are brittle
 * across font hinting / sub-pixel differences between environments.
 */

import { test, expect, Page } from '@playwright/test';
import { fakeState, installFakeBackend } from './support/fake-backend';
import { fakePyramidize, installFakePyramidize } from './support/fake-pyramidize';

const DOCUMENT = 'Subject | Details | Actions\n\nHi,\n\n**Main point**\n- One\n- Two\n\nBest';

async function gotoEnhance(page: Page): Promise<void> {
  await installFakeBackend(page, fakeState({ activeProvider: 'claude', keys: { claude: 'keyring' } }));
  await installFakePyramidize(page, fakePyramidize({ fullDocument: DOCUMENT }));
  await page.goto('/enhance');
  await expect(page.getByTestId('original-textarea')).toBeVisible();
  await page.waitForLoadState('networkidle');
}

async function toResult(page: Page): Promise<void> {
  await page.getByTestId('original-textarea').fill('hi, two things about the numbers');
  await page.getByTestId('choose-form-btn').getByRole('button').click();
  await page.getByTestId('pyramidize-btn').getByRole('button').click();
  await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
}

async function rect(page: Page, selector: string) {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { width: r.width, height: r.height, top: r.top, bottom: r.bottom };
  });
}

test.describe('Pyramidize — the text sheets fill the available space', () => {
  test('the Text step\'s sheet is tall, and stays inside the viewport with its buttons', async ({ page }) => {
    await gotoEnhance(page);
    const viewportHeight = page.viewportSize()!.height;
    const r = await rect(page, '[data-testid="original-textarea"]');
    expect(r.height).toBeGreaterThan(200);
    const go = await rect(page, '[data-testid="choose-form-btn"]');
    expect(go.bottom).toBeLessThanOrEqual(viewportHeight + 1);
  });

  test('the result\'s sheet is tall, and the controls under it are not clipped', async ({ page }) => {
    await gotoEnhance(page);
    await toResult(page);
    const viewportHeight = page.viewportSize()!.height;
    expect((await rect(page, '[data-testid="canvas-textarea"]')).height).toBeGreaterThan(200);
    for (const id of ['global-instruction-input', 'copy-markdown-btn', 'copy-rich-text-btn']) {
      const r = await rect(page, `[data-testid="${id}"]`);
      expect(r.height, id).toBeGreaterThan(0);
      expect(r.bottom, id).toBeLessThanOrEqual(viewportHeight + 1);
    }
  });

  test('the preview has a real height and does not overflow the viewport', async ({ page }) => {
    await gotoEnhance(page);
    await toResult(page);
    await page.getByTestId('segment-preview').click();
    const previewEl = page.getByTestId('canvas-preview');
    await expect(previewEl).toBeVisible();
    const r = await rect(page, '[data-testid="canvas-preview"]');
    expect(r.height).toBeGreaterThan(100);
    expect(r.bottom).toBeLessThanOrEqual(page.viewportSize()!.height + 1);
  });
});

test.describe('Pyramidize — the form choice', () => {
  test('lays the five forms out in one row at the default width', async ({ page }) => {
    await page.setViewportSize({ width: 1264, height: 740 });
    await gotoEnhance(page);
    await page.getByTestId('original-textarea').fill('some text');
    await page.getByTestId('choose-form-btn').getByRole('button').click();
    const tops = await page.getByRole('radio').evaluateAll(els => els.map(el => Math.round(el.getBoundingClientRect().top)));
    expect(tops).toHaveLength(5);
    expect(new Set(tops).size, 'the forms wrapped').toBe(1);
  });
});

test.describe('Pyramidize — history column', () => {
  test('starts closed, opens from the result\'s tools, and closes again', async ({ page }) => {
    await gotoEnhance(page);
    await toResult(page);
    const panel = page.locator('[data-testid="trace-log-panel"]');
    await expect(panel).toHaveCount(0);

    await page.getByTestId('trace-log-toggle').getByRole('button').click();
    await expect(panel).toBeVisible();
    const r = await rect(page, '[data-testid="trace-log-panel"]');
    expect(r.width).toBeGreaterThan(200);
    // A column of its own: the editor beside it keeps its full height.
    expect(r.height).toBeGreaterThan(400);

    await page.getByTestId('trace-log-close').getByRole('button').click();
    await expect(panel).toHaveCount(0);
  });
});

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

    await page.locator('.collapse-btn').click();
    await page.waitForTimeout(300);

    const expandedWidth = await sidebar.evaluate((el) => el.getBoundingClientRect().width);
    expect(expandedWidth).toBeGreaterThan(100);
  });
});
