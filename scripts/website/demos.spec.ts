/**
 * The two staged demo clips for keylint.io (command: see capture.spec.ts).
 *
 * The scenes (scenes/chat.html, scenes/mail.html) are mock-ups of a generic
 * chat app and a generic webmail client: no real product, names or colours.
 * What KeyLint does in them is either the real app or exactly what the Go
 * side does:
 * - Silent Fix runs in Go with no window (internal/features/silentfix): it
 *   copies the selection, marks the tray icon busy, fixes, writes the result
 *   to the clipboard and pastes it over the selection; success is silent.
 *   The scene does those steps itself, with the recorded result.
 * - Pyramidize is the real app (production build) in an iframe. The hotkey
 *   is the real `shortcut:pyramidize` event dispatched into it, which reads
 *   the clipboard (the fake answers with the notes) like the app does. Send
 *   back runs in Go (internal/features/pyramidize/sendback.go): it puts the
 *   text and the HTML the app handed it on the clipboard, brings the source
 *   window to the front and sends Ctrl+V into it. The scene does those steps
 *   with exactly what the app handed over, and the paste is a real Ctrl+V.
 */
import { test, expect, Page, Frame } from '@playwright/test';
import * as http from 'http';
import * as path from 'path';
import { installFakeBackend } from '../../frontend/e2e/support/fake-backend';
import { installFakePyramidize, fakePyramidize } from '../../frontend/e2e/support/fake-pyramidize';
import {
  BASE_URL, CHAT_INPUT, CHAT_OUTPUT, OUT, PYRAMIDIZE_INPUT, PYRAMIDIZE_OUTPUT,
  configured, installFakeUpdater, startScreencast, startServer, stopServer,
} from './support';

// Scene pixels are 1280×800; the scene is zoomed 2x (scenes/stage.css), so the
// viewport, every coordinate here and the recorded frames are twice that.
const Z = 2;

// Read from the recorded samples, so a refreshed sample needs no edit here:
// the first words of the notes (the Form step's excerpt) and the start of the
// document's first line (its subject).
const escapeRe = (text: string) => text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
const NOTES_START = PYRAMIDIZE_INPUT.split(/\s+/).slice(0, 4).join(' ');
const SUBJECT_START = new RegExp(escapeRe(PYRAMIDIZE_OUTPUT.split('\n')[0].trim().slice(0, 30)));
const STAGE = { width: 1280 * Z, height: 800 * Z };
let server: http.Server;

test.use({ viewport: STAGE, deviceScaleFactor: 2, baseURL: BASE_URL });
test.beforeAll(async () => { server = await startServer(); });
test.afterAll(async () => { await stopServer(server); });

// ── Pointer and captions ─────────────────────────────────────────────────

let at = { x: STAGE.width - 160 * Z, y: STAGE.height - 160 * Z };

async function placePointer(page: Page, x: number, y: number): Promise<void> {
  at = { x, y };
  await page.mouse.move(x, y);
  await page.evaluate(([px, py]) => (window as any).stage.pointer(px, py), [x, y]);
}

/** Moves the visible pointer (and the mouse) to x,y with an ease-in-out. */
async function glide(page: Page, x: number, y: number, ms = 550): Promise<void> {
  const from = { ...at };
  const steps = Math.max(8, Math.round(ms / 22));
  for (let i = 1; i <= steps; i++) {
    const t = i / steps;
    const e = t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2;
    const px = from.x + (x - from.x) * e;
    const py = from.y + (y - from.y) * e;
    await page.mouse.move(px, py);
    await page.evaluate(([a, b]) => (window as any).stage.pointer(a, b), [px, py]);
  }
  at = { x, y };
}

async function clickAt(page: Page, x: number, y: number, clickCount = 1): Promise<void> {
  await glide(page, x, y);
  await page.waitForTimeout(140);
  await page.evaluate(() => (window as any).stage.click());
  await page.mouse.click(x, y, { clickCount });
}

async function center(locator: { boundingBox(): Promise<{ x: number; y: number; width: number; height: number } | null> }) {
  const b = (await locator.boundingBox())!;
  return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
}

/**
 * Centre of an element inside the KeyLint iframe, in viewport pixels.
 * Playwright's boxes for iframe content ignore the scene's CSS zoom, so this
 * maps the element's own rect through the iframe's on-screen box.
 */
