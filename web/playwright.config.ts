import { defineConfig, devices } from "@playwright/test";

const useWebServer = process.env.PW_NO_WEBSERVER !== "1";
const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
const chromiumChannel = process.env.PLAYWRIGHT_CHROMIUM_CHANNEL;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  webServer: useWebServer
    ? {
        command: "pnpm build && pnpm start -- --port 4173",
        url: "http://127.0.0.1:4173",
        reuseExistingServer: false,
        timeout: 60_000,
      }
    : undefined,
  use: {
    baseURL: "http://127.0.0.1:4173",
    ...(chromiumChannel ? { channel: chromiumChannel } : {}),
    ...(chromiumExecutablePath ? { launchOptions: { executablePath: chromiumExecutablePath } } : {}),
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "mobile",
      use: { ...devices["Pixel 7"], viewport: { width: 390, height: 844 } },
    },
    {
      name: "desktop",
      use: { viewport: { width: 1280, height: 820 } },
    },
  ],
});
