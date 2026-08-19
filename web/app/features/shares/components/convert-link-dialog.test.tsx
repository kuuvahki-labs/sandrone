import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import type { ApiClient, FormatCapabilityList, FormatCapabilitySummary } from "~/shared/api/client";

import { ConvertLinkDialog } from "./convert-link-dialog";

const capabilities: FormatCapabilityList = {
  items: [
    capability("parse", "base64"),
    capability("parse", "json-nodes"),
    capability("parse", "uri-list"),
    capability("render", "base64"),
    capability("render", "json-nodes"),
    capability("render", "mihomo-proxies"),
  ],
};

describe("convert link dialog", () => {
  afterEach(() => {
    window.getSelection()?.removeAllRanges();
  });

  it("loads runtime formats and copies an encoded remote conversion URL", async () => {
    const user = userEvent.setup();
    const listFormatCapabilities = vi.fn().mockResolvedValue(capabilities);
    const onCopyUrl = vi.fn().mockResolvedValue({ copied: true });
    renderDialog({ listFormatCapabilities, onCopyUrl });

    const dialog = screen.getByRole("dialog", { name: "生成转换链接" });
    expect(await within(dialog).findByRole("combobox", { name: "输出格式" })).toHaveValue("base64");
    expect(within(dialog).getByRole("combobox", { name: "输入格式" })).toHaveValue("");
    expect(within(dialog).getByRole("combobox", { name: "响应模式" })).toHaveValue("raw");
    expect(within(dialog).getByText("输入格式")).toHaveAttribute("data-shrink", "true");
    expect(within(dialog).getByText("输出格式")).toHaveAttribute("data-shrink", "true");
    expect(within(dialog).getByText("响应模式")).toHaveAttribute("data-shrink", "true");

    await user.type(
      within(dialog).getByRole("textbox", { name: "远程订阅 URL" }),
      "https://subscription.example/nodes?token=a+b&name=HK#primary",
    );

    const expected = "https://public.example/convert?url=https%3A%2F%2Fsubscription.example%2Fnodes%3Ftoken%3Da%2Bb%26name%3DHK%23primary&to_format=base64";
    expect(within(dialog).getByText(expected)).toBeInTheDocument();
    const copyButton = within(dialog).getByRole("button", { name: "复制完整链接" });
    await waitFor(() => expect(copyButton).toBeEnabled());
    await user.click(copyButton);

    expect(onCopyUrl).toHaveBeenCalledWith(expected);
    expect(listFormatCapabilities).toHaveBeenCalledOnce();
  });

  it("switches to inline input, preserves special characters, and emits JSON mode", async () => {
    const user = userEvent.setup();
    const onCopyUrl = vi.fn().mockResolvedValue({ copied: true });
    renderDialog({ listFormatCapabilities: vi.fn().mockResolvedValue(capabilities), onCopyUrl });
    const dialog = screen.getByRole("dialog", { name: "生成转换链接" });
    await within(dialog).findByRole("combobox", { name: "输出格式" });

    await user.click(within(dialog).getByRole("button", { name: "内联内容" }));
    const content = "ss://method:secret@example.com:8388#HK+1\nvmess://example&next";
    await user.type(within(dialog).getByRole("textbox", { name: "节点内容" }), content);
    await user.selectOptions(within(dialog).getByRole("combobox", { name: "输出格式" }), "json-nodes");
    await user.selectOptions(within(dialog).getByRole("combobox", { name: "响应模式" }), "json");
    const copyButton = within(dialog).getByRole("button", { name: "复制完整链接" });
    await waitFor(() => expect(copyButton).toBeEnabled());
    await user.click(copyButton);

    const copied = new URL(onCopyUrl.mock.calls[0][0]);
    expect(copied.searchParams.get("content")).toBe(content);
    expect(copied.searchParams.get("from_format")).toBe("uri-list");
    expect(copied.searchParams.get("to_format")).toBe("json-nodes");
    expect(copied.searchParams.get("response")).toBe("json");
    expect(copied.searchParams.has("url")).toBe(false);
  });

  it("shows validation without copying and selects the generated URL when clipboard fallback is required", async () => {
    const user = userEvent.setup();
    const onCopyUrl = vi.fn().mockResolvedValue({ copied: false, url: "attempted" });
    renderDialog({ listFormatCapabilities: vi.fn().mockResolvedValue(capabilities), onCopyUrl });
    const dialog = screen.getByRole("dialog", { name: "生成转换链接" });
    await within(dialog).findByRole("combobox", { name: "输出格式" });

    const input = within(dialog).getByRole("textbox", { name: "远程订阅 URL" });
    await user.type(input, "ftp://example.com/sub");
    expect(within(dialog).getByText("远程订阅 URL 必须使用 HTTP 或 HTTPS。")).toBeInTheDocument();
    expect(within(dialog).getByRole("button", { name: "复制完整链接" })).toBeDisabled();
    expect(onCopyUrl).not.toHaveBeenCalled();

    await user.clear(input);
    await user.type(input, "https://example.com/sub");
    const generated = within(dialog).getByText(/https:\/\/public\.example\/convert\?/);
    const copyButton = within(dialog).getByRole("button", { name: "复制完整链接" });
    await waitFor(() => expect(copyButton).toBeEnabled());
    await user.click(copyButton);

    await waitFor(() => expect(window.getSelection()?.toString()).toBe(generated.textContent));
  });

  it("reports capability loading failures and retries", async () => {
    const user = userEvent.setup();
    const listFormatCapabilities = vi.fn()
      .mockRejectedValueOnce(new Error("catalog unavailable"))
      .mockResolvedValueOnce(capabilities);
    renderDialog({ listFormatCapabilities, onCopyUrl: vi.fn() });

    expect(await screen.findByText("catalog unavailable")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "重试" }));

    await waitFor(() => expect(listFormatCapabilities).toHaveBeenCalledTimes(2));
    expect(await screen.findByRole("combobox", { name: "输出格式" })).toHaveValue("base64");
  });
});

function renderDialog({
  listFormatCapabilities,
  onCopyUrl,
}: {
  listFormatCapabilities: () => Promise<FormatCapabilityList>;
  onCopyUrl: (url: string) => Promise<CopyShareResult>;
}) {
  const client = { listFormatCapabilities } as unknown as ApiClient;
  return render(
    <ConvertLinkDialog
      client={client}
      publicBaseUrl="https://public.example"
      onClose={vi.fn()}
      onCopyUrl={onCopyUrl}
    />,
  );
}

function capability(direction: FormatCapabilitySummary["direction"], format: string): FormatCapabilitySummary {
  return {
    direction,
    field_counts: { lossy: 0, raw_only: 0, supported: 1 },
    format,
    href: `/v1/capabilities/formats/${direction}/${format}`,
    node_types: ["ss"],
    reversible: false,
    revisions: [],
  };
}
