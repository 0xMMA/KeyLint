/**
 * Screenshots and the Pyramidize animation for keylint.io (website/img/).
 *
 * Rerun after a UI change, from the repo root:
 *
 *   (cd frontend && npm ci && npx ng build) && \
 *     NODE_PATH=frontend/node_modules frontend/node_modules/.bin/playwright test -c scripts/website && \
 *     python3 scripts/website/encode.py
 *
 * It serves the production build (frontend/dist/browser — no Dev Tools nav
 * item, no dev-mode UI) on its own port (127.0.0.1:47813), so it never collides with an
 * `ng serve` on :4200, and answers the Wails binding calls with the e2e fake
 * backend. Raw frames land in scripts/website/out/ (gitignored); encode.py
 * turns them into the WebP files the page uses.
 *
 * Not part of the e2e suite on purpose: CI runs every spec in frontend/e2e/.
 *
 * The sample texts are not invented results: the outputs below are what the
 * real pipeline returned for the inputs (`KeyLint -fix` and
 * `KeyLint -pyramidize -type email`, claude-code provider, 2026-10-11). The
 * fake backend only replays them, so the app is shown doing what it does.
 */
import { test, expect, Page, CDPSession } from '@playwright/test';
import * as fs from 'fs';
import * as http from 'http';
import * as path from 'path';
import { installFakeBackend, fakeState, ANTHROPIC_MODELS, methodIDs } from '../../frontend/e2e/support/fake-backend';
import { installFakePyramidize, fakePyramidize } from '../../frontend/e2e/support/fake-pyramidize';

const ROOT = path.join(__dirname, '../..');
const DIST = path.join(ROOT, 'frontend/dist/browser');
const OUT = path.join(__dirname, 'out');
// Not a well-known dev-server default, so it never meets another tool's server.
const PORT = 47813;

// The default window (main.go: 1280×800 outer) leaves about this much page.
const VIEWPORT = { width: 1264, height: 740 };
const APP_VERSION = '4.7.1-beta';

const FIX_INPUT =
  'Hi team, i wanted to let you know that the release is moved to thursday. Their are still two bugs in the export wich we need too fix before we ship. Please dont merge anything into main untill then, and let me know if you have any question.';
const FIX_OUTPUT =
  "Hi team, I wanted to let you know that the release has been moved to Thursday. There are still two bugs in the export which we need to fix before we ship. Please don't merge anything into main until then, and let me know if you have any questions.";

const PYRAMIDIZE_INPUT =
  'hi all, quick update from the vendor call this morning. they cant deliver the new laptops before march 14 because of the chip shortage, so onboarding for the three new hires on march 3 is at risk. options: we rent laptops for two weeks (about 600 eur total) or the new hires start on loaner machines from the IT pool, but there are only two left in the pool. i would go for renting, its cheap and nobody has to wait. need a yes from finance by friday so i can still order in time. the vendor also offered 5% off if we accept the march 14 date as is. alex';
const PYRAMIDIZE_OUTPUT = `Laptop Delivery Delayed to March 14 | Recommend Renting for Two Weeks (~600 EUR) | @Finance Yes Needed by Friday

hi all,

**Recommendation: rent laptops for two weeks (~600 EUR total)**
- Cheap, and nobody has to wait
- Alternative: new hires start on loaner machines from the IT pool, but only two are left for three hires

**Finance yes needed by Friday**
- @Finance: yes needed by Friday so Alex can still order in time

**Onboarding on March 3 is at risk**
- Vendor cannot deliver the new laptops before March 14 because of the chip shortage
- Three new hires start on March 3

**Vendor offers 5% off if we accept March 14 as is**
- Offered during this morning's vendor call

alex`;

let server: http.Server;

test.use({ viewport: VIEWPORT, deviceScaleFactor: 2, baseURL: `http://127.0.0.1:${PORT}` });

