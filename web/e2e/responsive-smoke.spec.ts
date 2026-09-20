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
    http: { listen: "127.0.0.1:19137" },
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
      fetch_ttl_seconds: 0,
    probe_ttl_seconds: 0,
    snapshot_ttl_seconds: 0,
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
  await page.route("**/v1/nodes/inspect", async (route) => {
    const request = route.request().postDataJSON() as { include?: string[] };
    if (request.include?.includes("ip")) {
      await route.fulfill({
        json: {
          ip: {
            server: "example.com",
            ip: "203.0.113.10",
            ip_version: 4,
            public: true,
            country_code: "US",
            country: "United States",
            continent_code: "NA",
            continent: "North America",
            asn: "AS64500",
            as_name: "Example Network",
            as_domain: "example.net",
            source: { name: "ipwho.is", url: "https://ipwho.is" },
          },
        },
      });
      return;
    }
    await route.fulfill({
      json: {
        uri: {
          value: "ss://fixture-node-uri",
          warnings: [],
        },
      },
    });
  });
  await page.route("**/v1/rule-set-catalog?target=*", async (route) => {
    await route.fulfill({ json: { items: [] } });
  });
  await page.route("**/v1/logs", async (route) => {
    await route.fulfill({ json: {
      instance_id: "runtime-example", snapshot_time: "2026-09-06T01:02:03.456Z", level: "info",
      max_entries: 1000, max_bytes: 2097152, max_entry_bytes: 8192, dropped: 0,
      entries: [{ id: 1, time: "2026-09-06T01:02:01.123Z", level: "info", message: "service render completed", attrs: { operation: "render", duration_ms: 12 }, truncated: false }],
    } });
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
  { project: "mobile", path: "/subscriptions/new?type=local", heading: "新建订阅", content: "内容", focus: true },
  { project: "mobile", path: "/subscriptions/remote/provider/preview", heading: "节点预览", content: longPreviewNode, focus: true },
  { project: "mobile", path: "/files/new?source=mihomo", heading: "新建文件", content: "节点来源", focus: true },
  { project: "mobile", path: "/shares", heading: "分享", content: "还没有分享链接", focus: false },
  { project: "mobile", path: "/settings/data", heading: "数据管理", content: "缓存", focus: false },
  { project: "desktop", path: "/subscriptions/new?type=local", heading: "新建订阅", content: "内容", focus: true },
];

test("sing-box regex groups add a visible processor and allow its removal", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile", "covered by the mobile editor smoke flow");
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/files/new?source=sing-box");
  const groups = page.getByRole("button", { name: "代理组", exact: true });
  if (await groups.getAttribute("aria-expanded") === "false") await groups.click();
  await page.getByRole("button", { name: "添加代理组", exact: true }).click();
  const customGroup = page.getByRole("button", { name: /代理组 自定义$/ }).last();
  await expect(customGroup).toBeVisible();
  if (await customGroup.getAttribute("aria-expanded") === "false") await customGroup.click();
  const memberSource = page.getByRole("combobox", { name: "成员来源", exact: true });
  await expect(memberSource).toBeVisible();
  await memberSource.click();
  await page.getByRole("option", { name: "正则筛选", exact: true }).click();
  const pattern = page.getByRole("textbox", { name: "包含正则", exact: true });
  await expect(pattern).toHaveValue(".*");
  await pattern.fill("(?i)HK|香港");
  const processorsInput = page.locator('input[name="processors"]');
  await expect.poll(async () => JSON.parse(await processorsInput.inputValue())[0]?.name).toBe("出站配置适配");
  const card = page.getByRole("group", { name: "处理器 出站配置适配", exact: true });
  await expect(card).toHaveCount(1);
  await card.getByRole("button", { name: "启用 出站配置适配", exact: true }).click();
  const notice = page.getByText(/正则分组需要启用/);
  await expect(notice).toBeVisible();
  await card.getByRole("button", { name: "启用 出站配置适配", exact: true }).click();
  await expect(notice).toHaveCount(0);
  await card.screenshot({ path: testInfo.outputPath("regex-group-processor.png") });
  if (testInfo.project.name === "mobile") {
    await card.getByRole("button", { name: "更多处理器操作：出站配置适配" }).click();
    await page.getByRole("menuitem", { name: "删除处理器" }).click();
  } else {
    await card.getByRole("button", { name: "删除处理器" }).click();
  }
  await expect(notice).toBeVisible();
  await pattern.fill("JP");
  await expect(card).toHaveCount(0);
  const config = JSON.parse(await page.locator('input[name="config"]').inputValue());
  expect(config.settings.groups.at(-1)).toMatchObject({ filter: "JP", outbounds: ["$nodes"] });
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(1);
  expect(errors).toEqual([]);
});

for (const route of routes) {
  test(`${route.project} ${route.path} renders without horizontal overflow`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== route.project, `covered by the ${route.project} smoke flow`);
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
      const nameBounds = await page.getByText(longPreviewNode, { exact: true }).boundingBox();
      const probeBounds = await page.getByText("42 ms", { exact: true }).boundingBox();
      expect(nameBounds).not.toBeNull();
      expect(probeBounds).not.toBeNull();
      expect(nameBounds!.x + nameBounds!.width, "node names must not overlap probe latency")
        .toBeLessThanOrEqual(probeBounds!.x);
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
      const processorCards = page.getByRole("group", { name: "处理器 脚本" });
      await expect(processorCards).toHaveCount(2);
      await expect(page.getByRole("group", { name: "处理链", exact: true })
        .locator('[data-slot="count-badge"]')).toHaveText("2");
      const firstProcessor = processorCards.nth(0);
      const secondProcessor = processorCards.nth(1);
      const firstDisclosure = firstProcessor.getByRole("button", { name: "收起处理器 1" });
      const secondDisclosure = secondProcessor.getByRole("button", { name: "收起处理器 2" });
      await firstDisclosure.click();
      await expect(firstProcessor.getByRole("button", { name: "展开处理器 1" }))
        .toHaveAttribute("aria-expanded", "false");
      await expect(firstProcessor.getByRole("separator")).toHaveCount(0);
      await expect(secondDisclosure).toHaveAttribute("aria-expanded", "true");
      const enabledButton = secondProcessor.getByRole("button", { name: "启用 处理器 2" });
      for (const [card, disclosure, enabled] of [
        [firstProcessor, firstProcessor.getByRole("button", { name: "展开处理器 1" }), firstProcessor.getByRole("button", { name: "启用 处理器 1" })],
        [secondProcessor, secondDisclosure, enabledButton],
      ] as const) {
        const centers = await disclosure.or(enabled).evaluateAll((buttons) => buttons.map((button) => {
          const bounds = button.getBoundingClientRect();
          return bounds.y + bounds.height / 2;
        }));
        expect(centers).toHaveLength(2);
        expect(Math.abs(centers[0] - centers[1]), "processor header icons should be vertically centered")
          .toBeLessThanOrEqual(1);
        await expect(card.locator('[data-slot="disclosure-indicator"]')).toBeVisible();
      }
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