async function appCenter(page: Page, locator: { evaluate<R>(fn: (el: Element) => R): Promise<R> }) {
  const frameBox = (await page.locator('#app').boundingBox())!;
  const scale = frameBox.width / 1264;
  const r = await locator.evaluate((el) => {
    const b = el.getBoundingClientRect();
    return { x: b.x + b.width / 2, y: b.y + b.height / 2 };
  });
  return { x: frameBox.x + r.x * scale, y: frameBox.y + r.y * scale };
}

async function caption(page: Page, caps: string[], hint: string, ms?: number): Promise<void> {
  await page.evaluate(([c, h, m]) => (window as any).stage.keys(c, h, m), [caps, hint, ms] as const);
}

/** Shows the caption, then presses the keys for real. */
async function press(page: Page, combo: string, caps: string[], hint: string): Promise<void> {
  await caption(page, caps, hint);
  await page.waitForTimeout(120);
  await page.keyboard.press(combo);
}

// ── Clip 1: Silent Fix in a chat app ─────────────────────────────────────

test('clip: Silent Fix in a chat app', async ({ page, context }) => {
  test.setTimeout(60_000);
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: BASE_URL });
  await page.goto('/scenes/chat.html');
  await page.evaluate(() => document.fonts.ready);
  await placePointer(page, at.x, at.y);

  const rec = await startScreencast(page, path.join(OUT, 'demo-fix-frames'));
  await page.waitForTimeout(500);

  const box = page.locator('#box');
  const b = await center(box);
  await clickAt(page, b.x - 260 * Z, b.y - 8 * Z);
  // Out of the way of the text while typing.
  await glide(page, b.x + 420 * Z, b.y - 150 * Z, 400);
  await box.pressSequentially(CHAT_INPUT, { delay: 8 });
  // The clip is the page's hero: it starts here, with the message written,
  // rather than on an empty chat (encode.py starts a moment before the mark).
  rec.mark('start');
  await page.waitForTimeout(600);

  await press(page, 'Control+A', ['Ctrl', 'A'], 'select the message');
  await page.waitForTimeout(700);

  // The hotkey. KeyLint copies the selection and marks its tray icon busy...
  await caption(page, ['Ctrl', 'G'], 'KeyLint fixes it in place', 1600);
  await page.evaluate(() => (window as any).stage.busy(true));
  // The poster: the selection, the hotkey and KeyLint at work in one frame —
  // what a visitor sees before the clip plays, and with reduced motion.
  await page.waitForTimeout(300);
  rec.mark('poster');
  await page.waitForTimeout(1200);
  // ...then writes the fix to the clipboard and pastes it over the selection.
  await page.evaluate((t) => navigator.clipboard.writeText(t), CHAT_OUTPUT);
  await page.keyboard.press('Control+V');
  await page.evaluate(() => (window as any).stage.busy(false));
  await expect(box).toHaveValue(CHAT_OUTPUT);
  await page.waitForTimeout(2100);

  await press(page, 'Enter', ['Enter'], 'send');
  await page.waitForTimeout(1200);
  await rec.stop();
});

// ── Clip 2: Pyramidize from a webmail client ─────────────────────────────

async function appFrame(page: Page): Promise<Frame> {
  await expect.poll(() => page.frames().some((f) => f.url().endsWith('/enhance'))).toBe(true);
  return page.frames().find((f) => f.url().endsWith('/enhance'))!;
}

