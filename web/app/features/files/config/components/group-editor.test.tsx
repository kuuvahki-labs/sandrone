import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ConfigMap } from "~/features/files/config/model/editor-model";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";

import { ProxyGroupEditor } from "./group-editor";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
});

describe("ProxyGroupEditor runtime-filtered Mihomo groups", () => {
  it("places hidden beside the member source", async () => {
    const user = userEvent.setup();
    render(<ControlledEditor
      initialGroups={[{ name: "Proxy", type: "select", proxies: ["$nodes"], hidden: true }]}
      onChange={vi.fn()}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));
    const memberSource = screen.getByRole("combobox", { name: /成员来源|Member source/i });
    const hidden = screen.getByRole("checkbox", { name: /隐藏分组|Hide group/i });

    expect(memberSource.closest("[data-group-membership-controls]"))
      .toBe(hidden.closest("[data-group-membership-controls]"));
  });

  it("switches a runtime filter to fixed members without retaining dynamic fields", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledEditor
      initialGroups={[{
        name: "香港节点",
        type: "url-test",
        "include-all-proxies": true,
        filter: "(?i)HK",
        "exclude-filter": "(?i)home",
        url: "https://cp.cloudflare.com",
        interval: 300,
        lazy: true,
      }]}
      onChange={onChange}
    />);

    expect(screen.getByText("动态筛选")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "展开代理组 香港节点" }));
    expect(screen.getByRole("combobox", { name: "成员来源" })).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "包含正则" })).toHaveValue("(?i)HK");
    expect(screen.getByRole("textbox", { name: "排除正则" })).toHaveValue("(?i)home");
    expect(screen.queryByRole("button", { name: "添加成员" })).not.toBeInTheDocument();

    await choose(user, "成员来源", "固定成员");

    expect(onChange).toHaveBeenLastCalledWith([{
      name: "香港节点",
      type: "url-test",
      proxies: ["$nodes"],
      url: "https://cp.cloudflare.com",
      interval: 300,
      lazy: true,
    }]);
  });

  it("restores the last ordered fixed members after a runtime-filter round trip", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledEditor
      initialGroups={[{ name: "Proxy", type: "select", proxies: ["Auto", "$nodes"] }]}
      onChange={onChange}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));
    await choose(user, "成员来源", "动态筛选");
    await choose(user, "成员来源", "固定成员");

    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Proxy",
      type: "select",
      proxies: ["Auto", "$nodes"],
    }]);
  });

  it("normalizes load-balance health-check fields and removes them when returning to select", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledEditor
      initialGroups={[{ name: "Balanced", type: "select", proxies: ["$nodes"] }]}
      onChange={onChange}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Balanced" }));
    await choose(user, "类型", "load-balance");
    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Balanced",
      type: "load-balance",
      proxies: ["$nodes"],
      url: "https://cp.cloudflare.com",
      interval: 300,
      lazy: true,
      strategy: "sticky-sessions",
    }]);

    await choose(user, "类型", "select");
    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Balanced",
      type: "select",
      proxies: ["$nodes"],
    }]);
  });
});

