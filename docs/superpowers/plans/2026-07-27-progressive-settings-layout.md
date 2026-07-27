# Progressive Settings Layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/settings` 改为适配 PC 与移动端的单栏渐进设置首页，并把运行默认值和数据管理拆到独立子页面。

**Architecture:** 保持 `routes/core -> features -> shared` 的现有依赖方向；三条 route adapter 只负责装配、导航和窄 props 转换，设置 feature 分别拥有首页、运行设置页、数据管理页及页面级 hooks。设置首页只加载版本，运行页只加载运行默认值，数据页只在操作时访问备份 API；恢复后的 fresh 请求替换旧的同键 in-flight 请求，保证后续页面不会复用恢复前结果。

**Tech Stack:** React 19、React Router 8 framework mode、TypeScript 5.9、Material UI 9、Tailwind CSS 4、Vitest/Testing Library、Playwright。

## Global Constraints

- 首页使用约 `760px` 的单栏内容列，PC 与移动端保持相同分组顺序。
- “高级设置”入口只显示标题和进入指示，不显示配置类别或内容摘要。
- `/settings/runtime` 默认只展开“远程请求”；“缓存”和“测活”默认收起。
- `/settings/data` 保留现有备份警告、下载、选择、恢复确认、失败重试和进行中锁定行为。
- 主题和语言即时生效；Public Base URL 与运行默认值显式保存。
- 不增加未保存离开提示、设置搜索、新配置项或服务端 API。
- feature 不读取 route params、不负责应用导航；路由变化同步维护独立 route 期望。
- 先跑相关窄测；交付前运行 Web 全量测试、类型检查、lint、构建和 Playwright 响应式 smoke。

---

## File Structure

### Create

- `web/app/features/settings/data/use-version-info.ts`：只加载版本与 revision。
- `web/app/features/settings/data/use-version-info.test.tsx`：版本 hook 的成功、失败和 locale 稳定性测试。
- `web/app/features/settings/data/use-backup-operations.ts`：下载、恢复及恢复后 fresh runtime 刷新。
- `web/app/features/settings/data/use-backup-operations.test.tsx`：临时下载链接、恢复和错误反馈测试。
- `web/app/features/settings/pages/settings-runtime-page.tsx`：运行默认值子页面。
- `web/app/features/settings/pages/settings-runtime-page.test.tsx`：返回、折叠和保存行为测试。
- `web/app/features/settings/pages/settings-data-page.tsx`：数据管理子页面。
- `web/app/features/settings/pages/settings-data-page.test.tsx`：返回和备份恢复行为测试。
- `web/app/features/settings/components/settings-page-heading.tsx`：设置 feature 专属的无描边标题与可选返回按钮。
- `web/app/features/settings/sections/appearance-settings-section.tsx`：主题和语言分组。
- `web/app/features/settings/sections/service-connection-section.tsx`：Public Base URL 与局部保存操作。
- `web/app/routes/settings.runtime.tsx`：运行设置 route adapter。
- `web/app/routes/settings.runtime.test.tsx`：运行设置 route 装配测试。
- `web/app/routes/settings.data.tsx`：数据管理 route adapter。
- `web/app/routes/settings.data.test.tsx`：数据管理 route 装配测试。

### Modify

- `web/app/shared/api/client.ts`
- `web/app/shared/api/client.dom.test.ts`
- `web/app/features/settings/data/use-runtime-settings.ts`
- `web/app/features/settings/data/use-runtime-settings.test.tsx`
- `web/app/features/settings/pages/settings-page.tsx`
- `web/app/features/settings/pages/settings-page.test.tsx`
- `web/app/features/settings/sections/runtime-settings-section.tsx`
- `web/app/features/settings/sections/data-settings-section.tsx`
- `web/app/routes/settings.tsx`
- `web/app/routes/settings.test.tsx`
- `web/app/routes.ts`
- `web/app/shared/i18n/translations/settings.zh-CN.ts`
- `web/app/shared/i18n/translations/settings.en-US.ts`
- `web/app/test/architecture/feature-boundaries.test.ts`
- `web/app/test/architecture/routes.test.ts`
- `web/app/test/integration/routing/app-routing.test-data.tsx`
- `web/app/test/integration/routing/app-routing-boot-auth.test.tsx`
- `web/e2e/responsive-smoke.spec.ts`

