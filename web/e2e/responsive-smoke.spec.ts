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
const subscriptionPreview = {
  subscription_name: "provider",
  format: "uri-list",
  before_count: 1,
  after_count: 1,
  status_counts: { added: 0, modified: 0, removed: 0, unchanged: 1 },
  nodes: [{
    runtime_id: "probe-runtime-id",
    status: "unchanged",
    after: {
      name: longPreviewNode,
      type: "ss",
      server: "example.com",
      port: 8388,
      meta: {
        "probe.alive": "true",
        "probe.duration_ms": "42",
        "probe.method": "url_test",
      },
    },
  }],
  warnings: [{
    code: "parse_unknown_field",
    field: "uri.query.mode",
    message: "field preserved in NodeIR Raw",
    node: "node-with-known-mode-field",
    source: "uri-list",
  }],
};
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
const formatCapabilities = {
  items: [
    {
      direction: "parse",
      format: "uri-list",
      node_types: ["ss"],
      reversible: false,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: [],
      href: "/v1/capabilities/formats/parse/uri-list",
    },
    {
      direction: "render",
      format: "base64",
      node_types: ["ss"],
      reversible: false,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: [],
      href: "/v1/capabilities/formats/render/base64",
    },
    {
      direction: "render",
      format: "mihomo-proxies",
      node_types: ["ss"],
      reversible: false,
      field_counts: { supported: 1, lossy: 0, raw_only: 0 },
      revisions: ["v1.19.25"],
      href: "/v1/capabilities/formats/render/mihomo-proxies",
    },
  ],
};
const uiCapabilities = {
  features: [
    { key: "probe.enabled", enabled: true },
    { key: "scheduler.enabled", enabled: true },
    { key: "core.mihomo", enabled: true },
    { key: "core.sing_box", enabled: true },
  ],
};

function settingsEnvelope(ignoredWarnings: Array<{ code: string; field?: string; source?: string; target?: string }> = []) {
  const settings = {
    schema_version: 1,
    http: { listen: "127.0.0.1:1137" },
    mcp: { path: "/mcp", max_output_bytes: 1048576 },
    log: { level: "info" },
    remote_defaults: { timeout_ms: 15000 },
    probe_defaults: {
      method: "url_test",
      core: "sing-box",
      url: "https://cp.cloudflare.com",
      ntp_server: "time.apple.com",
      timeout_ms: 5000,
      attempts: 1,
      concurrency: 10,
    },
    script_defaults: { timeout_ms: 2000 },
    cache_defaults: {
      remote_fetch_ttl_seconds: 0,
    probe_ttl_seconds: 0,
    subscription_snapshot_ttl_seconds: 0,
    },
    appearance: { theme_mode: "dark", locale: "zh-CN" },
    subscriptions: { auto_load_traffic: false, ignored_warnings: ignoredWarnings },
    scheduled_refresh: { enabled: false, schedule: "@every 10m", targets: [] },
  };
  return {
    settings,
    effective: settings,
    overrides: {},
    restart_required: [],
  };
}

