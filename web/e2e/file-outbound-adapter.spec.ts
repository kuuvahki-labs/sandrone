import { expect, type Page, test, type TestInfo } from "@playwright/test";

interface StoredFile {
  name: string;
  kind: string;
  source: { type: string; content: string };
  config: { subscriptions?: string[]; settings: { groups: Array<Record<string, unknown>>; [key: string]: unknown } };
  processors: Array<{ name?: string; type: string; stage: string; enabled?: boolean; params: { args?: Record<string, unknown>; [key: string]: unknown } }>;
}

const provider = { name: "provider", type: "remote", format: "uri-list" };
const preview = {
  subscription_name: "provider",
  format: "uri-list",
  before_count: 1,
  after_count: 1,
  status_counts: { added: 0, modified: 0, removed: 0, unchanged: 1 },
  nodes: [{ runtime_id: "fixture-hk", status: "unchanged", after: { name: "HK-01", type: "ss", server: "example.com", port: 8388 } }],
  warnings: [],
};
const settings = {
  schema_version: 1,
  http: { listen: "127.0.0.1:1137" },
  mcp: { path: "/mcp", max_output_bytes: 1048576 },
  log: { level: "info" },
  remote_defaults: { timeout_ms: 15000 },
  probe_defaults: { method: "url_test", core: "sing-box", url: "https://cp.cloudflare.com", ntp_server: "time.apple.com", timeout_ms: 5000, attempts: 1, concurrency: 10 },
  script_defaults: { timeout_ms: 2000 },
  cache_defaults: { remote_fetch_ttl_seconds: 0, probe_ttl_seconds: 0, subscription_snapshot_ttl_seconds: 0 },
  appearance: { theme_mode: "dark", locale: "en-US" },
  subscriptions: { auto_load_traffic: false, ignored_warnings: [] },
  scheduled_refresh: { enabled: false, schedule: "@every 10m", targets: [] },
};

async function mockFileAPI(page: Page) {
  const files = new Map<string, StoredFile>();
  const saves: StoredFile[] = [];
  const specReads: string[] = [];
  const unexpectedRequests: string[] = [];
  let subscriptionPreviews = 0;
  await page.addInitScript(() => {
    localStorage.setItem("sandrone.locale", "en-US");
    localStorage.setItem("sandrone.publicBaseUrl", window.location.origin);
  });
  // Every API response is local to this test; external requests are rejected.
  await page.route("**/*", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    if (url.origin !== "http://127.0.0.1:4173") {
      unexpectedRequests.push(request.url());
      await route.abort();
      return;
    }
    const path = url.pathname;
    if (path === "/v1/files") {
      if (request.method() === "POST") {
        const saved = request.postDataJSON() as StoredFile;
        saves.push(structuredClone(saved));
        files.set(saved.name, saved);
        await route.fulfill({ json: saved });
      } else {
        await route.fulfill({ json: { items: [...files.values()].map((file) => ({ name: file.name, type: "inline", target: file.kind, processor_count: file.processors.length })) } });
      }
      return;
    }
    if (path.startsWith("/v1/files/") && url.searchParams.get("mode") === "spec") {
      const name = decodeURIComponent(path.slice("/v1/files/".length));
      specReads.push(name);
      const saved = files.get(name);
      await route.fulfill({ status: saved ? 200 : 404, json: saved ?? { error: "missing fixture" } });
      return;
    }
    if (path === "/v1/subscriptions") {
      await route.fulfill({ json: { items: [provider] } });
      return;
    }
    if (path === "/v1/subscriptions/provider/preview") {
      subscriptionPreviews++;
      await route.fulfill({ json: preview });
      return;
    }
    if (path === "/v1/settings") {
      await route.fulfill({ json: { settings, effective: settings, overrides: {}, restart_required: [] } });
      return;
    }
    if (path === "/v1/capabilities/ui") {
      await route.fulfill({ json: { features: ["probe.enabled", "scheduler.enabled", "core.mihomo", "core.sing_box"].map((key) => ({ key, enabled: true })) } });
      return;
    }
    if (path === "/v1/capabilities/formats") {
      await route.fulfill({ json: { items: ["uri-list", "mihomo-proxies", "sing-box-outbounds"].map((format) => ({
        direction: format === "uri-list" ? "parse" : "render", format, node_types: ["ss"], reversible: false,
        field_counts: { supported: 1, lossy: 0, raw_only: 0 }, revisions: [], href: `/v1/capabilities/formats/render/${format}`,
      })) } });
      return;
    }
    if (path === "/v1/rule-set-catalog") {
      await route.fulfill({ json: { items: [] } });
      return;
    }
    if (path === "/version") {
      await route.fulfill({ json: { name: "sandrone", version: "0.1.0", revision: "0123456789abcdef" } });
      return;
    }
    if (path === "/healthz") {
      await route.fulfill({ body: "ok" });
      return;
    }
    if (path.startsWith("/v1/")) {
      unexpectedRequests.push(`${request.method()} ${path}${url.search}`);
      await route.abort();
      return;
    }
    await route.continue();
  });
  return { saves, specReads, unexpectedRequests, get subscriptionPreviews() { return subscriptionPreviews; } };
}