test.beforeAll(async () => {
  if (!fs.existsSync(path.join(DIST, 'index.html'))) {
    throw new Error('No production build: run `cd frontend && npx ng build` first.');
  }
  fs.mkdirSync(OUT, { recursive: true });
  const types: Record<string, string> = {
    '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml',
    '.woff2': 'font/woff2', '.ico': 'image/x-icon', '.png': 'image/png', '.json': 'application/json',
  };
  server = http.createServer((req, res) => {
    const url = decodeURIComponent((req.url ?? '/').split('?')[0]);
    let file = path.join(DIST, path.normalize(url));
    if (!file.startsWith(DIST) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
      file = path.join(DIST, 'index.html'); // SPA fallback: the router owns every other path.
    }
    res.writeHead(200, { 'Content-Type': types[path.extname(file)] ?? 'application/octet-stream' });
    fs.createReadStream(file).pipe(res);
  });
  // Fails fast (port taken, no permission) instead of hanging the run.
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(PORT, '127.0.0.1', resolve);
  });
});

test.afterAll(async () => {
  if (server?.listening) await new Promise<void>((resolve) => server.close(() => resolve()));
});

/** The version in the sidebar footer, and no update dot. */
async function installFakeUpdater(page: Page): Promise<void> {
  const ids = methodIDs('updater');
  await page.route('**/wails/runtime', async (route) => {
    let body: { args?: { methodID?: number } } = {};
    try { body = route.request().postDataJSON() ?? {}; } catch { /* not a binding call */ }
    switch (ids.get(body.args?.methodID ?? -1)) {
      case 'GetVersion':
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(APP_VERSION) });
      case 'CheckForUpdate':
        return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ is_available: false }) });
      default:
        return route.fallback();
    }
  });
}

/** The Fix call: Enhance(text) answers the recorded result. */
async function installFakeFix(page: Page): Promise<void> {
  const ids = methodIDs('enhance');
  await page.route('**/wails/runtime', async (route) => {
    let body: { args?: { methodID?: number } } = {};
    try { body = route.request().postDataJSON() ?? {}; } catch { /* not a binding call */ }
    if (ids.get(body.args?.methodID ?? -1) === 'Enhance') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(FIX_OUTPUT) });
    }
    return route.fallback();
  });
}

function configured(activeProvider: string) {
  const state = fakeState({
    activeProvider,
    keys: { claude: 'keyring' },
    cliSignedIn: true,
    modelSources: { claude: 'live' },
    modelLists: { claude: ANTHROPIC_MODELS },
  });
  // Answered, so the production build does not open the setup wizard.
  state.firstRun = false;
  return state;
}

async function settle(page: Page): Promise<void> {
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  // No caret or focus ring in a still.
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.waitForTimeout(300);
}

test('Fix, with the corrections marked', async ({ page }) => {
  await installFakeBackend(page, configured('claude-code'));
  await installFakeFix(page);
  await installFakeUpdater(page);
  await page.goto('/fix');
  await page.getByTestId('fix-input').fill(FIX_INPUT);
  await page.getByTestId('fix-btn').getByRole('button').click();
  await expect(page.locator('ins.fx').first()).toBeVisible();
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'fix.png') });
});

test('Settings › AI providers, with the installed CLI signed in', async ({ page }) => {
  const state = configured('claude-code');
  // A local Ollama that answers, so its row reads as set up rather than broken.
  state.modelSources['ollama'] = 'live';
  state.modelLists['ollama'] = [{ id: 'llama3.2', label: 'llama3.2', resolved: '' }];
  await installFakeBackend(page, state);
  await installFakeUpdater(page);
  await page.goto('/settings?tab=providers');
  await expect(page.getByTestId('active-provider-name')).toBeVisible();
  await settle(page);
  // Taller than the default window, so every connection is in the picture.
  const content = await page.locator('.layout-main').evaluate((el) => el.scrollHeight);
  await page.setViewportSize({ width: VIEWPORT.width, height: Math.max(VIEWPORT.height, content) });
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'providers.png') });
});