### Delete

- `web/app/features/settings/sections/general-settings-section.tsx`

---

### Task 1: Make fresh runtime reads replace stale in-flight reads

**Files:**

- Modify: `web/app/shared/api/client.ts`
- Test: `web/app/shared/api/client.dom.test.ts`

**Interfaces:**

- Consumes: `ApiClient.getRuntimeSettings(options?: { fresh?: boolean }): Promise<RuntimeSettingsInput>`.
- Produces: `fresh: true` starts a new request and installs that promise as the current deduplication entry; later ordinary reads share the fresh promise instead of an older pending request.

- [ ] **Step 1: Strengthen the failing client test**

Extend the existing “bypasses a pending runtime settings request” test so a third ordinary read starts after the fresh read:

```ts
const stale = client.getRuntimeSettings();
const fresh = client.getRuntimeSettings({ fresh: true });
const afterFresh = client.getRuntimeSettings();

expect(fetcher).toHaveBeenCalledTimes(2);
resolveFresh?.(jsonResponse(restoredSettings));
await expect(fresh).resolves.toEqual(restoredSettings);
await expect(afterFresh).resolves.toEqual(restoredSettings);

resolveStale?.(jsonResponse(staleSettings));
await expect(stale).resolves.toEqual(staleSettings);

const next = client.getRuntimeSettings();
expect(fetcher).toHaveBeenCalledTimes(3);
```

This captures both requirements: the fresh call bypasses the stale promise, and the stale promise's later `finally` cannot delete a newer entry.

- [ ] **Step 2: Run the focused test and verify the new assertion fails**

Run:

```bash
cd web
pnpm test:run app/shared/api/client.dom.test.ts
```

Expected: the new `afterFresh` assertion receives the stale result or the fetch count differs because `fresh` does not currently replace the dedupe entry.

- [ ] **Step 3: Add a guarded request replacement helper**

Change `getRuntimeSettings` and dedupe cleanup to use promise identity:

```ts
getRuntimeSettings(options: { fresh?: boolean } = {}): Promise<RuntimeSettingsInput> {
  if (options.fresh) {
    return this.replaceDedupedRequest("GET", "/v1/settings/runtime");
  }
  return this.dedupedRequest("GET", "/v1/settings/runtime");
}

private replaceDedupedRequest<T = unknown>(
  method: string,
  path: string,
  options: { method?: string; body?: unknown; auth?: boolean } = {},
): Promise<T> {
  const key = this.requestKey(method, path, options);
  const request = this.request<T>(path, options).finally(() => {
    if (inFlightRequests.get(key) === request) {
      inFlightRequests.delete(key);
    }
  });
  inFlightRequests.set(key, request);
  return request;
}
```

Apply the same identity guard inside `dedupedRequest`:

```ts
const request = this.request<T>(path, options).finally(() => {
  if (inFlightRequests.get(key) === request) {
    inFlightRequests.delete(key);
  }
});
```

- [ ] **Step 4: Run client tests**

Run:

```bash
cd web
pnpm test:run app/shared/api/client.dom.test.ts
```

