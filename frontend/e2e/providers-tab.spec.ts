/**
 * Settings › AI Providers after the redesign: the provider in use on top with
 * its models and effort, every connection below.
 *
 * The owner's words about the old page: "sehr unübersichtlich und macht
 * mehrere Dinge gleichzeitig" — it listed every provider's model pickers at
 * once. This runs the new layout in a real browser against the fake settings
 * backend, including the dropdown whose name and ID used to render jammed
 * together on one line.
 */
import { test, expect } from '@playwright/test';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS } from './support/fake-backend';

test.describe('Settings — AI Providers layout', () => {
  test('shows only the provider in use, with its models and effort', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'claude',
      cliSignedIn: true,
      keys: { claude: 'keyring' },
      modelSources: { claude: 'live' },
      modelLists: { claude: ANTHROPIC_MODELS },
    });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    const inUse = page.getByTestId('active-provider');
    await expect(inUse).toBeVisible();
    await expect(page.getByTestId('active-provider-name')).toHaveText('Anthropic API');

    // One provider's pickers, not four.
    await expect(page.locator('[data-testid^="model-fix-"]')).toHaveCount(1);
    await expect(page.getByTestId('effort-fix-claude')).toBeVisible();
    await expect(page.getByTestId('effort-pyramidize-claude')).toBeVisible();

    // The defaults say what they are today.
    await expect(page.getByTestId('model-fix-claude').locator('input')).toHaveAttribute('placeholder', 'Default: Haiku (latest)');
    await expect(page.getByTestId('model-effective-fix')).toHaveText('Now claude-haiku-5-5');
    await expect(page.getByTestId('model-effective-pyramidize')).toHaveText('Now claude-sonnet-5-5');
    await expect(page.getByTestId('fix-fast-note')).toBeVisible();

    await page.screenshot({ path: 'e2e/screenshots/providers-tab.png', fullPage: true });
  });

  test('lists name and ID on separate lines in the model dropdown', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'claude',
      keys: { claude: 'keyring' },
      modelSources: { claude: 'live' },
      modelLists: { claude: ANTHROPIC_MODELS },
    });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await page.getByTestId('model-pyramidize-claude').locator('.p-select-dropdown').click();

    const alias = page.getByTestId('model-option-sonnet');
    await expect(alias).toBeVisible();
    const name = alias.locator('.model-option-name');
    const id = alias.locator('.model-option-id');
    await expect(name).toHaveText('Sonnet (latest)');
    await expect(id).toHaveText('claude-sonnet-5-5');
    // Stacked: the ID starts below the name, not beside it.
    const nameBox = (await name.boundingBox())!;
    const idBox = (await id.boundingBox())!;
    expect(idBox.y).toBeGreaterThanOrEqual(nameBox.y + nameBox.height - 1);
    expect(Math.abs(idBox.x - nameBox.x)).toBeLessThan(2);

    await page.screenshot({ path: 'e2e/screenshots/providers-model-dropdown.png' });
  });

  test('switching the provider in use swaps the top block and keeps each choice', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'claude',
      cliSignedIn: true,
      keys: { claude: 'keyring' },
      modelSources: { claude: 'live' },
      modelLists: { claude: ANTHROPIC_MODELS },
    });
    state.settings['models'] = {
      claude: { fix: 'claude-haiku-4-5-20251001', pyramidize: '', fix_effort: '', pyramidize_effort: 'high' },
      'claude-code': { fix: 'opus', pyramidize: '', fix_effort: '', pyramidize_effort: '' },
    };
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('model-fix-claude').locator('input')).toHaveValue('claude-haiku-4-5-20251001');
    await expect(page.getByTestId('effort-pyramidize-claude')).toContainText('High');

    await page.getByTestId('provider-use-claude-code').getByRole('button').click();
    await expect(page.getByTestId('active-provider-name')).toHaveText('Claude Code (installed CLI)');
    await expect(page.getByTestId('model-fix-claude-code')).toContainText('Opus (latest)');
    await expect(page.getByTestId('model-fix-claude')).toHaveCount(0);
    await page.screenshot({ path: 'e2e/screenshots/providers-tab-cli.png', fullPage: true });

    await page.getByTestId('provider-use-claude').getByRole('button').click();
    await expect(page.getByTestId('model-fix-claude').locator('input')).toHaveValue('claude-haiku-4-5-20251001');
    await expect(page.getByTestId('effort-pyramidize-claude')).toContainText('High');
  });

  test('an effort chosen here is saved with Save', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'claude',
      keys: { claude: 'keyring' },
      modelSources: { claude: 'live' },
      modelLists: { claude: ANTHROPIC_MODELS },
    });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await page.getByTestId('effort-fix-claude').click();
    await page.getByRole('option', { name: 'Low' }).click();
    await expect(page.getByTestId('fix-fast-note')).toHaveCount(0);
    await page.getByTestId('save-btn').getByRole('button').click();

    await expect.poll(() => (state.saves.at(-1)?.['models'] as Record<string, Record<string, string>> | undefined)?.['claude']?.['fix_effort'])
      .toBe('low');
  });

  test("the General tab's pointer still leads here", async ({ page }) => {
    const state = fakeState({ activeProvider: 'claude', keys: { claude: 'keyring' } });
    await installFakeBackend(page, state);

    await page.goto('/settings');
    await expect(page.getByTestId('provider-pointer-text')).toContainText('Anthropic API');
    await page.getByTestId('provider-pointer-link').click();
    await expect(page.getByTestId('providers-summary')).toBeFocused();
    await expect(page.getByTestId('active-provider-name')).toHaveText('Anthropic API');
  });
});