/** A visible pointer for the animation: headless screenshots draw none. */
async function installPointer(page: Page): Promise<void> {
  await page.addInitScript(() => {
    window.addEventListener('DOMContentLoaded', () => {
      const dot = document.createElement('div');
      dot.id = 'demo-pointer';
      dot.innerHTML = '<svg width="22" height="22" viewBox="0 0 24 24"><path d="M5 3l14 8-6.5 1.5L9.5 19z" fill="#F4F4F5" stroke="#18181B" stroke-width="1.5" stroke-linejoin="round"/></svg>';
      Object.assign(dot.style, {
        position: 'fixed', left: '0', top: '0', zIndex: '2147483647', pointerEvents: 'none',
        transform: 'translate(-200px, -200px)', transition: 'none',
      });
      document.body.appendChild(dot);
      document.addEventListener('mousemove', (e) => {
        dot.style.transform = `translate(${e.clientX - 5}px, ${e.clientY - 3}px)`;
      }, true);
    });
  });
}

/** Moves the pointer to the middle of a locator in visible steps, then clicks. */
async function glideClick(page: Page, target: ReturnType<Page['locator']>): Promise<void> {
  const box = (await target.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 18 });
  await page.waitForTimeout(250);
  await target.click();
}

interface Frame { file: string; t: number }

async function startScreencast(page: Page, dir: string): Promise<{ cdp: CDPSession; frames: Frame[] }> {
  fs.rmSync(dir, { recursive: true, force: true });
  fs.mkdirSync(dir, { recursive: true });
  const cdp = await page.context().newCDPSession(page);
  const frames: Frame[] = [];
  cdp.on('Page.screencastFrame', async (f) => {
    const file = path.join(dir, `${String(frames.length).padStart(4, '0')}.png`);
    frames.push({ file, t: f.metadata.timestamp ?? Date.now() / 1000 });
    fs.writeFileSync(file, Buffer.from(f.data, 'base64'));
    await cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }).catch(() => undefined);
  });
  await cdp.send('Page.startScreencast', { format: 'png', everyNthFrame: 1 });
  return { cdp, frames };
}

test('Pyramidize: notes in, structured email out (animation and still)', async ({ page }) => {
  test.setTimeout(60_000);
  await installPointer(page);
  await installFakeBackend(page, configured('claude-code'));
  let release!: () => void;
  const hold = new Promise<void>((r) => { release = r; });
  await installFakePyramidize(page, fakePyramidize({
    clipboard: PYRAMIDIZE_INPUT,
    fullDocument: PYRAMIDIZE_OUTPUT,
    detectedType: 'email',
    hold,
  }));
  await installFakeUpdater(page);
  await page.goto('/enhance');
  await expect(page.getByTestId('model-select')).toContainText('KeyLint default');
  await page.getByTestId('tab-original').click();
  await expect(page.getByTestId('paste-from-clipboard-btn')).toBeVisible();
  await page.mouse.move(VIEWPORT.width - 120, VIEWPORT.height - 90);
  await settle(page);

  const dir = path.join(OUT, 'pyramidize-demo-frames');
  const { cdp, frames } = await startScreencast(page, dir);
  await page.waitForTimeout(1200);

  await glideClick(page, page.getByTestId('paste-from-clipboard-btn').getByRole('button'));
  await page.waitForTimeout(1400);

  await glideClick(page, page.getByTestId('pyramidize-btn').getByRole('button'));
  await expect(page.getByTestId('step-indicator')).toBeVisible();
  await page.waitForTimeout(1600);
  release();
  await expect(page.getByTestId('canvas-textarea')).toHaveValue(/Laptop Delivery Delayed/);
  await page.waitForTimeout(700);

  await glideClick(page, page.getByTestId('canvas-mode').getByText('Preview', { exact: true }));
  await expect(page.getByTestId('canvas-preview')).toBeVisible();
  await page.mouse.move(VIEWPORT.width - 120, VIEWPORT.height - 90, { steps: 12 });
  await page.waitForTimeout(3200);

  await cdp.send('Page.stopScreencast');
  await page.waitForTimeout(200);
  fs.writeFileSync(path.join(dir, 'frames.json'), JSON.stringify(frames, null, 1));

  // The still: the finished email, no pointer.
  await page.evaluate(() => document.getElementById('demo-pointer')?.remove());
  await settle(page);
  await page.screenshot({ path: path.join(OUT, 'pyramidize.png') });
});
