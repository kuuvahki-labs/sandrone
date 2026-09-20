import { defineConfig, devices } from "@playwright/test";

const useWebServer = process.env.PW_NO_WEBSERVER !== "1";
const chromiumExecutablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH;
const chromiumChannel = process.env.PLAYWRIGHT_CHROMIUM_CHANNEL;
const e2ePort = Number(process.env.SANDRONE_E2E_PORT ?? "19173");

if (!Number.isInteger(e2ePort) || e2ePort < 1 || e2ePort > 65_535) {
  throw new Error("SANDRONE_E2E_PORT must be an integer between 1 and 65535");
}

const e2eBaseURL = `http://127.0.0.1:${e2ePort}`;

export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  expect: { timeout: 5_000 },
  webServer: useWebServer
    ? {
        command: `pnpm build && pnpm start --port ${e2ePort} --strictPort`,
        url: e2eBaseURL,
        reuseExistingServer: false,
        timeout: 60_000,
      }
    : undefined,
  use: {
    baseURL: e2eBaseURL,
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
