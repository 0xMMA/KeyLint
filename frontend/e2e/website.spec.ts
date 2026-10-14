/**
 * keylint.io (website/), straight from disk: no `ng serve`, no app.
 *
 * Guards what went wrong or nearly did when the site got screenshots and the
 * provider section (PR #126):
 * - Download links use /releases/latest, which resolves to the newest
 *   non-prerelease (the site ships with the first stable 4.x). Every one must
 *   be exactly that, so no stale tag link creeps in and none is missed.
 * - Every image loads and carries width, height (no layout shift) and alt;
 *   every demo video has a poster, sources, size and a label, all on disk.
 * - Nothing scrolls sideways at phone width.
 */
import { test, expect, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';
import { pathToFileURL } from 'url';

const SITE = path.join(__dirname, '../../website');
const PAGES = ['index.html', '404.html'];
const LATEST = 'https://github.com/0xMMA/KeyLint/releases/latest';

function url(page: string): string {
  return pathToFileURL(path.join(SITE, page)).href;
}

/** Scrolls through the page so lazy images are fetched, then waits for each. */
async function loadAllImages(page: Page): Promise<void> {
  const height = await page.evaluate(() => document.documentElement.scrollHeight);
  for (let y = 0; y <= height; y += 300) {
    await page.evaluate((top) => window.scrollTo(0, top), y);
  }
  await expect.poll(() => page.evaluate(() =>
    [...document.images].filter((i) => !i.complete).length)).toBe(0);
}

/** Every link to a GitHub release page, plus the structured data's downloadUrl. */
function releaseLinks(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const hrefs = [...document.querySelectorAll('a[href]')]
      .map((a) => a.getAttribute('href')!)
      .filter((h) => h.includes('github.com/0xMMA/KeyLint/releases'));
    for (const s of document.querySelectorAll('script[type="application/ld+json"]')) {
      const data = JSON.parse(s.textContent ?? '{}');
      if (data.downloadUrl) hrefs.push(data.downloadUrl);
    }
    return hrefs;
  });
}

test.describe('website', () => {
  for (const name of PAGES) {
    test(`${name}: every image loads and has width, height and alt`, async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 900 });
      await page.goto(url(name));
      await loadAllImages(page);
      const images = await page.evaluate(() => [...document.images].map((i) => ({
        src: i.getAttribute('src'),
        loaded: i.naturalWidth > 0,
        width: i.getAttribute('width'),
        height: i.getAttribute('height'),
        alt: (i.getAttribute('alt') ?? '').trim(),
      })));
      for (const img of images) {
        expect(img.loaded, `${img.src} loads`).toBe(true);
        expect(Number(img.width), `${img.src} has a width`).toBeGreaterThan(0);
        expect(Number(img.height), `${img.src} has a height`).toBeGreaterThan(0);
        expect(img.alt, `${img.src} has alt text`).not.toBe('');
      }
    });

    test(`${name}: every video has a poster, sources, width, height and a label on disk`, async ({ page }) => {
      await page.goto(url(name));
      const videos = await page.evaluate(() => [...document.querySelectorAll('video')].map((v) => ({
        poster: v.getAttribute('poster'),
        sources: [...v.querySelectorAll('source')].map((s) => s.getAttribute('src')),
        width: v.getAttribute('width'),
        height: v.getAttribute('height'),
        label: (v.getAttribute('aria-label') ?? '').trim(),
      })));
      for (const v of videos) {
        const id = v.sources.join(', ');
        expect(v.poster, `${id} has a poster`).toBeTruthy();
        expect(fs.existsSync(path.join(SITE, v.poster!)), `${v.poster} exists`).toBe(true);
        expect(v.sources.length, 'a video has a source').toBeGreaterThan(0);
        for (const src of v.sources) {
          expect(fs.existsSync(path.join(SITE, src!)), `${src} exists`).toBe(true);
        }
        expect(Number(v.width), `${id} has a width`).toBeGreaterThan(0);
        expect(Number(v.height), `${id} has a height`).toBeGreaterThan(0);
        expect(v.label, `${id} has a label`).not.toBe('');
      }
    });

    test(`${name}: no horizontal scroll at 375 px`, async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 812 });
      await page.goto(url(name));
      await loadAllImages(page);
      const overflow = await page.evaluate(() =>
        document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }

  test('every download link is exactly /releases/latest', async ({ page }) => {
    for (const name of PAGES) {
      await page.goto(url(name));
      const links = await releaseLinks(page);
      expect(links.length, `${name} links to a release`).toBeGreaterThan(0);
      for (const link of links) {
        expect(link, `${name}: no tag or list link`).toBe(LATEST);
      }
    }
  });
});

/**
 * The texts the demo clips and stills replay (scripts/website/samples/,
 * recorded by refresh-samples.sh). Read from disk, no browser needed.
 */
test.describe('website demo samples', () => {
  const SAMPLES = path.join(__dirname, '../../scripts/website/samples');
  const read = (file: string) => fs.readFileSync(path.join(SAMPLES, file), 'utf8').trim();

  /** Words that differ between two texts: the longer length minus their longest common run. */
  function wordsChanged(before: string, after: string): number {
    const a = before.split(/\s+/);
    const b = after.split(/\s+/);
    const lcs = Array.from({ length: a.length + 1 }, () => new Array<number>(b.length + 1).fill(0));
    for (let i = a.length - 1; i >= 0; i--) {
      for (let j = b.length - 1; j >= 0; j--) {
        lcs[i][j] = a[i] === b[j] ? lcs[i + 1][j + 1] + 1 : Math.max(lcs[i + 1][j], lcs[i][j + 1]);
      }
    }
    return Math.max(a.length, b.length) - lcs[0][0];
  }

  test('every sample has a recorded output, and recorded.txt names it', () => {
    const inputs = fs.readdirSync(SAMPLES).filter((f) => f.endsWith('.in.txt'));
    expect(inputs.length).toBeGreaterThan(0);
    const recorded = read('recorded.txt').split('\n').filter((l) => !l.startsWith('#'));
    for (const input of inputs) {
      const name = input.replace(/\.in\.txt$/, '');
      const outputs = fs.readdirSync(SAMPLES).filter((f) => f.startsWith(`${name}.out.`));
      expect(outputs, `${name}: run scripts/website/refresh-samples.sh ${name}`).toHaveLength(1);
      expect(read(outputs[0]).length, `${name}: empty output`).toBeGreaterThan(0);
      expect(recorded.some((l) => l.startsWith(`${name} `)), `${name}: no line in recorded.txt`).toBe(true);
    }
  });

  // Owner feedback 2026-10-14: the Silent Fix clip's message had too few
  // mistakes to show what Fix does. Eight corrected words is the floor.
  test('the Silent Fix clip corrects at least eight words', () => {
    expect(wordsChanged(read('chat.in.txt'), read('chat.out.txt'))).toBeGreaterThanOrEqual(8);
  });
});
