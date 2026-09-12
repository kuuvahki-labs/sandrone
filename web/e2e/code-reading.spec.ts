import { expect, type Page, test } from "@playwright/test";

const previewBody = [
  "proxies:",
  `  - name: ${"very-long-node-name-".repeat(24)}`,
  ...Array.from({ length: 160 }, (_, index) => `# generated item ${index + 1}`),
].join("\n");
const warning = { code: "fixture_warning", message: "Fixture warning details", source: "fixture" };
const settings = {
  appearance: { locale: "zh-CN", theme_mode: "dark" },
  remote_defaults: { timeout_ms: 15000 },
  script_defaults: { timeout_ms: 2000 },
  cache_defaults: { remote_fetch_ttl_seconds: 0, probe_ttl_seconds: 0, subscription_snapshot_ttl_seconds: 0 },
  subscriptions: { auto_load_traffic: false, ignored_warnings: [] },
};

async function mockApp(page: Page) {
  let releaseRefresh: (() => void) | undefined;
  let delayRefresh = false;
  const issues: string[] = [];
  page.on("pageerror", (error) => issues.push(error.message));
  page.on("console", (message) => { if (message.type() === "error") issues.push(message.text()); });
  page.on("response", (response) => { if (response.status() >= 400) issues.push(`${response.status()} ${response.url()}`); });
  page.on("requestfailed", (request) => {
    if (request.failure()?.errorText !== "net::ERR_ABORTED") issues.push(`${request.url()}: ${request.failure()?.errorText}`);
  });
  await page.addInitScript(() => {
    localStorage.setItem("sandrone.locale", "zh-CN");
    localStorage.setItem("sandrone.publicBaseUrl", window.location.origin);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: async (value: string) => { sessionStorage.setItem("code-reading-copy", value); } },
    });
  });
  await page.route("**/v1/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname.startsWith("/v1/files/reader.yaml")) {
      if (delayRefresh && url.searchParams.has("refresh")) await new Promise<void>((resolve) => { releaseRefresh = resolve; });
      await route.fulfill({ json: { body: previewBody, content_type: "application/yaml", warnings: [warning] } });
    } else if (url.pathname === "/v1/files") {
      await route.fulfill({ json: { items: [{ name: "reader.yaml", kind: "static", type: "inline" }] } });
    } else if (url.pathname === "/v1/settings") {
      await route.fulfill({ json: { settings, effective: settings, overrides: {}, restart_required: [] } });
    } else if (url.pathname === "/v1/capabilities/ui") {
      await route.fulfill({ json: { features: [] } });
    } else {
      await route.fulfill({ json: { items: [] } });
    }
  });
  await page.route("**/version", (route) => route.fulfill({ json: { name: "sandrone", version: "fixture" } }));
  await page.route("**/healthz", (route) => route.fulfill({ body: "ok" }));
  return {
    issues,
    delayRefresh() { delayRefresh = true; },
    releaseRefresh() { releaseRefresh?.(); },
  };
}

test("file preview keeps warnings separate and locates text without moving the page", async ({ page }) => {
  const app = await mockApp(page);
  await page.goto("/files/reader.yaml/preview");
  const body = page.getByRole("region", { name: "最终文件内容", exact: true });
  const pre = body.locator("pre");
  await expect(pre).toContainText("generated item 160");
  await expect(body.locator("textarea")).toHaveCount(0);
  await expect(body).not.toContainText(/只读|换行|已更新/);
  const dimensions = await pre.evaluate((element) => ({
    height: element.clientHeight,
    width: element.clientWidth,
    scrollWidth: element.scrollWidth,
    bottom: element.getBoundingClientRect().bottom,
    pageHeight: document.documentElement.scrollHeight,
    viewport: window.innerHeight,
  }));
  expect(dimensions.height).toBeGreaterThan(dimensions.viewport / 2);
  expect(dimensions.bottom).toBeGreaterThan(dimensions.viewport - 40);
  expect(dimensions.scrollWidth).toBeLessThanOrEqual(dimensions.width + 1);
  expect(dimensions.pageHeight).toBeLessThanOrEqual(dimensions.viewport + 1);

  const initial = await body.boundingBox();
  await page.getByRole("button", { name: "展开预览警告" }).click();
  const warnings = page.getByRole("dialog", { name: "预览警告" });
  await expect(warnings).toContainText("Fixture warning details");
  // The body's DOM remains available while the modal makes the background inert.
  const background = page.locator('section[aria-label="最终文件内容"]');
  expect(await background.boundingBox()).toEqual(initial);
  await warnings.getByRole("button", { name: "关闭", exact: true }).click();
  await expect(warnings).toBeHidden();
  await expect(page.getByRole("button", { name: "展开预览警告" })).toBeFocused();

  await body.getByRole("button", { name: "查找最终文件内容" }).click();
  await page.getByRole("searchbox", { name: "查找最终文件内容" }).fill("generated item 150");
  const mark = body.locator("mark").first();
  await expect(mark).toBeInViewport();
  await expect(body.getByRole("status").filter({ hasText: "1 / 1" })).toBeVisible();
  await page.keyboard.press("Enter");
  await body.getByRole("button", { name: "复制最终文件内容" }).click();
  expect(await page.evaluate(() => sessionStorage.getItem("code-reading-copy"))).toBe(previewBody);
  await body.getByRole("button", { name: "关闭查找" }).click();
  const scrollTop = await pre.evaluate((element) => element.scrollTop);

  app.delayRefresh();
  await page.getByRole("button", { name: "刷新文件预览" }).click();
  await expect(page.getByRole("button", { name: "刷新文件预览" })).toBeDisabled();
  expect(await body.boundingBox()).toEqual(initial);
  expect(await pre.evaluate((element) => element.scrollTop)).toBe(scrollTop);
  app.releaseRefresh();
  await expect(page.getByRole("button", { name: "刷新文件预览" })).toBeEnabled();
  expect(await pre.evaluate((element) => element.scrollTop)).toBe(scrollTop);

  const original = await pre.elementHandle();
  await body.getByRole("button", { name: "放大预览" }).click();
  await expect(page.locator("dialog:modal")).toBeVisible();
  expect(await original!.evaluate((element) => element === document.querySelector("dialog:modal pre"))).toBe(true);
  await page.getByRole("button", { name: "查找最终文件内容" }).click();
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog:modal")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog:modal")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "放大预览" })).toBeFocused();
  expect(app.issues).toEqual([]);
});

