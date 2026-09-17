import { defineConfig, devices } from '@playwright/test';
import * as dotenv from 'dotenv';
import * as path from 'path';

// Load project-root .env so E2E tests can use ANTHROPIC_API_KEY etc.
dotenv.config({ path: path.join(__dirname, '../.env') });

export default defineConfig({
  testDir: './e2e',
  // A shared CI runner puts two workers on one `ng serve`; a test measured at
  // 1.7s in isolation peaked at 23.8s under that contention locally, which is
  // 79% of a 30s budget. Doubling it in CI costs nothing when tests pass.
  timeout: process.env['CI'] ? 60_000 : 30_000,
  // One retry in CI only: layout transitions on a shared runner occasionally
  // lose a race that is not a regression, and the HTML report keeps the failed
  // attempt so a genuinely flaky test still shows up rather than passing quietly.
  retries: process.env['CI'] ? 1 : 0,
  // The HTML report is what CI uploads when a run fails; 'never' keeps it from
  // trying to open a browser on a headless runner.
  reporter: [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: 'http://localhost:4200',
    screenshot: 'on',
    trace: 'retain-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  webServer: {
    command: 'npm run start',
    url: 'http://localhost:4200',
    // A cold `ng serve` on a CI runner is a lot slower than on a dev machine.
    timeout: process.env['CI'] ? 180_000 : 60_000,
    // Locally, reuse whatever the developer already has on :4200. In CI there is
    // never a server worth reusing, and silently attaching to a stale one would
    // test the wrong build.
    reuseExistingServer: !process.env['CI'],
    // Inject NG_APP_* vars so Angular's esbuild builder exposes them via import.meta.env
    env: {
      NG_APP_ANTHROPIC_API_KEY: process.env['ANTHROPIC_API_KEY'] ?? '',
    },
  },
});
