import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { defaultTranslator } from "~/shared/i18n/context";

import { useResourceTransferController } from "./resource-transfer-controller";

const originalClipboard = Object.getOwnPropertyDescriptor(navigator, "clipboard");
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

describe("ResourceTransferController", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    if (originalClipboard) {
      Object.defineProperty(navigator, "clipboard", originalClipboard);
    } else {
      Reflect.deleteProperty(navigator, "clipboard");
    }
    URL.createObjectURL = originalCreateObjectURL;
    URL.revokeObjectURL = originalRevokeObjectURL;
    vi.useRealTimers();
  });

  it("exports to the clipboard without downloading when clipboard writing succeeds", async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    setClipboard({ writeText });
    const loadResource = vi.fn().mockResolvedValue(subscriptionDefinition());
    const showNotice = vi.fn();
    render(<Harness loadResource={loadResource} showNotice={showNotice} />);

    await user.click(screen.getByRole("button", { name: "export" }));

    expect(loadResource).toHaveBeenCalledWith("provider");
    expect(JSON.parse(writeText.mock.calls[0][0])).toEqual({
      resource_type: "subscription",
      resource: subscriptionDefinition(),
    });
    expect(showNotice).toHaveBeenCalledWith("订阅定义已导出");
  });

  it("downloads the same JSON when clipboard writing fails", async () => {
    const user = userEvent.setup();
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error("denied")) });
    URL.createObjectURL = vi.fn().mockReturnValue("blob:resource");
    URL.revokeObjectURL = vi.fn();
    let downloaded = "";
    vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(function click(this: HTMLAnchorElement) {
      downloaded = this.download;
    });
    render(<Harness />);

    await user.click(screen.getByRole("button", { name: "export" }));

    expect(downloaded).toBe("subscription-provider.json");
    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob));
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:resource");
  });

  it("reports export failure when clipboard writing and download both fail", async () => {
    const user = userEvent.setup();
    setClipboard({ writeText: vi.fn().mockRejectedValue(new Error("denied")) });
    URL.createObjectURL = vi.fn(() => {
      throw new Error("download unavailable");
    });
    const showNotice = vi.fn();
    render(<Harness showNotice={showNotice} />);

    await user.click(screen.getByRole("button", { name: "export" }));

    expect(showNotice).toHaveBeenCalledWith("订阅定义导出失败", "error");
  });

  it("copies a complete definition under a new name with fresh timestamps", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(new Date("2026-08-15T01:02:03Z"));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const saveResource = vi.fn().mockResolvedValue({ ok: true });
    const onSaved = vi.fn();
    render(<Harness existingNames={["provider", "occupied"]} onSaved={onSaved} saveResource={saveResource} />);

    await user.click(screen.getByRole("button", { name: "copy" }));
    const dialog = screen.getByRole("dialog", { name: "复制订阅" });
    await user.type(within(dialog).getByRole("textbox", { name: /名称/ }), "provider-copy");
    await user.click(within(dialog).getByRole("button", { name: "复制" }));

    expect(saveResource).toHaveBeenCalledWith(expect.objectContaining({
      ...subscriptionDefinition(),
      name: "provider-copy",
      created_at: expect.stringMatching(/^2026-08-15T01:02:03\./),
      updated_at: expect.stringMatching(/^2026-08-15T01:02:03\./),
    }));
    expect(saveResource.mock.calls[0][0].created_at).toBe(saveResource.mock.calls[0][0].updated_at);
    expect(onSaved).toHaveBeenCalledWith(saveResource.mock.calls[0][0]);
  });

  it("rejects existing copy names before loading the source", async () => {
    const user = userEvent.setup();
    const loadResource = vi.fn().mockResolvedValue(subscriptionDefinition());
    render(<Harness existingNames={["provider", "occupied"]} loadResource={loadResource} />);

    await user.click(screen.getByRole("button", { name: "copy" }));
    const dialog = screen.getByRole("dialog", { name: "复制订阅" });
    await user.type(within(dialog).getByRole("textbox", { name: /名称/ }), "occupied");
    await user.click(within(dialog).getByRole("button", { name: "复制" }));

    expect(within(dialog).getByText("这个名称已经存在")).toBeInTheDocument();
    expect(loadResource).not.toHaveBeenCalled();
  });

  it("reads a valid clipboard definition, allows renaming, and warns before overwrite", async () => {
    const user = userEvent.setup();
    const definition = subscriptionDefinition();
    setClipboard({
      readText: vi.fn().mockResolvedValue(JSON.stringify({ resource_type: "subscription", resource: definition })),
    });
    const saveResource = vi.fn().mockResolvedValue({ ok: true });
    render(<Harness existingNames={["provider"]} saveResource={saveResource} />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });
    expect(within(dialog).getByText("“provider”已经存在，导入后将完整覆盖。")).toBeInTheDocument();
    const name = within(dialog).getByRole("textbox", { name: /名称/ });
    await user.clear(name);
    await user.type(name, "imported");
    await user.click(within(dialog).getByRole("button", { name: "导入" }));

    expect(saveResource).toHaveBeenCalledWith({ ...definition, name: "imported" });
  });

  it("falls back to file upload when clipboard reading fails", async () => {
    const user = userEvent.setup();
    setClipboard({ readText: vi.fn().mockRejectedValue(new Error("denied")) });
    const saveResource = vi.fn().mockResolvedValue({ ok: true });
    render(<Harness saveResource={saveResource} />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });
    const file = new File(["ignored"], "subscription.json", { type: "application/json" });
    Object.defineProperty(file, "text", {
      value: vi.fn().mockResolvedValue(JSON.stringify({ resource_type: "subscription", resource: subscriptionDefinition() })),
    });
    await user.upload(within(dialog).getByLabelText("选择 JSON 文件"), file);

    expect(await within(dialog).findByDisplayValue("provider")).toBeInTheDocument();
  });

  it("keeps file upload available for invalid or mismatched clipboard content", async () => {
    const user = userEvent.setup();
    setClipboard({
      readText: vi.fn().mockResolvedValue(JSON.stringify({ resource_type: "file", resource: { name: "client.yaml" } })),
    });
    render(<Harness />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });

    expect(within(dialog).getByRole("alert")).toHaveTextContent("资源类型与当前页面不匹配");
    expect(within(dialog).getByLabelText("选择 JSON 文件")).toBeInTheDocument();
  });

  it("keeps file upload available for invalid clipboard JSON", async () => {
    const user = userEvent.setup();
    setClipboard({ readText: vi.fn().mockResolvedValue("not json") });
    render(<Harness />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });

    expect(within(dialog).getByRole("alert")).toHaveTextContent("资源定义不是有效的 JSON");
    expect(within(dialog).getByLabelText("选择 JSON 文件")).toBeInTheDocument();
  });

  it("allows confirming a same-name import after showing the overwrite warning", async () => {
    const user = userEvent.setup();
    const definition = subscriptionDefinition();
    setClipboard({
      readText: vi.fn().mockResolvedValue(JSON.stringify({ resource_type: "subscription", resource: definition })),
    });
    const saveResource = vi.fn().mockResolvedValue({ ok: true });
    render(<Harness existingNames={["provider"]} saveResource={saveResource} />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });
    expect(within(dialog).getByText("“provider”已经存在，导入后将完整覆盖。")).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "导入" }));

    expect(saveResource).toHaveBeenCalledWith(definition);
  });

  it("keeps the import dialog open when saving fails", async () => {
    const user = userEvent.setup();
    setClipboard({
      readText: vi.fn().mockResolvedValue(JSON.stringify({
        resource_type: "subscription",
        resource: subscriptionDefinition(),
      })),
    });
    const saveResource = vi.fn().mockRejectedValue(new Error("save failed"));
    render(<Harness saveResource={saveResource} />);

    await user.click(screen.getByRole("button", { name: "import" }));
    const dialog = await screen.findByRole("dialog", { name: "导入订阅" });
    await user.click(within(dialog).getByRole("button", { name: "导入" }));

    expect(await within(dialog).findByText("save failed")).toBeInTheDocument();
    expect(within(dialog).getByDisplayValue("provider")).toBeInTheDocument();
  });
});

