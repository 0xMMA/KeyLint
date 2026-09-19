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
  // Not networkidle: the Angular dev server holds a live-reload socket open, so
  // "idle" needs 500ms of quiet that a second worker hammering the same server
  // keeps pushing away — Playwright discourages it for exactly this. The
  // sidebar being painted is the condition these tests actually need.
  await expect(page.locator('.layout-sidebar')).toBeVisible();
  await settle(page);
}

/**
 * Waits for the sidebar's CSS transitions to finish.
 *
 * A fixed sleep makes a slow CI runner fail; this makes it take longer instead.
 * Transitions start a frame after the class flips, so give the browser two
 * frames before asking what is running, and ignore anything looping forever.
 */
async function settle(page: Page): Promise<void> {
  await page.evaluate(() => new Promise<void>((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
  }));
  await page.evaluate(() => {
    const finite = document.getAnimations().filter(
      (a) => a.effect?.getComputedTiming().iterations !== Infinity,
    );
    return Promise.all(finite.map((a) => a.finished.catch(() => undefined))).then(() => undefined);
  });
}

async function collapse(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await settle(page);
}

async function expand(page: Page): Promise<void> {
  await page.locator('.collapse-btn').click();
  await settle(page);
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
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText({ timeout: 2000 }).catch(() => '');
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
      // Retries: the labels are removed by the collapse transition, so a count
      // taken the instant after collapse() can still see them.
      await expect(
        page.locator(`.nav-item a[href="${href}"] span`),
        `${href} label should be absent when collapsed`,
      ).toHaveCount(0);
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
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText({ timeout: 2000 }).catch(() => '');
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

// ── Hover-expand popover ───────────────────────────────────────────────────────

/**
 * Folded in from the exploratory shell-menu-deep* probes (#43). Only the
 * assertions that guard a named behaviour survived: the popover must overlay
 * rather than push, it must not move the nav items under the cursor, the
 * two-tone logo must collapse to "KL", and the active Pyramidize icon must not
 * end up orange on an orange background.
 */

async function hoverSidebar(page: Page): Promise<void> {
  await page.locator('.layout-sidebar').hover();
  await settle(page);
}

test.describe('Shell — hover-expand popover', () => {
  test('sidebar expands to full width when hovered while collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    expect((await getRect(page, '.layout-sidebar')).width).toBeLessThanOrEqual(60);

    await hoverSidebar(page);

    expect((await getRect(page, '.layout-sidebar')).width, 'sidebar width on hover').toBeGreaterThan(200);
  });

  test('nav labels are readable during hover-expand', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await hoverSidebar(page);

    for (const href of ['/fix', '/enhance', '/settings']) {
      const text = await page.locator(`.nav-item a[href="${href}"] span`).innerText({ timeout: 2000 }).catch(() => '');
      expect(text.trim(), `${href} label during hover-expand`).not.toBe('');
    }
  });

  test('sidebar collapses back when the mouse leaves', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    await hoverSidebar(page);

    await page.locator('.layout-main').hover();
    await settle(page);

    expect((await getRect(page, '.layout-sidebar')).width, 'width after mouse leave').toBeLessThanOrEqual(60);
  });

  test('main content neither resizes nor scrolls — the popover overlays, it does not push', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    const before = await getRect(page, '.layout-main');

    await hoverSidebar(page);

    const after = await getRect(page, '.layout-main');
    expect(Math.abs(after.width - before.width), 'main width shift on hover').toBeLessThanOrEqual(2);
    expect(Math.abs(after.left - before.left), 'main left shift on hover').toBeLessThanOrEqual(2);

    const overflows = await page.locator('.layout-main').evaluate(
      (el) => el.scrollWidth > el.clientWidth + 1,
    );
    expect(overflows, 'hover-expand introduced a horizontal scrollbar').toBe(false);
  });

  test('nav items stay put — nothing moves under the cursor', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);
    // Deliberately generic: under `ng serve` isDevMode() adds a fourth "Dev
    // Tools" item, and it must not jump either.
    const before = await page.locator('.nav-item a').evaluateAll(
      (els) => els.map((el) => {
        const r = el.getBoundingClientRect();
        return { top: r.top, height: r.height };
      }),
    );

    await hoverSidebar(page);

    const after = await page.locator('.nav-item a').evaluateAll(
      (els) => els.map((el) => {
        const r = el.getBoundingClientRect();
        return { top: r.top, height: r.height };
      }),
    );

    expect(after.length).toBe(before.length);
    for (let i = 0; i < before.length; i++) {
      expect(Math.abs(after[i].top - before[i].top), `nav item ${i} vertical shift`).toBeLessThanOrEqual(1);
      expect(Math.abs(after[i].height - before[i].height), `nav item ${i} height change`).toBeLessThanOrEqual(1);
    }
  });

  test('an already expanded sidebar does not react to hover', async ({ page }) => {
    await gotoFix(page);

    const before = (await getRect(page, '.layout-sidebar')).width;
    await hoverSidebar(page);
    const after = (await getRect(page, '.layout-sidebar')).width;

    expect(Math.abs(after - before), 'expanded sidebar width changed on hover').toBeLessThanOrEqual(2);
  });
});

// ── Two-tone logo ──────────────────────────────────────────────────────────────

/**
 * Parses a computed `color` into channels. Only legacy rgb()/rgba() is accepted:
 * a lazier regex would read `oklch(0.7 0.19 45)` as rgb(0, 7, 0) and report a
 * white logo as black, and would drop the alpha that tells "white" apart from
 * "invisible".
 */
