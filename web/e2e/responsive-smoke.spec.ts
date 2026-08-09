import { expect, test } from "@playwright/test";

const manifest = {
  subscriptions: [
    { name: "provider", type: "remote", format: "uri-list", meta: { description: "daily" } },
    { name: "default", type: "collection", meta: { node_count: "12", source_count: "1" } },
  ],
  files: [
    { name: "default.yaml", type: "inline", target: "mihomo", format: "yaml", meta: { description: "main config" } },
  ],
};

const longPreviewNode = "node-with-an-extremely-long-generated-name-abcdefghijklmnopqrstuvwxyz-0123456789";
const filePreview = {
  content_type: "application/yaml",
  body: [
    "proxies:",
    `  - name: ${longPreviewNode}`,
    "    type: ss",
    "    server: example.com",
    "    port: 8388",
    "proxy-groups:",
    "  - name: Proxy",
    "    type: select",
    `    proxies: [${longPreviewNode}, DIRECT]`,
    "rules:",
    "  - MATCH,Proxy",
    ...Array.from({ length: 80 }, (_, index) => `# generated line ${index + 1}`),
  ].join("\n"),
  warnings: [],
};

function settingsEnvelope() {
  const settings = {
    schema_version: 1,
    http: { listen: "127.0.0.1:1137" },
    mcp: { path: "/mcp", allow_management_tools: false, max_output_bytes: 1048576 },
    webui: { static_dir: "" },
    log: { level: "info" },
    remote_defaults: { user_agent: "sandrone/0.1.0", timeout_ms: 15000 },
    probe_defaults: {
      method: "url_test",
      core: "sing-box",
      url: "https://cp.cloudflare.com",
      ntp_server: "time.apple.com",
      timeout_ms: 5000,
      attempts: 1,
      concurrency: 10,
      cache_ttl_seconds: 0,
    },
    cache_defaults: {
      remote_fetch_ttl_seconds: 0,
      subscription_traffic_ttl_seconds: 60,
      subscription_render_ttl_seconds: 0,
      file_render_ttl_seconds: 0,
    },
    appearance: { theme_mode: "dark", locale: "zh-CN" },
    subscriptions: { auto_load_traffic: false },
  };
  return {
    settings,
    effective: settings,
    overrides: {},
    restart_required: [],
  };
}

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("sandrone.locale", "zh-CN");
    localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
  });
  await page.route("**/v1/subscriptions", async (route) => {
    await route.fulfill({ json: { items: manifest.subscriptions } });
  });
  await page.route("**/v1/files", async (route) => {
    await route.fulfill({ json: { items: manifest.files } });
  });
  await page.route("**/v1/rule-set-catalog?target=*", async (route) => {
    await route.fulfill({ json: { items: [] } });
  });
  await page.route("**/v1/settings", async (route) => {
    await route.fulfill({ json: settingsEnvelope() });
  });
  await page.route("**/version", async (route) => {
    await route.fulfill({
      json: {
        name: "sandrone",
        version: "0.1.0",
        revision: "0123456789abcdef",
      },
    });
  });
  await page.route("**/healthz", async (route) => {
    await route.fulfill({ body: "ok" });
  });
  await page.route("**/v1/files/**", async (route) => {
    await route.fulfill({ json: filePreview });
  });
});

const routes = [
  { path: "/subscriptions", heading: "我的订阅", content: "default", focus: false },
  { path: "/files", heading: "我的文件", content: "default.yaml", focus: false },
  { path: "/files/new?source=mihomo", heading: "新建文件", content: "节点来源", focus: true },
  { path: "/files/default.yaml/preview", heading: "文件预览", content: longPreviewNode, focus: true },
  { path: "/settings/runtime", heading: "高级设置", content: "远程请求", focus: false },
];

for (const route of routes) {
  test(`${route.path} renders without horizontal overflow`, async ({ page }, testInfo) => {
    const consoleIssues: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error" || message.type() === "warning") {
        consoleIssues.push(`${message.type()}: ${message.text()}`);
      }
    });

    await page.goto(route.path);

    await expect(page.getByRole("heading", { name: route.heading, level: 2 })).toBeVisible();
    const routeContent = route.path === "/files/new?source=mihomo"
      ? page.getByRole("group", { name: route.content }).first()
      : route.path === "/files/default.yaml/preview"
        ? page.getByRole("region", { name: "最终文件内容" })
        : route.path === "/settings/runtime"
          ? page.getByRole("button", { name: route.content })
          : page.getByText(route.content);
    await expect(routeContent).toBeVisible();

    if (route.path === "/files/default.yaml/preview") {
      await expect(routeContent).toContainText(longPreviewNode);
      const previewMetrics = await routeContent.locator("pre").evaluate((pre) => ({
        clientHeight: pre.clientHeight,
        overflowY: getComputedStyle(pre).overflowY,
        scrollHeight: pre.scrollHeight,
      }));
      expect(previewMetrics.overflowY).toBe("auto");
      expect(previewMetrics.scrollHeight).toBeGreaterThan(previewMetrics.clientHeight);
    }

    const pageMetrics = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }));
    expect(pageMetrics.scrollWidth, `${testInfo.project.name} ${route.path} should not scroll horizontally`)
      .toBeLessThanOrEqual(pageMetrics.clientWidth + 1);

    const bottomNav = page.getByRole("navigation", { name: "底部导航" });
    const drawer = page.getByRole("navigation", { name: "桌面导航" });
    if (route.focus) {
      await expect(bottomNav).toHaveCount(0);
    } else if (testInfo.project.name === "mobile") {
      await expect(bottomNav).toBeVisible();
      await expect(drawer).toBeHidden();
    } else {
      await expect(bottomNav).toBeHidden();
      await expect(drawer).toBeVisible();
    }
    expect(consoleIssues).toEqual([]);
  });
}