function Harness({
  existingNames = ["provider"],
  loadResource = vi.fn().mockResolvedValue(subscriptionDefinition()),
  onSaved = vi.fn(),
  saveResource = vi.fn().mockResolvedValue({ ok: true }),
  showNotice = vi.fn(),
}: {
  existingNames?: string[];
  loadResource?: (name: string) => Promise<unknown>;
  onSaved?: (resource: Record<string, unknown>) => void | Promise<void>;
  saveResource?: (resource: Record<string, unknown>) => Promise<unknown>;
  showNotice?: (message: string, severity?: "success" | "error" | "warning") => void;
}) {
  const transfer = useResourceTransferController({
    existingNames,
    loadResource,
    onSaved,
    resourceType: "subscription",
    saveResource,
    showNotice,
    t: defaultTranslator(),
  });
  return (
    <>
      <button type="button" onClick={() => transfer.copyResource("provider")}>copy</button>
      <button type="button" onClick={() => { void transfer.exportResource("provider"); }}>export</button>
      <button type="button" onClick={() => { void transfer.importResource(); }}>import</button>
      {transfer.dialogs}
    </>
  );
}

function subscriptionDefinition() {
  return {
    name: "provider",
    type: "remote",
    remote: { url: "https://example.test/sub", timeout_ms: 5000 },
    processors: [{ type: "rename", stage: "nodes", params: { prefix: "A" } }],
    created_at: "2026-08-01T00:00:00Z",
    updated_at: "2026-08-02T00:00:00Z",
    meta: { description: "Primary" },
  };
}

function setClipboard(value: { readText?: () => Promise<string>; writeText?: (text: string) => Promise<void> }) {
  Object.defineProperty(navigator, "clipboard", { configurable: true, value });
}
