import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type {
  SubscriptionPreview,
  SubscriptionPreviewWarning,
} from "~/features/subscriptions/model/types";
import {
  allStatusSubscriptionPreview,
  createAction,
  noop,
  subscriptionPreview,
  subscriptions,
  subscriptionTraffic,
} from "~/features/subscriptions/test-data";

import { SubscriptionPreviewPage } from "./subscription-preview-page";
import { SubscriptionsPage } from "./subscriptions-page";

describe("SubscriptionPreviewPage", () => {
  it("keeps the summary warning metric raw while grouping the warning list", () => {
    const repeatedWarnings: SubscriptionPreview = {
      ...subscriptionPreview,
      warnings: [{
        code: "parse_unknown_field",
        field: "uri.query.mode",
        message: "field preserved in NodeIR Raw",
        node: "node-a",
        source: "uri-list",
      }, {
        code: "parse_unknown_field",
        field: "uri.query.mode",
        message: "field preserved in NodeIR Raw",
        node: "node-b",
        source: "uri-list",
      }],
    };

    render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={repeatedWarnings} />);

    const summary = screen.getByLabelText("预览统计");
    expect(within(summary).getAllByText(/^\d+$/).map((node) => node.textContent)).toEqual(["2", "1", "1", "2"]);
    const warningRegion = screen.getByRole("region", { name: "预览警告" });
    expect(within(warningRegion).getByText("1 组警告 · 2 条记录")).toBeInTheDocument();
    expect(within(warningRegion).getByText("2 个节点或位置受到影响")).toBeInTheDocument();
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
    render(<SubscriptionPreviewPage item={subscriptions[0]} onBack={noop} onRefresh={noop} preview={previewWithWarning} />);

    expect(screen.getByRole("heading", { name: "节点预览" })).toBeInTheDocument();
    const summary = screen.getByLabelText("预览统计");
    expect(within(summary).getAllByText(/^\d+$/).map((node) => node.textContent)).toEqual(["2", "1", "1", "2"]);
    expect(within(summary).getAllByText(/处理前|处理后|已移除|警告/).map((node) => node.textContent)).toEqual(["处理前", "处理后", "已移除", "警告"]);
    expect(screen.getByText("keep")).toBeInTheDocument();
    expect(screen.getByText("source-keep")).toBeInTheDocument();
    expect(screen.getByText("name")).toBeInTheDocument();
    expect(screen.getByText("keep -> source-keep")).toBeInTheDocument();
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
    await user.click(screen.getByRole("button", { name: "已移除 1" }));

    expect(screen.queryByText("source-keep")).not.toBeInTheDocument();
    expect(screen.queryByText("移除节点")).not.toBeInTheDocument();
    expect(screen.getByText("drop")).toBeInTheDocument();
    const removedSummary = screen.getByRole("button", { name: /drop/ });
    await user.click(removedSummary);

    expect(removedSummary).toHaveAttribute("aria-expanded", "true");
    const detailBlock = screen.getByRole("region", { name: "节点详情" });
    expect(detailBlock).toHaveTextContent("json-diff");
    expect(detailBlock).toHaveTextContent('- "name": "drop"');
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toBeInTheDocument();
    const diffModeButton = within(detailBlock).getByRole("button", { name: "差异" });
    const diffCopyButton = within(detailBlock).getByRole("button", { name: "复制节点详情" });
    expect(diffModeButton).toHaveAttribute("aria-pressed", "true");
    expect(Boolean(diffModeButton.compareDocumentPosition(diffCopyButton) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
    expect(detailBlock).toHaveTextContent('"name": "drop"');
    await user.click(removedSummary);
    expect(removedSummary).toHaveAttribute("aria-expanded", "false");
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
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toBeInTheDocument();
    expect(detailBlock.querySelector('[data-diff-line="added"]')).toBeInTheDocument();
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
    expect(detailBlock.querySelector('[data-diff-line="removed"]')).toBeInTheDocument();
    expect(detailBlock.querySelector('[data-diff-line="added"]')).toBeInTheDocument();
    expect(Array.from(detailBlock.querySelectorAll('[data-diff-line="unchanged"]')).some((line) => line.textContent?.includes('"type"'))).toBe(true);
  });
});

describe("SubscriptionsPage", () => {
  it("searches display names, resource names, and status copy before editing", async () => {
    const user = userEvent.setup();
    const providerItem = { ...subscriptions[0], displayName: "机场主订阅", title: "机场主订阅" };
    const items = [
      providerItem,
      subscriptions[1],
      { kind: "local" as const, name: "local", title: "local", label: "本地订阅", status: "ready" as const, format: "uri-list" },
      subscriptions[2],
    ];
    const onEdit = vi.fn();
    render(
      <SubscriptionsPage
        createActions={[
          createAction("远程", noop, "新建远程订阅"),
          createAction("本地", noop, "新建本地订阅"),
          createAction("组合", noop, "新建组合订阅"),
        ]}
        items={items}
        onDelete={noop}
        onEdit={onEdit}
        onShare={noop}
      />,
    );

    expect(screen.getByText("机场主订阅")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();

    const searchbox = screen.getByRole("searchbox", { name: "搜索订阅" });
    fireEvent.change(searchbox, { target: { value: "机场" } });
    expect(screen.getByText("机场主订阅")).toBeInTheDocument();
    expect(screen.queryByText("default")).not.toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "provider" } });
    expect(screen.getByText("机场主订阅")).toBeInTheDocument();
    expect(screen.queryByText("default")).not.toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "暂时不可用" } });
    expect(screen.getByText("warn")).toBeInTheDocument();
    expect(screen.queryByText("default")).not.toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "" } });

    await user.click(screen.getByRole("button", { name: "default 更多操作" }));
    expect(screen.queryByRole("menuitem", { name: "刷新" })).not.toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "诊断" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: "编辑" }));
    expect(onEdit).toHaveBeenCalledWith(items[3]);
  });
  it("renders subscription traffic on the list page", () => {
    const trafficWithUsage = {
      ...subscriptionTraffic,
      traffic: {
        sourceName: "provider",
        sourceUrl: "https://example.test/sub",
        uploadBytes: 1024,
        downloadBytes: 2048,
        usedBytes: 3072,
        totalBytes: 10240,
        remainingBytes: 7168,
        expiresAt: "2026-07-01T00:00:00Z",
        remainingDays: 368,
        resetDay: 14,
        appUrl: "https://panel.example.test",
        planName: "VIP 1",
      },
    };
    render(
      <SubscriptionsPage
        createActions={[createAction("远程", noop, "新建远程订阅")]}
        getTrafficKey={(item) => `${item.kind}:${item.name}`}
        items={subscriptions}
        trafficByKey={{ "remote:provider": trafficWithUsage }}
        onDelete={noop}
        onEdit={noop}
        onShare={noop}
      />,
    );

    const list = screen.getByRole("list", { name: "订阅列表" });
    expect(within(list).getByText("VIP 1")).toBeInTheDocument();
    const trafficSummary = "↑ 1 KiB · ↓ 2 KiB · TOT 10 KiB · 1Y 3D";
    expect(within(list).getByText(trafficSummary)).toBeInTheDocument();
    const progress = within(list).getByRole("progressbar", { name: trafficSummary });
    expect(progress).toHaveAttribute("aria-valuenow", "30");
    expect(within(list).getByRole("button", { name: "provider 更多操作" })).toBeInTheDocument();
  });
});
