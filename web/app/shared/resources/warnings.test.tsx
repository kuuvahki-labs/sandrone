import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { CollapsibleWarningPanel } from "~/shared/resources/warning-panel";
import { WarningPreferencesProvider } from "~/shared/resources/warning-preferences";
import { WarningList } from "~/shared/resources/warnings";

describe("warning list", () => {
  it("keeps a warning panel collapsed with one summary until requested", async () => {
    const user = userEvent.setup();
    render(
      <CollapsibleWarningPanel
        label="预览警告"
        warnings={[
          { code: "parse_unknown_field", message: "field preserved", node: "node-a" },
          { code: "parse_unknown_field", message: "field preserved", node: "node-b" },
        ]}
      />,
    );

    const toggle = screen.getByRole("button", { name: "展开预览警告" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.getAllByText("1 组警告 · 2 条记录")).toHaveLength(1);
    expect(screen.queryByRole("heading", { name: "parse_unknown_field · field preserved" })).not.toBeInTheDocument();

    await user.click(toggle);

    const collapse = screen.getByRole("button", { name: "收起预览警告" });
    expect(collapse).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("heading", { name: "parse_unknown_field · field preserved" })).toBeInTheDocument();
    expect(screen.getAllByText("1 组警告 · 2 条记录")).toHaveLength(1);

    await user.click(collapse);

    expect(screen.getByRole("button", { name: "展开预览警告" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByRole("heading", { name: "parse_unknown_field · field preserved" })).not.toBeInTheDocument();
  });

  it("keeps a single warning directly expandable with its exact JSON", async () => {
    const user = userEvent.setup();
    render(
      <WarningList
        warnings={[{
          code: "parse_unknown_field",
          field: "uri.query.mode",
          message: "field preserved in NodeIR Raw",
          node: "[vless]剩余流量：94.89 GB",
          source: "uri-list",
        }]}
      />,
    );

    expect(screen.getByRole("heading", { name: "parse_unknown_field · field preserved in NodeIR Raw" })).toBeInTheDocument();
    expect(screen.getByText("[vless]剩余流量：94.89 GB").parentElement?.parentElement).toHaveTextContent(
      "node: [vless]剩余流量：94.89 GB · source: uri-list · field: uri.query.mode",
    );
    expect(screen.getByText("1 组警告 · 1 条记录")).toBeInTheDocument();

    const warningButton = screen.getByRole("button", { name: /\[vless\]剩余流量：94.89 GB/ });
    expect(warningButton).toHaveAttribute("aria-expanded", "false");
    warningButton.focus();
    await user.keyboard("{Enter}");

    expect(warningButton).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("region", { name: "警告详情" })).toHaveTextContent('"node": "[vless]剩余流量：94.89 GB"');
  });

  it("shows repeated fingerprints as a nested group with per-node details", async () => {
    const user = userEvent.setup();
    render(
      <WarningList
        warnings={[{
          code: "parse_unknown_field",
          field: "uri.query.mode",
          message: "field preserved in NodeIR Raw",
          node: "node-a",
          node_index: 0,
          source: "uri-list",
        }, {
          code: "parse_unknown_field",
          field: "uri.query.mode",
          message: "field preserved in NodeIR Raw",
          node: "node-b",
          node_context: { name: "node-b", raw: { mode: "multi" } },
          node_index: 1,
          source: "uri-list",
        }, {
          code: "parse_unknown_field",
          field: "uri.query.spx",
          message: "field preserved in NodeIR Raw",
          node: "node-c",
          node_index: 2,
          source: "uri-list",
        }]}
      />,
    );

    expect(screen.getByText("2 组警告 · 3 条记录")).toBeInTheDocument();
    expect(screen.getByText("2 个节点或位置受到影响")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /node-b/ })).not.toBeInTheDocument();

    const groupButton = screen.getByRole("button", { name: /2 条同类警告/ });
    expect(groupButton).toHaveAttribute("aria-expanded", "false");
    await user.click(groupButton);

    expect(groupButton).toHaveAttribute("aria-expanded", "true");
    const occurrenceList = screen.getByRole("list", { name: "同类警告节点" });
    expect(within(occurrenceList).getByRole("button", { name: /node-a/ })).toBeInTheDocument();
    const nodeBButton = within(occurrenceList).getByRole("button", { name: /node-b/ });
    nodeBButton.focus();
    await user.keyboard("{Enter}");

    expect(nodeBButton).toHaveAttribute("aria-expanded", "true");
    const details = within(occurrenceList).getByRole("region", { name: "警告详情" });
    expect(details).toHaveTextContent('"node": "node-b"');
    expect(details).toHaveTextContent('"node_index": 1');
    expect(details).toHaveTextContent('"mode": "multi"');
  });

  it("shows different node probe errors in one semantic group", async () => {
    const user = userEvent.setup();
    render(
      <WarningList
        warnings={[{
          code: "probe_timeout",
          message: "probe_timeout: dial tcp 192.0.2.1:443: i/o timeout",
          node: "node-a",
          node_index: 0,
        }, {
          code: "probe_core_api_failed",
          message: "probe_core_api_failed: read tcp 192.0.2.2:443: connection reset by peer",
          node: "node-b",
          node_index: 1,
        }]}
      />,
    );

    expect(screen.getByText("1 组警告 · 2 条记录")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "节点测活失败" })).toBeInTheDocument();
    expect(screen.queryByText("node-a")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /展开 节点测活失败 的 2 条同类警告/ }));

    expect(screen.getByText("node-a")).toBeInTheDocument();
    expect(screen.getByText("probe_timeout · probe_timeout: dial tcp 192.0.2.1:443: i/o timeout")).toBeInTheDocument();
    expect(screen.getByText("node-b")).toBeInTheDocument();
    expect(screen.getByText("probe_core_api_failed · probe_core_api_failed: read tcp 192.0.2.2:443: connection reset by peer")).toBeInTheDocument();
  });

  it("ignores one stable warning class without persisting instance context", async () => {
    const user = userEvent.setup();
    const onIgnore = vi.fn();
    render(
      <WarningPreferencesProvider ignoredWarnings={[]} onIgnore={onIgnore}>
        <WarningList
          warnings={[{
            code: "parse_unknown_field",
            field: "uri.query.mode",
            message: "field preserved in NodeIR Raw",
            node: "node-a",
            node_index: 7,
            source: "uri-list",
          }]}
        />
      </WarningPreferencesProvider>,
    );

    await user.click(screen.getByRole("button", { name: /node-a/ }));
    await user.click(screen.getByRole("button", { name: "忽略同类警告" }));

    expect(onIgnore).toHaveBeenCalledWith({
      code: "parse_unknown_field",
      field: "uri.query.mode",
      source: "uri-list",
    });
  });
});
