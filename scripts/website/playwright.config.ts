// Config for the website capture only (see capture.spec.ts for the command).
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: '.',
  testMatch: ['capture.spec.ts', 'demos.spec.ts'],
  outputDir: 'out/test-results',
  workers: 1,
  reporter: [['list']],
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
