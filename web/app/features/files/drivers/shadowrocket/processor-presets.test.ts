import { describe, expect, it } from "vitest";

import { recognizedFileProcessorPresetID } from "~/features/files/drivers/core/processor-presets";

import {
  defaultShadowrocketProcessors,
  shadowrocketProcessorPresets,
} from "./processor-presets";

describe("Shadowrocket processor presets", () => {
  it("uses traditional NTP direct as the only new-file default", () => {
    const processors = defaultShadowrocketProcessors();

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

  it("recognizes only the exact managed NTP script", () => {
    const preset = defaultShadowrocketProcessors()[0]!;

    expect(shadowrocketProcessorPresets.map((descriptor) => descriptor.id)).toEqual(["ntp-direct"]);
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, preset)).toBe("ntp-direct");
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      params: {
        ...preset.params,
        args: { preset_id: "ntp-direct", rules_json: "[]" },
      },
    })).toBeNull();
  });
});