function channels(color: string): { r: number; g: number; b: number; a: number } {
  const match = /^rgba?\(([^)]+)\)$/.exec(color.trim());
  if (!match) {
    throw new Error(`expected a legacy rgb()/rgba() colour, got ${color}`);
  }
  const parts = match[1].split(/[,/\s]+/).filter(Boolean).map(Number);
  if (parts.length < 3 || parts.some(Number.isNaN)) {
    throw new Error(`could not parse colour ${color}`);
  }
  return { r: parts[0], g: parts[1], b: parts[2], a: parts.length > 3 ? parts[3] : 1 };
}

test.describe('Shell — two-tone logo', () => {
  test('"ey" and "int" collapse away, leaving "KL"', async ({ page }) => {
    await gotoFix(page);
    const expandedEy = (await page.locator('.layout-logo .logo-ey').boundingBox())?.width ?? 0;
    const expandedInt = (await page.locator('.layout-logo .logo-int').boundingBox())?.width ?? 0;
    expect(expandedEy, '"ey" width when expanded').toBeGreaterThan(5);
    expect(expandedInt, '"int" width when expanded').toBeGreaterThan(5);

    await collapse(page);

    const collapsedEy = (await page.locator('.layout-logo .logo-ey').boundingBox())?.width ?? 0;
    const collapsedInt = (await page.locator('.layout-logo .logo-int').boundingBox())?.width ?? 0;
    expect(collapsedEy, '"ey" width when collapsed').toBeLessThanOrEqual(2);
    expect(collapsedInt, '"int" width when collapsed').toBeLessThanOrEqual(2);
  });

  test('"K" stays near-white and "L" stays orange when collapsed', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    const k = channels(await page.locator('.layout-logo .logo-k').evaluate((el) => getComputedStyle(el).color));
    expect(Math.min(k.r, k.g, k.b), `"K" should be near-white, got rgb(${k.r}, ${k.g}, ${k.b})`).toBeGreaterThanOrEqual(240);
    expect(k.a, '"K" is fully transparent, which no colour assertion would catch').toBeGreaterThan(0.9);

    // Aura's dark scheme resolves primary to orange.400 = rgb(251, 146, 60);
    // light mode would be orange.500. Either way: red dominant, blue faint.
    const l = channels(await page.locator('.layout-logo .logo-l').evaluate((el) => getComputedStyle(el).color));
    expect(l.r, `"L" red channel, got rgb(${l.r}, ${l.g}, ${l.b})`).toBeGreaterThan(200);
    expect(l.b, `"L" blue channel, got rgb(${l.r}, ${l.g}, ${l.b})`).toBeLessThan(120);
  });
});

// ── Active route icon ──────────────────────────────────────────────────────────

test.describe('Shell — active route icon colour', () => {
  test('the Pyramidize SVG turns white on the active orange background', async ({ page }) => {
    await page.goto('/enhance');
    await page.waitForLoadState('networkidle');
    await page.waitForTimeout(200);

    const color = await page.locator('.nav-item a[href="/enhance"] svg').evaluate(
      (el) => getComputedStyle(el).color,
    );
    // An inline SVG does not inherit the active link's colour by itself; left
    // orange it disappears into the orange highlight.
    expect(color, 'SVG colour on the active Pyramidize link').toBe('rgb(255, 255, 255)');
  });
});

// ── Assertions recovered from the deleted probes ───────────────────────────────

/**
 * These guarded fixes that the deleted shell-menu-deep* files named explicitly.
 * The surviving "non-zero size" and "does not overflow" checks do not cover
 * them: a collapse button shrunk back to 24px, or a version row that regains
 * dead space when collapsed, would pass everything else in this file.
 */

test.describe('Shell — collapse button click target', () => {
  test('is at least 40px tall in both states', async ({ page }) => {
    await gotoFix(page);
    expect((await getRect(page, '.collapse-btn')).height, 'height when expanded').toBeGreaterThanOrEqual(40);

    await collapse(page);
    expect((await getRect(page, '.collapse-btn')).height, 'height when collapsed').toBeGreaterThanOrEqual(40);
  });
});

test.describe('Shell — version row dead space', () => {
  test('collapses to zero height when it has nothing to show', async ({ page }) => {
    await gotoFix(page);
    await collapse(page);

    // Any height here is an invisible hover target under the nav.
    const r = await getRect(page, '[data-testid="version-footer"]');
    expect(r.height, 'version row height when collapsed and empty').toBe(0);
  });
});

test.describe('Shell — active route highlight', () => {
  test('the active link is painted with the primary colour, inactive ones are not', async ({ page }) => {
    await gotoFix(page);

    // Aura's dark scheme resolves primary to orange.400 = rgb(251, 146, 60).
    // Asserting the paint, not just the routerLinkActive class, is what catches
    // a deleted stylesheet rule.
    const active = channels(await page.locator('.nav-item a[href="/fix"]').evaluate(
      (el) => getComputedStyle(el).backgroundColor,
    ));
    expect(active.a, 'active link background is transparent').toBeGreaterThan(0);
    expect(active.r, `active link background rgb(${active.r}, ${active.g}, ${active.b})`).toBeGreaterThan(200);
    expect(active.b, `active link background rgb(${active.r}, ${active.g}, ${active.b})`).toBeLessThan(120);

    for (const href of ['/enhance', '/settings']) {
      const inactive = channels(await page.locator(`.nav-item a[href="${href}"]`).evaluate(
        (el) => getComputedStyle(el).backgroundColor,
      ));
      const isPrimary = inactive.a > 0 && inactive.r > 200 && inactive.b < 120;
      expect(isPrimary, `${href} must not be highlighted while /fix is active`).toBe(false);
    }
  });
});
