import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ShareItem } from "~/features/shares/model/types";

import { SharesPage } from "./shares-page";

const shares: ShareItem[] = [
  {
    id: "sh_123",
    title: "mobile",
    targetKind: "file",
    targetName: "default.yaml",
    validFrom: "2026-07-01T01:02:03Z",
    validUntil: "2026-08-01T01:02:03Z",
    createdAt: "2026-06-30T01:02:03Z",
    updatedAt: "2026-07-02T04:05:06Z",
    ageRecipient: "age1recipient",
    status: "valid",
    publicUrl: "https://example.com/s/sh_123",
  },
];

describe("shares page", () => {
  it("combines availability and target filters and clears each independently", async () => {
    const user = userEvent.setup();
    const items: ShareItem[] = [
      shares[0],
      { ...shares[0], id: "sh_active_subscription", publicUrl: "https://example.com/s/sh_active_subscription", targetKind: "subscription", status: "valid", title: "active subscription" },
      { ...shares[0], id: "sh_upcoming", publicUrl: "https://example.com/s/sh_upcoming", targetKind: "subscription", status: "upcoming", title: "future" },
      { ...shares[0], id: "sh_expired", publicUrl: "https://example.com/s/sh_expired", status: "expired", title: "old" },
    ];
    render(<SharesPage items={items} onCopy={vi.fn()} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);

    expect(within(screen.getByLabelText("分享链接摘要")).getAllByRole("button"))
      .toHaveLength(4);
    const unavailableFilter = screen.getByRole("button", { name: "按不可用筛选" });
    const subscriptionFilter = screen.getByRole("button", { name: "按订阅筛选" });
    await user.click(unavailableFilter);
    expect(unavailableFilter).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("future")).toBeInTheDocument();
    expect(screen.getByText("old")).toBeInTheDocument();
    expect(screen.queryByText("mobile")).not.toBeInTheDocument();
    expect(screen.queryByText("active subscription")).not.toBeInTheDocument();

    await user.click(subscriptionFilter);
    expect(subscriptionFilter).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByText("future")).toBeInTheDocument();
    expect(screen.queryByText("old")).not.toBeInTheDocument();

    await user.click(unavailableFilter);
    expect(unavailableFilter).toHaveAttribute("aria-pressed", "false");
    expect(screen.getByText("active subscription")).toBeInTheDocument();
    expect(screen.getByText("future")).toBeInTheDocument();
    expect(screen.queryByText("mobile")).not.toBeInTheDocument();

    await user.click(subscriptionFilter);
    expect(subscriptionFilter).toHaveAttribute("aria-pressed", "false");
    const fileFilter = screen.getByRole("button", { name: "按文件筛选" });
    await user.click(fileFilter);
    expect(screen.getByText("mobile")).toBeInTheDocument();
    expect(screen.getByText("old")).toBeInTheDocument();
    expect(screen.queryByText("active subscription")).not.toBeInTheDocument();
    expect(screen.queryByText("future")).not.toBeInTheDocument();
  });

  it("searches share fields and status copy together with the status filter", () => {
    const items: ShareItem[] = [
      shares[0],
      {
        ...shares[0],
        id: "sh_upcoming",
        publicUrl: "https://example.com/s/sh_upcoming?format=mihomo-proxies",
        status: "upcoming",
        targetFormat: "mihomo-proxies",
        targetKind: "subscription",
        targetName: "provider-main",
        title: "future",
      },
      {
        ...shares[0],
        id: "sh_expired",
        publicUrl: "https://example.com/s/sh_expired",
        status: "expired",
        targetName: "legacy.yaml",
        title: "old",
      },
    ];
    render(<SharesPage items={items} onCopy={vi.fn()} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);

    const searchbox = screen.getByRole("searchbox", { name: "搜索分享" });
    fireEvent.change(searchbox, { target: { value: "PROVIDER-MAIN" } });
    expect(screen.getByRole("button", { name: "查看详情：future" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看详情：mobile" })).not.toBeInTheDocument();

    fireEvent.change(searchbox, { target: { value: "已过期" } });
    expect(screen.getByRole("button", { name: "查看详情：old" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "查看详情：future" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "按有效筛选" }));
    expect(screen.getByText("没有符合当前筛选的项目")).toBeInTheDocument();
  });

  it("opens share details from the card and leaves only delete in a file share menu", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn().mockResolvedValue({ copied: true });
    const onDelete = vi.fn();
    render(<SharesPage items={shares} onCopy={onCopy} onCopyUrl={vi.fn()} onDelete={onDelete} onGenerateConvertLink={vi.fn()} />);

    expect(screen.getByRole("heading", { name: "分享" })).toBeInTheDocument();
    expect(screen.getByText("https://example.com/s/sh_123")).toBeInTheDocument();
    expect(screen.getAllByText("有效").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole("button", { name: "查看详情：mobile" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "查看详情：mobile" }));
    const dialog = screen.getByRole("dialog", { name: "分享详情" });
    expect(within(dialog).getByText("sh_123")).toBeInTheDocument();
    expect(within(dialog).getByText("default.yaml")).toBeInTheDocument();
    expect(within(dialog).getByText("age X25519")).toBeInTheDocument();
    expect(dialog.querySelector('time[datetime="2026-06-30T01:02:03Z"]')).toBeInTheDocument();
    expect(dialog.querySelector('time[datetime="2026-07-02T04:05:06Z"]')).toBeInTheDocument();
    expect(onCopy).not.toHaveBeenCalled();

    await user.click(within(dialog).getByRole("button", { name: "复制链接" }));
    expect(onCopy).toHaveBeenCalledWith(shares[0]);
    await user.click(within(dialog).getByRole("button", { name: "关闭" }));
    expect(screen.queryByRole("dialog", { name: "分享详情" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "mobile 更多操作" }));
    expect(screen.queryByRole("menuitem", { name: /复制为/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(onDelete).toHaveBeenCalledWith(shares[0]);
  });

  it("selects the public URL without triggering the card copy action", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn().mockResolvedValue({ copied: true });
    render(<SharesPage items={shares} onCopy={onCopy} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);

    await user.click(screen.getByText(shares[0].publicUrl));

    expect(window.getSelection()?.toString()).toBe(shares[0].publicUrl);
    expect(onCopy).not.toHaveBeenCalled();
  });

  it("preserves a partial public URL selection made by dragging", () => {
    render(<SharesPage items={shares} onCopy={vi.fn().mockResolvedValue({ copied: true })} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);
    const publicUrl = screen.getByText(shares[0].publicUrl);
    const text = publicUrl.firstChild;
    const selection = window.getSelection();
    const range = document.createRange();
    range.setStart(text!, 0);
    range.setEnd(text!, 5);
    selection?.removeAllRanges();
    selection?.addRange(range);

    fireEvent.click(publicUrl);

    expect(selection?.toString()).toBe("https");
  });

  it("selects the public URL in details when copying cannot use the clipboard", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn().mockResolvedValue({ copied: false, url: shares[0].publicUrl });
    render(<SharesPage items={shares} onCopy={onCopy} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "查看详情：mobile" }));
    const dialog = screen.getByRole("dialog", { name: "分享详情" });
    await user.click(within(dialog).getByRole("button", { name: "复制链接" }));

    expect(window.getSelection()?.toString()).toBe(shares[0].publicUrl);
    expect(screen.queryByRole("dialog", { name: "请手动复制链接" })).not.toBeInTheDocument();
  });

  it("offers every client format for subscription shares and forwards exact overrides", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn().mockResolvedValue({ copied: true });
    const item: ShareItem = {
      ...shares[0],
      id: "sh_nodes",
      publicUrl: "https://example.com/s/sh_nodes?format=json-nodes",
      targetFormat: "json-nodes",
      targetKind: "subscription",
      targetName: "provider",
      title: "nodes",
    };
    render(<SharesPage items={[item]} onCopy={onCopy} onCopyUrl={vi.fn()} onDelete={vi.fn()} onGenerateConvertLink={vi.fn()} />);

    const more = screen.getByRole("button", { name: "nodes 更多操作" });
    const cases = [
      ["复制为通用订阅（Base64）", "base64"],
      ["复制为 URI list", "uri-list"],
      ["复制为 Mihomo", "mihomo-proxies"],
      ["复制为 sing-box", "sing-box-outbounds"],
      ["复制为 Shadowrocket", "shadowrocket-proxies"],
    ] as const;

    for (const [label, format] of cases) {
      await user.click(more);
      await user.click(screen.getByRole("menuitem", { name: label }));
      expect(onCopy).toHaveBeenLastCalledWith(item, format);
    }

    expect(onCopy).toHaveBeenCalledTimes(5);
    expect(screen.queryByRole("dialog", { name: "请手动复制链接" })).not.toBeInTheDocument();
  });

  it("shows the exact attempted format URL when a menu copy fails", async () => {
    const user = userEvent.setup();
    const attemptedUrl = "https://example.com/s/sh_nodes/nodes.txt?format=base64#install";
    const onCopy = vi.fn().mockResolvedValue({ copied: false, url: attemptedUrl });
    const item: ShareItem = {
      ...shares[0],
      id: "sh_nodes",
      targetKind: "subscription",
      targetName: "provider",
      formatFilenames: { base64: "nodes.txt" },
      title: "nodes",
    };

    render(
      <SharesPage
        items={[item]}
        onCopy={onCopy}
        onCopyUrl={vi.fn().mockResolvedValue({ copied: true })}
        onDelete={vi.fn()}
        onGenerateConvertLink={vi.fn()}
      />,
    );

    await user.click(screen.getByRole("button", { name: "nodes 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "复制为通用订阅（Base64）" }));

    const dialog = await screen.findByRole("dialog", { name: "请手动复制链接" });
    expect(within(dialog).getByText(attemptedUrl)).toBeInTheDocument();
    await waitFor(() => expect(window.getSelection()?.toString()).toBe(attemptedUrl));
  });

  it("offers the convert link generator even when there are no stored shares", async () => {
    const user = userEvent.setup();
    const onGenerateConvertLink = vi.fn();
    render(
      <SharesPage
        items={[]}
        onCopy={vi.fn().mockResolvedValue({ copied: true })}
        onCopyUrl={vi.fn()}
        onDelete={vi.fn()}
        onGenerateConvertLink={onGenerateConvertLink}
      />,
    );

    await user.click(screen.getByRole("button", { name: "转换链接" }));

    expect(onGenerateConvertLink).toHaveBeenCalledOnce();
    expect(screen.getByText("还没有分享链接")).toBeInTheDocument();
  });

});
