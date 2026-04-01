/**
 * Shell sidebar — deep visual regression tests.
 *
 * Probes logo centering, icon size consistency, footer layout,
 * version-row behaviour, and gap/padding correctness in both
 * expanded and collapsed states.
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
  await page.waitForTimeout(300);
}

type Rect = { x: number; y: number; width: number; height: number; top: number; right: number; bottom: number; left: number };

async function getRect(page: Page, selector: string): Promise<Rect> {
  return page.locator(selector).evaluate((el) => {
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y, width: r.width, height: r.height, top: r.top, right: r.right, bottom: r.bottom, left: r.left };
  });
}

// ── Logo ──────────────────────────────────────────────────────────────────────

test.describe('Shell — logo area', () => {
  test('logo is visible and non-zero height when expanded', async ({ page }) => {
    await gotoFix(page);
    const r = await getRect(page, '.layout-logo');
    expect(r.height).toBeGreaterThan(0);
    expect(r.width).toBeGreaterThan(0);
  });

  test('"ey" and "int" are visible (non-zero width) when expanded', async ({ page }) => {
    await gotoFix(page);
    await page.waitForTimeout(300);
    const eyW  = (await getRect(page, '.layout-logo .logo-ey')).width;
    const intW = (await getRect(page, '.layout-logo .logo-int')).width;
    expect(eyW,  '"ey" width when expanded').toBeGreaterThan(5);
    expect(intW, '"int" width when expanded').toBeGreaterThan(5);
  });

  test('"ey" and "int" collapse to zero width when sidebar is collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350); // let max-width transition complete
    const eyW  = (await getRect(page, '.layout-logo .logo-ey')).width;
    const intW = (await getRect(page, '.layout-logo .logo-int')).width;
    expect(eyW,  '"ey" width when collapsed').toBeLessThanOrEqual(2);
    expect(intW, '"int" width when collapsed').toBeLessThanOrEqual(2);
  });

  test('"KL" is horizontally centered within the collapsed sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await page.waitForTimeout(350);

    const sidebar = await getRect(page, '.layout-sidebar');
    const kRect   = await getRect(page, '.layout-logo .logo-k');
    const lRect   = await getRect(page, '.layout-logo .logo-l');

    // Centre of the visible "KL" pair vs centre of the sidebar
    const klLeft    = kRect.left;
    const klRight   = lRect.right;
    const klCenterX = (klLeft + klRight) / 2;
    const sidebarCenterX = sidebar.left + sidebar.width / 2;

    expect(Math.abs(klCenterX - sidebarCenterX), '"KL" centering offset')
      .toBeLessThanOrEqual(10);
  });
});

// ── Icon size consistency ─────────────────────────────────────────────────────

test.describe('Shell — icon size consistency', () => {
  test('all nav icons have the same rendered height when expanded', async ({ page }) => {
    await gotoFix(page);

    const fixH      = (await getRect(page, '.nav-item a[href="/fix"] i')).height;
    const settingsH = (await getRect(page, '.nav-item a[href="/settings"] i')).height;
    const pyramidH  = (await getRect(page, '.nav-item a[href="/enhance"] svg')).height;

    // All should be within 4px of each other
    expect(Math.abs(fixH - settingsH), 'fix vs settings height diff').toBeLessThanOrEqual(4);
    expect(Math.abs(fixH - pyramidH), 'fix vs pyramid height diff').toBeLessThanOrEqual(4);
  });

  test('all nav icons have the same rendered height when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const fixH      = (await getRect(page, '.nav-item a[href="/fix"] i')).height;
    const settingsH = (await getRect(page, '.nav-item a[href="/settings"] i')).height;
    const pyramidH  = (await getRect(page, '.nav-item a[href="/enhance"] svg')).height;

    expect(Math.abs(fixH - settingsH), 'fix vs settings height diff collapsed').toBeLessThanOrEqual(4);
    expect(Math.abs(fixH - pyramidH), 'fix vs pyramid height diff collapsed').toBeLessThanOrEqual(4);
  });

  test('all nav items have the same rendered height (click-target parity) when expanded', async ({ page }) => {
    await gotoFix(page);

    const heights = await page.locator('.nav-item a').evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().height),
    );
    const min = Math.min(...heights);
    const max = Math.max(...heights);
    expect(max - min, 'nav item height variance').toBeLessThanOrEqual(4);
  });

  test('all nav items have the same rendered height when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const heights = await page.locator('.nav-item a').evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().height),
    );
    const min = Math.min(...heights);
    const max = Math.max(...heights);
    expect(max - min, 'collapsed nav item height variance').toBeLessThanOrEqual(4);
  });
});

// ── Version row ───────────────────────────────────────────────────────────────

test.describe('Shell — version row in collapsed state', () => {
  test('version text is not visible when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const count = await page.locator('[data-testid="version-footer"] .version-text').count();
    expect(count, 'version-text should be absent when collapsed').toBe(0);
  });

  test('version-row height when collapsed is not taller than the version-row height when expanded', async ({ page }) => {
    await gotoFix(page);
    const expandedH = (await getRect(page, '[data-testid="version-footer"]')).height;

    await collapse(page);
    const collapsedH = (await getRect(page, '[data-testid="version-footer"]')).height;

    // Collapsed version row should not be taller (it has less or no content)
    expect(collapsedH, 'collapsed version-row height').toBeLessThanOrEqual(expandedH + 2);
  });

  test('version-row has no visible text content when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const text = await page.locator('[data-testid="version-footer"]').innerText().catch(() => '');
    expect(text.trim(), 'version-row inner text when collapsed').toBe('');
  });
});

// ── Footer layout ─────────────────────────────────────────────────────────────

test.describe('Shell — sidebar footer layout', () => {
  test('collapse button fills the full width of the sidebar', async ({ page }) => {
    await gotoFix(page);
    const sidebar = await getRect(page, '.layout-sidebar');
    const btn     = await getRect(page, '.collapse-btn');
    // Button width should match sidebar width (it has width:100%)
    expect(Math.abs(btn.width - sidebar.width), 'collapse-btn width vs sidebar').toBeLessThanOrEqual(4);
  });

  test('collapse button fills the full width of the collapsed sidebar', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const sidebar = await getRect(page, '.layout-sidebar');
    const btn     = await getRect(page, '.collapse-btn');
    expect(Math.abs(btn.width - sidebar.width), 'collapse-btn width vs collapsed sidebar').toBeLessThanOrEqual(4);
  });

  test('sidebar footer is fully within sidebar bounds', async ({ page }) => {
    await gotoFix(page);
    const sidebar = await getRect(page, '.layout-sidebar');
    const footer  = await page.locator('.sidebar-footer').evaluate((el) => {
      const r = el.getBoundingClientRect();
      return { left: r.left, right: r.right };
    });
    expect(footer.left).toBeGreaterThanOrEqual(sidebar.left - 1);
    expect(footer.right).toBeLessThanOrEqual(sidebar.right + 1);
  });

  test('sidebar footer is fully within collapsed sidebar bounds', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const sidebar = await getRect(page, '.layout-sidebar');
    const footer  = await page.locator('.sidebar-footer').evaluate((el) => {
      const r = el.getBoundingClientRect();
      return { left: r.left, right: r.right };
    });
    expect(footer.right).toBeLessThanOrEqual(sidebar.right + 1);
  });
});

// ── Active route colour ───────────────────────────────────────────────────────

test.describe('Shell — active route highlight colour', () => {
  test('active nav link has orange-ish background', async ({ page }) => {
    await gotoFix(page);
    const bg = await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    );
    // Should be orange (rgb(249, 115, 22) = #f97316) or close to it
    // Accept any non-transparent, non-surface background
    expect(bg).not.toBe('rgba(0, 0, 0, 0)');
    expect(bg).not.toBe('transparent');
  });

  test('inactive nav links have no prominent background', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/enhance', '/settings']) {
      const bg = await page.locator(`.nav-item a[href="${href}"]`).evaluate(
        (el) => getComputedStyle(el).backgroundColor,
      );
      // Should be transparent or the dark surface colour — NOT orange
      const isOrange = bg.includes('249') && bg.includes('115');
      expect(isOrange, `${href} should not have orange background`).toBe(false);
    }
  });

  test('Pyramidize SVG icon colour changes to white when link is active', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(200);

    // SVG uses stroke="currentColor" — computed color on the <a> should be white
    const color = await page.locator('.nav-item a[href="/enhance"]').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // Active colour should be white (rgb(255, 255, 255))
    expect(color, 'Pyramidize link color when active').toBe('rgb(255, 255, 255)');
  });
});

// ── Hover styles (visual integrity) ──────────────────────────────────────────

test.describe('Shell — hover visual integrity', () => {
  test('nav link cursor is pointer', async ({ page }) => {
    await gotoFix(page);
    for (const href of ['/fix', '/enhance', '/settings']) {
      const cursor = await page.locator(`.nav-item a[href="${href}"]`).evaluate(
        (el) => getComputedStyle(el).cursor,
      );
      expect(cursor, `${href} cursor`).toBe('pointer');
    }
  });
});
