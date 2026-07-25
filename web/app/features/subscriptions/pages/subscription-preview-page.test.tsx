import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type {
  SubscriptionPreview,
  SubscriptionPreviewWarning,
} from "~/features/subscriptions/model/types";
import {
  allStatusSubscriptionPreview,
  noop,
  subscriptionPreview,
  subscriptions,
} from "~/features/subscriptions/test-data";

import { SubscriptionPreviewPage } from "./subscription-preview-page";

describe("SubscriptionPreviewPage", () => {
  it("renders preview metrics with removed followed by warning count", () => {
    render(
      <SubscriptionPreviewPage
        item={subscriptions[0]}
        preview={{
          ...subscriptionPreview,
          beforeCount: 5,
          afterCount: 4,
          statusCounts: { added: 0, modified: 2, removed: 1, unchanged: 0 },
          warnings: [
            { code: "quick_settings_warning", message: "left unchanged", node: "keep" },
            { code: "rename_warning", message: "name unchanged", node: "drop" },
          ],
        }}
        onBack={noop}
        onRefresh={noop}
      />,
    );

    const summary = screen.getByLabelText("预览统计");
    expect(within(summary).getByText("已移除")).toBeInTheDocument();
    expect(within(summary).getByText("警告")).toBeInTheDocument();
    expect(within(summary).getAllByText("2")).toHaveLength(1);
    const labels = within(summary).getAllByText(/处理前|处理后|已移除|警告/).map((node) => node.textContent);
    expect(labels).toEqual(["处理前", "处理后", "已移除", "警告"]);
  });
  it("keeps subscription preview status copy concise and refresh context accessible", () => {
    const { rerender } = render(
      <SubscriptionPreviewPage item={subscriptions[0]} pending onBack={noop} onRefresh={noop} />,
    );

    const refresh = screen.getByRole("button", { name: "刷新订阅预览" });
    expect(refresh).toHaveTextContent("刷新");
    expect(screen.getByRole("heading", { name: "正在计算" })).toBeInTheDocument();
    expect(screen.getByText("以已保存的订阅定义为准。")).toBeInTheDocument();

    rerender(<SubscriptionPreviewPage item={subscriptions[0]} failed onBack={noop} onRefresh={noop} />);

    expect(screen.getByRole("heading", { name: "计算失败" })).toBeInTheDocument();
    expect(screen.getByText("检查来源和处理器配置。")).toBeInTheDocument();
  });
  it("renders source preview cards with filters and expandable details", async () => {
    const user = userEvent.setup();
    const previewWithWarning = {
      ...subscriptionPreview,
      warnings: [{
        code: "quick_settings_warning",
        message: "left unchanged",
        node: "keep",
        node_index: 7,
        node_context: {
          format: "mihomo",
          name: "keep",
          raw: { name: "keep", type: "ss", server: "example.com", port: 8388 },
        },
      }, {
        code: "probe_tcp_failed",
        message: "probe result reported probe_tcp_failed",
        node_index: 2,
        node_context: {
          name: "probe-dead",
          type: "vless",
          server: "dead.example.com",
          port: 443,
        },
      } as SubscriptionPreviewWarning],
    };
    const { container } = render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={previewWithWarning} />);

    expect(screen.getByRole("heading", { name: "节点预览" })).toBeInTheDocument();
    const summary = screen.getByLabelText("预览统计");
    expect(within(summary).getAllByText(/^\d+$/).map((node) => node.textContent)).toEqual(["2", "1", "1", "2"]);
    expect(within(summary).getAllByText(/处理前|处理后|已移除|警告/).map((node) => node.textContent)).toEqual(["处理前", "处理后", "已移除", "警告"]);
    expect(screen.getByText("keep")).toBeInTheDocument();
    expect(screen.getByText("source-keep")).toBeInTheDocument();
    expect(screen.getByText("source-keep")).toHaveClass("break-words", "[overflow-wrap:anywhere]");
    expect(screen.getByText("name")).toBeInTheDocument();
    expect(screen.getByText("keep -> source-keep")).toHaveClass("break-words", "[overflow-wrap:anywhere]");
    expect(screen.getAllByText((_, element) => element?.textContent === "ss · example.com:8388")).toHaveLength(1);
    expect(screen.getAllByText("已移除").length).toBeGreaterThan(0);
    const warningRegion = screen.getByRole("region", { name: "预览警告" });
    expect(within(warningRegion).getByText("警告")).toBeInTheDocument();
    expect(within(warningRegion).getByText("quick_settings_warning · left unchanged")).toBeInTheDocument();
    expect(within(warningRegion).getByText("probe-dead")).toBeInTheDocument();
    expect(within(warningRegion).getByText("probe_tcp_failed · probe result reported probe_tcp_failed")).toBeInTheDocument();
    expect(within(warningRegion).queryByRole("region", { name: "警告详情" })).not.toBeInTheDocument();
    await user.click(within(warningRegion).getByRole("button", { name: /keep/ }));
    const warningJson = within(warningRegion).getByRole("region", { name: "警告详情" });
    expect(warningJson).toHaveTextContent('"node": "keep"');
    expect(warningJson).toHaveTextContent('"node_index": 7');
    expect(warningJson).toHaveTextContent('"node_context"');
    expect(screen.getByText("source-keep").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-modified", "border-l-4", "border-l-warning");
    expect(screen.getByText("source-keep").closest(".MuiCard-root")?.querySelector(".MuiChip-root")).toBeNull();

    await user.click(screen.getByRole("button", { name: "已移除 1" }));

    expect(screen.queryByText("source-keep")).not.toBeInTheDocument();
    expect(screen.queryByText("移除节点")).not.toBeInTheDocument();
    expect(screen.getByText("drop")).toBeInTheDocument();
    expect(screen.getByText("drop").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-removed", "border-l-error");
    expect(screen.getByText("drop").closest(".MuiCard-root")?.querySelector(".MuiChip-root")).toBeNull();

    const removedSummary = screen.getByRole("button", { name: /drop/ });
    await user.click(removedSummary);

    expect(removedSummary).toHaveAttribute("aria-expanded", "true");
    const detailBlock = screen.getByRole("region", { name: "节点详情" });
    expect(detailBlock).toHaveTextContent("json-diff");
    expect(detailBlock).toHaveTextContent('- "name": "drop"');
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toHaveClass("code-diff-line-removed");
    expect(detailBlock.querySelector("pre")).toHaveClass("max-h-[min(70vh,640px)]", "overflow-auto", "whitespace-pre");
    expect(detailBlock.querySelector("pre")).not.toHaveClass("whitespace-pre-wrap");
    expect(detailBlock.querySelector("pre")).not.toHaveClass("break-words");
    expect(detailBlock).not.toHaveClass("bg-gray-100", "text-gray-900");
    expect(container.querySelector("details")).toBeNull();
    const diffModeButton = within(detailBlock).getByRole("button", { name: "差异" });
    const diffCopyButton = within(detailBlock).getByRole("button", { name: "复制节点详情" });
    expect(diffModeButton).toHaveAttribute("aria-pressed", "true");
    expect(Boolean(diffModeButton.compareDocumentPosition(diffCopyButton) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
    expect(detailBlock).toHaveTextContent('"name": "drop"');
    await user.click(removedSummary);
    expect(removedSummary).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("button", { name: "已移除 1" })).toHaveClass("MuiButton-colorError");

    await user.click(screen.getByRole("button", { name: "已修改 1" }));
    const modifiedSummary = screen.getByRole("button", { name: /source-keep/ });
    await user.click(modifiedSummary);

    const modifiedDetailBlock = screen.getByRole("region", { name: "节点详情" });
    expect(within(modifiedDetailBlock).getByRole("button", { name: "差异" })).toHaveAttribute("aria-pressed", "true");
    expect(modifiedDetailBlock).toHaveTextContent('- "name": "keep"');
    expect(modifiedDetailBlock).toHaveTextContent('+ "name": "source-keep"');
    expect(modifiedDetailBlock).toHaveTextContent(`+ "password": "${"a".repeat(160)}"`);
  });
  it("shows node metadata in a separate preview detail tab", async () => {
    const user = userEvent.setup();
    const previewWithMetadata: SubscriptionPreview = {
      ...subscriptionPreview,
      beforeCount: 1,
      afterCount: 1,
      statusCounts: { added: 0, modified: 0, removed: 0, unchanged: 1 },
      warnings: [],
      nodes: [{
        identity: "sha256:probe",
        status: "unchanged",
        before: {
          name: "node-a",
          type: "ss",
          server: "before.example.com",
          port: 8388,
          endpoint: "before.example.com:8388",
          raw: { meta: { source: "fixture" } },
        },
        after: {
          name: "node-a",
          type: "ss",
          server: "example.com",
          port: 8388,
          endpoint: "example.com:8388",
          raw: {
            meta: {
              "probe.alive": "true",
              "probe.duration_ms": "11",
              "probe.method": "tcp_connect",
            },
          },
        },
      }],
    };

    render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={previewWithMetadata} />);

    await user.click(screen.getByRole("button", { name: /node-a/ }));

    const detailBlock = screen.getByRole("region", { name: "节点详情" });
    expect(within(detailBlock).getByRole("button", { name: "差异" })).toHaveAttribute("aria-pressed", "true");
    expect(detailBlock).toHaveTextContent("json-diff");
    expect(detailBlock).toHaveTextContent('- "server": "before.example.com"');
    expect(detailBlock).toHaveTextContent('+ "server": "example.com"');
    expect(detailBlock).toHaveTextContent('"type": "ss"');
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toHaveClass("code-diff-line-removed");
    expect(detailBlock.querySelector('[data-diff-line="added"]')).toHaveClass("code-diff-line-added");
    expect(detailBlock).not.toHaveTextContent('"meta"');
    expect(detailBlock).not.toHaveTextContent('"probe.duration_ms"');
    const metadataButton = within(detailBlock).getByRole("button", { name: "元数据" });
    expect(metadataButton).toBeInTheDocument();

    await user.click(metadataButton);

    expect(metadataButton).toHaveAttribute("aria-pressed", "true");
    expect(detailBlock).toHaveTextContent("json-diff");
    expect(detailBlock).toHaveTextContent('+ "probe.duration_ms": "11"');
    expect(detailBlock).toHaveTextContent('- "source": "fixture"');
    expect(detailBlock).not.toHaveTextContent('"after"');
    expect(detailBlock).not.toHaveTextContent('"before"');
    expect(detailBlock).not.toHaveTextContent('"server": "example.com"');
  });
  it("puts name changes first in modified node diffs", async () => {
    const user = userEvent.setup();
    render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={allStatusSubscriptionPreview} />);

    await user.click(screen.getByRole("button", { name: /after-node/ }));

    const detailBlock = screen.getByRole("region", { name: "节点详情" });
    const text = (detailBlock.textContent ?? "").replace(/\s+/g, " ");
    expect(text.indexOf('- "name": "before-node"')).toBeGreaterThan(-1);
    expect(text.indexOf('- "name": "before-node"')).toBeLessThan(text.indexOf('- "endpoint": "before.example.com:8388"'));
    expect(detailBlock).toHaveTextContent('+ "name": "after-node"');
    expect(detailBlock).toHaveTextContent('"type": "ss"');
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toHaveClass("code-diff-line-removed");
    expect(detailBlock.querySelector('[data-diff-line="added"]')).toHaveClass("code-diff-line-added");
    expect(Array.from(detailBlock.querySelectorAll('[data-diff-line="unchanged"]')).some((line) => line.textContent?.includes('"type"'))).toBe(true);
  });
  it("keeps preview status treatment out of node cards", () => {
    render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={allStatusSubscriptionPreview} />);

    expect(screen.getByText("after-node").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-modified", "border-l-warning", "border-l-4");
    expect(screen.getByText("removed-node").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-removed", "border-l-error", "border-l-4");
    expect(screen.getByText("added-node").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-added", "border-l-success", "border-l-4");
    expect(screen.getByText("stable-node").closest(".MuiCard-root")).not.toHaveClass("node-status-stripe-unchanged", "border-l-info", "border-l-4");
    expect(screen.getAllByRole("article").some((card) => card.querySelector(".MuiChip-root"))).toBe(false);
  });
});