describe("ProxyGroupEditor Shadowrocket schema", () => {
  it("places hidden beside the member source", async () => {
    const user = userEvent.setup();
    render(<ControlledEditor
      initialGroups={[{ name: "Proxy", type: "select", proxies: ["$nodes"], hidden: true }]}
      kind="shadowrocket"
      onChange={vi.fn()}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));
    const memberSource = screen.getByRole("combobox", { name: /成员来源|Member source/i });
    const hidden = screen.getByRole("checkbox", { name: /隐藏分组|Hide group/i });

    expect(memberSource.closest("[data-group-membership-controls]"))
      .toBe(hidden.closest("[data-group-membership-controls]"));
  });

  it("switches between policy regex and fixed proxies without retaining stale fields", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledEditor
      initialGroups={[{
        name: "Hong Kong",
        type: "url-test",
        "policy-regex-filter": "(?i)HK",
        interval: 300,
        timeout: 5,
        tolerance: 50,
      }]}
      kind="shadowrocket"
      onChange={onChange}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Hong Kong" }));
    expect(screen.getByRole("textbox", { name: "包含正则" })).toHaveValue("(?i)HK");
    expect(screen.queryByRole("textbox", { name: "排除正则" })).not.toBeInTheDocument();
    await choose(user, "成员来源", "固定成员");

    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Hong Kong",
      type: "url-test",
      proxies: ["$nodes"],
      interval: 300,
      timeout: 5,
      tolerance: 50,
    }]);

    await choose(user, "成员来源", "动态筛选");
    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Hong Kong",
      type: "url-test",
      "policy-regex-filter": "(?i)HK",
      interval: 300,
      timeout: 5,
      tolerance: 50,
    }]);
  });

  it("offers exactly the five Shadowrocket group types and no health-check URL field", async () => {
    const user = userEvent.setup();
    render(<ControlledEditor
      initialGroups={[{ name: "Proxy", type: "url-test", proxies: ["$nodes"], interval: 300, timeout: 5, tolerance: 50 }]}
      kind="shadowrocket"
      onChange={vi.fn()}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));
    await user.click(screen.getByRole("combobox", { name: "类型" }));
    expect((await screen.findAllByRole("option")).map((option) => option.textContent)).toEqual([
      "select", "url-test", "fallback", "load-balance", "random",
    ]);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: /检测地址|Check URL/i })).not.toBeInTheDocument();
    expect(screen.getByRole("spinbutton", { name: /间隔|Interval/i })).toHaveValue(300);
    expect(screen.getByRole("spinbutton", { name: /超时|Timeout/i })).toHaveValue(5);
    expect(screen.getByRole("spinbutton", { name: /容差|Tolerance/i })).toHaveValue(50);
  });

  it("treats Shadowrocket health interval and timeout as optional and removes cleared values", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ControlledEditor
      initialGroups={[{ name: "Auto", type: "url-test", proxies: ["DIRECT"], interval: 300, timeout: 5 }]}
      kind="shadowrocket"
      onChange={onChange}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Auto" }));
    const interval = screen.getByRole("spinbutton", { name: /间隔|Interval/i });
    const timeout = screen.getByRole("spinbutton", { name: /超时|Timeout/i });
    expect(interval).not.toBeRequired();
    expect(timeout).not.toBeRequired();

    await user.clear(interval);
    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Auto",
      type: "url-test",
      proxies: ["DIRECT"],
      timeout: 5,
    }]);
    await user.clear(timeout);
    expect(onChange).toHaveBeenLastCalledWith([{
      name: "Auto",
      type: "url-test",
      proxies: ["DIRECT"],
    }]);
  });
});

describe("ProxyGroupEditor sing-box summaries", () => {
  it("does not offer a hidden group control", async () => {
    const user = userEvent.setup();
    render(<ControlledEditor
      initialGroups={[{ type: "selector", tag: "Proxy", outbounds: ["direct"] }]}
      kind="sing-box"
      onChange={vi.fn()}
    />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));

    expect(screen.queryByRole("checkbox", { name: /隐藏分组|Hide group/i })).not.toBeInTheDocument();
  });

  it("uses native type labels for collapsed selector and urltest groups", () => {
    render(<ControlledEditor
      initialGroups={[
        { type: "selector", tag: "Proxy", outbounds: ["direct"] },
        { type: "urltest", tag: "Auto", outbounds: ["$nodes"], url: "https://example.com/generate_204", interval: "5m" },
      ]}
      kind="sing-box"
      onChange={vi.fn()}
    />);

    expect(screen.getByText("selector", { exact: true })).toBeInTheDocument();
    expect(screen.getByText("urltest", { exact: true })).toBeInTheDocument();
    expect(screen.queryByText("select", { exact: true })).not.toBeInTheDocument();
    expect(screen.queryByText("url-test", { exact: true })).not.toBeInTheDocument();
  });
});

function ControlledEditor({ initialGroups, kind = "mihomo", onChange }: {
  initialGroups: ConfigMap[];
  kind?: string;
  onChange: (groups: ConfigMap[]) => void;
}) {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  const adapter = configuration.adapter;
  const initialDrafts = adapter.groups.project(initialGroups);
  if (!initialDrafts) throw new Error(`expected representable ${kind} groups`);
  const [groups, setGroups] = useState(initialDrafts);
  return (
    <ProxyGroupEditor
      adapter={adapter}
      groups={groups}
      inboundReferences={{}}
      issues={[]}
      nodes={[]}
      ui={requireFileDriverUI(kind)}
      onChange={(next) => {
        setGroups(next);
        onChange(adapter.groups.serialize(next));
      }}
    />
  );
}

async function choose(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  option: string,
): Promise<void> {
  await user.click(screen.getByRole("combobox", { name: label }));
  await user.click(await screen.findByRole("option", { name: option }));
}
