import { expect, type Locator, test } from "@playwright/test";

const manifest = {
  subscriptions: [
    { name: "provider", type: "remote", format: "uri-list", meta: { description: "daily" } },
    { name: "default", type: "collection", meta: { node_count: "12", source_count: "1" } },
  ],
  files: [{ name: "default.yaml", type: "inline", target: "mihomo", format: "yaml", meta: { description: "main config" } }],
  shares: [{
    id: "sh_123",
    name: "mobile",
    target_kind: "subscription",
    target_name: "provider",
    target_format: "uri-list",
    public_filename: "mobile.txt",
    format_filenames: {
      "base64": "mobile.txt",
      "uri-list": "mobile.txt",
      "mihomo-proxies": "mobile.yaml",
      "sing-box-outbounds": "mobile.json",
      "shadowrocket-proxies": "mobile.conf",
      "json-nodes": "mobile.json",
    },
  }],
};

const subscriptionDetail = {
  name: "provider",
  type: "remote",
  format: "uri-list",
  remote: {
    url: "https://example.com/sub",
    user_agent: "Sandrone Test",
    proxy: "http://127.0.0.1:7890",
    timeout_ms: 10000,
  },
  processors: [],
  meta: { description: "daily", ui: "web" },
};

const collectionDetail = {
  name: "default",
  type: "collection",
  inputs: [
    { name: "provider", type: "subscription", ref: { kind: "subscription", name: "provider" } },
  ],
  processors: [],
  meta: { description: "default collection", ui: "web" },
};

const fileDetail = {
  name: "default.yaml",
  kind: "mihomo",
  source: {},
  config: {
    subscriptions: ["provider"],
    settings: {
      groups: [
        { name: "Proxy", type: "select", proxies: ["Auto", "$nodes", "DIRECT"] },
        { name: "Auto", type: "url-test", proxies: ["$nodes"], url: "http://www.gstatic.com/generate_204", interval: 300 },
        { name: "Final", type: "select", proxies: ["Proxy", "DIRECT"] },
      ],
      rule_sets: [{ name: "private", type: "inline", behavior: "classical", payload: ["DOMAIN-SUFFIX,local"] }],
      rules: ["RULE-SET,private,DIRECT", "MATCH,Final"],
    },
  },
  processors: [],
  meta: { description: "main config" },
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
    "rule-providers:",
    "  private:",
    "    type: inline",
    "    behavior: classical",
    "    payload: [DOMAIN-SUFFIX,local]",
    "rules:",
    "  - RULE-SET,private,DIRECT",
    "  - MATCH,Proxy",
    ...Array.from({ length: 80 }, (_, index) => `# generated line ${index + 1}`),
  ].join("\n"),
  warnings: [{ code: "file_warning", message: "preview warning" }],
};

const fileSource = {
  content_type: "application/yaml",
  body: "mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\nproxies: []\nproxy-groups: []\nrule-providers: {}\nrules: []",
};

const previewBeforeName = "keep-node-with-an-extremely-long-original-identity-segment-abcdefghijklmnopqrstuvwxyz-0123456789";
const previewAfterName = "source-node-with-an-extremely-long-renamed-identity-segment-abcdefghijklmnopqrstuvwxyz-9876543210";

const subscriptionPreview = {
  subscription_name: "provider",
  format: "uri-list",
  before_count: 1,
  after_count: 1,
  status_counts: { added: 0, modified: 1, removed: 0, unchanged: 0 },
  warnings: [{ code: "processor_warning", message: "preview warning" }],
  nodes: [
    {
      identity: "sha256:one",
      status: "modified",
      before: { name: previewBeforeName, type: "ss", server: "example.com", port: 8388 },
      after: {
        name: previewAfterName,
        type: "ss",
        server: "example.com",
        port: 8388,
        password: "a".repeat(320),
      },
    },
  ],
};

