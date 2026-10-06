import { defineConfig } from '@playwright/test';

// A dedicated, migrated PostgreSQL/Redis pair must be running. The empty bot
// token permits locally signed fixtures; never point this suite at production.
export default defineConfig({
  testDir: './tests/journey',
  workers: 1,
  timeout: 90_000,
  use: { baseURL: 'http://localhost:3000', timezoneId: 'Asia/Tashkent', actionTimeout: 10_000, navigationTimeout: 15_000, trace: 'retain-on-failure' },
  webServer: [
    {
      command: 'go run -C ../backend ./cmd/api',
      url: 'http://localhost:58080/readyz',
      reuseExistingServer: false,
      env: {
        APP_ENV: 'development', HTTP_ADDR: '127.0.0.1:58080', TELEGRAM_BOT_TOKEN: '',
        DATABASE_URL: process.env.JOURNEY_DATABASE_URL ?? 'postgres://meetus:meetus@localhost:55432/meetus?sslmode=disable',
        REDIS_ADDR: process.env.JOURNEY_REDIS_ADDR ?? 'localhost:56379',
        JWT_SECRET: 'journey-only-jwt', TICKET_SECRET: 'journey-only-ticket',
        UPLOAD_DIR: '/tmp/meetus-journey-uploads', API_BASE_URL: 'http://localhost:58080', WEB_BASE_URL: 'http://localhost:3000',
      },
    },
    { command: 'npm run start -- --hostname 127.0.0.1 --port 3000', url: 'http://localhost:3000/en/login', reuseExistingServer: false },
  ],
});
