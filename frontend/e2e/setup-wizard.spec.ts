/**
 * The setup wizard, end to end against the fake backend.
 *
 * The owner met the old wizard after a reinstall with an Anthropic key in the
 * keyring and the Claude Code CLI signed in, and it would not let him past the
 * key field: "das ist so ne art von bevormundung". The backend now skips the
 * wizard for anyone with something usable (welcome.Service.IsFirstRun, tested
 * in Go). These cover the wizard itself for those who still see it — what
 * already works is preselected, a stored key is never asked for again, and
 * "Set up later" always gets you into the app.
 */
import { test, expect } from '@playwright/test';
import { installFakeBackend, fakeState, FakeBackendState } from './support/fake-backend';

function newUser(opts: Parameters<typeof fakeState>[0]): FakeBackendState {
  const state = fakeState(opts);
  state.settings = { ...state.settings, completed_setup: false };
  state.firstRun = true;
  return state;
}

test.describe('Setup wizard', () => {
  test('a new user picks a provider, pastes a key and lands in the app', async ({ page }) => {
    const state = newUser({ activeProvider: 'openai' });
    await installFakeBackend(page, state);

    await page.goto('/');
    await expect(page).toHaveURL(/\/welcome$/);
    await expect(page.getByTestId('wizard-welcome')).toBeVisible();
    await expect(page.locator('.keycap')).toHaveText(['Ctrl', 'G']);
    await page.screenshot({ path: 'e2e/screenshots/wizard-welcome.png', fullPage: true });

    await page.getByTestId('wizard-next').getByRole('button').click();
    await expect(page.getByTestId('wizard-provider')).toBeVisible();
    // Nothing works yet, so nothing is assumed.
    await expect(page.getByRole('radio', { checked: true })).toHaveCount(0);
    await expect(page.getByTestId('wizard-finish').getByRole('button')).toBeDisabled();

    await page.getByTestId('wizard-radio-claude').check();
    await page.getByTestId('wizard-key-input').fill('sk-ant-test');
    await page.screenshot({ path: 'e2e/screenshots/wizard-key.png', fullPage: true });
    await page.getByTestId('wizard-finish').getByRole('button').click();

    await expect(page).toHaveURL(/\/fix$/);
    expect(state.settings['active_provider']).toBe('claude');
    expect(state.keys['claude']).toBe('keyring');
    expect(state.completeSetupCalls).toBe(1);
  });

  test('a signed-in CLI is preselected and needs no key', async ({ page }) => {
    const state = newUser({ activeProvider: 'openai', cliSignedIn: true });
    await installFakeBackend(page, state);

    await page.goto('/');
    await page.getByTestId('wizard-next').getByRole('button').click();

    await expect(page.getByTestId('wizard-radio-claude-code')).toBeChecked();
    await expect(page.getByTestId('wizard-status-claude-code')).toContainText('signed in');
    await expect(page.getByTestId('wizard-key-input')).toHaveCount(0);
    await page.screenshot({ path: 'e2e/screenshots/wizard-cli.png', fullPage: true });

    await page.getByTestId('wizard-finish').getByRole('button').click();
    await expect(page).toHaveURL(/\/fix$/);
    expect(state.settings['active_provider']).toBe('claude-code');
    expect(state.keys).toEqual({});
  });

  test('a stored key is recognised, not asked for again', async ({ page }) => {
    const state = newUser({ activeProvider: 'openai', keys: { claude: 'keyring' } });
    await installFakeBackend(page, state);

    await page.goto('/');
    await page.getByTestId('wizard-next').getByRole('button').click();

    await expect(page.getByTestId('wizard-radio-claude')).toBeChecked();
    await expect(page.getByTestId('wizard-key-stored')).toContainText('already saved');
    await expect(page.getByTestId('wizard-finish').getByRole('button')).toBeEnabled();

    await page.getByTestId('wizard-finish').getByRole('button').click();
    await expect(page).toHaveURL(/\/fix$/);
    expect(state.settings['active_provider']).toBe('claude');
  });

  test('Set up later gets into the app without choosing anything', async ({ page }) => {
    const state = newUser({ activeProvider: 'openai' });
    await installFakeBackend(page, state);

    await page.goto('/');
    await page.getByTestId('wizard-skip').getByRole('button').click();

    await expect(page).toHaveURL(/\/fix$/);
    expect(state.completeSetupCalls).toBe(1);
    // Nothing chosen on the user's behalf.
    expect(state.saves).toEqual([]);
  });
});
