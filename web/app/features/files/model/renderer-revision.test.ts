import { describe, expect, it } from "vitest";

import { rendererRevisionFromCapabilityIndex } from "./renderer-revision";

const shadowrocketRevision = "5f1916b5897fc59fb7172aca59ae52050a3532fe";

const capabilityIndex = {
  items: [
    {
      format: "mihomo-proxies",
      direction: "parse",
      revisions: ["must-not-be-guessed"],
    },
    {
      format: "mihomo-proxies",
      direction: "render",
      revisions: [" v1.19.25 ", "   ", "v1.19.25"],
    },
    {
      format: "sing-box-outbounds",
      direction: "render",
      revisions: ["v1.13.14", " v1.13.14 "],
    },
    {
      format: "shadowrocket-proxies",
      direction: "render",
      revisions: [shadowrocketRevision],
    },
  ],
};

describe("rendererRevisionFromCapabilityIndex", () => {
  it("selects the sole trimmed revision from the exact render capability", () => {
    expect(rendererRevisionFromCapabilityIndex(capabilityIndex, "mihomo-proxies")).toBe("v1.19.25");
    expect(rendererRevisionFromCapabilityIndex(capabilityIndex, "sing-box-outbounds")).toBe("v1.13.14");
    expect(rendererRevisionFromCapabilityIndex(capabilityIndex, "shadowrocket-proxies")).toBe(shadowrocketRevision);
  });

  it("does not guess from parse entries, adjacent formats, or blank revisions", () => {
    expect(rendererRevisionFromCapabilityIndex(capabilityIndex, "mihomo")).toBeUndefined();
    expect(rendererRevisionFromCapabilityIndex({
      items: [{
        format: "mihomo-proxies",
        direction: "render",
        revisions: ["  "],
      }],
    }, "mihomo-proxies")).toBeUndefined();
  });

  it("returns undefined when a matching render entry cites multiple revisions", () => {
    expect(rendererRevisionFromCapabilityIndex({
      items: [{
        format: "sing-box-outbounds",
        direction: "render",
        revisions: ["v1.13.14", "v1.14.0"],
      }],
    }, "sing-box-outbounds")).toBeUndefined();
  });

  it.each([undefined, null, {}, { items: {} }])(
    "returns undefined for a missing capability list (%j)",
    (value) => {
      expect(rendererRevisionFromCapabilityIndex(value, "mihomo-proxies")).toBeUndefined();
    },
  );
});