test.beforeEach(async ({ page }) => {
  let ignoredWarnings: Array<{ code: string; field?: string; source?: string; target?: string }> = [];
  await page.addInitScript(() => {
    localStorage.setItem("sandrone.locale", "zh-CN");
    localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
  });
  await page.route("**/v1/subscriptions", async (route) => {
    await route.fulfill({ json: { items: manifest.subscriptions } });
  });
  await page.route("**/v1/subscriptions/provider/preview", async (route) => {
    await route.fulfill({ json: subscriptionPreview });
  });
  await page.route("**/v1/files", async (route) => {
    await route.fulfill({ json: { items: manifest.files } });
  });
  await page.route("**/v1/shares", async (route) => {
    await route.fulfill({ json: { shares: [] } });
  });
  await page.route("**/v1/capabilities/formats", async (route) => {
    await route.fulfill({ json: formatCapabilities });
  });
  await page.route("**/v1/capabilities/ui", async (route) => {
    await route.fulfill({ json: uiCapabilities });
  });
  await page.route("**/v1/rule-set-catalog?target=*", async (route) => {
    await route.fulfill({ json: { items: [] } });
  });
  await page.route("**/v1/settings", async (route) => {
    if (route.request().method() === "PUT") {
      const update = route.request().postDataJSON() as {
        subscriptions?: { ignored_warnings?: typeof ignoredWarnings };
      };
      ignoredWarnings = update.subscriptions?.ignored_warnings ?? ignoredWarnings;
    }
    await route.fulfill({ json: settingsEnvelope(ignoredWarnings) });
  });
  await page.route("**/v1/settings/scheduled-refresh-status", async (route) => {
    await route.fulfill({
      json: {
        enabled: false,
        running: false,
        last_success_count: 0,
        last_failure_count: 0,
        skipped_count: 0,
      },
    });
  });
  await page.route("**/v1/cache**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (request.method() === "DELETE" && path === "/v1/cache") {
      await route.fulfill({ status: 204 });
      return;
    }
    await route.fallback();
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
  { path: "/subscriptions/new?type=local", heading: "新建订阅", content: "内容", focus: true },
  { path: "/subscriptions/remote/provider/preview", heading: "节点预览", content: longPreviewNode, focus: true },
  { path: "/files", heading: "我的文件", content: "default.yaml", focus: false },
  { path: "/files/new?source=mihomo", heading: "新建文件", content: "节点来源", focus: true },
  { path: "/files/default.yaml/preview", heading: "文件预览", content: longPreviewNode, focus: true },
  { path: "/shares", heading: "分享", content: "还没有分享链接", focus: false },
  { path: "/settings/service", heading: "服务设置", content: "远程请求", focus: false },
  { path: "/settings/data", heading: "数据管理", content: "缓存", focus: false },
];

test("settings overview opens service settings", async ({ page }) => {
  const consoleIssues: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });
  await page.goto("/settings");

  await page.getByRole("button", { name: "打开服务设置" }).click();

  await expect(page).toHaveURL(/\/settings\/service$/);
  await expect(page.getByRole("heading", { exact: true, level: 2, name: "服务设置" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "远程请求" })).toBeVisible();
  expect(consoleIssues).toEqual([]);
});

test("ignored subscription warnings persist and can be restored", async ({ page }) => {
  const consoleIssues: string[] = [];
  page.on("console", (message) => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });
  await page.goto("/subscriptions/remote/provider/preview");

  await page.getByRole("button", { name: "展开预览警告" }).click();
  await page.getByRole("button", { name: /node-with-known-mode-field/ }).click();
  const ignoredRequest = page.waitForRequest((request) => (
    request.method() === "PUT" && new URL(request.url()).pathname === "/v1/settings"
  ));
  await page.getByRole("button", { name: "忽略同类警告" }).click();
  const ignoredBody = (await ignoredRequest).postDataJSON() as {
    subscriptions: { ignored_warnings: unknown[] };
  };

  expect(ignoredBody.subscriptions.ignored_warnings).toEqual([{
    code: "parse_unknown_field",
    field: "uri.query.mode",
    source: "uri-list",
  }]);
  await expect(page.getByRole("button", { name: "展开预览警告" })).toHaveCount(0);
  await page.reload();
  await expect(page.getByRole("button", { name: "展开预览警告" })).toHaveCount(0);

  await page.goto("/settings/service");
  const ignoredWarnings = page.getByRole("article").filter({
    has: page.getByRole("heading", { name: "已忽略的订阅警告" }),
  });
  await expect(ignoredWarnings).toContainText("uri.query.mode");
  await expect(ignoredWarnings).toContainText("1 条规则");
  await ignoredWarnings.getByRole("button", { name: "恢复 uri.query.mode" }).click();
  const restoredRequest = page.waitForRequest((request) => (
    request.method() === "PUT" && new URL(request.url()).pathname === "/v1/settings"
  ));
  await page.getByRole("button", { name: "保存设置" }).click();
  const restoredBody = (await restoredRequest).postDataJSON() as {
    subscriptions: { ignored_warnings: unknown[] };
  };
  expect(restoredBody.subscriptions.ignored_warnings).toEqual([]);

  await page.goto("/subscriptions/remote/provider/preview");
  await expect(page.getByRole("button", { name: "展开预览警告" })).toBeVisible();
  expect(consoleIssues).toEqual([]);
});

