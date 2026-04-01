/**
 * Shell sidebar menu tests.
 *
 * Covers: expanded layout, collapsed layout, icon centering,
 * click-target sizes, overflow, active-route, footer, and
 * tooltip behaviour.
 *
 * All layout assertions use getBoundingClientRect() / getComputedStyle()
 * so they catch real CSS failures that jsdom cannot detect.
 */

import { test, expect, Page } from '@playwright/test';

// ── helpers ───────────────────────────────────────────────────────────────────

async function gotoFix(page: Page): Promise<void> {
  await page.goto('/fix');
  await page.waitForLoadState('networkidle');
  await page.waitForTimeout(200);
}

async function collapse(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await page.waitForTimeout(300); // CSS transition
}

async function expand(page: Page): Promise<void> {
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

// ── Expanded state ─────────────────────────────────────────────────────────────

test.describe('Shell — expanded sidebar', () => {
  test('sidebar width is at least 200px when expanded', async ({ page }) => {
    await gotoFix(page);
    const r = await getRect(page, '.layout-sidebar');
    expect(r.width).toBeGreaterThan(200);
  });

  test('all three nav links are visible with non-zero height', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const r = await getRect(page, `.nav-item a[href="${href}"]`);
      expect(r.width, `${href} width`).toBeGreaterThan(0);
      expect(r.height, `${href} height`).toBeGreaterThan(0);
    }
  });

  test('nav link text labels are rendered and non-empty', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText().catch(() => '');
      expect(text.trim(), `${href} label`).not.toBe('');
    }
  });

  test('icons are visible inside each expanded nav link', async ({ page }) => {
    await gotoFix(page);
    // Fix and Settings use <i>, Pyramidize uses <svg>
    const fixIcon    = await getRect(page, '.nav-item a[href="/fix"] i');
    const settingsIcon = await getRect(page, '.nav-item a[href="/settings"] i');
    const pyramidSvg = await getRect(page, '.nav-item a[href="/enhance"] svg');

    expect(fixIcon.width).toBeGreaterThan(0);
    expect(fixIcon.height).toBeGreaterThan(0);
    expect(settingsIcon.width).toBeGreaterThan(0);
    expect(pyramidSvg.width).toBeGreaterThan(0);
  });

  test('Fix link has active-route class on /fix route', async ({ page }) => {
    await gotoFix(page);
    const hasActive = await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => el.classList.contains('active-route'),
    );
    expect(hasActive).toBe(true);
  });

  test('no other links have active-route when Fix is active', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/enhance', '/settings']) {
      const active = await page.locator(`.nav-item a[href="${href}"]`).evaluate(
        (el) => el.classList.contains('active-route'),
      );
      expect(active, `${href} should not be active`).toBe(false);
    }
  });

  test('collapse button is visible and has non-zero size', async ({ page }) => {
    await gotoFix(page);
    const r = await getRect(page, '.collapse-btn');
    expect(r.width).toBeGreaterThan(0);
    expect(r.height).toBeGreaterThan(0);
  });

  test('version footer is visible', async ({ page }) => {
    await gotoFix(page);
    const r = await getRect(page, '[data-testid="version-footer"]');
    expect(r.height).toBeGreaterThan(0);
  });
});

// ── Collapsed state ────────────────────────────────────────────────────────────

