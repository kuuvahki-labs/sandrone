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
  it("shows a display title while loading the canonical subscription name", async () => {
    const user = userEvent.setup();
    const loadPreview = vi.fn<LoadSubscriptionPreview>().mockResolvedValue(preview);
    const titledSubscriptions = [{ name: "provider", title: "Provider Main" }];
    render(<NodeSourceHarness loadPreview={loadPreview} subscriptions={titledSubscriptions} />);

    const picker = screen.getByRole("combobox", { name: "订阅" });
    await user.click(picker);
    await user.click(await screen.findByRole("option", { name: "Provider Main (provider)" }));

    expect(picker).toHaveValue("Provider Main (provider)");
    expect(loadPreview).toHaveBeenCalledWith("provider");
  });

  it("loads a collapsed sanitized preview and reuses it after switching sources", async () => {
    const user = userEvent.setup();
    const providerRequest = deferred<ConfigNodePreviewInput>();
    const allPreview: ConfigNodePreviewInput = {
      ...preview,
      subscriptionName: "all",
      nodes: [{
        identity: "sha256:all",
        after: { name: "all-node", type: "vmess", endpoint: "all.example:443" },
      }],
    };
    const loadPreview = vi.fn<LoadSubscriptionPreview>((name) => (
      name === "provider" ? providerRequest.promise : Promise.resolve(allPreview)
    ));
    render(<NodeSourceHarness loadPreview={loadPreview} />);

    const section = screen.getByRole("group", { name: "节点来源" });
    await selectSubscription(user, "provider");
    expect(within(section).getByRole("status")).toHaveTextContent("正在加载节点");
    expect(currentState()).toEqual({
      status: "loading",
      subscriptionName: "provider",
      preview: null,
    });
    await act(async () => providerRequest.resolve(preview));

    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    const expand = within(section).getByRole("button", { name: "展开" });
    expect(expand).toHaveTextContent("展开");
    expect(within(section).queryByText("hk")).not.toBeInTheDocument();
    await user.click(expand);
    expect(within(section).getByText("hk")).toBeInTheDocument();
    expect(within(section).getByText("ss · hk.example:8388")).toBeInTheDocument();
    expect(within(section).queryByText("removed")).not.toBeInTheDocument();
    expect(within(section).getByRole("button", { name: "刷新节点" })).toHaveTextContent("刷新");
    expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "provider",
      preview: { options: [{ name: "hk" }] },
    });

    await selectSubscription(user, "all");
    await waitFor(() => expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "all",
      preview: { options: [{ name: "all-node" }] },
    }));
    await selectSubscription(user, "provider");
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "provider",
      preview: { options: [{ name: "hk" }] },
    });
    expect(loadPreview.mock.calls.map(([name]) => name)).toEqual(["provider", "all"]);
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

  it("retries both generic errors and uncached preview identity mismatches", async () => {
    const user = userEvent.setup();
    const mismatched = { ...preview, subscriptionName: "provider" };
    const allPreview = { ...preview, subscriptionName: "all" };
    const loadPreview = vi.fn<LoadSubscriptionPreview>()
      .mockRejectedValueOnce(new Error("preview offline"))
      .mockResolvedValueOnce(preview)
      .mockResolvedValueOnce(mismatched)
      .mockResolvedValueOnce(allPreview);
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await selectSubscription(user, "provider");
    expect(await within(section).findByRole("alert")).toHaveTextContent("preview offline");
    expect(within(section).getByRole("combobox", { name: "订阅" })).toHaveValue("Provider (provider)");
    expect(currentState()).toEqual({
      status: "error",
      subscriptionName: "provider",
      preview: null,
      error: "preview offline",
    });
    const retryProvider = within(section).getByRole("button", { name: "重试加载节点" });
    expect(retryProvider).toHaveTextContent("重试");
    await user.click(retryProvider);
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(within(section).queryByRole("alert")).not.toBeInTheDocument();

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
    expect(currentState()).toMatchObject({ status: "ready", subscriptionName: "all" });
    expect(loadPreview.mock.calls.map(([name]) => name)).toEqual([
      "provider",
      "provider",
      "all",
      "all",
    ]);
  });

  it("invalidates on refresh and source switches while reusing an in-flight request", async () => {
    const user = userEvent.setup();
    const refreshRequest = deferred<ConfigNodePreviewInput>();
    const allRequest = deferred<ConfigNodePreviewInput>();
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
    let providerCalls = 0;
    const loadPreview = vi.fn<LoadSubscriptionPreview>((name) => {
      if (name === "all") return allRequest.promise;
      providerCalls += 1;
      if (providerCalls === 1) {
        return Promise.resolve(preview);
      }
      return refreshRequest.promise;
    });
    render(<NodeSourceHarness loadPreview={loadPreview} />);
    const section = screen.getByRole("group", { name: "节点来源" });

    await selectSubscription(user, "provider");
    expect(await within(section).findByText("已加载 1 个节点")).toBeInTheDocument();
    await user.click(within(section).getByRole("button", { name: "刷新节点" }));
    expect(currentState()).toEqual({
      status: "loading",
      subscriptionName: "provider",
      preview: null,
    });

    await selectSubscription(user, "all");
    expect(currentState()).toEqual({
      status: "loading",
      subscriptionName: "all",
      preview: null,
    });
    await act(async () => allRequest.resolve(allPreview));
    await waitFor(() => expect(currentState()).toMatchObject({ status: "ready", subscriptionName: "all" }));

    await selectSubscription(user, "provider");

    expect(currentState()).toEqual({
      status: "loading",
      subscriptionName: "provider",
      preview: null,
    });
    expect(within(section).queryByText("已加载 1 个节点")).not.toBeInTheDocument();
    expect(loadPreview.mock.calls.map(([name]) => name)).toEqual(["provider", "provider", "all"]);

    await act(async () => refreshRequest.resolve(refreshedPreview));
    await waitFor(() => expect(currentState()).toMatchObject({
      status: "ready",
      subscriptionName: "provider",
      preview: { options: [{ name: "fresh-hk" }] },
    }));
  });

  it("deduplicates the initial preview request under React Strict Mode", async () => {
    const loadPreview = vi.fn<LoadSubscriptionPreview>().mockResolvedValue(preview);
    render(<StrictMode><NodeSourceHarness initialSelected="provider" loadPreview={loadPreview} /></StrictMode>);

    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(loadPreview).toHaveBeenCalledTimes(1);
  });
});

function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, reject, resolve };
}

function NodeSourceHarness({ initialSelected = "", loadPreview, subscriptions: availableSubscriptions = subscriptions }: { initialSelected?: string; loadPreview: LoadSubscriptionPreview; subscriptions?: ResourceOption[] }) {
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
        subscriptions={availableSubscriptions}
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
