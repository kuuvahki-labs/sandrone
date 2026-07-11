import { StrictMode, useState } from "react";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { ConfigNodePreviewInput } from "~/features/files/config/model/node-source";
import type { ResourceOption } from "~/shared/resources/types";

import {
  ConfigNodeSourceSection,
  type ConfigNodeSourceState,
  type LoadSubscriptionPreview,
} from "./node-source";

const subscriptions: ResourceOption[] = [
  { name: "provider", title: "Provider" },
  { name: "all", title: "All" },
];

const preview: ConfigNodePreviewInput = {
  subscriptionName: "provider",
  nodes: [
    {
      identity: "sha256:current",
      after: { name: "hk", type: "ss", endpoint: "hk.example:8388" },
    },
    {
      identity: "sha256:removed",
    },
  ],
  warnings: [],
};

describe("config node source section", () => {
  it("selects one subscription, loads its preview, and shows only current nodes", async () => {
    const user = userEvent.setup();
    const loadPreview = vi.fn<LoadSubscriptionPreview>().mockResolvedValue(preview);
    render(<NodeSourceHarness loadPreview={loadPreview} />);

    const section = screen.getByRole("group", { name: "节点来源" });
    await user.click(within(section).getByRole("combobox", { name: "订阅" }));
    await user.click(await screen.findByRole("option", { name: /provider/ }));

    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    const expand = within(section).getByRole("button", { name: "展开" });
    expect(expand).toHaveTextContent("展开");
    await user.click(expand);
    expect(within(section).getByText("hk")).toBeInTheDocument();
    expect(within(section).getByText("ss · hk.example:8388")).toBeInTheDocument();
    expect(within(section).queryByText("removed")).not.toBeInTheDocument();
    expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "provider",
      preview: { options: [{ name: "hk" }] },
    });
    expect(loadPreview).toHaveBeenCalledTimes(1);
    expect(loadPreview).toHaveBeenCalledWith("provider");
  });

  it("shows an inline error and retries without changing the selected subscription", async () => {
    const user = userEvent.setup();
    const loadPreview = vi.fn<LoadSubscriptionPreview>()
      .mockRejectedValueOnce(new Error("preview offline"))
      .mockResolvedValueOnce(preview);
    render(<NodeSourceHarness loadPreview={loadPreview} />);

    const section = screen.getByRole("group", { name: "节点来源" });
    await user.click(within(section).getByRole("combobox", { name: "订阅" }));
    await user.click(await screen.findByRole("option", { name: /provider/ }));

    expect(await within(section).findByRole("alert")).toHaveTextContent("preview offline");
    expect(within(section).getByRole("combobox", { name: "订阅" })).toHaveValue("provider");
    const retry = within(section).getByRole("button", { name: "重试加载节点" });
    expect(retry).toHaveTextContent("重试");
    await user.click(retry);

    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(within(section).queryByRole("alert")).not.toBeInTheDocument();
    expect(loadPreview).toHaveBeenCalledTimes(2);
  });

  it("reuses sanitized previews when switching back to a subscription", async () => {
    const user = userEvent.setup();
    const allPreview: ConfigNodePreviewInput = {
      ...preview,
      subscriptionName: "all",
      nodes: [{ identity: "sha256:all", after: { name: "all-node", type: "vmess", endpoint: "all.example:443" } }],
    };
    const loadPreview = vi.fn<LoadSubscriptionPreview>(async (name) => name === "provider" ? preview : allPreview);
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });
    const picker = within(section).getByRole("combobox", { name: "订阅" });

    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: /provider/ }));
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: /all/ }));
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: /provider/ }));

    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(loadPreview.mock.calls.map(([name]) => name)).toEqual(["provider", "all"]);
  });

  it("shows loading state and keeps the loaded node list collapsible", async () => {
    const user = userEvent.setup();
    let resolvePreview: ((value: ConfigNodePreviewInput) => void) | undefined;
    const loadPreview = vi.fn<LoadSubscriptionPreview>(() => new Promise((resolve) => { resolvePreview = resolve; }));
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await user.click(within(section).getByRole("combobox", { name: "订阅" }));
    await user.click(await screen.findByRole("option", { name: /provider/ }));
    expect(within(section).getByRole("status")).toHaveTextContent("正在加载节点");
    await act(async () => { resolvePreview?.(preview); });

    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(within(section).queryByText("hk")).not.toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "展开" }));
    expect(within(section).getByText("hk")).toBeInTheDocument();
    expect(within(section).getByRole("button", { name: "刷新节点" })).toHaveTextContent("刷新");
  });

  it("shows sanitized preview warnings and candidate omissions without blocking the source", async () => {
    const user = userEvent.setup();
    const warningPreview: ConfigNodePreviewInput = {
      ...preview,
      nodes: [
        { identity: "one", after: { name: "duplicate", type: "ss", endpoint: "one.example:1" } },
        { identity: "two", after: { name: "duplicate", type: "vmess", endpoint: "two.example:2" } },
        { identity: "three", after: { name: " ", type: "ss", endpoint: "three.example:3" } },
      ],
      warnings: [{
        code: "parse_unknown_field",
        field: "uri.query.mode",
        message: "field preserved in NodeIR Raw",
        node: "duplicate",
        source: "uri-list",
        raw: "secret://credential",
      } as never],
    };
    render(<NodeSourceHarness loadPreview={vi.fn().mockResolvedValue(warningPreview)} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await user.click(within(section).getByRole("combobox", { name: "订阅" }));
    await user.click(await screen.findByRole("option", { name: /provider/ }));

    expect(await within(section).findByText("已加载 2 个节点，告警（1）")).toBeInTheDocument();
    expect(within(section).queryByRole("heading", { name: "告警（1）" })).not.toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "展开" }));

    expect(await within(section).findByText(/重复节点名称.*duplicate/)).toBeInTheDocument();
    expect(within(section).getByText(/1 个节点没有名称/)).toBeInTheDocument();
    expect(within(section).getByRole("heading", { name: "告警（1）" })).toBeInTheDocument();
    expect(within(section).getByRole("heading", { name: "parse_unknown_field · field preserved in NodeIR Raw" })).toBeInTheDocument();
    const warningDetail = within(section).getByRole("button", { name: /duplicate.*警告详情/ });
    expect(within(warningDetail).getByText("duplicate").parentElement?.parentElement).toHaveTextContent(
      "node: duplicate · source: uri-list · field: uri.query.mode",
    );
    expect(section).not.toHaveTextContent("secret://credential");
    expect(currentState()).toMatchObject({
      status: "ready",
      preview: { options: [{ name: "duplicate" }] },
    });
  });

  it("invalidates the parent preview immediately when another source starts loading", async () => {
    const user = userEvent.setup();
    let resolveAll: ((value: ConfigNodePreviewInput) => void) | undefined;
    const allPreview: ConfigNodePreviewInput = { ...preview, subscriptionName: "all" };
    const loadPreview = vi.fn<LoadSubscriptionPreview>((name) => {
      if (name === "provider") return Promise.resolve(preview);
      return new Promise((resolve) => { resolveAll = resolve; });
    });
    render(<NodeSourceHarness loadPreview={loadPreview} />);

    await selectSubscription(user, "provider");
    await waitFor(() => expect(screen.getByTestId("preview-state")).toHaveTextContent('"status":"ready"'));
    await selectSubscription(user, "all");
    expect(screen.getByTestId("preview-state")).toHaveTextContent(JSON.stringify({
      status: "loading",
      subscriptionName: "all",
      preview: null,
    }));

    await act(async () => resolveAll?.(allPreview));
    expect(screen.getByTestId("preview-state")).toHaveTextContent('"subscriptionName":"all"');
    expect(screen.getByTestId("preview-state")).toHaveTextContent('"status":"ready"');
  });

  it("invalidates the parent preview immediately during a forced refresh", async () => {
    const user = userEvent.setup();
    let resolveRefresh: ((value: ConfigNodePreviewInput) => void) | undefined;
    const loadPreview = vi.fn<LoadSubscriptionPreview>()
      .mockResolvedValueOnce(preview)
      .mockImplementationOnce(() => new Promise((resolve) => { resolveRefresh = resolve; }));
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await selectSubscription(user, "provider");
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "刷新节点" }));
    expect(screen.getByTestId("preview-state")).toHaveTextContent(JSON.stringify({
      status: "loading",
      subscriptionName: "provider",
      preview: null,
    }));

    await act(async () => resolveRefresh?.(preview));
    expect(screen.getByTestId("preview-state")).toHaveTextContent('"status":"ready"');
    expect(loadPreview).toHaveBeenCalledTimes(2);
  });

  it("keeps a refreshed subscription loading when switching away and back", async () => {
    const user = userEvent.setup();
    let resolveRefresh: ((value: ConfigNodePreviewInput) => void) | undefined;
    const refreshedPreview: ConfigNodePreviewInput = {
      ...preview,
      nodes: [{
        identity: "sha256:fresh",
        after: { name: "fresh-hk", type: "ss", endpoint: "fresh.example:8388" },
      }],
    };
    const allPreview: ConfigNodePreviewInput = {
      ...preview,
      subscriptionName: "all",
      nodes: [{
        identity: "sha256:all",
        after: { name: "all-node", type: "vmess", endpoint: "all.example:443" },
      }],
    };
    const loadPreview = vi.fn<LoadSubscriptionPreview>((name) => {
      if (name === "all") return Promise.resolve(allPreview);
      if (loadPreview.mock.calls.filter(([calledName]) => calledName === "provider").length === 1) {
        return Promise.resolve(preview);
      }
      return new Promise((resolve) => { resolveRefresh = resolve; });
    });
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await selectSubscription(user, "provider");
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "刷新节点" }));
    await selectSubscription(user, "all");
    await waitFor(() => expect(currentState()).toMatchObject({ status: "ready", subscriptionName: "all" }));
    await selectSubscription(user, "provider");

    expect(currentState()).toEqual({
      status: "loading",
      subscriptionName: "provider",
      preview: null,
    });
    expect(within(section).queryByText("已加载 1 个节点")).not.toBeInTheDocument();
    expect(loadPreview.mock.calls.map(([name]) => name)).toEqual(["provider", "provider", "all"]);

    await act(async () => resolveRefresh?.(refreshedPreview));
    await waitFor(() => expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "provider",
      preview: { options: [{ name: "fresh-hk" }] },
    }));
  });

  it("rejects a mismatched preview identity without rendering or caching its nodes", async () => {
    const user = userEvent.setup();
    const mismatched = { ...preview, subscriptionName: "provider" };
    const allPreview = { ...preview, subscriptionName: "all" };
    const loadPreview = vi.fn<LoadSubscriptionPreview>()
      .mockResolvedValueOnce(mismatched)
      .mockResolvedValueOnce(allPreview);
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await selectSubscription(user, "all");
    expect(await within(section).findByRole("alert"))
      .toHaveTextContent("返回的节点预览与所选订阅不匹配。");
    expect(currentState()).toEqual({
      status: "error",
      subscriptionName: "all",
      preview: null,
      error: "返回的节点预览与所选订阅不匹配。",
    });
    expect(within(section).queryByText("已加载 1 个节点")).not.toBeInTheDocument();
    expect(screen.getByTestId("preview-state")).not.toHaveTextContent('"name":"hk"');

    await user.click(within(section).getByRole("button", { name: "重试加载节点" }));
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(loadPreview).toHaveBeenCalledTimes(2);
    expect(currentState()).toMatchObject({ status: "ready", subscriptionName: "all" });
  });

  it("deduplicates the initial preview request under React Strict Mode", async () => {
    const loadPreview = vi.fn<LoadSubscriptionPreview>().mockResolvedValue(preview);
    render(<StrictMode><NodeSourceHarness initialSelected="provider" loadPreview={loadPreview} /></StrictMode>);

    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(loadPreview).toHaveBeenCalledTimes(1);
  });
});

function NodeSourceHarness({ initialSelected = "", loadPreview }: { initialSelected?: string; loadPreview: LoadSubscriptionPreview }) {
  const [selected, setSelected] = useState(initialSelected);
  const [state, setState] = useState<ConfigNodeSourceState>({
    status: "idle",
    subscriptionName: "",
    preview: null,
  });
  return (
    <>
      <ConfigNodeSourceSection
        loadPreview={loadPreview}
        selected={selected}
        subscriptions={subscriptions}
        onSelectedChange={setSelected}
        onStateChange={setState}
      />
      <output data-testid="preview-state">{JSON.stringify(state)}</output>
    </>
  );
}

async function selectSubscription(
  user: ReturnType<typeof userEvent.setup>,
  name: string,
): Promise<void> {
  const section = screen.getByRole("group", { name: "节点来源" });
  const picker = within(section).getByRole("combobox", { name: "订阅" });
  await user.click(picker);
  await user.click(await screen.findByRole("option", { name: new RegExp(name) }));
}

function currentState(): ConfigNodeSourceState {
  return JSON.parse(screen.getByTestId("preview-state").textContent ?? "null") as ConfigNodeSourceState;
}
