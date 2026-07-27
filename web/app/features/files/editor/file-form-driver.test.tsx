import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FileConfigEditor } from "~/features/files/config/components/editor";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";

import { FileFormFields } from "./file-form";
import { FileTypeSummary } from "./file-type-summary";
import { RawFileConfigEditor } from "./raw-config-editor";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
});

describe("file form drivers", () => {
  it("serializes structured settings in the new orchestration envelope and keeps empty arrays", () => {
    render(<FileConfigEditor
      adapter={structuredAdapter("mihomo")}
      baseEditor={<div />}
      defaultValue={{ subscriptions: ["provider"], settingsMode: "structured", groups: [], rule_sets: [], rules: [] }}
      mode="edit"
      subscriptions={[]}
      ui={requireFileDriverUI("mihomo")}
    />);

    expect(currentConfig()).toEqual({
      subscriptions: ["provider"],
		settings: { groups: [], rule_sets: [], rules: [] },
    });
    expect(currentConfig()).not.toHaveProperty("group_preset");
    expect(currentConfig()).not.toHaveProperty("ruleset_preset");
  });

  it("submits an advanced native group without rewriting its wire settings", () => {
    const adapter = structuredAdapter("mihomo");
    const settings = {
      groups: [{ name: "Provider", type: "select", use: ["airport"] }],
      rule_sets: [],
      rules: [],
    };
    const defaultValue = adapter.decode({ settingsPresent: true, settings }, "en-US");

    render(<FileConfigEditor
      adapter={adapter}
      baseEditor={<div />}
      defaultValue={defaultValue}
      mode="edit"
      subscriptions={[]}
      ui={requireFileDriverUI("mihomo")}
    />);

    expect(currentConfig()).toEqual({ settings });
  });

  it.each(["mihomo", "shadowrocket"])("normalizes %s adaptive settings only after an option changes", async (kind) => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    const adapter = structuredAdapter(kind);
    const settings = { adaptive_groups: { regions: ["us", "hk"] } };

    render(<FileConfigEditor
      adapter={adapter}
      baseEditor={<div />}
      defaultValue={adapter.decode({ settingsPresent: true, settings }, "en-US")}
      mode="edit"
      subscriptions={[]}
      ui={requireFileDriverUI(kind)}
    />);

    await user.click(screen.getByRole("combobox", { name: "Proxy group type" }));
    await user.click(await screen.findByRole("option", { name: "load-balance" }));

    expect(currentConfig()).toEqual({
      settings: {
        adaptive_groups: {
          type: "load-balance",
          regions: ["hk", "us"],
        },
      },
    });
  });

  it("round-trips raw settings until replacement is explicitly confirmed", async () => {
    const user = userEvent.setup();
    vi.spyOn(window, "confirm").mockReturnValue(false);
    render(<FileConfigEditor
      adapter={structuredAdapter("mihomo")}
      baseEditor={<div />}
      defaultValue={{ subscriptions: ["provider"], settingsMode: "raw", rawSettings: { future_nested: { keep: true } } }}
      mode="edit"
      subscriptions={[]}
      ui={requireFileDriverUI("mihomo")}
    />);

    expect(screen.getByRole("textbox", { name: /settings JSON/i })).toHaveValue(JSON.stringify({ future_nested: { keep: true } }, null, 2));
    expect(currentConfig()).toEqual({ subscriptions: ["provider"], settings: { future_nested: { keep: true } } });

    await user.click(screen.getByRole("button", { name: /structured settings|结构化 settings/i }));
    expect(window.confirm).toHaveBeenCalledOnce();
    expect(screen.getByRole("textbox", { name: /settings JSON/i })).toBeInTheDocument();
    expect(currentConfig()).toEqual({ subscriptions: ["provider"], settings: { future_nested: { keep: true } } });
  });

  it("labels raw configuration as editable configuration content", () => {
    localStorage.setItem("sandrone.locale", "en-US");

    render(<RawFileConfigEditor baseEditor={<div />} subscriptions={[]} />);

    expect(screen.getByRole("heading", { name: "Configuration content" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Base configuration content" })).toBeInTheDocument();
  });

  it("keeps omitted Shadowrocket sections absent until that section is edited", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    render(<FileConfigEditor
      adapter={structuredAdapter("shadowrocket")}
      baseEditor={<div />}
      defaultValue={{ settingsMode: "structured" }}
      mode="edit"
      subscriptions={[]}
      ui={requireFileDriverUI("shadowrocket")}
    />);

    expect(currentConfig()).toEqual({ settings: {} });

    const groups = screen.getByRole("group", { name: "Proxy groups" });
    expect(within(groups).getAllByRole("button", { name: /^Expand proxy group/ })).toHaveLength(1);
    await user.click(within(groups).getByRole("button", { name: "Expand proxy group Proxy" }));
    fireEvent.change(within(groups).getByRole("textbox", { name: "Name" }), { target: { value: "Manual" } });
    expect(currentSettings().groups).toEqual(expect.arrayContaining([expect.objectContaining({ name: "Manual" })]));
    expect(currentSettings()).not.toHaveProperty("rule_sets");
    expect(currentSettings()).not.toHaveProperty("rules");

    await user.click(within(screen.getByRole("group", { name: "Rule sets" })).getByRole("button", { name: "Add rule set" }));
    expect(currentSettings()).toMatchObject({ rule_sets: [expect.objectContaining({ name: "custom" })] });
    expect(currentSettings()).not.toHaveProperty("rules");

    const rules = screen.getByRole("group", { name: "Rules" });
    await user.click(within(rules).getByRole("button", { name: /^Rules/ }));
    await user.click(within(rules).getByRole("button", { name: "Add rule" }));
    expect(currentSettings()).toHaveProperty("rules");
  });

  it("preserves multiple subscriptions and marks the form invalid", async () => {
    const onValidityChange = vi.fn();
    render(<FileConfigEditor
      adapter={structuredAdapter("mihomo")}
      baseEditor={<div />}
      defaultValue={{ subscriptions: ["one", "two"], settingsMode: "structured" }}
      mode="edit"
      onValidityChange={onValidityChange}
      subscriptions={[]}
      ui={requireFileDriverUI("mihomo")}
    />);

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));
    expect(screen.getByText(/one, two/)).toHaveTextContent(/collection subscription|集合订阅/i);
    expect(currentConfig().subscriptions).toEqual(["one", "two"]);
  });

  it.each([
    ["file", "通用", '[data-testid="DescriptionOutlinedIcon"]'],
    ["mihomo", "mihomo", 'img[src="/brand/clients/mihomo.webp"]'],
    ["sing-box", "sing-box", 'img[src="/brand/clients/sing-box.svg"]'],
    ["rocket", "Shadowrocket", '[data-testid="RocketLaunchOutlinedIcon"]'],
  ] as const)("renders the %s FileTypeSummary as static metadata", (icon, label, iconSelector) => {
    render(
      <FileTypeSummary icon={icon} label={label} title="文件类型" />,
    );

    const metadata = screen.getByRole("group", { name: "文件类型" });
    expect(metadata).toHaveTextContent(label);
    expect(metadata.querySelector(iconSelector)).toBeInTheDocument();
    expect(metadata.querySelector("button, input, select, textarea")).not.toBeInTheDocument();
    expect(metadata.querySelector(".MuiChip-root")).not.toBeInTheDocument();
  });

  it("composes static file-type metadata without a hidden kind input", () => {
    render(<FileFormFields defaultName="client.conf" driver={requireFileDriver("static")} mode="edit" />);

    expect(screen.getByRole("group", { name: "文件类型" })).toBeInTheDocument();
    expect(document.querySelector('input[name="kind"]')).not.toBeInTheDocument();
  });

  it("aggregates sing-box source validity into form validity", async () => {
    const onValidityChange = vi.fn();
    render(<FileFormFields
      defaultName="client.json"
      driver={requireFileDriver("sing-box")}
      mode="edit"
      onValidityChange={onValidityChange}
      sourceDefault={{ type: "inline", content: "{}" }}
    />);

    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
    const baseContent = within(screen.getByRole("group", { name: "基础配置内容" }))
      .getByRole("textbox", { name: "内容" });

    fireEvent.change(baseContent, { target: { value: "[]" } });
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(false));

    fireEvent.change(baseContent, { target: { value: "{}" } });
    await waitFor(() => expect(onValidityChange).toHaveBeenLastCalledWith(true));
  });

  it("uses the existing config naming locale when an edited remote source becomes inline", async () => {
    localStorage.setItem("sandrone.locale", "en-US");
    const user = userEvent.setup();
    render(<FileFormFields
      configDefault={{
        settingsPresent: true,
        settings: {
          groups: [{ type: "selector", tag: "🚀 节点选择", outbounds: ["direct"] }],
          rule_sets: [],
          rules: [],
        },
      }}
      defaultName="client.json"
      driver={requireFileDriver("sing-box")}
      mode="edit"
      sourceDefault={{ type: "remote", remote: { url: "https://example.com/config.json" } }}
    />);

    await user.click(screen.getByRole("button", { name: "Local" }));

    const source = currentSource();
    const base = JSON.parse(String(source.content)) as {
      dns: { servers: Array<{ detour?: string }> };
      route: { final?: string };
    };
    expect(base.dns.servers[1]?.detour).toBe("🚀 节点选择");
    expect(base.route.final).toBe("🚀 节点选择");
  });

});

function currentConfig(): Record<string, unknown> {
  const input = document.querySelector<HTMLInputElement>('input[name="config"]');
  if (!input) throw new Error("expected config input");
  return JSON.parse(input.value) as Record<string, unknown>;
}

function currentSettings(): Record<string, unknown> {
  return currentConfig().settings as Record<string, unknown>;
}

function currentSource(): Record<string, unknown> {
  const input = document.querySelector<HTMLInputElement>('input[name="source"]');
  if (!input) throw new Error("expected serialized source input");
  return JSON.parse(input.value) as Record<string, unknown>;
}

function structuredAdapter(kind: string) {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}
