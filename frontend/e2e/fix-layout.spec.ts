/**
 * The Fix page's two halves in a real browser: side by side on a wide window,
 * stacked on a narrow one. jsdom applies no layout, so this lives here.
 */
import { test, expect, Page } from '@playwright/test';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS } from './support/fake-backend';

async function open(page: Page, width: number, height: number): Promise<void> {
  await page.setViewportSize({ width, height });
  await installFakeBackend(page, fakeState({
    activeProvider: 'claude',
    keys: { claude: 'keyring' },
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  }));
  await page.goto('/fix');
  await page.getByTestId('fix-input').fill('Some text here. '.repeat(60));
}

/** Boxes and divider of the halves, measured in the page. */
async function halves(page: Page) {
  return page.evaluate(() => {
    const box = (s: string) => document.querySelector(s)!.getBoundingClientRect();
    const style = getComputedStyle(document.querySelector('.fix-input-half')!);
    const sheet = box('.fix-sheet');
    const input = box('.fix-input-half');
    const result = box('.fix-result-half');
    const textarea = box('[data-testid="fix-input"]');
    const output = box('[data-testid="fix-output"]');
    return {
      stacked: result.top >= input.bottom - 1,
      inSheet: input.bottom <= sheet.bottom + 1 && result.bottom <= sheet.bottom + 1,
      textareaInHalf: textarea.bottom <= input.bottom + 1,
      outputInHalf: output.bottom <= result.bottom + 1,
      outputHeight: output.height,
      borderRight: style.borderRightWidth,
      borderBottom: style.borderBottomWidth,
    };
  });
}

test.describe('Fix — layout', () => {
  for (const [w, h] of [[640, 600], [640, 480]]) {
    test(`stacked at ${w}×${h}: both halves whole, text scrolls inside them, divider between`, async ({ page }) => {
      await open(page, w, h);
      const m = await halves(page);
      expect(m.stacked, 'one column here').toBe(true);
      expect(m.inSheet, 'neither half runs past the sheet').toBe(true);
      expect(m.textareaInHalf, 'the text field ends inside its half').toBe(true);
      expect(m.outputInHalf, 'the result ends inside its half').toBe(true);
      expect(m.outputHeight).toBeGreaterThan(80);
      expect(m.borderRight).toBe('0px');
      expect(m.borderBottom).toBe('1px');
      // The text field scrolls rather than growing past its half.
      const scrolls = await page.getByTestId('fix-input').evaluate((el) => el.scrollHeight > el.clientHeight);
      expect(scrolls).toBe(true);
    });
  }

  test('side by side on a wide window, divider between', async ({ page }) => {
    await open(page, 1280, 800);
    const m = await halves(page);
    expect(m.stacked).toBe(false);
    expect(m.inSheet).toBe(true);
    expect(m.borderRight).toBe('1px');
    expect(m.borderBottom).toBe('0px');
  });
});
