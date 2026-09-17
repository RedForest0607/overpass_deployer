import { defineConfig } from '@playwright/test';

function port(name: string, fallback: number) {
  const value = Number(process.env[name] ?? fallback);
  if (!Number.isInteger(value) || value < 1024 || value > 65535) {
    throw new Error(`${name} must be a port between 1024 and 65535`);
  }
  return value;
}

const frontendPort = port('STUDIO_BROWSER_E2E_FRONTEND_PORT', 3100);
const backendPort = port('STUDIO_BROWSER_E2E_BACKEND_PORT', 8180);
const baseURL = `http://127.0.0.1:${frontendPort}`;
const runId = `${process.pid}-${Date.now()}`;
const macChrome =
  '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const executablePath =
  process.env.PLAYWRIGHT_CHROME_PATH ??
  (process.platform === 'darwin' ? macChrome : undefined);

export default defineConfig({
  testDir: './e2e',
  outputDir: '../.cache/studio-browser-e2e/playwright-results',
  fullyParallel: false,
  workers: 1,
  timeout: 60_000,
  expect: { timeout: 10_000 },
  reporter: [
    ['line'],
    [
      'html',
      {
        outputFolder: '../.cache/studio-browser-e2e/playwright-report',
        open: 'never',
      },
    ],
  ],
  use: {
    baseURL,
    browserName: 'chromium',
    headless: true,
    launchOptions: executablePath ? { executablePath } : undefined,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    acceptDownloads: true,
  },
  webServer: {
    command:
      `GOCACHE=/private/tmp/overpass-studio-browser-go-cache ` +
      `GOFLAGS=-buildvcs=false ` +
      `STUDIO_LOCAL_DATA_DIR=../.cache/studio-browser-e2e/data/${runId} ` +
      `STUDIO_LOCAL_BACKEND_ADDRESS=127.0.0.1:${backendPort} ` +
      `STUDIO_LOCAL_FRONTEND_HOST=127.0.0.1 ` +
      `STUDIO_LOCAL_FRONTEND_PORT=${frontendPort} ` +
      `./scripts/dev-local.sh`,
    cwd: '.',
    url: `${baseURL}/healthz`,
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: 'pipe',
    stderr: 'pipe',
  },
});
