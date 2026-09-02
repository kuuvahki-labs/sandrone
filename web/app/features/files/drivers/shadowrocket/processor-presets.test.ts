import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { createTranslator } from "~/shared/i18n/context";
import { enUS } from "~/shared/i18n/translations/en-US";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  defaultShadowrocketProcessors,
  shadowrocketProcessorPresets,
} from "./processor-presets";

const en = createTranslator("en-US");
const zh = createTranslator("zh-CN");

describe("Shadowrocket processor presets", () => {
  it("uses GitHub acceleration as the new-file default", () => {
    const processors = defaultShadowrocketProcessors(en);

    expect(processors.map((processor) => processor.name)).toEqual(["GitHub acceleration"]);
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of shadowrocketProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
  });

  it("declares only the supported presets", () => {
    expect(shadowrocketProcessorPresets.map((descriptor) => descriptor.id)).toEqual([
      "github-rule-source-mirror",
      "tailscale-native",
      "tailscale-external",
    ]);
    expect(defaultShadowrocketProcessors(en).map((processor) => processor.name))
      .toEqual(["GitHub acceleration"]);
  });

  it("builds the exact native Tailscale ordered rules without a module warning", () => {
    const descriptor = presetDescriptor("tailscale-native");
    const processor = descriptor.build(en);

    expect(processor).toEqual({
      name: "Native Tailscale",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          insert_mode: "top",
          preset_id: "tailscale-native",
          rules_json: JSON.stringify([
            "DOMAIN-SUFFIX,ts.net,TAILSCALE",
            "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
            "IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
          ]),
        },
      },
    });
    expect(descriptor).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: ["tailscale-external"],
    });
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, processor))
      .toBe("tailscale-native");
  });

  it("builds and recognizes the exact external Tailscale script", () => {
    const descriptor = presetDescriptor("tailscale-external");
    const processor = descriptor.build(en);

    expect(processor).toEqual({
      name: "Tailscale coexistence",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
      },
    });
    expect(descriptor).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: ["tailscale-native"],
    });
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, processor))
      .toBe("tailscale-external");
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...processor,
      params: { ...processor.params, args: {} },
    })).toBeNull();
  });

  it("treats native and external Tailscale as mutually exclusive", () => {
    const native = presetDescriptor("tailscale-native").build(en);
    const external = presetDescriptor("tailscale-external").build(en);
    expect(presetDescriptor("tailscale-native").conflicts).toEqual(["tailscale-external"]);
    expect(planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "tailscale-native",
      [native],
      en,
    ).additions).toEqual([]);
    const plan = planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "tailscale-external",
      [native],
      en,
    );
    expect(plan.addedPresetIDs).toEqual(["tailscale-external"]);
    expect(plan.removedPresetIDs).toEqual(["tailscale-native"]);
    expect(plan.additions).toEqual([{
      presetID: "tailscale-external",
      processor: external,
      beforeIndex: null,
    }]);
  });

  it("exposes no Shadowrocket QUIC preset surface", () => {
    const surface = shadowrocketProcessorPresets.map((preset) => ({
      id: preset.id,
      labelKey: preset.labelKey,
      labelEN: enUS[preset.labelKey],
      labelZH: zhCN[preset.labelKey],
      processor: preset.build(en),
    }));
    expect(JSON.stringify(surface).toLowerCase()).not.toContain("quic");
  });
});

function presetDescriptor(id: string) {
  const descriptor = shadowrocketProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing Shadowrocket processor preset: ${id}`);
  return descriptor;
}
