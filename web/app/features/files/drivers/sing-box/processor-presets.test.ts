import { describe, expect, it } from "vitest";

import { defaultSingBoxProcessors } from "./processor-presets";

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
          { protocol: "dns", action: "hijack-dns" },
        ],
      },
    });
    expect(defaultSingBoxProcessors()[0]).not.toBe(processors[0]);
  });
});