test("inline source and JavaScript editors preserve text, form data, and native undo while expanding", async ({ page }, testInfo) => {
  const app = await mockApp(page);
  await page.goto("/subscriptions/new?type=local");
  const source = page.getByRole("textbox", { name: "内容", exact: true });
  await expect(source).toBeVisible();
  const initialHeight = await source.evaluate((element) => element.clientHeight);
  const longSource = Array.from({ length: 80 }, (_, index) => `# source line ${index + 1}`).join("\n");
  await source.fill(longSource);
  await expect.poll(() => source.evaluate((element) => element.clientHeight)).toBeGreaterThan(initialHeight);
  expect(await source.evaluate((element) => element.clientHeight)).toBeLessThanOrEqual(page.viewportSize()!.height * 0.6 + 1);

  if (testInfo.project.name === "desktop") {
    await source.scrollIntoViewIfNeeded();
    const initialBox = (await source.boundingBox())!;
    await source.hover({ position: { x: initialBox.width - 4, y: initialBox.height - 4 } });
    const box = (await source.boundingBox())!;
    await page.mouse.down();
    await page.mouse.move(box.x + box.width - 4, box.y + box.height - 84, { steps: 8 });
    await page.mouse.up();
    const resized = await source.evaluate((element) => element.clientHeight);
    expect(resized).toBeLessThan(box.height - 20);
    await source.fill(`${longSource}\n# another line`);
    expect(await source.evaluate((element) => element.clientHeight)).toBe(resized);
  }

  const beforeExpansion = await source.inputValue();
  const original = await source.elementHandle();
  await page.getByRole("button", { name: "放大编辑", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "内容", exact: true });
  await expect(dialog).toBeVisible();
  expect(await original!.evaluate((element) => element === document.querySelector("dialog:modal textarea"))).toBe(true);
  expect(await source.evaluate((element: HTMLTextAreaElement) => new FormData(element.form!).get("source_input"))).toBe(beforeExpansion);
  await source.press("ControlOrMeta+End");
  await source.pressSequentially("x");
  await dialog.getByRole("button", { name: "收起编辑" }).click();
  await source.focus();
  await source.press("ControlOrMeta+z");
  await expect(source).toHaveValue(beforeExpansion);
  expect(await source.evaluate((element: HTMLTextAreaElement) => new FormData(element.form!).get("source_input"))).toBe(beforeExpansion);

  await page.getByRole("button", { name: "添加处理器", exact: true }).click();
  const processor = page.getByRole("group", { name: "处理器 脚本", exact: true });
  const script = processor.getByRole("textbox", { name: "代码", exact: true });
  const scriptSource = `// ${"长名称".repeat(80)} target\nconst target = 1;`;
  await script.fill(scriptSource);
  await processor.getByRole("button", { name: "查找代码" }).click();
  await page.getByRole("searchbox", { name: "查找代码" }).fill("target");
  await expect(processor.getByRole("status").filter({ hasText: "1 / 2" })).toBeVisible();
  const highlightBounds = await processor.locator('[data-highlighted-textarea="javascript"] [data-highlighted-textarea-layer]').boundingBox();
  const matchBounds = await processor.locator("mark").first().boundingBox();
  expect(matchBounds!.x).toBeGreaterThanOrEqual(highlightBounds!.x);
  expect(matchBounds!.x + matchBounds!.width).toBeLessThanOrEqual(highlightBounds!.x + highlightBounds!.width + 1);
  const serialized = await script.evaluate((element: HTMLTextAreaElement) => new FormData(element.form!).get("processors"));
  expect(JSON.parse(String(serialized))[0].params.source.content).toBe(scriptSource);
  await processor.getByRole("button", { name: "放大编辑" }).click();
  await expect(page.getByRole("dialog", { name: "代码", exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog:modal")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.locator("dialog:modal")).toHaveCount(0);
  expect(app.issues).toEqual([]);
});
