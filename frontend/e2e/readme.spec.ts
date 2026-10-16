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
    const images = [...README.matchAll(/!\[[^\]]*\]\(([^)]+)\)/g)].map((m) => m[1]).filter((src) => !/^https?:/.test(src));
    expect(images.length).toBeGreaterThan(0);
    for (const src of images) expect(fs.existsSync(path.join(ROOT, src)), src).toBe(true);
  });

  test('names no feature the app no longer has, and promises nothing untrue', () => {
    expect(README).not.toMatch(/Deep Enhance|BYOK|never leave your machine|local Go backend/i);
  });
});
