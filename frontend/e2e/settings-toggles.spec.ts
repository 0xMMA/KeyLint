/**
 * Toggle switch sizing on Settings › General (#26).
 *
 * A toggle row with a long hint beside the switch used to squeeze the switch
 * narrower than its track, so the knob slid past the edge. jsdom applies no
 * layout, so only a real browser can catch this.
 */
import { test, expect } from '@playwright/test';

test.describe('Settings — toggle switches', () => {
  // Narrow enough that the Sensitive Logging hint wraps and pushes hardest.
  test.use({ viewport: { width: 600, height: 900 } });

  test('every toggle switch keeps its full width beside a long hint', async ({ page }) => {
    await page.goto('/settings');
    await page.waitForLoadState('networkidle');
    await expect(page.locator('[data-testid="sensitive-logging-section"] .p-toggleswitch')).toBeVisible();

    const widths = await page.locator('.toggle-row .p-toggleswitch').evaluateAll((els) =>
      els.map((el) => el.getBoundingClientRect().width),
    );
    console.log(`toggle widths: ${widths.join(', ')}`);
    expect(widths.length, 'no toggle rows found').toBeGreaterThanOrEqual(2);
    // Aura's switch is 2.5rem wide.
    for (const w of widths) expect(w).toBe(40);
  });
});
