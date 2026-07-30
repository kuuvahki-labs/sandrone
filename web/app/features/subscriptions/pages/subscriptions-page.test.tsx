import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  createAction,
  noop,
  subscriptions,
  subscriptionTraffic,
} from "~/features/subscriptions/test-data";

import { SubscriptionsPage } from "./subscriptions-page";

describe("SubscriptionsPage", () => {
  it("renders the subscription home as a searchable MUI list screen", async () => {
    const user = userEvent.setup();
    const providerItem = { ...subscriptions[0], displayName: "机场主订阅", title: "机场主订阅" };
    const items = [
      providerItem,
      subscriptions[1],
      { kind: "local" as const, name: "local", title: "local", label: "本地订阅", status: "ready" as const, format: "uri-list" },
      subscriptions[2],
    ];
    const onEdit = vi.fn();
    const { container } = render(
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

    expect(screen.getByRole("heading", { name: "我的订阅" })).toBeInTheDocument();
    const summary = screen.getByLabelText("订阅摘要");
    expect(Array.from(summary.children).map((item) => item.textContent)).toEqual(["4总数", "1组合", "2远程", "1本地"]);
    expect(screen.getAllByRole("button", { name: "新建订阅" })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "编辑：provider" })).toBeInTheDocument();
    expect(screen.getByText("机场主订阅")).toBeInTheDocument();
    expect(screen.getByText("default")).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "订阅列表" });
    expect(iconForListItem(list, "编辑：provider", "CloudDownloadOutlinedIcon")).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：warn", "CloudDownloadOutlinedIcon")).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：local", "LinkOutlinedIcon")).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：default", "AccountTreeOutlinedIcon")).toBeInTheDocument();
    expect(within(list).getAllByText("远程").length).toBeGreaterThan(0);

    const searchbox = screen.getByRole("searchbox", { name: "搜索订阅" });
    const searchLabel = container.querySelector(`label[for="${searchbox.id}"]`);
    expect(searchLabel).toHaveTextContent("搜索");
    expect(searchbox).not.toHaveAttribute("placeholder");
    expect(searchLabel).not.toHaveClass("MuiInputLabel-shrink");
    await user.click(searchbox);
    expect(searchLabel).toHaveClass("MuiInputLabel-shrink");
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
  it("does not reserve a details area when traffic is unavailable", () => {
    render(
      <SubscriptionsPage
        createActions={[createAction("远程", noop, "新建远程订阅")]}
        items={subscriptions}
        onDelete={noop}
        onEdit={noop}
        onShare={noop}
      />,
    );

    const primaryRow = screen.getByRole("button", { name: "编辑：provider" }).parentElement;
    expect(primaryRow).not.toBeNull();
    expect(primaryRow?.nextElementSibling).toBeNull();
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

function iconForListItem(list: HTMLElement, actionName: string, iconTestId: string): Element | null {
  const action = within(list).getByRole("button", { name: actionName });
  const item = action.closest("li");
  expect(item).not.toBeNull();
  return item?.querySelector(`[data-testid="${iconTestId}"]`) ?? null;
}
