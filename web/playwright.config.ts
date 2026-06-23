import { defineConfig, devices } from '@playwright/test';

const appPort = process.env.E2E_APP_PORT ?? '17000';
const readyPort = process.env.E2E_READY_PORT ?? '17099';

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  timeout: 120_000,
  use: {
    baseURL: `http://127.0.0.1:${appPort}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] }
    }
  ],
  webServer: {
    command: 'node ../scripts/e2e-runtime.mjs',
    url: `http://127.0.0.1:${readyPort}/health`,
    cwd: '.',
    reuseExistingServer: false,
    timeout: 120_000
  }
});
