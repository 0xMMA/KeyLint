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
import { installFakeBackend, fakeState, ANTHROPIC_MODELS, OPENROUTER_MODELS } from './support/fake-backend';

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
    await page.getByTestId('model-pyramidize-claude').getByRole('button', { name: 'Show options' }).click();

    const alias = page.getByTestId('model-option-sonnet');
    await expect(alias).toBeVisible();
    await expect(alias).toHaveAttribute('role', 'option');
    const name = alias.getByText('Sonnet (latest)', { exact: true });
    const id = alias.getByText('claude-sonnet-5-5', { exact: true });
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

  // OpenRouter (roadmap E10): its catalogue is public, so the picker is the
  // real one before a key is set — which must not read as "the key works" —
  // and at ~450 entries it is searched, not scrolled.
  test('OpenRouter: the catalogue is there before the key, and typing searches it', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'openrouter',
      modelSources: { openrouter: 'live' },
      modelLists: { openrouter: OPENROUTER_MODELS },
    });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    await expect(page.getByTestId('active-provider-name')).toHaveText('OpenRouter');
    await expect(page.getByTestId('provider-not-ready-openrouter')).toContainText('no API key is set');
    await expect(page.getByTestId('models-note-openrouter')).toContainText("OpenRouter's whole catalogue");
    await expect(page.getByTestId('models-note-openrouter')).toContainText('your OpenRouter account');
    await expect(page.getByTestId('model-fix-openrouter').locator('input')).toHaveAttribute('placeholder', 'Default: Anthropic: Claude Haiku 5.5');
    await expect(page.getByTestId('model-pyramidize-openrouter').locator('input')).toHaveAttribute('placeholder', 'Default: Anthropic: Claude Sonnet 5.5');
    await expect(page.getByTestId('fix-fast-note')).toContainText('Anthropic: Claude Haiku 5.5');
    // An explicit ID is not an alias: no "Now <model>" line under it.
    await expect(page.getByTestId('model-effective-fix')).toHaveCount(0);

    // A search given up on changes nothing: the field goes back to the default.
    const pyramidize = page.getByTestId('model-pyramidize-openrouter').locator('input');
    await pyramidize.click();
    await pyramidize.pressSequentially('gemini');
    await expect(page.getByRole('option')).toHaveCount(1);
    await page.getByTestId('active-provider-name').click();
    await expect(page.getByRole('listbox')).toHaveCount(0);
    await expect(pyramidize).toHaveValue('');

    // Typing narrows the list to what matches every word, and a click takes it.
    await pyramidize.click();
    await pyramidize.pressSequentially('gpt luna');
    await expect(page.getByRole('option')).toHaveCount(1);
    await expect(page.getByRole('option')).toContainText('OpenAI: GPT-6 Luna');
    await page.screenshot({ path: 'e2e/screenshots/providers-openrouter-search.png' });
    await page.getByRole('option').click();
    await expect(pyramidize).toHaveValue('openai/gpt-6-luna');

    // The key goes in like any other, and the warning goes with it.
    await page.getByTestId('key-edit-openrouter').getByRole('button').click();
    await page.getByTestId('key-input-openrouter').fill('sk-or-v1-test');
    await page.getByTestId('key-save-openrouter').getByRole('button').click();
    await expect(page.getByTestId('key-status-openrouter')).toHaveText('Key saved in your system keyring');
    await expect(page.getByTestId('provider-not-ready-openrouter')).toHaveCount(0);

    await page.getByTestId('save-btn').getByRole('button').click();
    await expect.poll(() => (state.saves.at(-1)?.['models'] as Record<string, Record<string, string>> | undefined)?.['openrouter']?.['pyramidize'])
      .toBe('openai/gpt-6-luna');
    await page.screenshot({ path: 'e2e/screenshots/providers-openrouter.png', fullPage: true });
  });

  test('OpenRouter can be chosen with one click, like any provider', async ({ page }) => {
    const state = fakeState({
      activeProvider: 'claude',
      keys: { claude: 'keyring', openrouter: 'keyring' },
      modelSources: { claude: 'live', openrouter: 'live' },
      modelLists: { claude: ANTHROPIC_MODELS, openrouter: OPENROUTER_MODELS },
    });
    await installFakeBackend(page, state);

    await page.goto('/settings?tab=providers');
    // A user on another provider sends openrouter.ai nothing, key or not.
    await expect.poll(() => state.listModelsCalls).toContain('claude');
    expect(state.listModelsCalls).not.toContain('openrouter');

    await page.getByTestId('provider-use-openrouter').getByRole('button').click();
    await expect(page.getByTestId('active-provider-name')).toHaveText('OpenRouter');
    expect(state.settings['active_provider']).toBe('openrouter');
    // Chosen: now the catalogue is asked for, and the picker names its default.
    await expect.poll(() => state.listModelsCalls).toContain('openrouter');
    await expect(page.getByTestId('model-fix-openrouter').locator('input')).toHaveAttribute('placeholder', 'Default: Anthropic: Claude Haiku 5.5');
  });
});
