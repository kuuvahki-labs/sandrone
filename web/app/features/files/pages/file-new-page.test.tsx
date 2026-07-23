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
  it("uses the file context for static content labels and remote help", () => {
    const { rerender } = render(<FileNewPage source="local" onBack={noop} onSave={noop} />);

    const localName = screen.getByRole("textbox", { name: "名称" });
    expect(localName).toHaveValue("");
    expect(localName).toBeRequired();
    const contentSource = screen.getByRole("group", { name: "内容来源" });
    expect(within(contentSource).getByRole("textbox", { name: "内容" })).not.toHaveAttribute("placeholder");

    rerender(<FileNewPage key="remote" source="remote" onBack={noop} onSave={noop} />);
    const remoteName = screen.getByRole("textbox", { name: "名称" });
    expect(remoteName).toHaveValue("");
    expect(remoteName).toBeRequired();
    expect(screen.getByText("预览时会重新读取。")).toBeInTheDocument();
  });

  it.each([
    ["mihomo", "mihomo.yaml"],
    ["sing-box", "sing-box.json"],
    ["shadowrocket", "shadowrocket.conf"],
  ] as const)("keeps the %s preset name for typed file creation", (source, expectedName) => {
    render(<FileNewPage source={source} onBack={noop} onSave={noop} />);

    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue(expectedName);
  });

  it.each(["mihomo", "sing-box", "shadowrocket"] as const)(
    "selects the standard template for new %s files",
    (source) => {
      render(<FileNewPage source={source} onBack={noop} onSave={noop} />);

      expect(screen.getByRole("radio", { name: "标准" })).toBeChecked();
      expect(screen.queryByText("已自定义")).not.toBeInTheDocument();
    },
  );

  it("submits the resolved driver kind separately from form data", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_kind: string, _form: FormData) => undefined);
    render(<FileNewPage source="local" onBack={noop} onSave={onSave} />);

    await user.type(screen.getByRole("textbox", { name: "名称" }), "local.txt");
    await user.click(screen.getByRole("button", { name: "保存文件" }));

    expect(onSave).toHaveBeenCalledWith("static", expect.any(FormData));
    const submittedForm = onSave.mock.calls[0]?.[1];
    expect(submittedForm?.has("kind")).toBe(false);
  });

  it("serializes a complete sing-box form with an explicit JSON base", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_kind: string, _form: FormData) => undefined);

    render(
      <FileNewPage
        loadSubscriptionPreview={vi.fn().mockResolvedValue(configNodePreview("provider", ["Node 1"]))}
        source="sing-box"
        onBack={noop}
        onSave={onSave}
        subscriptions={subscriptionOptions}
      />,
    );

    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("sing-box.json");
    const baseContent = within(screen.getByRole("group", { name: "基础配置内容" }))
      .getByRole("textbox", { name: "内容" });
    expect(JSON.parse((baseContent as HTMLTextAreaElement).value)).toMatchObject({
      dns: {
        final: "dns-remote",
        servers: expect.arrayContaining([expect.objectContaining({ tag: "dns-remote", detour: "🚀 节点选择" })]),
      },
      route: { final: "🚀 节点选择" },
      inbounds: expect.arrayContaining([{ type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 2080 }]),
    });
    expect(baseContent.closest("[data-highlighted-textarea]")).toHaveAttribute("data-highlighted-textarea", "json");

    await selectMuiOption(user, screen.getByRole("combobox", { name: "订阅" }), "provider");
    expect(await screen.findByText("已加载 1 个节点")).toBeInTheDocument();
    const saveFile = screen.getByRole("button", { name: "保存文件" });
    expect(saveFile).toHaveTextContent("保存");
    await waitFor(() => expect(saveFile).toBeEnabled());
    await user.click(saveFile);

    expect(onSave.mock.calls[0]?.[0]).toBe("sing-box");
    const saved = onSave.mock.calls[0]?.[1] as FormData;
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