test.beforeEach(async ({ page }) => {
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"], { origin: "http://127.0.0.1:4173" });
  await page.route("**/v1/subscriptions", async (route) => {
    await route.fulfill({ json: { items: manifest.subscriptions } });
  });
  await page.route("**/v1/files", async (route) => {
    await route.fulfill({ json: { items: manifest.files } });
  });
  await page.route("**/v1/shares", async (route) => {
    if (route.request().method() === "POST") {
      await route.fulfill({
        status: 201,
        json: {
          share: {
            id: "sh_created",
            name: "provider",
            target_kind: "subscription",
            target_name: "provider",
            target_format: "uri-list",
            public_filename: "provider.txt",
          },
        },
      });
      return;
    }
    await route.fulfill({ json: { shares: manifest.shares } });
  });
  await page.route("**/v1/settings", async (route) => {
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
        url: "http://www.gstatic.com/generate_204",
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
    await route.fulfill({
      json: {
        settings,
        effective: settings,
        overrides: {},
        restart_required: [],
      },
    });
  });
  await page.route("**/v1/subscriptions/provider/preview", async (route) => {
    await route.fulfill({ json: subscriptionPreview });
  });
  await page.route("**/v1/subscriptions/provider/traffic", async (route) => {
    await route.fulfill({
      json: {
        subscription_name: "provider",
        type: "remote",
        format: "uri-list",
        traffic: { upload_bytes: 1024, download_bytes: 2048, used_bytes: 3072, total_bytes: 10240 },
      },
    });
  });
  await page.route("**/v1/subscriptions/provider", async (route) => {
    await route.fulfill({ json: subscriptionDetail });
  });
  await page.route("**/v1/subscriptions/default", async (route) => {
    await route.fulfill({ json: collectionDetail });
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
    const url = new URL(route.request().url());
    const mode = url.searchParams.get("mode");
    await route.fulfill({ json: mode === "spec" ? fileDetail : mode === "source" ? fileSource : filePreview });
  });
});

const routes = [
  { path: "/", heading: "我的订阅", text: "default", focus: false, responsive: true },
  { path: "/subscriptions", heading: "我的订阅", text: "default", focus: false, responsive: true },
  { path: "/subscriptions/collection/default/edit", heading: "编辑订阅", text: "组合信息", focus: true, responsive: false },
  { path: "/subscriptions/remote/provider/edit", heading: "编辑订阅", text: "基本信息", focus: true, responsive: true },
  { path: "/files", heading: "我的文件", text: "default.yaml", focus: false, responsive: true },
  { path: "/files/new?source=mihomo", heading: "新建文件", text: "节点来源", focus: true, responsive: false },
  { path: "/files/new?source=sing-box", heading: "新建文件", text: "基础配置", focus: true, responsive: false },
  { path: "/files/new?source=shadowrocket", heading: "新建文件", text: "基础配置", focus: true, responsive: false },
  { path: "/files/default.yaml/edit", heading: "编辑文件", text: "配置模板", focus: true, responsive: true },
  { path: "/files/default.yaml/preview", heading: "文件预览", text: longPreviewNode, focus: true, responsive: true },
  { path: "/shares", heading: "分享", text: "https://example.com/s/sh_123/mobile.txt?format=uri-list", focus: false, responsive: true },
  { path: "/settings", heading: "设置", text: "高级设置", focus: false, responsive: true },
  { path: "/settings/runtime", heading: "高级设置", text: "远程请求", focus: false, responsive: true },
  { path: "/settings/data", heading: "数据管理", text: "备份是未加密的明文", focus: false, responsive: true },
];

const smokeRoutes = new Set([
  "/subscriptions",
  "/files",
  "/files/new?source=mihomo",
  "/files/default.yaml/preview",
  "/settings/runtime",
]);

