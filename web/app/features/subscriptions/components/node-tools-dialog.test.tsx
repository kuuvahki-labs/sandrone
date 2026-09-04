import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { NodeIPInfo, SubscriptionPreviewNode } from "~/features/subscriptions/model/types";
import { I18nProvider } from "~/shared/i18n/context";

import { NodeInfoDialog } from "./node-tools-dialog";

const node: SubscriptionPreviewNode = {
  endpoint: "proxy.example.com:443",
  name: "fixture-node",
  raw: { name: "fixture-node", server: "proxy.example.com", type: "trojan" },
  server: "proxy.example.com",
  type: "trojan",
};
const uri = "trojan://fixture-password@proxy.example.com:443#fixture-node";
const originalShare = Object.getOwnPropertyDescriptor(navigator, "share");

afterEach(() => {
  if (originalShare) Object.defineProperty(navigator, "share", originalShare);
  else Reflect.deleteProperty(navigator, "share");
  window.getSelection()?.removeAllRanges();
});

describe("NodeInfoDialog", () => {
  it("shows warnings, QR, exact URI, and shares only that URI", async () => {
    const user = userEvent.setup();
    const share = vi.fn(async () => undefined);
    Object.defineProperty(navigator, "share", { configurable: true, value: share });
    renderDialog({
      uriResult: {
        uri,
        warnings: [{ code: "render_lossy_field", field: "flow", message: "field omitted", node: "fixture-node" }],
      },
    });

    const dialog = await screen.findByRole("dialog", { name: "节点信息" });
    expect(within(dialog).getByText("fixture-node")).toBeInTheDocument();
    expect(await within(dialog).findByText(uri)).toBeInTheDocument();
    expect(within(dialog).getByRole("region", { name: "URI 转换警告" })).toBeInTheDocument();
    const qrCode = await within(dialog).findByRole("img", { name: "fixture-node 的节点 URI 二维码" });
    const attributionHeading = within(dialog).getByRole("heading", { name: "节点归属" });
    expect(attributionHeading.compareDocumentPosition(qrCode) & Node.DOCUMENT_POSITION_FOLLOWING).not.toBe(0);
    await user.click(within(dialog).getByRole("button", { name: "系统分享" }));
    expect(share).toHaveBeenCalledWith({ text: uri, title: "fixture-node" });
  });

  it("hides unsupported system sharing and selects the URI after copy failure", async () => {
    const user = userEvent.setup();
    Reflect.deleteProperty(navigator, "share");
    renderDialog({ uriResult: { uri, warnings: [] }, onCopyURI: vi.fn().mockResolvedValue(false) });

    expect(screen.queryByRole("button", { name: "系统分享" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: "复制节点 URI" }));
    expect(window.getSelection()?.toString()).toBe(uri);
  });

  it("ignores cancellation but keeps the copy path after other share failures", async () => {
    const user = userEvent.setup();
    const share = vi.fn()
      .mockRejectedValueOnce(new DOMException("cancelled", "AbortError"))
      .mockRejectedValueOnce(new Error("share failed"));
    Object.defineProperty(navigator, "share", { configurable: true, value: share });
    renderDialog({ uriResult: { uri, warnings: [] } });

    await user.click(await screen.findByRole("button", { name: "系统分享" }));
    expect(screen.queryByText("系统分享失败，请改用复制 URI。")).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "系统分享" }));
    expect(await screen.findByText("系统分享失败，请改用复制 URI。")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "复制节点 URI" })).toBeEnabled();
  });

  it("renders on demand and reports unsupported nodes without a stale URI", async () => {
    const onRenderURI = vi.fn().mockRejectedValue(new Error("unsupported protocol"));
    renderDialog({ onRenderURI });

    expect(screen.getByText("正在生成节点 URI…")).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent("unsupported protocol");
    expect(screen.queryByText(uri)).not.toBeInTheDocument();
    expect(onRenderURI).toHaveBeenCalledWith(node);
  });

  it("looks up attribution once when opened and credits ipwho.is", async () => {
    const onLookupIP = vi.fn().mockResolvedValue({
      server: "proxy.example.com",
      ip: "8.8.8.8",
      ipVersion: 4,
      public: true,
      countryCode: "US",
      country: "United States",
      continentCode: "NA",
      continent: "North America",
      asn: "AS64500",
      asName: "Example Network",
      asDomain: "example.net",
      source: { name: "ipwho.is", url: "https://ipwho.is" },
    });
    renderDialog({ uriResult: { uri, warnings: [] }, onLookupIP });

    expect(onLookupIP).toHaveBeenCalledWith(node);
    expect(await screen.findByText("8.8.8.8")).toBeInTheDocument();
    expect(screen.getByText("United States (US) · North America (NA)")).toBeInTheDocument();
    expect(screen.getByText("AS64500 · Example Network · example.net")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "ipwho.is" })).toHaveAttribute("href", "https://ipwho.is");
  });

  it("explains locally classified addresses without provider attribution", async () => {
    renderDialog({
      uriResult: { uri, warnings: [] },
      onLookupIP: vi.fn().mockResolvedValue({ server: "proxy.example.com", ip: "198.18.0.1", ipVersion: 4, public: false }),
    });

    expect(await screen.findByText("198.18.0.1 是非公开地址。")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "ipwho.is" })).not.toBeInTheDocument();
  });

  it("offers retry when automatic attribution lookup fails", async () => {
    const user = userEvent.setup();
    const onLookupIP = vi.fn()
      .mockRejectedValueOnce(new Error("lookup failed"))
      .mockResolvedValueOnce({ server: "proxy.example.com", ip: "198.18.0.1", ipVersion: 4, public: false });
    renderDialog({ onLookupIP });

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("lookup failed");
    await user.click(within(alert).getByRole("button", { name: "重试" }));

    expect(await screen.findByText("198.18.0.1 是非公开地址。")).toBeInTheDocument();
    expect(onLookupIP).toHaveBeenCalledTimes(2);
  });
});

function renderDialog({
  uriResult,
  onCopyURI = vi.fn().mockResolvedValue(true),
  onLookupIP = vi.fn().mockResolvedValue(undefined),
  onRenderURI = vi.fn().mockResolvedValue(uriResult ?? { uri, warnings: [] }),
}: {
  uriResult?: { uri: string; warnings: Array<{ code: string; message: string; field?: string; node?: string }> };
  onCopyURI?: (value: string) => Promise<boolean>;
  onLookupIP?: (value: SubscriptionPreviewNode) => Promise<NodeIPInfo>;
  onRenderURI?: (value: SubscriptionPreviewNode) => Promise<{ uri: string; warnings: [] }>;
} = {}) {
  return render(
    <I18nProvider>
      <NodeInfoDialog
        node={node}
        onClose={vi.fn()}
        onCopyURI={onCopyURI}
        onLookupIP={onLookupIP}
        onRenderURI={onRenderURI}
      />
    </I18nProvider>,
  );
}
