import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 45_000,
  fullyParallel: false,
  workers: 1,
  reporter: 'list',
  webServer: {
    command: '..\\velocity.exe -port 18089 -open=false -tray=false',
    url: 'http://127.0.0.1:18089/api/status',
    reuseExistingServer: !process.env.CI,
    timeout: 30_000
  },
  use: {
    baseURL: process.env.BASE_URL || 'http://127.0.0.1:18089',
    headless: true,
    viewport: { width: 1440, height: 900 },
    actionTimeout: 15_000
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium' }
    }
  ]
});