for (const route of routes) {
  const smokeTag = smokeRoutes.has(route.path) ? "@smoke " : "";
  test(`${smokeTag}${route.path} renders without horizontal overflow`, async ({ page }, testInfo) => {
    const isDesktop = testInfo.project.name === "desktop" || testInfo.project.name === "smoke";
    test.skip(!route.responsive && !isDesktop, "route smoke runs once; responsive routes run at every viewport");

    const consoleIssues: string[] = [];
    page.on("console", (message) => {
      if (message.type() === "error" || message.type() === "warning") {
        consoleIssues.push(`${message.type()}: ${message.text()}`);
      }
    });
    await page.addInitScript(() => {
      localStorage.setItem("sandrone.locale", "zh-CN");
      localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
    });
    const versionResponse = route.path === "/settings"
      ? page.waitForResponse((response) => new URL(response.url()).pathname === "/version")
      : null;
    await page.goto(route.path);

    await expect(page.getByRole("heading", { name: route.heading, level: 2 })).toBeVisible();
    const initialFocusHeaderHeight = route.focus && testInfo.project.name === "mobile"
      ? await page.locator("[data-page-header-compact]").evaluate((header) => header.getBoundingClientRect().height)
      : null;
    if (testInfo.project.name === "mobile" && route.path === "/files/default.yaml/edit") {
      await expect(page.getByRole("button", { name: "保存文件" })).toBeVisible();
      await expect(page.getByRole("button", { name: "预览文件" })).toBeHidden();
      await expect(page.getByRole("button", { name: "分享文件" })).toBeHidden();
      const more = page.getByRole("button", { exact: true, name: "更多操作" });
      await expect(more).toBeVisible();
      await more.click();
      await expect(page.getByRole("menuitem", { name: "预览文件" })).toBeVisible();
      await expect(page.getByRole("menuitem", { name: "分享文件" })).toBeVisible();
      await page.keyboard.press("Escape");
    }
    if (testInfo.project.name === "mobile" && route.path === "/subscriptions/remote/provider/edit") {
      await expect(page.getByRole("button", { name: "保存订阅" })).toBeVisible();
      await expect(page.getByRole("button", { name: "预览订阅" })).toBeHidden();
      await expect(page.getByRole("button", { name: "分享订阅" })).toBeHidden();
      const more = page.getByRole("button", { exact: true, name: "更多操作" });
      await expect(more).toBeVisible();
      await more.click();
      await expect(page.getByRole("menuitem", { name: "预览订阅" })).toBeVisible();
      await expect(page.getByRole("menuitem", { name: "分享订阅" })).toBeVisible();
      await page.keyboard.press("Escape");
    }
    const routeContent = route.path === "/files/default.yaml/preview"
      ? page.getByRole("region", { name: "最终文件内容" })
      : route.path === "/files/default.yaml/edit"
        ? page.getByRole("group", { name: route.text })
        : route.path === "/settings/runtime"
          ? page.getByRole("button", { name: route.text })
          : route.path === "/files/new?source=mihomo"
            ? page.getByRole("group", { name: "节点来源" }).first()
            : route.path === "/files/new?source=sing-box" || route.path === "/files/new?source=shadowrocket"
              ? page.getByRole("group", { name: "基础配置" }).first()
              : page.getByText(route.text);
    await expect(routeContent).toBeVisible();
    if (route.focus) {
      await expect(page.locator("[data-page-header-compact]")).toHaveAttribute("data-page-header-compact", "false");
    }

    if (route.path === "/subscriptions") {
      await expect(page.getByRole("button", { name: "新建订阅" })).toBeVisible();
      const searchBounds = await page.getByRole("searchbox", { name: "搜索订阅" }).evaluate((input) => {
        const root = input.closest(".MuiTextField-root");
        const icon = root?.querySelector(".MuiInputAdornment-root svg");
        const label = root?.querySelector("label");
        const box = (element?: Element | null) => {
          const rect = element?.getBoundingClientRect();
          return rect ? { x: rect.x, width: rect.width } : null;
        };
        return { icon: box(icon), label: box(label) };
      });
      expect(searchBounds.label?.x ?? 0, "subscription search label should not overlap the search icon").toBeGreaterThanOrEqual((searchBounds.icon?.x ?? 0) + (searchBounds.icon?.width ?? 0) + 4);
      await page.getByRole("button", { name: "provider 更多操作" }).click();
      await expect(page.getByRole("menuitem", { name: "编辑" })).toBeVisible();
      if (testInfo.project.name === "mobile") {
        await page.getByRole("menuitem", { name: "分享" }).click();
        const create = page.getByRole("dialog", { name: "创建分享链接" });
        await create.getByRole("button", { name: "保存分享链接" }).click();

        const result = page.getByRole("dialog", { name: "分享链接已创建" });
        const publicUrl = "https://example.com/s/sh_created/provider.txt?format=uri-list";
        await expect(result.getByText(publicUrl)).toBeVisible();
        const dialogMetrics = await result.evaluate((dialog) => ({
          clientWidth: dialog.clientWidth,
          scrollWidth: dialog.scrollWidth,
        }));
        expect(dialogMetrics.scrollWidth).toBeLessThanOrEqual(dialogMetrics.clientWidth);

        await result.getByRole("button", { name: "复制链接" }).click();
        await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(publicUrl);
        await result.getByRole("button", { name: "完成" }).click();
        await expect(page.getByRole("heading", { name: "我的订阅", level: 2 })).toBeVisible();
      } else {
        await page.keyboard.press("Escape");
      }
      expect(consoleIssues).toEqual([]);
    }
    if (route.path === "/files") {
      await page.getByRole("button", { name: "default.yaml 更多操作" }).click();
      await expect(page.getByRole("menuitem", { name: "编辑" })).toBeVisible();
      await expect(page.getByRole("menuitem", { name: "分享" })).toBeVisible();
      await page.keyboard.press("Escape");
    }
    if ([
      "/files/new?source=mihomo",
      "/files/new?source=sing-box",
      "/files/new?source=shadowrocket",
    ].includes(route.path)) {
      await page.getByRole("combobox", { exact: true, name: "类型" }).click();
      await expect(page.getByRole("option", { name: "合并" })).toBeVisible();
      await page.keyboard.press("Escape");
    }
    if (route.path === "/files/new?source=mihomo") {
      await expect(page.getByPlaceholder("mixed-port: 7890")).toHaveValue(/mixed-port: 7890/);
      await page.getByRole("combobox", { name: "订阅" }).click();
      await page.getByRole("option", { name: "provider" }).click();
      await expect(page.getByText("已加载 1 个节点")).toBeVisible();
      await page.getByRole("button", { name: "展开代理组 🚀 节点选择" }).click();
      await page.getByRole("combobox", { name: "成员 1" }).click();
      await expect(page.getByRole("option", { name: new RegExp(previewAfterName) })).toBeVisible();
    }
    if (route.path === "/files/new?source=sing-box") {
      await expect(page.getByPlaceholder('{\n  "log": { "level": "info" }\n}')).toHaveValue(/"type": "tun"/);
    }
    if (route.path === "/files/new?source=shadowrocket") {
      await expect(page.getByPlaceholder("[General]\nipv6 = false")).toHaveValue(/\[Proxy Group\]/);
      await page.getByRole("combobox", { name: "订阅" }).click();
      await page.getByRole("option", { name: "provider" }).click();
      await expect(page.getByText("已加载 1 个节点")).toBeVisible();
      await expect(page.getByRole("button", { name: "展开代理组 🚀 节点选择" })).toBeVisible();
      await expect(page.getByRole("button", { name: "保存文件" })).toBeVisible();
    }
    if (route.path.startsWith("/files/") && route.path.endsWith("/edit")) {
      const templateSection = page.getByRole("group", { name: "配置模板" });
      await templateSection.getByRole("radio", { name: "标准" }).click();
      const dialog = page.getByRole("dialog", { name: "替换当前配置？" });
      await expect(dialog).toBeVisible();
      const replace = dialog.getByRole("button", { name: "替换当前配置" });
      await expect(replace).toHaveText("替换");
      await replace.click();
      await expect(page.getByRole("status").filter({ hasText: "配置已由模板替换" })).toBeVisible();
    }
    if (route.path.startsWith("/files/") && route.path.endsWith("/preview")) {
      await expect(page.getByText("preview warning")).toBeVisible();
      const sourceBlock = page.getByRole("region", { name: "最终文件内容" });
      await expect(sourceBlock).toContainText(longPreviewNode);
      await expect(page.getByRole("tab")).toHaveCount(0);
      const layout = await sourceBlock.evaluate((block) => {
        const shellContent = block.closest("main")?.querySelector(":scope > section");
        const pre = block.querySelector("pre");
        const blockBounds = block.getBoundingClientRect();
        return {
          blockBottom: blockBounds.bottom,
          expectedBottom: window.innerHeight - Number.parseFloat(getComputedStyle(shellContent!).paddingBottom),
          overflowY: pre ? getComputedStyle(pre).overflowY : "",
          preClientHeight: pre?.clientHeight ?? 0,
          preScrollHeight: pre?.scrollHeight ?? 0,
        };
      });
      expect(Math.abs(layout.blockBottom - layout.expectedBottom)).toBeLessThanOrEqual(1);
      expect(layout.overflowY).toBe("auto");
      expect(layout.preScrollHeight).toBeGreaterThan(layout.preClientHeight);
      const scrollTop = await sourceBlock.locator("pre").evaluate((pre) => {
        pre.scrollTop = pre.scrollHeight;
        return pre.scrollTop;
      });
      expect(scrollTop).toBeGreaterThan(0);
      expect(consoleIssues).toEqual([]);
    }
    if (route.path === "/shares") {
      const publicUrl = "https://example.com/s/sh_123/mobile.txt?format=uri-list";
      await page.getByText(publicUrl).click();
      await expect.poll(() => page.evaluate(() => window.getSelection()?.toString())).toBe(publicUrl);

      await page.getByRole("button", { name: "复制链接：mobile" }).click();
      await expect(page.getByRole("status")).toHaveText("已复制链接");
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(publicUrl);

      await page.getByRole("button", { name: "mobile 更多操作" }).click();
      const shareMenu = page.getByRole("menu");
      await expect(shareMenu.getByRole("menuitem")).toHaveText([
        "复制为通用订阅（Base64）",
        "复制为 URI list",
        "复制为 Mihomo",
        "复制为 sing-box",
        "复制为 Shadowrocket",
        "删除",
      ]);
      await page.evaluate(() => {
        Object.defineProperty(navigator, "clipboard", {
          configurable: true,
          value: undefined,
        });
      });
      await shareMenu.getByRole("menuitem", { name: "复制为通用订阅（Base64）" }).click();
      const attemptedUrl = "https://example.com/s/sh_123/mobile.txt?format=base64";
      const manualCopyDialog = page.getByRole("dialog", { name: "请手动复制链接" });
      await expect(manualCopyDialog.getByText(attemptedUrl)).toBeVisible();
      await expect.poll(() => page.evaluate(() => window.getSelection()?.toString())).toBe(attemptedUrl);
      const dialogMetrics = await manualCopyDialog.evaluate((dialog) => ({
        clientWidth: dialog.clientWidth,
        scrollWidth: dialog.scrollWidth,
      }));
      expect(dialogMetrics.scrollWidth, `${testInfo.project.name} manual copy dialog should not overflow`).toBeLessThanOrEqual(dialogMetrics.clientWidth);
      await manualCopyDialog.getByRole("button", { name: "完成" }).click();
      await expect(manualCopyDialog).toBeHidden();
      expect(consoleIssues).toEqual([]);
    }
    if (route.path === "/settings") {
      expect((await versionResponse)?.status()).toBe(200);
      await expect(page.getByRole("textbox", { name: "User-Agent" })).toHaveCount(0);
      await expect(page.getByRole("button", { name: "下载备份" })).toHaveCount(0);
      await expect(page.getByRole("note")).toHaveCount(0);
      const settingsColumnMetrics = await page.getByRole("heading", { level: 2, name: "设置" })
        .locator("xpath=ancestor::section[1]")
        .evaluate((section) => ({
        width: section.getBoundingClientRect().width,
        viewportWidth: document.documentElement.clientWidth,
      }));
      expect(settingsColumnMetrics.width, `${testInfo.project.name} settings column should fit the viewport`).toBeLessThanOrEqual(settingsColumnMetrics.viewportWidth);
      const themeLabel = page.locator("label.MuiInputLabel-root").filter({ hasText: /^主题模式$/u });
      const themeControl = page.getByRole("combobox", { name: "主题模式" });
      const languageLabel = page.locator("label.MuiInputLabel-root").filter({ hasText: /^语言$/u });
      const languageControl = page.getByRole("combobox", { name: "语言" });
      const baseUrlLabel = page.locator("label.MuiInputLabel-root").filter({ hasText: /^Public Base URL$/u });
      const baseUrlControl = page.getByRole("textbox", { name: "Public Base URL" });
      const saveBaseUrl = page.getByRole("button", { name: "保存服务地址" });
      const serviceCard = page.getByRole("heading", { name: "服务连接", level: 3 }).locator("xpath=ancestor::article[1]");
      const accountTitle = page.getByText("管理员 token", { exact: true });
      const signOut = page.getByRole("button", { name: "退出登录" });
      const bounds = async (locator: Locator) => {
        const box = await locator.boundingBox();
        expect(box).not.toBeNull();
        return box!;
      };
      for (const [label, control, name] of [
        [themeLabel, themeControl, "theme"],
        [languageLabel, languageControl, "language"],
        [baseUrlLabel, baseUrlControl, "Public Base URL"],
      ] as const) {
        const labelBox = await bounds(label);
        const controlBox = await bounds(control);
        expect(labelBox.y, `${testInfo.project.name} ${name} label should start above its field border`).toBeLessThanOrEqual(controlBox.y);
        expect(labelBox.y + labelBox.height, `${testInfo.project.name} ${name} label should overlap its field border`).toBeGreaterThan(controlBox.y);
      }

      if (testInfo.project.name === "mobile") {
        const accountBox = await bounds(accountTitle);
        const signOutBox = await bounds(signOut);
        expect(signOutBox.y, "mobile sign-out should stack below the account copy").toBeGreaterThanOrEqual(accountBox.y + accountBox.height);
        expect(Math.abs(signOutBox.x - accountBox.x), "mobile sign-out should align with the account copy").toBeLessThanOrEqual(1);
      } else {
        const cardBox = await bounds(serviceCard);
        const saveBox = await bounds(saveBaseUrl);
        expect(saveBox.x, `${testInfo.project.name} save action should align to the right side of its card`).toBeGreaterThan(cardBox.x + cardBox.width / 2);
        expect(saveBox.width, `${testInfo.project.name} save action should not span its card`).toBeLessThan(cardBox.width / 2);

        const accountBox = await bounds(accountTitle);
        const signOutBox = await bounds(signOut);
        expect(signOutBox.x, `${testInfo.project.name} sign-out should sit to the right of the account copy`).toBeGreaterThan(accountBox.x + accountBox.width);
        expect(Math.abs(signOutBox.y - accountBox.y), `${testInfo.project.name} account row should return to horizontal alignment`).toBeLessThanOrEqual(8);
      }
      expect(consoleIssues).toEqual([]);
    }
    if (route.path === "/settings/runtime") {
      await expect(page.getByRole("button", { name: "远程请求" })).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByRole("button", { name: "缓存" })).toHaveAttribute("aria-expanded", "false");
      await expect(page.getByRole("button", { name: "测活" })).toHaveAttribute("aria-expanded", "false");
      expect(consoleIssues).toEqual([]);
    }
    if (route.path === "/settings/data") {
      const dataManagement = page.getByRole("note").locator("xpath=ancestor::article[1]");
      await expect(dataManagement).toBeVisible();
      await expect(dataManagement.getByRole("note")).toContainText("备份是未加密的明文");
      await expect(dataManagement.getByRole("button", { name: "下载备份" })).toBeVisible();
      await expect(dataManagement.getByRole("button", { name: "选择 ZIP" })).toBeVisible();
      await expect(dataManagement.getByRole("button", { name: "恢复备份" })).toBeDisabled();
      const cardMetrics = await dataManagement.evaluate((card) => ({
        clientWidth: card.clientWidth,
        scrollWidth: card.scrollWidth,
      }));
      expect(cardMetrics.scrollWidth, `${testInfo.project.name} data management card should not overflow`).toBeLessThanOrEqual(cardMetrics.clientWidth);
      expect(consoleIssues).toEqual([]);
    }

    const metrics = await page.evaluate(() => {
      return {
        scrollWidth: document.documentElement.scrollWidth,
        clientWidth: document.documentElement.clientWidth,
      };
    });

    expect(metrics.scrollWidth, `${testInfo.project.name} ${route.path} should not scroll horizontally`).toBeLessThanOrEqual(metrics.clientWidth + 1);
    const bottomNav = page.getByRole("navigation", { name: "底部导航" });
    const drawer = page.getByRole("navigation", { name: "桌面导航" });
    if (route.focus) {
      const pageHeader = page.locator("[data-page-header-compact]");
      if (testInfo.project.name === "mobile" && route.path === "/files/default.yaml/edit") {
        await expect(page.getByRole("button", { name: "保存文件" })).toBeVisible();
        await expect(page.getByRole("button", { name: "预览文件" })).toBeHidden();
        await expect(page.getByRole("button", { name: "分享文件" })).toBeHidden();
        await expect(page.getByRole("button", { exact: true, name: "更多操作" })).toBeVisible();
      }
      if (testInfo.project.name === "mobile" && route.path === "/subscriptions/remote/provider/edit") {
        await expect(page.getByRole("button", { name: "保存订阅" })).toBeVisible();
        await expect(page.getByRole("button", { name: "预览订阅" })).toBeHidden();
        await expect(page.getByRole("button", { name: "分享订阅" })).toBeHidden();
        await expect(page.getByRole("button", { exact: true, name: "更多操作" })).toBeVisible();
      }
      await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
      const crossedStickyThreshold = await pageHeader.evaluate((header) => (header.previousElementSibling?.getBoundingClientRect().bottom ?? 0) < 0);
      await expect(pageHeader).toHaveAttribute("data-page-header-compact", crossedStickyThreshold ? "true" : "false");
      if (initialFocusHeaderHeight !== null) {
        const stickyHeaderHeight = await pageHeader.evaluate((header) => header.getBoundingClientRect().height);
        expect(stickyHeaderHeight).toBe(initialFocusHeaderHeight);
      }
      if (testInfo.project.name === "mobile" && route.path === "/files/default.yaml/edit" && crossedStickyThreshold) {
        const actionBounds = await pageHeader.evaluate((header) => {
          const buttons = Array.from(header.querySelectorAll("button"));
          const save = buttons.find((button) => button.textContent?.includes("保存"))?.getBoundingClientRect();
          const more = buttons.find((button) => button.getAttribute("aria-label") === "更多操作")?.getBoundingClientRect();
          const headerBounds = header.getBoundingClientRect();
          return save && more ? {
            headerLeft: headerBounds.left,
            headerRight: headerBounds.right,
            saveLeft: save.left,
            saveRight: save.right,
            moreLeft: more.left,
            moreRight: more.right,
          } : null;
        });
        expect(actionBounds).not.toBeNull();
        expect(actionBounds!.saveLeft).toBeGreaterThanOrEqual(actionBounds!.headerLeft);
        expect(actionBounds!.moreRight).toBeLessThanOrEqual(actionBounds!.headerRight);
        expect(actionBounds!.saveRight).toBeLessThanOrEqual(actionBounds!.moreLeft);
      }
      await expect(page.getByRole("heading", { name: route.heading, level: 2 })).toBeVisible();
      await expect(bottomNav).toHaveCount(0);
    } else if (testInfo.project.name === "mobile") {
      await expect(bottomNav).toBeVisible();
      await expect(drawer).toBeHidden();
    } else {
      await expect(bottomNav).toBeHidden();
      await expect(drawer).toBeVisible();
    }
  });
}

