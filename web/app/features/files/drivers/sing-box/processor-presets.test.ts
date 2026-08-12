import { describe, expect, it } from "vitest";

import { recognizedFileProcessorPresetID } from "~/features/files/drivers/core/processor-presets";

import {
  defaultSingBoxProcessors,
  singBoxProcessorPresets,
} from "./processor-presets";

describe("sing-box file processor defaults", () => {
  it("prepends sniff and DNS hijack route actions with an editable JSON override", () => {
    const processors = defaultSingBoxProcessors();

    expect(processors).toHaveLength(1);
    expect(processors[0]).toMatchObject({
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: { mode: "json_override" },
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
    expect(singBoxProcessorPresets.map((descriptor) => descriptor.id)).toEqual(["sniff"]);
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
