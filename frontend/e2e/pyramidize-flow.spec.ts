/**
 * Pyramidize past its empty state, in real Chromium, without a provider: the
 * settings and Pyramidize services are faked (support/fake-backend.ts,
 * support/fake-pyramidize.ts), so every state of the page can be reached —
 * Text, Form, the working step, the result, the history, an error, a provider
 * that cannot run, and text brought by the shortcut. Each test also leaves a
 * screenshot in e2e/screenshots/.
 */
import { test, expect, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { ANTHROPIC_MODELS, fakeState, installFakeBackend } from './support/fake-backend';
import { fakePyramidize, installFakePyramidize, pressPyramidizeShortcut, type FakePyramidizeState } from './support/fake-pyramidize';

const SCREENSHOT_DIR = path.join(__dirname, 'screenshots');

async function screenshot(page: Page, name: string): Promise<void> {
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
  await page.screenshot({ path: path.join(SCREENSHOT_DIR, `pyramidize-${name}.png`) });
}

const ORIGINAL = `hi sarah, thanks for sending the numbers so quick. two things: travel is 12% over budget for q3 already and we should talk about it in the meeting tomorrow. also the august vendor invoices arent in the totals, i think they got booked to the wrong cost center. can you check with finance? best, jonas`;

const DOCUMENT = `# Q3 travel is 12% over budget, and August invoices are missing from the totals

Hi Sarah,

**Two things need a decision before tomorrow's meeting.**

1. **Travel is already 12% over the Q3 budget.** We should agree in the meeting whether to freeze bookings or move budget.
2. **The August vendor invoices are not in the totals.** They were probably booked to the wrong cost center. Could you check with finance?

Thanks for sending the numbers so quickly. Happy to walk through them together if that's easier.

Best,
Jonas`;

const OUTLOOK = { sourceApp: 'Inbox - Outlook', sourcePreset: { app: 'Outlook', documentType: 'email', remembered: false } };

async function open(page: Page, pyramidize: Partial<FakePyramidizeState> = {}, activeProvider = 'claude'): Promise<FakePyramidizeState> {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installFakeBackend(page, fakeState({
    activeProvider,
    keys: { claude: 'keyring' },
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
  const state = fakePyramidize({ fullDocument: DOCUMENT, clipboard: ORIGINAL, ...pyramidize });
  await installFakePyramidize(page, state);
  await page.goto('/enhance');
  await expect(page.getByTestId('original-textarea')).toBeVisible();
  await page.waitForLoadState('networkidle');
  return state;
}

const radio = (page: Page, name: string) => page.getByRole('radiogroup', { name: 'Form' }).getByRole('radio', { name: new RegExp(`^\\d ${name}\\b`) });

async function toForm(page: Page): Promise<void> {
  await page.getByTestId('original-textarea').fill(ORIGINAL);
  await page.getByTestId('choose-form-btn').getByRole('button').click();
  await expect(page.getByRole('radiogroup', { name: 'Form' })).toBeVisible();
}

async function pyramidizeOnce(page: Page): Promise<void> {
  await toForm(page);
  await page.getByTestId('pyramidize-btn').getByRole('button').click();
  await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
}

test.describe('Pyramidize — flow', () => {
  test('opens on the Text step, with paste offered and the next step waiting for text', async ({ page }) => {
    await open(page);
    await expect(page.getByRole('heading', { name: 'Your text' })).toBeVisible();
    await expect(page.getByTestId('paste-from-clipboard-btn')).toBeVisible();
    await expect(page.getByTestId('choose-form-btn').getByRole('button')).toBeDisabled();
    await expect(page.getByTestId('step-choice')).toBeDisabled();
    await screenshot(page, '01-text');
  });

  test('pastes from the clipboard and goes on with Enter', async ({ page }) => {
    await open(page);
    await page.getByTestId('paste-from-clipboard-btn').getByRole('button').click();
    await expect(page.getByTestId('original-textarea')).toHaveValue(ORIGINAL);
    // Enter outside the text (in it, Enter is a new line).
    await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
    await page.keyboard.press('Enter');
    await expect(page.getByRole('heading', { name: 'What should it become?' })).toBeVisible();
  });

  test('offers the five forms as large buttons, email first and picked, the run line from Settings', async ({ page }) => {
    await open(page);
    await toForm(page);
    await expect(page.getByRole('radiogroup', { name: 'Form' }).getByRole('radio')).toHaveCount(5);
    for (const name of ['Email', 'Chat', 'Memo', 'Wiki', 'Presentation']) await expect(radio(page, name)).toBeVisible();
    await expect(radio(page, 'Email')).toHaveAttribute('aria-checked', 'true');
    await expect(page.getByTestId('run-settings')).toContainText('Sonnet (latest) · Anthropic API');
    await expect(page.getByTestId('remember-checkbox')).toHaveCount(0);
    await expect(page.getByLabel('Instructions (optional)')).toHaveAttribute('placeholder', 'e.g. more formal · in English · for the whole department');
    // A form card is large: a target, not a list row.
    const box = (await radio(page, 'Chat').boundingBox())!;
    expect(box.height).toBeGreaterThanOrEqual(150);
    await screenshot(page, '02-form');
  });

  test('picks with 1–5, runs with Enter, and not while typing digits in the instructions', async ({ page }) => {
    const state = await open(page);
    await toForm(page);
    await page.keyboard.press('3');
    await expect(radio(page, 'Memo')).toHaveAttribute('aria-checked', 'true');
    await page.getByLabel('Instructions (optional)').fill('');
    await page.getByLabel('Instructions (optional)').pressSequentially('for 2 teams');
    await expect(page.getByLabel('Instructions (optional)')).toHaveValue('for 2 teams');
    await expect(radio(page, 'Memo')).toHaveAttribute('aria-checked', 'true');
    await page.getByLabel('Instructions (optional)').press('Enter');
    await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
    expect(state.requests[0]).toMatchObject({ text: ORIGINAL, documentType: 'memo', customInstructions: 'for 2 teams', provider: '' });
    expect(state.requests[0]).not.toHaveProperty('communicationStyle');
  });

  test('shows the working step, then the result on a sheet', async ({ page }) => {
    let release!: () => void;
    await open(page, { hold: new Promise<void>(r => (release = r)) });
    await toForm(page);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('step-label')).toHaveText('Writing the email…');
    await expect(page.getByTestId('cancel-btn')).toBeVisible();
    await screenshot(page, '03-working');

    release();
    await expect(page.getByTestId('step-indicator')).toHaveCount(0);
    await expect(page.getByTestId('result-title')).toHaveText('Email');
    await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
    // The user's text is set in Literata (docs/ui/design.md, "Type").
    const font = await page.getByTestId('canvas-textarea').evaluate(el => getComputedStyle(el).fontFamily);
    expect(font).toContain('Literata');
    await screenshot(page, '04-result-edit');

    await page.getByTestId('segment-preview').click();
    await expect(page.getByTestId('canvas-preview').locator('h1')).toHaveText(/Q3 travel/);
    // Numbered, despite Tailwind's preflight taking list markers off (theme.css).
    const list = page.getByTestId('canvas-preview').locator('ol');
    expect(await list.evaluate(el => getComputedStyle(el).listStyleType)).toBe('decimal');
    await screenshot(page, '05-result-preview');
  });

  test('Cancel goes back to the form choice', async ({ page }) => {
    await open(page, { hold: new Promise<void>(() => {}) });
    await toForm(page);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await page.getByTestId('cancel-btn').getByRole('button').click();
    await expect(page.getByRole('heading', { name: 'What should it become?' })).toBeVisible();
    await expect(page.getByTestId('step-result')).toBeDisabled();
  });

  test('tries another form and keeps the first result selectable', async ({ page }) => {
    const state = await open(page);
    await pyramidizeOnce(page);
    await page.getByTestId('canvas-textarea').fill('My edited email');

    await page.getByTestId('try-another-form-btn').getByRole('button').click();
    await radio(page, 'Chat').click();
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('result-title')).toHaveText('Chat');
    await expect(page.getByTestId('result-version')).toHaveText(['Email', 'Chat']);
    expect(state.requests.map(r => [r['text'], r['documentType']])).toEqual([[ORIGINAL, 'email'], [ORIGINAL, 'chat']]);
    await screenshot(page, '06-versions');

    await page.getByTestId('result-version').first().click();
    await expect(page.getByTestId('canvas-textarea')).toHaveValue('My edited email');
  });

  test('keeps a history to preview, pin and revert to', async ({ page }) => {
    await open(page);
    await pyramidizeOnce(page);

    await page.getByTestId('trace-log-toggle').getByRole('button').click();
    await expect(page.getByTestId('trace-log-panel')).toBeVisible();
    // The original, then the result.
    await expect(page.getByTestId('trace-entry')).toHaveCount(2);

    await page.getByTestId('canvas-textarea').fill('A shorter draft.');
    await page.getByTestId('add-checkpoint-btn').getByRole('button').click();
    await expect(page.getByTestId('trace-entry')).toHaveCount(3);
    await expect(page.getByTestId('trace-log-toggle')).toContainText('History, 3 steps');

    const result = page.getByTestId('trace-entry').nth(1);
    await result.hover();
    await expect(page.getByTestId('trace-peek-overlay')).toBeVisible();
    await expect(page.getByTestId('trace-peek-content')).toContainText('Q3 travel');
    await screenshot(page, '07-history-peek');

    await result.click();
    await expect(page.getByTestId('trace-peek-close')).toBeVisible();
    await page.getByTestId('trace-revert-btn').getByRole('button').click();
    await expect(page.getByTestId('trace-peek-overlay')).toHaveCount(0);
    await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
  });

  test('puts an instruction bubble on a raised surface over a selection', async ({ page }) => {
    await open(page);
    await pyramidizeOnce(page);

    const box = (await page.getByTestId('canvas-textarea').boundingBox())!;
    await page.mouse.move(box.x + 40, box.y + 40);
    await page.mouse.down();
    await page.mouse.move(box.x + 360, box.y + 40, { steps: 5 });
    await page.mouse.up();

    const bubble = page.getByTestId('selection-bubble');
    await expect(bubble).toBeVisible();
    const bg = await bubble.evaluate(el => getComputedStyle(el).backgroundColor);
    expect(bg).toBe('rgb(42, 42, 47)');
    await screenshot(page, '08-selection-bubble');

    await page.getByTestId('selection-instruction-input').press('Escape');
    await expect(bubble).toHaveCount(0);
  });

  test('shows a failure in the error row on the Form step, with Copy error and Retry', async ({ page }) => {
    await open(page, { failPyramidize: 'Anthropic rejected the key. Check it in Settings, then try again.' });
    await toForm(page);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();

    const row = page.getByTestId('error-row');
    await expect(row).toBeVisible();
    await expect(row).toContainText('Anthropic rejected the key');
    await expect(page.getByTestId('copy-error-btn')).toBeVisible();
    await expect(page.getByTestId('retry-btn')).toBeVisible();
    await expect(page.getByRole('radiogroup', { name: 'Form' })).toBeVisible();
    await screenshot(page, '09-error');
  });

  test('explains a saved provider that cannot run, and keeps Pyramidize off', async ({ page }) => {
    await open(page, {}, 'bedrock');
    await toForm(page);
    await expect(page.getByTestId('provider-unavailable')).toContainText('AWS Bedrock is not available yet');
    await expect(page.getByTestId('pyramidize-btn').getByRole('button')).toBeDisabled();
    await screenshot(page, '10-provider-unavailable');
  });

  test('brings text from an app with the shortcut, remembers its form, and sends the result back', async ({ page }) => {
    const state = await open(page, OUTLOOK);
    await pressPyramidizeShortcut(page);

    await expect(page.getByRole('heading', { name: 'What should it become?' })).toBeVisible();
    await expect(page.getByTestId('source-chip')).toHaveText('From Outlook');
    await expect(radio(page, 'Email')).toContainText('Default');
    await radio(page, 'Memo').click();
    await page.getByTestId('remember-checkbox').getByRole('checkbox').check();
    await screenshot(page, '11-remember');
    expect(state.presetsSet).toEqual([]);

    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
    expect(state.presetsSet).toEqual([{ sourceApp: 'Outlook', documentType: 'memo' }]);

    const send = page.getByTestId('send-back-btn');
    await expect(send).toHaveText(/Send back to Outlook/);
    await send.getByRole('button').click();
    await expect.poll(() => state.sentBack).toEqual([DOCUMENT]);

    await page.getByTestId('try-another-form-btn').getByRole('button').click();
    await expect(page.getByTestId('remembered-note')).toContainText('Memo is remembered for Outlook.');
    await page.getByTestId('forget-preset-btn').click();
    await expect.poll(() => state.presetsDeleted).toEqual(['Outlook']);
    await expect(page.getByTestId('remember-checkbox')).toContainText('Remember Memo for Outlook');
  });
});
