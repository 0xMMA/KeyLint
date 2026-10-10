/**
 * Settings › About: unlocking the developer options by tapping the version.
 *
 * Runs against the fake settings backend, so the unlock is seen to be saved
 * and to survive a reload. The updater service has no fake: its calls 404 as
 * they always do under `ng serve`, which makes this also the check that a dev
 * channel with no backend to ask shows a message rather than a broken screen.
 */
import { test, expect } from '@playwright/test';
import { installFakeBackend, fakeState } from './support/fake-backend';

test.describe('Settings — developer options', () => {
  test('seven taps on the version unlock the dev channel, saved at once; the switch hides it again', async ({ page }) => {
    const state = fakeState({ activeProvider: 'claude', keys: { claude: 'keyring' } });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=about');
    const version = page.getByTestId('version-tap');
    await expect(version).toBeVisible();
    await expect(page.getByTestId('dev-channel')).toHaveCount(0);

    for (let i = 0; i < 6; i++) await version.click();
    await expect(page.getByTestId('unlock-hint')).toHaveText('1 more tap to turn on the developer options.');
    await expect(page.getByTestId('dev-channel')).toHaveCount(0);

    await version.click();
    await expect(page.getByTestId('unlock-hint')).toHaveText('Developer options are on.');
    await expect(page.getByTestId('dev-channel')).toBeVisible();
    await expect(page.getByTestId('dev-channel-error')).toBeVisible();
    expect(state.saves.at(-1)?.['developer_options']).toBe(true);

    // Saved, not just shown.
    await page.reload();
    await expect(page.getByTestId('dev-channel')).toBeVisible();

    await page.getByTestId('developer-options-toggle').click();
    await expect(page.getByTestId('dev-channel')).toHaveCount(0);
    await expect(page.getByTestId('developer-options-section')).toHaveCount(0);
    await expect.poll(() => state.saves.at(-1)?.['developer_options']).toBe(false);
  });
});