Expected: all client DOM tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/shared/api/client.ts web/app/shared/api/client.dom.test.ts
git commit -m "fix(web): replace stale runtime settings requests"
```

---

### Task 2: Split settings data orchestration by page

**Files:**

- Create: `web/app/features/settings/data/use-version-info.ts`
- Create: `web/app/features/settings/data/use-version-info.test.tsx`
- Create: `web/app/features/settings/data/use-backup-operations.ts`
- Create: `web/app/features/settings/data/use-backup-operations.test.tsx`
- Modify: `web/app/features/settings/data/use-runtime-settings.ts`
- Modify: `web/app/features/settings/data/use-runtime-settings.test.tsx`

**Interfaces:**

- Produces: `useVersionInfo({ client }): { version?: string; revision?: string }`.
- Produces: `useRuntimeSettings({ client, showNotice, t }): { runtimeSettings: RuntimeSettingsInput; saveRuntimeSettings(value): Promise<void> }`.
- Produces: `useBackupOperations({ client, showNotice, t }): { downloadBackup(): Promise<void>; restoreBackup(file: Blob): Promise<void> }`.
- The hooks consume `ApiClient`, `Translator`, and the existing global notice signature.

- [ ] **Step 1: Write failing page-specific hook tests**

Replace the combined hook expectations with three focused suites. The version suite must assert silent failure:

```ts
it("loads build identity without showing an error when unavailable", async () => {
  const getVersion = vi.fn().mockRejectedValue(new Error("offline"));
  const client = { getVersion } as unknown as ApiClient;
  const { result } = renderHook(() => useVersionInfo({ client }));

  await waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
  expect(result.current).toEqual({ revision: undefined, version: undefined });
});
```

The runtime suite must assert it does not access version or backup methods:

```ts
it("loads and saves only runtime settings", async () => {
  const loaded = runtimeSettings("loaded-agent", 15000);
  const saved = runtimeSettings("saved-agent", 30000);
  const client = {
    getRuntimeSettings: vi.fn().mockResolvedValue(loaded),
    updateRuntimeSettings: vi.fn().mockResolvedValue({ ok: true }),
  } as unknown as ApiClient;
  const showNotice = vi.fn();
  const { result } = renderHook(() => useRuntimeSettings({ client, showNotice, t }));

  await waitFor(() => expect(result.current.runtimeSettings).toEqual(loaded));
  await act(async () => result.current.saveRuntimeSettings(saved));

  expect(client.updateRuntimeSettings).toHaveBeenCalledWith(saved);
  expect(showNotice).toHaveBeenCalledWith("设置已保存");
});
```

The backup suite must assert a successful restore performs the replacement read:

```ts
it("refreshes runtime settings after restoring a backup", async () => {
  const file = new Blob(["backup"], { type: "application/zip" });
  const client = {
    restoreBackup: vi.fn().mockResolvedValue(undefined),
    getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings("restored", 30000)),
  } as unknown as ApiClient;
  const showNotice = vi.fn();
  const { result } = renderHook(() => useBackupOperations({ client, showNotice, t }));

  await act(async () => result.current.restoreBackup(file));

  expect(client.restoreBackup).toHaveBeenCalledWith(file);
  expect(client.getRuntimeSettings).toHaveBeenCalledWith({ fresh: true });
  expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
});
```

Also move the existing temporary-anchor cleanup, restore failure, and post-restore refresh failure assertions into `use-backup-operations.test.tsx`.

- [ ] **Step 2: Run hook tests and verify imports/exports fail**

Run:

```bash
cd web
pnpm test:run app/features/settings/data
```

Expected: tests fail because `useVersionInfo` and `useBackupOperations` do not exist and `useRuntimeSettings` still exposes unrelated responsibilities.

- [ ] **Step 3: Implement the three focused hooks**

`use-version-info.ts` loads build identity once per client and silently leaves values undefined on failure:

```ts
export function useVersionInfo({ client }: { client: ApiClient }) {
  const [version, setVersion] = useState<string>();
  const [revision, setRevision] = useState<string>();

  useEffect(() => {
    let cancelled = false;
    void client.getVersion()
      .then((info) => {
        if (!cancelled) {
          setVersion(info.version);
          setRevision(info.revision);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [client]);

  return { revision, version };
}
```

Keep only initial runtime loading and `saveRuntimeSettings` in `use-runtime-settings.ts`. Keep its request-generation guard so an obsolete request cannot update state after unmount or a newer load.

`use-backup-operations.ts` owns download cleanup and restore orchestration:

```ts
const restoreBackup = useCallback(async (file: Blob) => {
  try {
    await client.restoreBackup(file);
  } catch (error) {
    showNotice(error instanceof Error ? error.message : t("settings.data.restoreFailed"), "error");
    throw error;
  }

  try {
    await client.getRuntimeSettings({ fresh: true });
  } catch {
    showNotice(t("errors.settingsLoadFailed"), "error");
  }
  showNotice(t("settings.data.restoreSucceeded"));
}, [client, showNotice, t]);
```

Preserve the existing `try/finally` cleanup of the temporary anchor and object URL in `downloadBackup`.

- [ ] **Step 4: Run settings data tests**

Run:

```bash
cd web
pnpm test:run app/features/settings/data
```

Expected: all settings data hook tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/features/settings/data
git commit -m "refactor(web): split settings page data hooks"
```

---

### Task 3: Build the two focused settings subpages

**Files:**

- Create: `web/app/features/settings/components/settings-page-heading.tsx`
- Create: `web/app/features/settings/pages/settings-runtime-page.tsx`
- Create: `web/app/features/settings/pages/settings-runtime-page.test.tsx`
- Create: `web/app/features/settings/pages/settings-data-page.tsx`
- Create: `web/app/features/settings/pages/settings-data-page.test.tsx`
- Modify: `web/app/features/settings/sections/runtime-settings-section.tsx`
- Modify: `web/app/features/settings/sections/data-settings-section.tsx`

**Interfaces:**

- Produces: `SettingsPageHeading({ title, description?, onBack? })`.
- Produces: `SettingsRuntimePage({ runtimeSettings, onBack, onSaveRuntimeSettings })`.
- Produces: `SettingsDataPage({ onBack, onDownloadBackup, onRestoreBackup })`.
- Consumes: existing `RuntimeSettingsInput` and async backup callbacks.

- [ ] **Step 1: Write failing subpage component tests**

For the runtime page, assert the heading, return callback, default accordion state, and save behavior:

```ts
expect(screen.getByRole("heading", { name: "运行默认值" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "返回" })).toBeInTheDocument();
expect(screen.getByRole("button", { name: "远程请求" })).toHaveAttribute("aria-expanded", "true");
expect(screen.getByRole("button", { name: "缓存" })).toHaveAttribute("aria-expanded", "false");
expect(screen.getByRole("button", { name: "测活" })).toHaveAttribute("aria-expanded", "false");

await user.click(screen.getByRole("button", { name: "返回" }));
expect(onBack).toHaveBeenCalledTimes(1);
```

Move the existing runtime field-edit/save assertions from `settings-page.test.tsx` into this suite.

For the data page, assert the return callback and preserve existing backup interactions:

```ts
expect(screen.getByRole("heading", { name: "数据管理" })).toBeInTheDocument();
expect(screen.getByRole("note")).toHaveTextContent("备份是未加密的明文");
await user.click(screen.getByRole("button", { name: "返回" }));
expect(onBack).toHaveBeenCalledTimes(1);
```

Move selected ZIP, cancel/confirm restore, pending lock, and failed restore assertions into this suite.

- [ ] **Step 2: Run the new page tests and verify modules are missing**

Run:

```bash
cd web
pnpm test:run app/features/settings/pages/settings-runtime-page.test.tsx app/features/settings/pages/settings-data-page.test.tsx
```

Expected: tests fail because both page modules and `SettingsPageHeading` are absent.

- [ ] **Step 3: Implement the feature-local heading and subpages**

Use a plain heading container rather than the shared outlined `PageHeader`:

```tsx
export function SettingsPageHeading({
  description,
  onBack,
  title,
}: {
  description?: string;
  onBack?: () => void;
  title: string;
}) {
  const { t } = useI18n();
  return (
    <header className="grid gap-2">
      {onBack ? (
        <Button className="w-fit px-0" startIcon={<ArrowBackIcon aria-hidden />} onClick={onBack}>
          {t("actions.back")}
        </Button>
      ) : null}
      <div>
        <Typography component="h2" variant="h4">{title}</Typography>
        {description ? <Typography color="text.secondary">{description}</Typography> : null}
      </div>
    </header>
  );
}
```

Both subpages use the same bounded column:

```tsx
<section className="mx-auto grid w-full max-w-[760px] gap-4">
  <SettingsPageHeading title={t("settings.runtime.title")} onBack={onBack} />
  <RuntimeSettingsSection
    runtimeSettings={runtimeSettings}
    onSaveRuntimeSettings={onSaveRuntimeSettings}
  />
</section>
```

Add `defaultExpanded` to `RuntimeSettingsGroup`, pass `true` only for the remote group, and remove the full-width `md:col-span-2` card class. Align the runtime save button with `className="justify-self-end"` instead of stretching across the card.

Wrap `DataSettingsSection` in the same `max-w-[760px]` page shell and remove its `md:col-span-2` class. Do not change its dialog or async state machine.

- [ ] **Step 4: Run subpage and section behavior tests**

Run:

```bash
cd web
pnpm test:run app/features/settings/pages/settings-runtime-page.test.tsx app/features/settings/pages/settings-data-page.test.tsx
```

Expected: both suites pass, including all migrated field and backup behaviors.

- [ ] **Step 5: Commit**

```bash
git add web/app/features/settings/components web/app/features/settings/pages/settings-runtime-page.tsx web/app/features/settings/pages/settings-runtime-page.test.tsx web/app/features/settings/pages/settings-data-page.tsx web/app/features/settings/pages/settings-data-page.test.tsx web/app/features/settings/sections/runtime-settings-section.tsx web/app/features/settings/sections/data-settings-section.tsx
git commit -m "feat(web): add focused settings subpages"
```

---

### Task 4: Redesign the settings overview as a progressive single column

**Files:**

- Create: `web/app/features/settings/sections/appearance-settings-section.tsx`
- Create: `web/app/features/settings/sections/service-connection-section.tsx`
- Modify: `web/app/features/settings/pages/settings-page.tsx`
- Modify: `web/app/features/settings/pages/settings-page.test.tsx`
- Modify: `web/app/shared/i18n/translations/settings.zh-CN.ts`
- Modify: `web/app/shared/i18n/translations/settings.en-US.ts`
- Delete: `web/app/features/settings/sections/general-settings-section.tsx`

**Interfaces:**

- Produces: `SettingsPage` props limited to build identity, preferences, sign-out, and two navigation callbacks.
- `onOpenRuntime(): void` and `onOpenData(): void` are supplied by the route; the feature page does not call React Router.
- Produces: separate appearance and service section components with existing preference callback signatures.

- [ ] **Step 1: Rewrite the settings overview test as a failing information-architecture contract**

Render `SettingsPage` with `onOpenRuntime` and `onOpenData`, then assert:

```ts
expect(screen.getByRole("heading", { name: "设置" })).toBeInTheDocument();
expect(screen.getByText("管理界面偏好与服务配置")).toBeInTheDocument();
expect(screen.getByRole("heading", { name: "外观与语言" })).toBeInTheDocument();
expect(screen.getByRole("heading", { name: "服务连接" })).toBeInTheDocument();
expect(screen.getByRole("heading", { name: "数据与账户" })).toBeInTheDocument();
expect(screen.getByRole("heading", { name: "关于 Sandrone" })).toBeInTheDocument();

const advanced = screen.getByRole("button", { name: "打开高级设置" });
expect(advanced).toHaveTextContent("高级设置");
expect(advanced).not.toHaveTextContent(/远程请求|缓存|测活/);
expect(screen.queryByRole("heading", { name: "运行默认值" })).not.toBeInTheDocument();
expect(screen.queryByRole("note")).not.toBeInTheDocument();

await user.click(advanced);
await user.click(screen.getByRole("button", { name: "管理备份与恢复" }));
expect(onOpenRuntime).toHaveBeenCalledTimes(1);
expect(onOpenData).toHaveBeenCalledTimes(1);
```

Keep the existing theme, locale, Public Base URL, build identity, and sign-out confirmation assertions. Add a DOM assertion that the root content section has `max-w-[760px]`.

- [ ] **Step 2: Run the overview test and verify the old flattened page fails**

Run:

```bash
cd web
pnpm test:run app/features/settings/pages/settings-page.test.tsx
```

Expected: failures show the old outlined page header, missing navigation callbacks, runtime form, and backup warning still rendered on the overview.

- [ ] **Step 3: Split general settings and implement the overview**

Move theme/locale state and controls unchanged into `AppearanceSettingsSection`. Move the local Base URL draft and save button into `ServiceConnectionSection`. Each component returns one `Card`, not a fragment of sibling cards.

Use these `SettingsPage` props:

```ts
export interface SettingsPageProps {
  publicBaseUrl: string;
  revision?: string;
  themeMode: ThemeMode;
  version?: string;
  onOpenData: () => void;
  onOpenRuntime: () => void;
  onSaveBaseUrl: (value: string) => void;
  onSignOut: () => void;
  onThemeMode: (mode: ThemeMode) => void;
}
```

Build the overview with:

```tsx
<section className="mx-auto grid w-full max-w-[760px] gap-4">
  <SettingsPageHeading
    description={t("settings.description")}
    title={t("settings.title")}
  />
  <AppearanceSettingsSection
    themeMode={themeMode}
    onThemeMode={onThemeMode}
  />
  <ServiceConnectionSection
    publicBaseUrl={publicBaseUrl}
    onSaveBaseUrl={onSaveBaseUrl}
  />
  <Card component="article" variant="outlined">
    <CardActionArea aria-label={t("settings.advanced.open")} onClick={onOpenRuntime}>
      <CardContent className="flex items-center justify-between gap-4">
        <Typography component="h3" variant="h6">{t("settings.advanced.title")}</Typography>
        <ChevronRightIcon aria-hidden color="action" />
      </CardContent>
    </CardActionArea>
  </Card>
  <Card component="article" variant="outlined">
    <CardContent className="grid gap-3">
      <Typography component="h3" variant="h6">{t("settings.dataAndAccount.title")}</Typography>
      <Button
        aria-label={t("settings.data.open")}
        className="justify-between"
        endIcon={<ChevronRightIcon aria-hidden />}
        onClick={onOpenData}
      >
        {t("settings.data.entry")}
      </Button>
      <div className="flex items-center justify-between gap-4 border-t border-divider pt-3">
        <div>
          <Typography>{t("settings.adminToken.title")}</Typography>
          <Typography color="text.secondary" variant="body2">
            {t("settings.adminToken.description")}
          </Typography>
        </div>
        <Button color="error" startIcon={<LogoutIcon aria-hidden />} onClick={() => setConfirmSignOut(true)}>
          {t("settings.signOut.action")}
        </Button>
      </div>
    </CardContent>
  </Card>
  <Card component="article" variant="outlined">
    <CardContent className="grid gap-3">
      <div className="flex items-center justify-between gap-4">
        <Typography component="h3" variant="h6">{t("settings.about.title")}</Typography>
        <Typography color="text.secondary" variant="body2">
          {version
            ? `${version === "dev" ? "dev" : `v${version}`}${revision ? ` (${revision.slice(0, 12)})` : ""}`
            : t("settings.about.versionUnavailable")}
        </Typography>
      </div>
      <Link href="https://github.com/kuuvahki-labs/sandrone" rel="noreferrer" target="_blank">
        GitHub
      </Link>
    </CardContent>
  </Card>
</section>
```

Keep the existing sign-out confirmation `Dialog` after the section and call `onSignOut` only from its confirmed action.

Add exact translation keys in both locales:

```ts
"settings.description": "管理界面偏好与服务配置",
"settings.advanced.title": "高级设置",
"settings.advanced.open": "打开高级设置",
"settings.dataAndAccount.title": "数据与账户",
"settings.data.entry": "备份与恢复",
"settings.data.open": "管理备份与恢复",
```

English values:

```ts
"settings.description": "Manage interface preferences and service configuration",
"settings.advanced.title": "Advanced settings",
"settings.advanced.open": "Open advanced settings",
"settings.dataAndAccount.title": "Data and account",
"settings.data.entry": "Backup and restore",
"settings.data.open": "Manage backup and restore",
```

Update the existing section title translations to `外观与语言` / `Appearance and language` and `服务连接` / `Service connection`.

- [ ] **Step 4: Run overview and translation tests**

Run:

```bash
cd web
pnpm test:run app/features/settings/pages/settings-page.test.tsx app/shared/i18n
```

Expected: overview behavior and locale integrity tests pass.

- [ ] **Step 5: Delete the obsolete combined section and update the architecture inventory**

Delete `general-settings-section.tsx`. Replace its inventory entry in `feature-boundaries.test.ts` with:

```ts
"features/settings/components/settings-page-heading.tsx",
"features/settings/data/use-backup-operations.ts",
"features/settings/data/use-runtime-settings.ts",
"features/settings/data/use-version-info.ts",
"features/settings/model/runtime-settings.ts",
"features/settings/pages/settings-data-page.tsx",
"features/settings/pages/settings-page.tsx",
"features/settings/pages/settings-runtime-page.tsx",
"features/settings/sections/appearance-settings-section.tsx",
"features/settings/sections/data-settings-section.tsx",
"features/settings/sections/runtime-settings-section.tsx",
"features/settings/sections/service-connection-section.tsx",
```

Run:

```bash
cd web
pnpm test:run app/test/architecture/feature-boundaries.test.ts
```

Expected: the exact feature-owned module inventory passes.

- [ ] **Step 6: Commit**

```bash
git add web/app/features/settings web/app/shared/i18n/translations/settings.zh-CN.ts web/app/shared/i18n/translations/settings.en-US.ts web/app/test/architecture/feature-boundaries.test.ts
git commit -m "feat(web): simplify settings overview"
```

---

### Task 5: Wire routes, resource isolation, and responsive coverage

**Files:**

- Create: `web/app/routes/settings.runtime.tsx`
- Create: `web/app/routes/settings.runtime.test.tsx`
- Create: `web/app/routes/settings.data.tsx`
- Create: `web/app/routes/settings.data.test.tsx`
- Modify: `web/app/routes/settings.tsx`
- Modify: `web/app/routes/settings.test.tsx`
- Modify: `web/app/routes.ts`
- Modify: `web/app/test/architecture/routes.test.ts`
- Modify: `web/app/test/integration/routing/app-routing.test-data.tsx`
- Modify: `web/app/test/integration/routing/app-routing-boot-auth.test.tsx`
- Modify: `web/e2e/responsive-smoke.spec.ts`

**Interfaces:**

- Produces routes `settings`, `settings-runtime`, and `settings-data`.
- Overview navigation uses `navigate("/settings/runtime")` and `navigate("/settings/data")`.
- Both child routes pass `onBack={() => navigate("/settings")}`.

- [ ] **Step 1: Write failing route and resource-isolation tests**

Add route unit tests that mock `useSandrone` and assert each page calls only its owned resources:

```ts
it("loads only version information on the settings overview", async () => {
  renderSettingsRoute();
  await waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
  expect(getRuntimeSettings).not.toHaveBeenCalled();
  expect(downloadBackup).not.toHaveBeenCalled();
});
```

```ts
it("loads and saves runtime defaults on the runtime route", async () => {
  renderSettingsRuntimeRoute();
  await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));
  expect(getVersion).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "保存运行默认值" }));
  expect(updateRuntimeSettings).toHaveBeenCalledTimes(1);
});
```

```ts
it("does not fetch settings resources before a data operation", () => {
  renderSettingsDataRoute();
  expect(getVersion).not.toHaveBeenCalled();
  expect(getRuntimeSettings).not.toHaveBeenCalled();
  expect(screen.getByRole("button", { name: "下载备份" })).toBeInTheDocument();
});
```

In `app-routing-boot-auth.test.tsx`, assert request paths:

```ts
renderApp("/settings");
await screen.findByRole("heading", { name: "设置" });
expect(settingsRequestPaths(requests)).toEqual(["/version"]);
```

Add corresponding `/settings/runtime` and `/settings/data` cases. Define the helper in the same test file:

```ts
function settingsRequestPaths(requests: Array<{ url: string; init?: RequestInit }>): string[] {
  return requests
    .filter((request) => (request.init?.method ?? "GET") === "GET")
    .map((request) => request.url)
    .filter((url) => ["/version", "/v1/settings/runtime"].includes(url));
}
```

The expected arrays are `["/v1/settings/runtime"]` for `/settings/runtime` and `[]` for `/settings/data` before any operation.

- [ ] **Step 2: Run route tests and verify missing route failures**

Run:

```bash
cd web
pnpm test:run app/routes/settings.test.tsx app/routes/settings.runtime.test.tsx app/routes/settings.data.test.tsx app/test/architecture/routes.test.ts app/test/integration/routing/app-routing-boot-auth.test.tsx
```

Expected: new route imports/config expectations fail and the old overview still loads all resources.

- [ ] **Step 3: Implement the three narrow route adapters**

Overview:

```tsx
export default function SettingsRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const version = useVersionInfo({ client: app.client });
  return (
    <SettingsPage
      publicBaseUrl={app.publicBaseUrl}
      revision={version.revision}
      themeMode={app.themeMode}
      version={version.version}
      onOpenData={() => navigate("/settings/data")}
      onOpenRuntime={() => navigate("/settings/runtime")}
      onSaveBaseUrl={app.saveBaseUrl}
      onSignOut={app.signOut}
      onThemeMode={app.updateThemeMode}
    />
  );
}
```

Runtime route:

```tsx
export default function SettingsRuntimeRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const { t } = useI18n();
  const settings = useRuntimeSettings({ client: app.client, showNotice: app.showNotice, t });
  return (
    <SettingsRuntimePage
      runtimeSettings={settings.runtimeSettings}
      onBack={() => navigate("/settings")}
      onSaveRuntimeSettings={settings.saveRuntimeSettings}
    />
  );
}
```

Data route:

```tsx
export default function SettingsDataRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const { t } = useI18n();
  const backup = useBackupOperations({ client: app.client, showNotice: app.showNotice, t });
  return (
    <SettingsDataPage
      onBack={() => navigate("/settings")}
      onDownloadBackup={backup.downloadBackup}
      onRestoreBackup={backup.restoreBackup}
    />
  );
}
```

- [ ] **Step 4: Register routes and independent integration entries**

Add to `web/app/routes.ts`:

```ts
route("settings", "routes/settings.tsx", { id: "settings" }),
route("settings/runtime", "routes/settings.runtime.tsx", { id: "settings-runtime" }),
route("settings/data", "routes/settings.data.tsx", { id: "settings-data" }),
```

Import both route components in `app-routing.test-data.tsx` and add matching entries. Update `routes.test.ts` from an exact 11-route contract to the exact 13-route contract with the same order and file names.

- [ ] **Step 5: Run route and integration tests**

Run:

```bash
cd web
pnpm test:run app/routes/settings.test.tsx app/routes/settings.runtime.test.tsx app/routes/settings.data.test.tsx app/test/architecture/routes.test.ts app/test/integration/routing/app-routing-boot-auth.test.tsx
```

Expected: all settings route, route contract, navigation, and resource-isolation tests pass.

- [ ] **Step 6: Extend Playwright responsive smoke**

Add route fixtures:

```ts
{ path: "/settings", heading: "设置", text: "高级设置", focus: false, responsive: true },
{ path: "/settings/runtime", heading: "运行默认值", text: "远程请求", focus: false, responsive: true },
{ path: "/settings/data", heading: "数据管理", text: "备份是未加密的明文", focus: false, responsive: true },
```

For `/settings`, assert the overview has no runtime fields or backup note, and that its content column does not exceed the available viewport. For `/settings/runtime`, assert remote is expanded while cache and probe are collapsed. Move the existing data-card overflow assertions to `/settings/data`. Keep the global `scrollWidth <= clientWidth + 1` assertion for all three routes.

- [ ] **Step 7: Build and run the Playwright smoke**

Run from the repository root:

```bash
make build-webui
make test-webui-e2e
```

Expected: all configured desktop and mobile projects pass, with no horizontal overflow or console issues.

- [ ] **Step 8: Run the full Web gate**

Run:

```bash
cd web
pnpm test:run
pnpm typecheck
pnpm lint
pnpm build
```

Expected: every command exits successfully with zero failing tests, type errors, lint errors, or build errors.

- [ ] **Step 9: Confirm obsolete identifiers and generated files are absent**

Run from the repository root:

```bash
test ! -e web/app/features/settings/sections/general-settings-section.tsx
rg -n "GeneralSettingsSection|md:col-span-2" web/app/features/settings web/app/routes
git status --short
```

Expected: `GeneralSettingsSection` has no matches; any remaining `md:col-span-2` match is reviewed and unrelated to full-width settings cards; generated `web/build` and copied static files are not staged.

- [ ] **Step 10: Commit**

```bash
git add web/app/routes.ts web/app/routes/settings.tsx web/app/routes/settings.test.tsx web/app/routes/settings.runtime.tsx web/app/routes/settings.runtime.test.tsx web/app/routes/settings.data.tsx web/app/routes/settings.data.test.tsx web/app/test/architecture/routes.test.ts web/app/test/integration/routing/app-routing.test-data.tsx web/app/test/integration/routing/app-routing-boot-auth.test.tsx web/e2e/responsive-smoke.spec.ts
git commit -m "feat(web): route progressive settings pages"
```

---

## Final Verification

- [ ] Confirm the settings overview contains no runtime fields, backup warning, or advanced-setting summary.
- [ ] Confirm `/settings/runtime` and `/settings/data` both return to `/settings`.
- [ ] Confirm restore success installs a fresh runtime request and stale pending reads cannot replace it.
- [ ] Confirm all three routes use the same single-column order on PC and mobile.
- [ ] Confirm `git diff --check` and `git status --short` show no unintended changes.