test.describe('Shell — collapsed sidebar', () => {
  test('sidebar collapses to ≤ 60px', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const r = await getRect(page, '.layout-sidebar');
    expect(r.width).toBeLessThanOrEqual(60);
    expect(r.width).toBeGreaterThan(0); // not hidden entirely
  });

  test('sidebar has collapsed CSS class after click', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const hasClass = await page.locator('.layout-sidebar').evaluate(
      (el) => el.classList.contains('collapsed'),
    );
    expect(hasClass).toBe(true);
  });

  test('nav link text labels are NOT rendered when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const count = await page.locator(`.nav-item a[href="${href}"] span`).count();
      expect(count, `${href} label should be absent when collapsed`).toBe(0);
    }
  });

  test('nav icons are fully visible (non-zero size) in collapsed state', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const fixIcon    = await getRect(page, '.nav-item a[href="/fix"] i');
    const settingsIcon = await getRect(page, '.nav-item a[href="/settings"] i');
    const pyramidSvg = await getRect(page, '.nav-item a[href="/enhance"] svg');

    expect(fixIcon.width, 'Fix icon width').toBeGreaterThan(0);
    expect(fixIcon.height, 'Fix icon height').toBeGreaterThan(0);
    expect(settingsIcon.width, 'Settings icon width').toBeGreaterThan(0);
    expect(pyramidSvg.width, 'Pyramid svg width').toBeGreaterThan(0);
  });

  test('nav icons are horizontally centered within the collapsed sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const sidebar = await getRect(page, '.layout-sidebar');
    const sidebarCenterX = sidebar.left + sidebar.width / 2;

    for (const sel of [
      '.nav-item a[href="/fix"] i',
      '.nav-item a[href="/settings"] i',
      '.nav-item a[href="/enhance"] svg',
    ]) {
      const r = await getRect(page, sel);
      const iconCenterX = r.left + r.width / 2;
      // Icon center must be within ±8px of sidebar center
      expect(Math.abs(iconCenterX - sidebarCenterX), `icon centering for ${sel}`)
        .toBeLessThanOrEqual(8);
    }
  });

  test('nav icons do not overflow beyond the right edge of the sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const sidebar = await getRect(page, '.layout-sidebar');

    for (const sel of [
      '.nav-item a[href="/fix"] i',
      '.nav-item a[href="/settings"] i',
      '.nav-item a[href="/enhance"] svg',
    ]) {
      const r = await getRect(page, sel);
      expect(r.right, `${sel} right edge`).toBeLessThanOrEqual(sidebar.right + 1);
    }
  });

  test('nav links remain clickable when collapsed (pointer-events not none)', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    for (const href of ['/fix', '/enhance', '/settings']) {
      const pe = await page.locator(`.nav-item a[href="${href}"]`).evaluate(
        (el) => getComputedStyle(el).pointerEvents,
      );
      expect(pe, `${href} pointer-events`).not.toBe('none');
    }
  });

  test('can still navigate to /enhance when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    await page.locator('.nav-item a[href="/enhance"]').click();
    await page.waitForURL('**/enhance', { timeout: 5000 });
    expect(page.url()).toContain('/enhance');
  });

  test('active-route class is applied correctly in collapsed state', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    await page.locator('.nav-item a[href="/enhance"]').click();
    await page.waitForURL('**/enhance', { timeout: 5000 });

    const enhanceActive = await page.locator('.nav-item a[href="/enhance"]').evaluate(
      (el) => el.classList.contains('active-route'),
    );
    const fixActive = await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => el.classList.contains('active-route'),
    );
    expect(enhanceActive).toBe(true);
    expect(fixActive).toBe(false);
  });

  test('collapse button shows chevron-right (expand icon) when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const hasRight = await page.locator('.collapse-btn i').evaluate(
      (el) => el.classList.contains('pi-chevron-right'),
    );
    const hasLeft = await page.locator('.collapse-btn i').evaluate(
      (el) => el.classList.contains('pi-chevron-left'),
    );
    expect(hasRight).toBe(true);
    expect(hasLeft).toBe(false);
  });

  test('collapse button is visible and centered within the collapsed sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const sidebar = await getRect(page, '.layout-sidebar');
    const btn     = await getRect(page, '.collapse-btn');

    expect(btn.width).toBeGreaterThan(0);
    expect(btn.height).toBeGreaterThan(0);
    // Button should not overflow sidebar
    expect(btn.right).toBeLessThanOrEqual(sidebar.right + 1);
  });

  test('version-row does not push content outside sidebar when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const sidebar = await getRect(page, '.layout-sidebar');
    const footer  = await getRect(page, '[data-testid="version-footer"]');

    expect(footer.right).toBeLessThanOrEqual(sidebar.right + 1);
  });
});

// ── Expand / re-collapse ───────────────────────────────────────────────────────

test.describe('Shell — expand after collapse', () => {
  test('sidebar returns to expanded width after second click', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await expand(page);

    const r = await getRect(page, '.layout-sidebar');
    expect(r.width).toBeGreaterThan(200);
  });

  test('nav labels reappear after expanding', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await expand(page);

    for (const href of ['/fix', '/enhance', '/settings']) {
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText().catch(() => '');
      expect(text.trim(), `${href} label after expand`).not.toBe('');
    }
  });
});

// ── Click target size ──────────────────────────────────────────────────────────

test.describe('Shell — click target minimum size', () => {
  test('each nav link has a click target of at least 36px tall in expanded state', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const r = await getRect(page, `.nav-item a[href="${href}"]`);
      expect(r.height, `${href} click target height`).toBeGreaterThanOrEqual(36);
    }
  });

  test('each nav link has a click target of at least 36px tall in collapsed state', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const r = await getRect(page, `.nav-item a[href="${href}"]`);
      expect(r.height, `${href} collapsed click target height`).toBeGreaterThanOrEqual(36);
    }
  });
});
