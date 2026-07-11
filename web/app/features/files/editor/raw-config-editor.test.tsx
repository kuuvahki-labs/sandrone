import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";

import { FileKindConfigWorkbench } from "./file-form";

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
    validate: () => [],
  },
};

function currentConfig(): Record<string, unknown> {
  const input = document.querySelector<HTMLInputElement>('input[name="config"]');
  if (!input) throw new Error("expected config input");
  return JSON.parse(input.value) as Record<string, unknown>;
}