for (const route of routes) {
  test(`${route.path} renders without horizontal overflow`, async ({ page }, testInfo) => {
    const consoleIssues: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error" || message.type() === "warning") {
        consoleIssues.push(`${message.type()}: ${message.text()}`);
      }
    });

    await page.goto(route.path);

    await expect(page.getByRole("heading", { exact: true, name: route.heading, level: 2 })).toBeVisible();
    const routeContent = route.path === "/files/new?source=mihomo"
      ? page.getByRole("group", { name: route.content }).first()
      : route.path === "/files/default.yaml/preview"
        ? page.getByRole("region", { name: "最终文件内容" })
        : route.path === "/settings/service" || route.path === "/settings/data"
          ? page.getByRole("heading", { exact: true, name: route.content })
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
    if (route.path === "/subscriptions/remote/provider/preview") {
      await expect(page.getByText("42 ms")).toBeVisible();
      const previewFilters = page.getByLabel("节点状态筛选");
      const filterMetrics = await previewFilters.getByRole("button").evaluateAll((buttons) => ({
        tops: buttons.map((button) => Math.round(button.getBoundingClientRect().top)),
        widths: buttons.map((button) => button.getBoundingClientRect().width),
      }));
      expect(filterMetrics.tops).toHaveLength(5);
      expect(new Set(filterMetrics.tops).size, "preview filters should stay on one row").toBe(1);
      expect(Math.max(...filterMetrics.widths) - Math.min(...filterMetrics.widths), "preview filters should have equal widths").toBeLessThanOrEqual(1);
      if (testInfo.project.name === "mobile") {
        const previewSummary = page.getByLabel("预览统计");
        const summaryMetrics = await previewSummary.locator("strong").evaluateAll((metrics) => ({
          height: metrics[0]?.parentElement?.parentElement?.parentElement?.getBoundingClientRect().height ?? 0,
          tops: metrics.map((metric) => Math.round(metric.getBoundingClientRect().top)),
        }));
        expect(new Set(summaryMetrics.tops).size, "preview metrics should stay on one mobile row").toBe(1);
        expect(summaryMetrics.height, "preview metrics should stay compact on mobile").toBeLessThanOrEqual(80);
      }
      const searchbox = page.getByRole("searchbox", { name: "搜索预览节点" });
      await searchbox.fill("not-found");
      await expect(page.getByRole("heading", { name: "没有匹配节点" })).toBeVisible();
      await expect(page.getByText(longPreviewNode)).toBeHidden();
      await searchbox.fill("example.com");
      await expect(page.getByText(longPreviewNode)).toBeVisible();
    }
    if (route.path === "/files") {
      await expect(page.getByText("main config")).toBeHidden();
    }
    if (route.path === "/shares" && testInfo.project.name === "mobile") {
      const shareMetrics = await page.getByLabel("分享链接摘要").getByRole("button").evaluateAll((buttons) => ({
        tops: buttons.map((button) => Math.round(button.getBoundingClientRect().top)),
        widths: buttons.map((button) => button.getBoundingClientRect().width),
      }));
      expect(shareMetrics.tops).toHaveLength(4);
      expect(new Set(shareMetrics.tops).size, "share metrics should stay on one mobile row").toBe(1);
      expect(Math.max(...shareMetrics.widths) - Math.min(...shareMetrics.widths), "share metrics should have equal widths").toBeLessThanOrEqual(1);
    }
    if (route.path === "/subscriptions/new?type=local") {
      const addProcessor = page.getByRole("button", { name: "添加处理器" });
      await addProcessor.click();
      await addProcessor.click();
      const processorCards = page.getByRole("group", { name: "处理器 过滤" });
      await expect(processorCards).toHaveCount(2);
      const secondProcessor = processorCards.nth(1);
      const enabledButton = secondProcessor.getByRole("button", { name: "启用 处理器 2" });
      await expect(enabledButton).toHaveAttribute("aria-pressed", "true");
      await enabledButton.click();
      await expect(enabledButton).toHaveAttribute("aria-pressed", "false");
      const moreActions = secondProcessor.getByRole("button", { name: "更多处理器操作：处理器 2" });
      if (testInfo.project.name === "mobile") {
        await expect(moreActions).toBeVisible();
        await moreActions.click();
        await expect(page.getByRole("menuitem", { name: "编辑名称" })).toBeVisible();
        await expect(page.getByRole("menuitem", { name: "删除处理器" })).toBeVisible();
        await page.getByRole("menuitem", { name: "编辑名称" }).click();
        await expect(secondProcessor.getByRole("textbox", { name: "名称" })).toBeVisible();
      } else {
        await expect(moreActions).toBeHidden();
        await expect(secondProcessor.getByRole("button", { name: "编辑名称" })).toBeVisible();
      }
      const processorMetrics = await secondProcessor.evaluate((card) => ({
        clientWidth: card.clientWidth,
        scrollWidth: card.scrollWidth,
      }));
      expect(processorMetrics.scrollWidth, "processor actions should fit within their card")
        .toBeLessThanOrEqual(processorMetrics.clientWidth + 1);
    }

    const pageMetrics = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }));
    expect(pageMetrics.scrollWidth, `${testInfo.project.name} ${route.path} should not scroll horizontally`)
      .toBeLessThanOrEqual(pageMetrics.clientWidth + 1);

    if (testInfo.project.name === "mobile") {
      const undersizedInputs = await page.locator("input:not([type=hidden]):not([type=checkbox]):not([type=radio]):not([type=file]), textarea, select").evaluateAll((elements) => (
        elements
          .filter((element) => {
            const style = getComputedStyle(element);
            return style.display !== "none" && style.visibility !== "hidden" && element.getBoundingClientRect().width > 0;
          })
          .map((element) => ({
            fontSize: Number.parseFloat(getComputedStyle(element).fontSize),
            label: element.getAttribute("aria-label") || element.getAttribute("name") || element.tagName.toLowerCase(),
          }))
          .filter(({ fontSize }) => fontSize < 16)
      ));
      expect(undersizedInputs, `${route.path} inputs should not trigger iOS focus zoom`).toEqual([]);

      if (route.path === "/subscriptions/new?type=local") {
        const contentInput = page.getByRole("textbox", { name: "内容" });
        const viewportBeforeFocus = await page.evaluate(() => ({
          clientWidth: document.documentElement.clientWidth,
          scale: window.visualViewport?.scale ?? 1,
        }));
        await contentInput.focus();
        await expect(contentInput).toBeFocused();
        await expect.poll(() => contentInput.evaluate((element) => Number.parseFloat(getComputedStyle(element).fontSize))).toBeGreaterThanOrEqual(16);
        expect(await page.evaluate(() => ({
          clientWidth: document.documentElement.clientWidth,
          scale: window.visualViewport?.scale ?? 1,
        }))).toEqual(viewportBeforeFocus);
      }
    }

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