for (const kind of ["mihomo", "sing-box", "shadowrocket"] as const) {
  test(`${kind} config summaries wrap without chips and expose responsive row actions`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name === "tablet", "mobile and desktop cover the two responsive action layouts");

    await page.addInitScript(() => {
      localStorage.setItem("sandrone.locale", "zh-CN");
      localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
    });
    await page.goto(`/files/new?source=${kind}`);

    await expect(page.getByRole("heading", { name: "新建文件", level: 2 })).toBeVisible();
    const groups = page.getByRole("group", { exact: true, name: "代理组" });
    const ruleSets = page.getByRole("group", { exact: true, name: "规则集" });
    const rules = page.getByRole("group", { exact: true, name: "规则策略" });

    await expect(groups.getByRole("button", { name: "添加代理组" })).toBeVisible();
    const fromLibrary = ruleSets.getByRole("button", { name: "从规则集库添加" });
    await expect(fromLibrary).toHaveText("从库添加");
    await expect(ruleSets.getByRole("button", { name: "添加规则集" })).toBeVisible();
    await expect(page.getByRole("button", { name: "规则策略" }))
      .toHaveAttribute("aria-expanded", "true");
    await expect(rules.getByRole("button", { name: "添加规则" })).toBeVisible();

    for (const section of [groups, ruleSets, rules]) {
      const metrics = await section.evaluate((element) => ({
        clientWidth: element.clientWidth,
        scrollWidth: element.scrollWidth,
      }));
      expect(metrics.scrollWidth).toBeLessThanOrEqual(metrics.clientWidth);
    }

    await expect(groups.locator(".MuiChip-root")).toHaveCount(0);
    await expect(ruleSets.locator(".MuiChip-root")).toHaveCount(0);

    await page.getByRole("button", { name: "展开代理组 🚀 节点选择" }).click();
    const longName = "Developer Tools With A Very Long Mobile Proxy Group Name";
    const groupName = groups.getByRole("textbox", { name: "名称" }).first();
    await groupName.fill(longName);
    await page.getByRole("button", { name: `收起代理组 ${longName}` }).click();

    const groupSummary = groups.getByText(longName, { exact: true });
    await expect(groupSummary).toBeVisible();
    const summaryMetrics = await groupSummary.evaluate((element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();
      return {
        height: rect.height,
        lineHeight: Number.parseFloat(style.lineHeight),
        overflowWrap: style.overflowWrap,
        textOverflow: style.textOverflow,
        whiteSpace: style.whiteSpace,
      };
    });
    expect(summaryMetrics.whiteSpace).toBe("normal");
    expect(summaryMetrics.textOverflow).not.toBe("ellipsis");
    expect(summaryMetrics.overflowWrap).toMatch(/anywhere|break-word/);
    if (testInfo.project.name === "mobile") {
      expect(summaryMetrics.height).toBeGreaterThan(summaryMetrics.lineHeight);
    }

    const groupRow = page.getByRole("button", { name: `展开代理组 ${longName}` }).locator("xpath=ancestor::div[contains(@class, 'bg-background-paper')][1]");
    const groupRowMetrics = await groupRow.evaluate((element) => ({ clientWidth: element.clientWidth, scrollWidth: element.scrollWidth }));
    expect(groupRowMetrics.scrollWidth).toBeLessThanOrEqual(groupRowMetrics.clientWidth);

    const groupMore = page.getByRole("button", { name: `${longName} 更多操作` });
    const groupDrag = page.getByRole("button", { name: `拖拽代理组 ${longName}` });
    const groupMoveUp = page.getByRole("button", { name: `上移代理组 ${longName}` });
    if (testInfo.project.name === "mobile") {
      await expect(groupMore).toBeVisible();
      await expect(groupDrag).toBeHidden();
      await expect(groupMoveUp).toBeHidden();
      const expectedFirstAfterMove = await withinSectionButtons(groups, "展开代理组").nth(1).getAttribute("aria-label");
      expect(expectedFirstAfterMove).not.toBeNull();
      await groupMore.click();
      await page.getByRole("menuitem", { name: `下移代理组 ${longName}` }).click();
      await expect(withinSectionButtons(groups, "展开代理组").first()).toHaveAccessibleName(expectedFirstAfterMove!);
    } else {
      await expect(groupMore).toBeHidden();
      await expect(groupDrag).toBeVisible();
      await expect(groupMoveUp).toBeVisible();
    }

    const firstRuleSet = ruleSets.locator('[role="group"]').first();
    const ruleSetMore = firstRuleSet.getByRole("button", { name: /更多操作$/ });
    const ruleSetDelete = firstRuleSet.getByRole("button", { name: /^删除规则集/ });
    if (testInfo.project.name === "mobile") {
      await expect(ruleSetMore).toBeVisible();
      await expect(ruleSetDelete).toBeHidden();
    } else {
      await expect(ruleSetMore).toBeHidden();
      await expect(ruleSetDelete).toBeVisible();
    }

    await expect(rules.locator(".MuiChip-root")).toHaveCount(0);
    const firstRule = page.getByRole("group", { name: /^规则 1$/ }).first();
    const ruleRowMetrics = await firstRule.evaluate((element) => ({ clientWidth: element.clientWidth, scrollWidth: element.scrollWidth }));
    expect(ruleRowMetrics.scrollWidth).toBeLessThanOrEqual(ruleRowMetrics.clientWidth);
    if (testInfo.project.name === "mobile") {
      await expect(firstRule.getByRole("button", { name: "规则 1 更多操作" })).toBeVisible();
      await expect(firstRule.getByRole("button", { name: "拖拽规则 1" })).toBeHidden();
    } else {
      await expect(firstRule.getByRole("button", { name: "规则 1 更多操作" })).toBeHidden();
      await expect(firstRule.getByRole("button", { name: "拖拽规则 1" })).toBeVisible();
    }
  });
}

