import { describe, expect, it } from "vitest";

import { recognizedFileProcessorPresetID } from "~/features/files/drivers/core/processor-presets";

import {
  defaultSingBoxProcessors,
  singBoxProcessorPresets,
} from "./processor-presets";

describe("sing-box file processor defaults", () => {
  it("uses sniff and DNS hijack, then traditional NTP direct as the new-file defaults", () => {
    const processors = defaultSingBoxProcessors();

    expect(processors.map((processor) => processor.name)).toEqual([
      "Sniff & DNS Hijack",
      "Traditional NTP Direct",
    ]);
    expect(processors[0]).toMatchObject({
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: { mode: "json_override" },
    });
    expect(processors[1]).toMatchObject({
      name: "Traditional NTP Direct",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "ntp-direct",
          rules_json: JSON.stringify([{ network: "udp", port: 123, outbound: "direct" }]),
        },
      },
    });
    expect(JSON.parse(String(processors[0].params?.content))).toEqual({
      route: {
        "+rules": [
          { action: "sniff" },
          {
            type: "logical",
            mode: "or",
            rules: [{ protocol: "dns" }, { port: 53 }],
            action: "hijack-dns",
          },
        ],
      },
    });
    expect(defaultSingBoxProcessors()[0]).not.toBe(processors[0]);
  });

  it("recognizes only the exact managed JSON override", () => {
    const preset = defaultSingBoxProcessors()[0]!;
    expect(singBoxProcessorPresets.map((descriptor) => descriptor.id)).toEqual(["sniff", "ntp-direct"]);
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe("sniff");
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...preset,
      params: { ...preset.params, content: `${String(preset.params?.content)}\n` },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...preset,
      params: { ...preset.params, mode: "json_overlay" },
    })).toBeNull();
  });
});