test("cache cleanup requires confirmation and completes", async ({ page }) => {
  const cacheCard = page.getByRole("article").filter({ has: page.getByRole("heading", { name: "缓存" }) });
  await page.goto("/settings/data");

  await cacheCard.getByRole("button", { name: "清空缓存" }).click();
  const dialog = page.getByRole("dialog", { name: "清空缓存？" });
  await expect(dialog).toContainText("后续请求可能暂时变慢");
  const request = page.waitForRequest((candidate) => (
    candidate.method() === "DELETE" && new URL(candidate.url()).pathname === "/v1/cache"
  ));
  await dialog.getByRole("button", { name: "清空缓存" }).click();
  await request;
  await expect(dialog).toBeHidden();
  await expect(page.getByText("缓存已清空")).toBeVisible();

  const pageMetrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(pageMetrics.scrollWidth).toBeLessThanOrEqual(pageMetrics.clientWidth + 1);
});

test("the convert link dialog stays usable and does not execute the generated URL", async ({ page }) => {
  const convertRequests: string[] = [];
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.pathname === "/convert") convertRequests.push(request.url());
  });
  await page.addInitScript(() => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText(value: string) {
          sessionStorage.setItem("copied-convert-link", value);
          return Promise.resolve();
        },
      },
    });
  });
  await page.goto("/shares");
  await page.getByRole("button", { name: "转换链接" }).click();

  const dialog = page.getByRole("dialog", { name: "生成转换链接" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("combobox", { name: "输出格式" })).toHaveValue("base64");
  const sourceURL = "https://subscription.example/nodes?token=a+b&name=HK#primary";
  await dialog.getByRole("textbox", { name: "远程订阅 URL" }).fill(sourceURL);
  const expected = "https://example.com/convert?url=https%3A%2F%2Fsubscription.example%2Fnodes%3Ftoken%3Da%2Bb%26name%3DHK%23primary&to_format=base64";
  await expect(dialog.getByText(expected)).toBeVisible();
  await dialog.getByRole("button", { name: "复制完整链接" }).click();

  await expect.poll(() => page.evaluate(() => sessionStorage.getItem("copied-convert-link"))).toBe(expected);
  expect(convertRequests).toEqual([]);
  const pageMetrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(pageMetrics.scrollWidth).toBeLessThanOrEqual(pageMetrics.clientWidth + 1);
});
