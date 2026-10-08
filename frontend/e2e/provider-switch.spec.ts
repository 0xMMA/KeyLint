/**
 * Settings › AI Providers: which provider KeyLint uses, and switching it.
 *
 * The owner's report: with both an Anthropic key and the Claude Code CLI set
 * up, nothing said which one was used. The unit specs cover the logic; this
 * runs it in a real browser against a fake settings backend, so the switch is
 * seen to survive a reload rather than only to move a marker.
 */
import { test, expect, Page } from '@playwright/test';
import { installFakeBackend, fakeState } from './support/fake-backend';

/** The cards currently marked "In use", by provider ID. */
async function inUse(page: Page): Promise<string[]> {
  const ids = await page.locator('[data-testid^="provider-in-use-"]:visible').evaluateAll(els =>
    els.map(e => e.getAttribute('data-testid')!.replace('provider-in-use-', '')));
  return ids;
}

test.describe('Settings — provider in use', () => {
  test('both configured: the marker names one, and one click moves it for good', async ({ page }) => {
    const state = fakeState({ activeProvider: 'claude-code', cliSignedIn: true, keys: { claude: 'keyring' } });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('provider-in-use-claude-code')).toBeVisible();
    expect(await inUse(page)).toEqual(['claude-code']);
    await expect(page.getByTestId('providers-summary')).toContainText('Claude Code (installed CLI)');

    await page.getByTestId('provider-use-claude').getByRole('button', { name: /^Use this/ }).click();

    await expect(page.getByTestId('provider-in-use-claude')).toBeVisible();
    expect(await inUse(page)).toEqual(['claude']);
    // The marker is shown once the backend answered; the record is the fake's.
    await expect.poll(() => state.saves.at(-1)?.['active_provider']).toBe('claude');

    // Saved, not just shown: a fresh load reads it back from the backend.
    await page.reload();
    await expect(page.getByTestId('provider-in-use-claude')).toBeVisible();
    expect(await inUse(page)).toEqual(['claude']);
  });

  test('a provider that cannot work yet can be chosen, and says why', async ({ page }) => {
    // No Ollama daemon answers in this test, which is the case being shown.
    const state = fakeState({ activeProvider: 'claude', keys: { claude: 'keyring' }, modelSources: { ollama: 'unreachable' } });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('provider-in-use-claude')).toBeVisible();

    await page.getByTestId('provider-use-ollama').getByRole('button', { name: /^Use this/ }).click();

    await expect(page.getByTestId('provider-in-use-ollama')).toBeVisible();
    await expect(page.getByTestId('provider-not-ready-ollama')).toContainText('cannot reach Ollama');
    // The marker is shown once the backend answered; the record is the fake's.
    await expect.poll(() => state.saves.at(-1)?.['active_provider']).toBe('ollama');
  });

  test('Pyramidize follows a switch made in Settings', async ({ page }) => {
    const state = fakeState({ activeProvider: 'claude', cliSignedIn: true, keys: { claude: 'keyring' } });
    await installFakeBackend(page, state);

    await page.goto('/enhance');
    await expect(page.getByTestId('provider-select')).toContainText('Anthropic API');

    await page.locator('a[routerLink="/settings"], a[href="/settings"]').first().click();
    await page.getByRole('tab', { name: 'AI Providers' }).click();
    await page.getByTestId('provider-use-claude-code').getByRole('button').click();
    await expect(page.getByTestId('provider-in-use-claude-code')).toBeVisible();
    await expect.poll(() => state.saves.at(-1)?.['active_provider']).toBe('claude-code');

    await page.locator('a[routerLink="/enhance"], a[href="/enhance"]').first().click();
    await expect(page.getByTestId('provider-select')).toContainText('Claude Code (installed CLI)');
    await expect(page.getByTestId('provider-session-override')).toHaveCount(0);
  });

  test('a switch the backend refuses leaves the marker where it was', async ({ page }) => {
    const state = fakeState({ activeProvider: 'claude-code', cliSignedIn: true, keys: { claude: 'keyring' } });
    state.failSetActiveProvider = 'settings.json is locked';
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('provider-in-use-claude-code')).toBeVisible();
    await page.getByTestId('provider-use-claude').getByRole('button', { name: /^Use this/ }).click();

    await expect(page.getByTestId('provider-switch-error')).toContainText('settings.json is locked');
    expect(await inUse(page)).toEqual(['claude-code']);
    await expect(page.getByTestId('provider-heading-claude-code')).toBeFocused();
    expect(state.saves).toEqual([]);
  });

  test('the General tab names the provider and points to where it is changed', async ({ page }) => {
    await installFakeBackend(page, fakeState({ activeProvider: 'claude', cliSignedIn: true, keys: { claude: 'keyring' } }));

    await page.goto('/settings');
    await expect(page.getByTestId('provider-pointer-text')).toContainText('Using: Anthropic API');
    await expect(page.getByTestId('active-provider-select')).toHaveCount(0);

    await page.getByTestId('provider-pointer-link').click();
    await expect(page.getByRole('tab', { name: 'AI Providers' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByTestId('provider-in-use-claude')).toBeVisible();
  });
});