function collectConsoleIssues(page: Page): string[] {
  const issues: string[] = [];
  page.on("pageerror", (error) => issues.push(error.message));
  page.on("console", (message) => {
    if (message.type() === "error" || message.type() === "warning") issues.push(`${message.type()}: ${message.text()}`);
  });
  return issues;
}

async function assertPageIdentity(page: Page, heading: "New file" | "Edit file") {
  await expect(page).toHaveTitle("Sandrone");
  await expect(page.getByRole("heading", { name: heading, exact: true })).toBeVisible();
  await expect(page.locator("vite-error-overlay")).toHaveCount(0);
  await expect(page.getByText("Unexpected Application Error!", { exact: true })).toHaveCount(0);
}

async function captureEvidence(page: Page, testInfo: TestInfo, filename: string, issues: string[], api: Awaited<ReturnType<typeof mockFileAPI>>) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)).toBeLessThanOrEqual(1);
  expect(issues).toEqual([]);
  expect(api.unexpectedRequests).toEqual([]);
  await page.screenshot({ path: testInfo.outputPath(filename), fullPage: false });
  await testInfo.attach("browser-health", { body: JSON.stringify({ url: page.url(), title: await page.title(), consoleIssues: issues, unexpectedRequests: api.unexpectedRequests, specReads: api.specReads }), contentType: "application/json" });
}

test("new sing-box saves an explicit default in the adapter and reopens a missing-target warning", async ({ page }, testInfo) => {
  const issues = collectConsoleIssues(page);
  const api = await mockFileAPI(page);
  await page.goto("/files/new?source=sing-box");
  await assertPageIdentity(page, "New file");
  const processorInput = page.locator('input[name="processors"]');
  await expect.poll(async () => JSON.parse(await processorInput.inputValue())[0]?.params?.args).toEqual({ default_outbound: "Proxy" });
  const source = JSON.parse(await page.locator('input[name="source"]').inputValue()) as StoredFile["source"];
  expect(JSON.parse(source.content).route).not.toHaveProperty("final");
  const card = page.getByRole("group", { name: "Processor Outbound configuration adaptation", exact: true });
  await expect(card).toHaveCount(1);
  await expect(card.getByRole("button", { name: "Enable Outbound configuration adaptation", exact: true })).toHaveAttribute("aria-pressed", "true");

  await page.getByRole("combobox", { name: "Subscription", exact: true }).click();
  await page.getByRole("option", { name: "provider", exact: true }).click();
  await expect(page.getByText("Loaded 1 nodes", { exact: true })).toBeVisible();
  await card.getByRole("textbox", { name: "Arguments", exact: true }).fill("default_outbound=MissingTarget");
  const notice = page.getByText(/Default outbound “MissingTarget” was not found/);
  await expect(notice).toBeVisible();
  await expect(page.getByRole("button", { name: "Save file", exact: true })).toBeEnabled();
  await notice.scrollIntoViewIfNeeded();
  await captureEvidence(page, testInfo, "sing-box-before-save.png", issues, api);

  await page.getByRole("button", { name: "Save file", exact: true }).click();
  await expect(page).toHaveURL(/\/files\/sing-box\.json\/edit$/);
  await assertPageIdentity(page, "Edit file");
  expect(api.saves).toHaveLength(1);
  expect(api.saves[0].processors[0]).toMatchObject({ type: "script", stage: "file", params: { args: { default_outbound: "MissingTarget" } } });
  expect(JSON.parse(api.saves[0].source.content).route).not.toHaveProperty("final");

  await page.reload();
  await assertPageIdentity(page, "Edit file");
  expect(api.specReads.filter((name) => name === "sing-box.json").length).toBeGreaterThanOrEqual(2);
  await expect(card.getByRole("textbox", { name: "Arguments", exact: true })).toHaveValue("default_outbound=MissingTarget");
  await expect.poll(async () => JSON.parse(await processorInput.inputValue())).toEqual(api.saves[0].processors);
  await expect(notice).toBeVisible();
  await expect(page.getByRole("button", { name: "Save file", exact: true })).toBeEnabled();
  await notice.scrollIntoViewIfNeeded();
  await captureEvidence(page, testInfo, "sing-box-reopened.png", issues, api);
});
