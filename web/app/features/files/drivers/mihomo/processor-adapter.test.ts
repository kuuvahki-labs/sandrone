import { describe, expect, it } from "vitest";

import { mihomoProcessorAdapter } from "./processor-adapter";
import { defaultMihomoProcessors, recognizeMihomoProcessorPreset } from "./processor-presets";

describe("Mihomo processor adapter", () => {
  it("offers the compatibility preset without adding it to the default chain", () => {
    const optionValues = (mihomoProcessorAdapter.options?.((key) => key) ?? []).map((option) => option.value);
    expect(optionValues).toContain("mihomo-preset:fake-ip-compat");

    const additions = mihomoProcessorAdapter.addPreset?.("mihomo-preset:fake-ip-compat", []);
    expect(additions).toHaveLength(1);
    expect(recognizeMihomoProcessorPreset(additions![0]!)).toBe("fake-ip-compat");
    expect(defaultMihomoProcessors().map((processor) => processor.name)).toEqual(["Sniffer", "TUN"]);
  });
});
