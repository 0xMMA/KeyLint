/**
 * README.md, read from disk: no browser needed.
 *
 * The README had fallen behind the app (a "Deep Enhance" feature that no
 * longer exists, four providers in the wrong order, keys that "never leave
 * your machine"). These keep the parts that drift with the app honest.
 */
import { test, expect } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const ROOT = path.join(__dirname, '../..');
const README = fs.readFileSync(path.join(ROOT, 'README.md'), 'utf8');

test.describe('README', () => {
  test('lists the providers the app offers, in its order', () => {
    const source = fs.readFileSync(path.join(__dirname, '../src/app/core/providers.ts'), 'utf8');
    const start = source.indexOf('PROVIDER_OPTIONS');
    const block = source.slice(start, source.indexOf('];', start));
    const appLabels = [...block.matchAll(/label: '([^']+)'/g)].map((m) => m[1]);
    expect(appLabels.length, 'labels read from providers.ts').toBeGreaterThan(4);
    // The provider table's rows: | **Label** | … |
    const rows = [...README.matchAll(/^\| \*\*([^*]+)\*\* \|/gm)].map((m) => m[1]);
    expect(rows).toEqual(appLabels);
  });

  test('names every key environment variable the backend reads', () => {
    const source = fs.readFileSync(path.join(__dirname, '../src/app/core/providers.ts'), 'utf8');
    const start = source.indexOf('ENV_KEY_VARS');
    const block = source.slice(start, source.indexOf('};', start));
    const vars = [...block.matchAll(/: '([A-Z_]+)'/g)].map((m) => m[1]);
    expect(vars.length).toBeGreaterThan(2);
    for (const v of vars) expect(README, v).toContain(v);
  });

  test('every local image it shows is on disk', () => {
    const images = [
      ...[...README.matchAll(/!\[[^\]]*\]\(([^)]+)\)/g)].map((m) => m[1]),
      ...[...README.matchAll(/<(?:img|source)\b[^>]*?(?:src|srcset)="([^"]+)"/g)].map((m) => m[1]),
    ].filter((src) => !/^https?:/.test(src));
    expect(images.length).toBeGreaterThan(0);
    for (const src of images) expect(fs.existsSync(path.join(ROOT, src)), src).toBe(true);
  });

  // Owner, 2026-10-16: the title in the website's colours, "Key" and an
  // orange "Lint". GitHub strips colour from Markdown, so it is an SVG
  // wordmark per theme inside the h1, with "KeyLint" as its alt text.
  test('opens with the KeyLint wordmark as its title, for both themes', () => {
    const h1 = /^<h1>([\s\S]*?)<\/h1>/.exec(README.trimStart())?.[1] ?? '';
    expect(h1, 'the README starts with an h1').not.toBe('');
    expect(h1).toMatch(/<source media="\(prefers-color-scheme: dark\)" srcset="docs\/img\/keylint-wordmark-dark\.svg">/);
    expect(h1).toMatch(/<img alt="KeyLint" src="docs\/img\/keylint-wordmark-light\.svg"/);
    for (const mode of ['dark', 'light']) {
      const svg = fs.readFileSync(path.join(ROOT, `docs/img/keylint-wordmark-${mode}.svg`), 'utf8');
      // The website's orange on "Lint" (a darker shade on white, for contrast).
      expect(svg, mode).toMatch(mode === 'dark' ? /<tspan fill="#f97316">Lint<\/tspan>/ : /<tspan fill="#ea580c">Lint<\/tspan>/);
      expect(svg, mode).toMatch(/<tspan fill="#[0-9a-f]{6}">Key<\/tspan>/);
    }
  });

  // keylint.io's download button only leads to the GitHub release, so the
  // install step links there directly, once (owner, 2026-10-16).
  test('the download step links straight to the latest release', () => {
    const step = README.split('\n').find((line) => /^1\. Download/.test(line));
    expect(step, 'an install step starting "1. Download"').toBeTruthy();
    const links = [...step!.matchAll(/\]\(([^)]+)\)/g)].map((m) => m[1]);
    expect(links).toEqual(['https://github.com/0xMMA/KeyLint/releases/latest']);
  });

  test('names no feature the app no longer has, and promises nothing untrue', () => {
    expect(README).not.toMatch(/Deep Enhance|BYOK|never leave your machine|local Go backend/i);
  });
});
