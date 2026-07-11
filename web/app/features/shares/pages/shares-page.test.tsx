import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ShareItem } from "~/features/shares/model/types";

import { SharesPage } from "./shares-page";

const shares: ShareItem[] = [
  { id: "sh_123", title: "mobile", targetKind: "file", targetName: "default.yaml", status: "valid", publicUrl: "https://example.com/s/sh_123" },
];

describe("shares page", () => {
  it("copies a file share with the primary action and leaves only delete in its menu", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn();
    const onDelete = vi.fn();
    render(<SharesPage items={shares} onCopy={onCopy} onDelete={onDelete} />);

    expect(screen.getByRole("heading", { name: "分享" })).toBeInTheDocument();
    expect(screen.getByText("https://example.com/s/sh_123")).toBeInTheDocument();
    expect(screen.getAllByText("有效").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByRole("button", { name: "复制链接：mobile" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "复制链接：mobile" }));
    await user.click(screen.getByRole("button", { name: "mobile 更多操作" }));
    expect(screen.queryByRole("menuitem", { name: /复制为/ })).not.toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(onCopy).toHaveBeenCalledWith(shares[0]);
    expect(onDelete).toHaveBeenCalledWith(shares[0]);
  });

  it("offers every client format for subscription shares and forwards exact overrides", async () => {
    const user = userEvent.setup();
    const onCopy = vi.fn();
    const item: ShareItem = {
      ...shares[0],
      id: "sh_nodes",
      publicUrl: "https://example.com/s/sh_nodes?format=json-nodes",
      targetFormat: "json-nodes",
      targetKind: "subscription",
      targetName: "provider",
      title: "nodes",
    };
    render(<SharesPage items={[item]} onCopy={onCopy} onDelete={vi.fn()} />);

    const more = screen.getByRole("button", { name: "nodes 更多操作" });
    const cases = [
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

    expect(onCopy).toHaveBeenCalledTimes(4);
  });

  it("summarizes each share status in a balanced four-card grid", () => {
    const items: ShareItem[] = [
      shares[0],
      { ...shares[0], id: "valid-two", title: "valid-two" },
      { ...shares[0], id: "upcoming", title: "upcoming", status: "upcoming" },
      { ...shares[0], id: "expired", title: "expired", status: "expired" },
      { ...shares[0], id: "exhausted", title: "exhausted", status: "exhausted" },
    ];

    render(<SharesPage items={items} onCopy={vi.fn()} onDelete={vi.fn()} />);

    const summary = screen.getByLabelText("分享链接摘要");
    expect(Array.from(summary.children).map((item) => item.textContent)).toEqual([
      "2有效",
      "1未生效",
      "1已过期",
      "1次数用尽",
    ]);
    expect(summary).toHaveClass("grid-cols-2", "md:grid-cols-4");
    expect(within(summary).queryByText("全部")).not.toBeInTheDocument();
  });
});
