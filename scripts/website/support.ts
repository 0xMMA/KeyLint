/**
 * Shared by the website captures (capture.spec.ts: stills, demos.spec.ts:
 * clips). See capture.spec.ts for the command that runs them.
 *
 * The sample texts are not invented results: samples/<name>.out.* is what
 * KeyLint really returned for samples/<name>.in.txt, recorded by
 * refresh-samples.sh (samples/recorded.txt says when and from which commit).
 * The fake backend only replays them, so the app is shown doing what it does.
 */
import { Page, CDPSession } from '@playwright/test';
import { execSync } from 'child_process';
import * as fs from 'fs';
import * as http from 'http';
import * as path from 'path';
import { fakeState, ANTHROPIC_MODELS, methodIDs } from '../../frontend/e2e/support/fake-backend';

export const ROOT = path.join(__dirname, '../..');
export const DIST = path.join(ROOT, 'frontend/dist/browser');
export const SCENES = path.join(__dirname, 'scenes');
export const OUT = path.join(__dirname, 'out');
// Not a well-known dev-server default, so it never meets another tool's server.
export const PORT = 47813;
export const BASE_URL = `http://127.0.0.1:${PORT}`;

// The default window (main.go: 1280×800 outer) leaves about this much page.
export const VIEWPORT = { width: 1264, height: 740 };
/**
 * The version in the sidebar footer: the newest release tag at capture time
 * (the moving dev-channel tags v0.0.0-* are not releases).
 */
export const APP_VERSION = execSync("git describe --tags --abbrev=0 --exclude 'v0.0.0-*'", { cwd: ROOT, encoding: 'utf8' })
  .trim().replace(/^v/, '');

export const SAMPLES = path.join(__dirname, 'samples');

/** A recorded sample file, without the trailing newline the CLI prints. */
export function sample(file: string): string {
  return fs.readFileSync(path.join(SAMPLES, file), 'utf8').replace(/\n+$/, '');
}

export const FIX_INPUT = sample('fix.in.txt');
export const FIX_OUTPUT = sample('fix.out.txt');

export const CHAT_INPUT = sample('chat.in.txt');
export const CHAT_OUTPUT = sample('chat.out.txt');

export const PYRAMIDIZE_INPUT = sample('pyramidize.in.txt');
export const PYRAMIDIZE_OUTPUT = sample('pyramidize.out.md');

const TYPES: Record<string, string> = {
  '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml',
  '.woff2': 'font/woff2', '.ico': 'image/x-icon', '.png': 'image/png', '.json': 'application/json',
};

/**
 * Serves the production build at / (no Dev Tools nav item, no dev-mode UI),
 * the demo scenes at /scenes/ and the app icon at /build/appicon.png, on
 * 127.0.0.1 only. Fails fast if the port is taken.
 */
export async function startServer(): Promise<http.Server> {
  if (!fs.existsSync(path.join(DIST, 'index.html'))) {
    throw new Error('No production build: run `cd frontend && npx ng build` first.');
  }
  fs.mkdirSync(OUT, { recursive: true });
  const server = http.createServer((req, res) => {
    const url = decodeURIComponent((req.url ?? '/').split('?')[0]);
    let file: string;
    if (url.startsWith('/scenes/')) {
      file = path.join(SCENES, path.normalize(url.slice('/scenes/'.length)));
      if (!file.startsWith(SCENES) || !fs.existsSync(file)) { res.writeHead(404).end(); return; }
    } else if (url === '/build/appicon.png') {
      file = path.join(ROOT, 'build/appicon.png');
    } else {
      file = path.join(DIST, path.normalize(url));
      if (!file.startsWith(DIST) || !fs.existsSync(file) || fs.statSync(file).isDirectory()) {
        file = path.join(DIST, 'index.html'); // SPA fallback: the router owns every other path.
      }
    }
    res.writeHead(200, { 'Content-Type': TYPES[path.extname(file)] ?? 'application/octet-stream' });
    fs.createReadStream(file).pipe(res);
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(PORT, '127.0.0.1', resolve);
  });
  return server;
}

export async function stopServer(server: http.Server | undefined): Promise<void> {
  if (server?.listening) await new Promise<void>((resolve) => server.close(() => resolve()));
}

function bindingCall(route: { request(): { postDataJSON(): unknown } }): { methodID: number; args: unknown[] } {
  let body: { args?: { methodID?: number; args?: unknown[] } } = {};
  try { body = (route.request().postDataJSON() ?? {}) as typeof body; } catch { /* not a binding call */ }
  return { methodID: body.args?.methodID ?? -1, args: body.args?.args ?? [] };
}

/** The version in the sidebar footer, and no update dot. */
export async function installFakeUpdater(page: Page): Promise<void> {
  const ids = methodIDs('updater');
  await page.route('**/wails/runtime', async (route) => {
    switch (ids.get(bindingCall(route).methodID)) {
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
export async function installFakeFix(page: Page): Promise<void> {
  const ids = methodIDs('enhance');
  await page.route('**/wails/runtime', async (route) => {
    if (ids.get(bindingCall(route).methodID) === 'Enhance') {
      return route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(FIX_OUTPUT) });
    }
    return route.fallback();
  });
}

export function configured(activeProvider: string) {
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

export async function settle(page: Page): Promise<void> {
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  // No caret or focus ring in a still.
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.waitForTimeout(300);
}

export interface Frame { file: string; t: number }

/** Records every frame the page paints, with its timestamp, into dir. */
export async function startScreencast(page: Page, dir: string): Promise<{ cdp: CDPSession; frames: Frame[]; stop(): Promise<void> }> {
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
  // Frames come at CSS-pixel size whatever the device scale factor, so a
  // crisp recording needs a big viewport (demos.spec.ts zooms its scenes).
  await cdp.send('Page.startScreencast', { format: 'png', everyNthFrame: 1 });
  return {
    cdp,
    frames,
    async stop() {
      const end = Date.now() / 1000;
      await cdp.send('Page.stopScreencast');
      await page.waitForTimeout(200);
      // The end time, so the encoder holds the last frame until the stop.
      fs.writeFileSync(path.join(dir, 'frames.json'), JSON.stringify({ frames, end }, null, 1));
    },
  };
}
