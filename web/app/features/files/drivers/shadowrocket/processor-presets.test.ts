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
  it("uses traditional NTP direct as the only new-file default", () => {
    const processors = defaultShadowrocketProcessors(en);

    expect(processors.map((processor) => processor.name)).toEqual(["Traditional NTP Direct"]);
    expect(processors[0]).toMatchObject({
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "ntp-direct",
          rules_json: JSON.stringify(["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"]),
        },
      },
    });
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of shadowrocketProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
  });

  it("recognizes only the exact managed NTP script", () => {
    const preset = defaultShadowrocketProcessors(en)[0]!;

    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, preset)).toBe("ntp-direct");
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      params: {
        ...preset.params,
        args: { preset_id: "ntp-direct", rules_json: "[]" },
      },
    })).toBeNull();
  });

  it("declares only the supported presets", () => {
    expect(shadowrocketProcessorPresets.map((descriptor) => descriptor.id)).toEqual([
      "ntp-direct",
      "github-rule-source-mirror",
      "tailscale-native",
    ]);
    expect(defaultShadowrocketProcessors(en).map((processor) => processor.name)).toEqual([
      "Traditional NTP Direct",
    ]);
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
      conflicts: [],
    });
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, processor))
      .toBe("tailscale-native");
  });

  it("does not declare conflicts for native Tailscale", () => {
    const tailscale = presetDescriptor("tailscale-native").build(en);
    expect(presetDescriptor("tailscale-native").conflicts).toEqual([]);
    expect(planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "tailscale-native",
      [tailscale],
      en,
    ).additions).toEqual([]);
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
