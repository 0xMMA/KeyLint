/**
 * Pyramidize past its empty state, in real Chromium, without a provider: the
 * settings and Pyramidize services are faked (support/fake-backend.ts,
 * support/fake-pyramidize.ts), so every state of the page can be reached —
 * the step while it works, the result, the history, an error, a provider that
 * cannot run. Each test also leaves a screenshot in e2e/screenshots/.
 */
import { test, expect, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { ANTHROPIC_MODELS, fakeState, installFakeBackend } from './support/fake-backend';
import { fakePyramidize, installFakePyramidize, type FakePyramidizeState } from './support/fake-pyramidize';

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

async function open(page: Page, pyramidize: Partial<FakePyramidizeState> = {}, activeProvider = 'claude'): Promise<FakePyramidizeState> {
  await page.setViewportSize({ width: 1280, height: 800 });
  await installFakeBackend(page, fakeState({
    activeProvider,
    keys: { claude: 'keyring' },
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
  const state = fakePyramidize({ fullDocument: DOCUMENT, ...pyramidize });
  await installFakePyramidize(page, state);
  await page.goto('/enhance');
  await expect(page.getByTestId('model-select')).toContainText('KeyLint default');
  return state;
}

async function pyramidizeOnce(page: Page): Promise<void> {
  await page.getByTestId('original-textarea').fill(ORIGINAL);
  await page.getByTestId('pyramidize-btn').getByRole('button').click();
  await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
}

test.describe('Pyramidize — flow', () => {
  test('starts empty, with paste offered and Pyramidize waiting for text', async ({ page }) => {
    await open(page);

    await expect(page.getByTestId('paste-from-clipboard-btn')).toBeVisible();
    await expect(page.getByTestId('pyramidize-btn').getByRole('button')).toBeDisabled();
    await expect(page.getByTestId('doc-type-select')).toContainText('Detect automatically');
    await expect(page.getByTestId('provider-select')).toContainText('Anthropic API');
    await screenshot(page, '01-empty');
  });

  test('pastes from the clipboard', async ({ page }) => {
    await open(page, { clipboard: ORIGINAL });

    await page.getByTestId('paste-from-clipboard-btn').getByRole('button').click();

    await expect(page.getByTestId('original-textarea')).toHaveValue(ORIGINAL);
    await expect(page.getByTestId('pyramidize-btn').getByRole('button')).toBeEnabled();
  });

  test('shows the step while it works, then the result on a sheet in the editor', async ({ page }) => {
    let release!: () => void;
    const state = await open(page, { hold: new Promise<void>(r => (release = r)) });

    await page.getByTestId('original-textarea').fill(ORIGINAL);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();
    await expect(page.getByTestId('step-label')).toContainText('Step 1/2');
    await expect(page.getByTestId('cancel-btn')).toBeVisible();
    await screenshot(page, '02-loading');

    release();
    await expect(page.getByTestId('step-indicator')).toHaveCount(0);
    await expect(page.getByRole('tab', { name: 'Editor' })).toHaveAttribute('aria-selected', 'true');
    await expect(page.getByTestId('canvas-textarea')).toHaveValue(DOCUMENT);
    await expect(page.getByTestId('detected-type')).toHaveText(/Detected:\s*Email/);
    await expect(page.getByTestId('pyramidize-btn')).toContainText('Pyramidize again');
    expect(state.requests[0]).toMatchObject({ text: ORIGINAL, documentType: 'auto', provider: 'claude' });

    // The user's text is set in Literata (docs/ui/design.md, "Type").
    const font = await page.getByTestId('canvas-textarea').evaluate(el => getComputedStyle(el).fontFamily);
    expect(font).toContain('Literata');
    await screenshot(page, '03-result-edit');

    await page.getByTestId('segment-preview').click();
    await expect(page.getByTestId('canvas-preview').locator('h1')).toHaveText(/Q3 travel/);
    await screenshot(page, '04-result-preview');

    await page.getByRole('tab', { name: 'Original' }).click();
    await expect(page.getByTestId('original-textarea')).toHaveValue(ORIGINAL);
    await screenshot(page, '05-original-tab');
  });

  test('keeps a history to preview, pin and revert to', async ({ page }) => {
    await open(page);
    await pyramidizeOnce(page);

    await page.getByTestId('trace-log-toggle').getByRole('button').click();
    const panel = page.getByTestId('trace-log-panel');
    await expect(panel).toBeVisible();
    await expect(page.getByTestId('trace-entry')).toHaveCount(1);

    await page.getByTestId('canvas-textarea').fill('A shorter draft.');
    await page.getByTestId('add-checkpoint-btn').getByRole('button').click();
    await expect(page.getByTestId('trace-entry')).toHaveCount(2);
    await expect(page.getByTestId('trace-log-toggle')).toContainText('History, 2 steps');

    await page.getByTestId('trace-entry').first().hover();
    await expect(page.getByTestId('trace-peek-overlay')).toBeVisible();
    await expect(page.getByTestId('trace-peek-content')).toContainText('Q3 travel');
    await screenshot(page, '06-history-peek');

    await page.getByTestId('trace-entry').first().click();
    await expect(page.getByTestId('trace-peek-close')).toBeVisible();
    await screenshot(page, '07-history-pinned');

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
    // It used to be transparent: its background was a token Aura never defined.
    const bg = await bubble.evaluate(el => getComputedStyle(el).backgroundColor);
    expect(bg).toBe('rgb(42, 42, 47)');
    await screenshot(page, '08-selection-bubble');

    await page.getByTestId('selection-instruction-input').press('Escape');
    await expect(bubble).toHaveCount(0);
  });

  test('shows a failure in the error row, with Copy error and Retry', async ({ page }) => {
    await open(page, { failPyramidize: 'Anthropic rejected the key. Check it in Settings, then try again.' });

    await page.getByTestId('original-textarea').fill(ORIGINAL);
    await page.getByTestId('pyramidize-btn').getByRole('button').click();

    const row = page.getByTestId('error-row');
    await expect(row).toBeVisible();
    await expect(row).toContainText('Anthropic rejected the key');
    await expect(page.getByTestId('copy-error-btn')).toBeVisible();
    await expect(page.getByTestId('retry-btn')).toBeVisible();
    await screenshot(page, '09-error');
  });

  test('explains a saved provider that cannot run, and keeps Pyramidize off', async ({ page }) => {
    await open(page, {}, 'bedrock');

    await expect(page.getByTestId('provider-unavailable')).toContainText('AWS Bedrock is not available yet');
    await page.getByTestId('original-textarea').fill(ORIGINAL);
    await expect(page.getByTestId('pyramidize-btn').getByRole('button')).toBeDisabled();
    await screenshot(page, '10-provider-unavailable');
  });

  test('opens Advanced with the quality threshold', async ({ page }) => {
    await open(page);

    const toggle = page.getByTestId('advanced-toggle');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await expect(page.getByLabel('Quality threshold')).toHaveValue('0.65');
    await screenshot(page, '11-advanced');
  });

  test('sends the editor text back to the window it came from', async ({ page }) => {
    const state = await open(page, { sourceApp: 'Untitled - Notepad' });
    await pyramidizeOnce(page);

    const send = page.getByTestId('send-back-btn');
    await expect(send).toHaveText(/Send back to Untitled - Notepad/);
    await send.getByRole('button').click();
    await expect.poll(() => state.sentBack).toEqual([DOCUMENT]);
  });
});