function withinSectionButtons(section: Locator, prefix: string) {
  return section.getByRole("button", { name: new RegExp(`^${prefix}`) });
}

test("subscription preview JSON stays inside the viewport and uses theme colors", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name === "tablet", "mobile and desktop cover the two preview layouts");

  await page.addInitScript(() => {
    localStorage.setItem("sandrone.locale", "zh-CN");
    localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
  });
  await page.goto("/subscriptions/remote/provider/preview");

  await expect(page.getByRole("heading", { name: "节点预览", level: 2 })).toBeVisible();
  await expect(page.getByText("preview warning")).toBeVisible();
  await expect(page.getByRole("navigation", { name: "底部导航" })).toHaveCount(0);
  const previewCard = page.getByRole("button", { name: `${previewAfterName} 节点详情` });
  await expect(previewCard).toBeVisible();
  if (testInfo.project.name === "mobile") {
    const backButton = page.getByRole("button", { name: "返回" });
    const backBox = await backButton.boundingBox();
    expect(backBox?.x, "mobile preview back button should stay left aligned").toBeLessThanOrEqual(32);
    expect(backBox?.width, "mobile preview back button should not stretch across the viewport").toBeLessThan(160);
    await expect(page.locator(".MuiChip-root", { hasText: "已修改" })).toHaveCount(0);
    const summaryMetrics = await page.evaluate(() => ({
      clientWidth: document.documentElement.clientWidth,
      scrollWidth: document.documentElement.scrollWidth,
    }));
    expect(summaryMetrics.scrollWidth, "mobile preview summary should not scroll page horizontally").toBeLessThanOrEqual(summaryMetrics.clientWidth + 1);
  }
  await previewCard.click();
  await expect(page.getByRole("group", { name: "节点详情显示方式" })).toBeVisible();

  const metrics = await page.evaluate(() => ({
    clientWidth: document.documentElement.clientWidth,
    scrollWidth: document.documentElement.scrollWidth,
  }));
  expect(metrics.scrollWidth, `${testInfo.project.name} preview JSON should not scroll page horizontally`).toBeLessThanOrEqual(metrics.clientWidth + 1);

  const jsonBlock = page.locator("pre").first();
  await expect(jsonBlock).toHaveClass(/bg-background-default/);
  await expect(jsonBlock).toHaveClass(/text-text-primary/);
  await expect(jsonBlock).not.toHaveClass(/bg-gray-100/);
  await expect(jsonBlock).not.toHaveClass(/text-gray-900/);
});

for (const mode of ["dark", "light"] as const) {
  test(`/${mode} color scheme renders main destinations`, async ({ page }, testInfo) => {
    test.skip(testInfo.project.name !== "desktop", "color-scheme navigation is viewport-independent");

    await page.addInitScript((selectedMode) => {
      localStorage.setItem("sandrone.locale", "zh-CN");
      localStorage.setItem("sandrone.publicBaseUrl", "https://example.com");
      localStorage.setItem("sandrone.theme", JSON.stringify({ mode: selectedMode, preset: "ocean" }));
    }, mode);

    await page.goto("/subscriptions");
    await expect(page.locator("html")).toHaveAttribute("data-mui-color-scheme", mode);
    await expect(page.getByRole("heading", { name: "我的订阅", level: 2 })).toBeVisible();

    await page.getByRole("link", { name: "文件" }).first().click();
    await expect(page.getByRole("heading", { name: "我的文件", level: 2 })).toBeVisible();

    await page.getByRole("link", { name: "分享" }).first().click();
    await expect(page.getByRole("heading", { name: "分享", level: 2 })).toBeVisible();
  });
}
