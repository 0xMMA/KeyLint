/**
 * keylint.io (website/), straight from disk: no `ng serve`, no app.
 *
 * Guards what went wrong or nearly did when the site got screenshots and the
 * provider section (PR #126):
 * - /releases/latest skips pre-releases, and every 4.x is one, so it sent
 *   visitors to the v1 app. Download links point at one release tag instead,
 *   and all of them must match, so bumping the tag cannot miss one.
 * - Every image loads and carries width, height (no layout shift) and alt.
 * - Nothing scrolls sideways at phone width.
 */
import { test, expect, Page } from '@playwright/test';
import * as path from 'path';
import { pathToFileURL } from 'url';

const SITE = path.join(__dirname, '../../website');
const PAGES = ['index.html', '404.html'];

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

    test(`${name}: no horizontal scroll at 375 px`, async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 812 });
      await page.goto(url(name));
      await loadAllImages(page);
      const overflow = await page.evaluate(() =>
        document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflow).toBeLessThanOrEqual(0);
    });
  }

  test('download links avoid /releases/latest and all point at the same release', async ({ page }) => {
    const all: string[] = [];
    for (const name of PAGES) {
      await page.goto(url(name));
      const links = await releaseLinks(page);
      expect(links.length, `${name} links to a release`).toBeGreaterThan(0);
      all.push(...links);
    }
    for (const link of all) {
      expect(link, 'GitHub skips pre-releases on /latest').not.toContain('/releases/latest');
    }
    expect(new Set(all).size, `one release URL everywhere, found: ${[...new Set(all)].join(', ')}`).toBe(1);
  });
});