test('clip: Pyramidize from a webmail client', async ({ page, context }) => {
  test.setTimeout(90_000);
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: BASE_URL });
  await installFakeBackend(page, configured('claude-code'));
  let release!: () => void;
  const hold = new Promise<void>((r) => { release = r; });
  const fake = fakePyramidize({
    // What the hotkey copies from the mail body, and the window it came from:
    // the app is the title's last segment, as GetSourcePreset splits it.
    clipboard: PYRAMIDIZE_INPUT,
    sourceApp: 'New message – Webmail',
    sourcePreset: { app: 'Webmail', documentType: 'email', remembered: false },
    fullDocument: PYRAMIDIZE_OUTPUT,
    hold,
  });
  await installFakePyramidize(page, fake);
  await installFakeUpdater(page);

  await page.goto('/scenes/mail.html');
  await page.evaluate((text) => {
    const p = document.createElement('p');
    p.textContent = text;
    document.getElementById('body')!.appendChild(p);
  }, PYRAMIDIZE_INPUT);
  const frame = await appFrame(page);
  const app = page.frameLocator('#app');
  await expect(app.getByTestId('original-textarea')).toBeVisible();
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
  await placePointer(page, 1180 * Z, 560 * Z);

  const rec = await startScreencast(page, path.join(OUT, 'demo-pyramidize-frames'));
  await page.waitForTimeout(600);

  const body = page.locator('#body');
  const bc = await center(body);
  await clickAt(page, bc.x + 120 * Z, bc.y - 40 * Z);
  await page.waitForTimeout(200);
  await press(page, 'Control+A', ['Ctrl', 'A'], 'select the notes');
  await page.waitForTimeout(700);

  // Hold Ctrl, tap G twice: KeyLint copies the selection and opens Pyramidize.
  await caption(page, ['Ctrl', 'G', 'G'], 'hold Ctrl, tap G twice: Pyramidize', 1700);
  await page.waitForTimeout(220);
  await page.evaluate(() => (window as any).stage.tapLast());
  await page.waitForTimeout(250);
  await page.evaluate(() => document.getElementById('klwin')!.classList.add('show'));
  await frame.evaluate(() => (window as any)._wails.dispatchWailsEvent({ name: 'shortcut:pyramidize', data: 'hotkey' }));
  // The shortcut's text opens on the Form step, email preselected.
  await expect(app.getByTestId('form-email')).toHaveAttribute('aria-checked', 'true');
  await expect(app.getByTestId('text-excerpt')).toContainText(NOTES_START);
  await page.waitForTimeout(1600);

  const pz = await appCenter(page, app.getByTestId('pyramidize-btn').getByRole('button'));
  await clickAt(page, pz.x, pz.y);
  await expect(app.getByTestId('step-indicator')).toBeVisible();
  await page.waitForTimeout(1700);
  release();
  await expect(app.getByTestId('canvas-textarea')).toHaveValue(SUBJECT_START);
  await page.waitForTimeout(500);

  const pv = await appCenter(page, app.getByTestId('canvas-mode').getByText('Preview', { exact: true }));
  await clickAt(page, pv.x, pv.y);
  await expect(app.getByTestId('canvas-preview')).toBeVisible();
  await page.waitForTimeout(1900);

  const send = await appCenter(page, app.getByTestId('send-back-btn').getByRole('button'));
  await clickAt(page, send.x, send.y);
  await expect.poll(() => fake.sentBackHTML.length).toBe(1);

  // What Go does next: the clipboard, the mail's window to the front (its
  // notes still selected, as when the shortcut copied them), then Ctrl+V.
  await page.evaluate(async ([text, html]) => {
    document.getElementById('klwin')!.classList.remove('show');
    const mail = document.getElementById('body')!;
    mail.focus();
    const range = document.createRange();
    range.selectNodeContents(mail);
    window.getSelection()!.removeAllRanges();
    window.getSelection()!.addRange(range);
    await navigator.clipboard.write([new ClipboardItem({
      'text/html': new Blob([html], { type: 'text/html' }),
      'text/plain': new Blob([text], { type: 'text/plain' }),
    })]);
  }, [fake.sentBack[0], fake.sentBackHTML[0]]);
  await page.waitForTimeout(250);
  await page.keyboard.press('Control+V');
  await expect(body.locator('strong').first()).toBeVisible();
  await page.waitForTimeout(1500);

  // The first line of the copied text is the subject: move it there.
  const first = await center(body.locator('p').first());
  await clickAt(page, first.x - 120 * Z, first.y, 3);
  await page.waitForTimeout(300);
  await press(page, 'Control+X', ['Ctrl', 'X'], 'cut the subject line');
  await page.waitForTimeout(450);
  const subject = await center(page.locator('#subject'));
  await clickAt(page, subject.x - 200 * Z, subject.y);
  await press(page, 'Control+V', ['Ctrl', 'V'], 'paste it as the subject');
  await expect(page.locator('#subject')).toHaveValue(SUBJECT_START);
  await page.waitForTimeout(500);
  // A click back into the body: the subject field shows its start again.
  const bb = (await body.boundingBox())!;
  await clickAt(page, bb.x + bb.width * 0.7, bb.y + bb.height - 30 * Z);
  await page.waitForTimeout(2000);
  await rec.stop();
});
