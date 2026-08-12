import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { FileConfigEditor } from "~/features/files/config/components/editor";
import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import { requireFileDriver } from "~/features/files/drivers/registry";
import { requireFileDriverUI } from "~/features/files/editor/file-driver-ui-registry";

import { FileFormFields, FileKindConfigWorkbench } from "./file-form";
import { RawFileConfigEditor } from "./raw-config-editor";
import { FileSourceEditor } from "./source-editor";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
  vi.restoreAllMocks();
});

describe("file form drivers", () => {
  it("shows a subscription display title while serializing its canonical name", async () => {
    const user = userEvent.setup();
    const { container } = render(
      <RawFileConfigEditor
        baseEditor={<div />}
        configDefault={{ subscriptions: ["provider"], settingsPresent: false }}
        subscriptions={[{ name: "provider", title: "Provider Main" }]}
      />,
    );

    const picker = screen.getByRole("combobox", { name: "订阅" });
    await user.click(picker);
    expect(await screen.findByRole("option", { name: "Provider Main (provider)" })).toBeInTheDocument();
    expect(JSON.parse(container.querySelector<HTMLInputElement>('input[name="config"]')?.value ?? "null")).toEqual({
      subscriptions: ["provider"],
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
    expect(base.dns.servers[2]?.detour).toBe("🚀 节点选择");
    expect(base.route.final).toBe("🚀 节点选择");
  });

});

describe("FileSourceEditor", () => {
  it("serializes remote metadata and removes it when switching to inline content", async () => {
    const user = userEvent.setup();
    render(
      <FileSourceEditor
        defaultValue={{
          type: "remote",
          remote: {
            url: "https://example.com/config.yaml",
            user_agent: "Sandrone Tests",
            proxy: "http://127.0.0.1:7890",
            timeout_ms: 2500,
            cache_ttl_seconds: 45,
          },
        }}
      />,
    );

    expect(currentSource()).toEqual({
      type: "remote",
      remote: {
        url: "https://example.com/config.yaml",
        user_agent: "Sandrone Tests",
        proxy: "http://127.0.0.1:7890",
        timeout_ms: 2500,
        cache_ttl_seconds: 45,
      },
    });

    await user.click(screen.getByRole("button", { name: "本地" }));
    fireEvent.change(screen.getByRole("textbox", { name: "内容" }), { target: { value: "port: 7890" } });

    expect(currentSource()).toEqual({ type: "inline", content: "port: 7890" });
  });

  it("shows the driver base for an implicit source while preserving the empty source object", () => {
    render(
      <FileSourceEditor
        defaultValue={{}}
        inlineFallback="mixed-port: 7890"
        preserveImplicit
      />,
    );

    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("mixed-port: 7890");
    expect(currentSource()).toEqual({});
  });

  it("keeps explicit empty remote content when switching to inline", async () => {
    const user = userEvent.setup();
    render(
      <FileSourceEditor
        defaultValue={{
          type: "remote",
          content: "",
          remote: { url: "https://example.com/empty.yaml" },
        }}
        inlineFallback="default: true\n"
      />,
    );

    await user.click(screen.getByRole("button", { name: "本地" }));

    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("");
    expect(currentSource()).toEqual({ type: "inline", content: "" });
  });
});

describe("raw-only file kind workbench", () => {
  it("edits a third kind through the generic raw settings path without a structured adapter", () => {
    const onValidityChange = vi.fn();
    render(
      <FileKindConfigWorkbench
        baseEditor={<div>future base</div>}
        configDefault={{ subscriptions: ["provider"], settingsPresent: true, settings: { future: true } }}
        createNamingLocale="en-US"
        mode="edit"
        driver={rawOnlyDriver}
        subscriptions={[{ name: "provider", title: "provider" }]}
        onValidityChange={onValidityChange}
      />,
    );

    const settings = screen.getByRole("textbox", { name: /raw settings|原始 settings/i });
    expect(settings).toHaveValue(JSON.stringify({ future: true }, null, 2));
    expect(currentConfig()).toEqual({ subscriptions: ["provider"], settings: { future: true } });

    fireEvent.change(settings, { target: { value: "{invalid" } });
    expect(onValidityChange).toHaveBeenLastCalledWith(false);
    fireEvent.change(settings, { target: { value: "{\"future\":false}" } });
    expect(currentConfig()).toEqual({ subscriptions: ["provider"], settings: { future: false } });
    expect(onValidityChange).toHaveBeenLastCalledWith(true);
  });

  it("preserves an omitted settings member until the raw editor changes it", () => {
    render(
      <FileKindConfigWorkbench
        baseEditor={<div />}
        configDefault={{ subscriptions: ["provider"], settingsPresent: false }}
        createNamingLocale="en-US"
        mode="edit"
        driver={rawOnlyDriver}
        subscriptions={[{ name: "provider", title: "provider" }]}
      />,
    );

    expect(currentConfig()).toEqual({ subscriptions: ["provider"] });
    fireEvent.change(screen.getByRole("textbox", { name: /raw settings|原始 settings/i }), {
      target: { value: "{\"future\":true}" },
    });
    expect(currentConfig()).toEqual({ subscriptions: ["provider"], settings: { future: true } });
  });
});

const rawOnlyDriver: FileDriverDefinition = {
  kind: "future-client",
  presentation: { labelKey: "files.kind.static", icon: "file" },
  configuration: { mode: "raw" },
  createPresets: [{ kind: "future-client", source: "future-client", sourceType: "inline", order: 100, initialName: "future.json" }],
  source: {
    defaultBase: () => "{}",
    basePlaceholder: "{}",
    remoteURLPlaceholder: "https://example.com/future.json",
    syntax: "json",
    strategy: "optional-base",
    validate: () => null,
  },
  processors: {
    defaults: () => [],
    mergeModes: ["json_overlay", "json_override"],
    presets: [],
    validate: () => [],
  },
};

function currentConfig(): Record<string, unknown> {
  const input = document.querySelector<HTMLInputElement>('input[name="config"]');
  if (!input) throw new Error("expected config input");
  return JSON.parse(input.value) as Record<string, unknown>;
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
