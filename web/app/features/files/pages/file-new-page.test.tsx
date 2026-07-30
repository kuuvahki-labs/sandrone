import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { requireFileDriver } from "~/features/files/drivers/registry";
import {
  configNodePreview,
  noop,
  selectMuiOption,
  subscriptionOptions,
} from "~/features/files/test-data";

import { FileNewPage } from "./file-new-page";

describe("FileNewPage", () => {
  it("blocks a new sing-box file until preview, then submits its resolved driver", async () => {
    const user = userEvent.setup();
    const loadSubscriptionPreview = vi.fn().mockResolvedValue(configNodePreview("provider", ["Node 1"]));
    const onSave = vi.fn(async (_kind: string, _form: FormData) => undefined);

    render(
      <FileNewPage
        loadSubscriptionPreview={loadSubscriptionPreview}
        source="sing-box"
        onBack={noop}
        onSave={onSave}
        subscriptions={subscriptionOptions}
      />,
    );

    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("sing-box.json");
    expect(screen.getByRole("radio", { name: "标准" })).toBeChecked();
    const saveFile = screen.getByRole("button", { name: "保存文件" });
    expect(saveFile).toBeDisabled();
    const baseContent = within(screen.getByRole("group", { name: "基础配置内容" }))
      .getByRole("textbox", { name: "内容" });
    const originalBaseContent = (baseContent as HTMLTextAreaElement).value;
    expect(JSON.parse(originalBaseContent)).toMatchObject({
      dns: {
        final: "dns-remote",
        servers: expect.arrayContaining([expect.objectContaining({ tag: "dns-remote", detour: "🚀 节点选择" })]),
      },
      route: { final: "🚀 节点选择" },
      inbounds: expect.arrayContaining([{ type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 2080 }]),
    });
    expect(baseContent.closest("[data-highlighted-textarea]")).toHaveAttribute("data-highlighted-textarea", "json");

    await selectMuiOption(user, screen.getByRole("combobox", { name: "订阅" }), "provider");
    await waitFor(() => expect(loadSubscriptionPreview).toHaveBeenCalledWith("provider"));
    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    expect(saveFile).toHaveTextContent("保存");
    await waitFor(() => expect(saveFile).toBeEnabled());
    await user.click(saveFile);

    expect(onSave.mock.calls[0]?.[0]).toBe("sing-box");
    const saved = onSave.mock.calls[0]?.[1] as FormData;
    expect(saved.has("kind")).toBe(false);
    expect(JSON.parse(String(saved.get("source")))).toEqual({
      type: "inline",
      content: expect.stringContaining('"listen_port": 2080'),
    });
    const singBoxDriver = requireFileDriver("sing-box");
    if (singBoxDriver.configuration.mode !== "structured") throw new Error("expected structured sing-box driver");
    const template = singBoxDriver.configuration.adapter.templates.create("standard", "zh-CN");
    expect(JSON.parse(String(saved.get("config")))).toEqual({
      subscriptions: ["provider"],
      settings: {
        groups: template.groups,
        rule_sets: template.rule_sets,
        rules: template.rules,
      },
    });
    const processors = JSON.parse(String(saved.get("processors")));
    expect(processors).toMatchObject([{
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: { mode: "json_override" },
    }]);
    expect(JSON.parse(processors[0].params.content)).toEqual({
      route: { "+rules": [{ action: "sniff" }, { protocol: "dns", action: "hijack-dns" }] },
    });
  });
});
