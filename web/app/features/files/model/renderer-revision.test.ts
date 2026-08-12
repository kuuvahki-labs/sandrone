import { describe, expect, it } from "vitest";

import { rendererRevisionFromInspect } from "./renderer-revision";

const shadowrocketRevision = "5f1916b5897fc59fb7172aca59ae52050a3532fe";

const inspectResponse = {
  capabilities: {
    parse_formats: ["mihomo", "sing-box"],
    render_formats: ["mihomo-proxies", "sing-box-outbounds", "shadowrocket-proxies"],
    capabilities: [
      {
        format: "mihomo-proxies",
        direction: "parse",
        fields: [field("must-not-be-guessed")],
      },
      {
        format: "mihomo-proxies",
        direction: "render",
        fields: [field(" v1.19.25 "), field("   ")],
        lossy: [field("v1.19.25", "lossy")],
        raw_only: [field("v1.19.25", "raw_only")],
      },
      {
        format: "sing-box-outbounds",
        direction: "render",
        fields: [field("v1.13.14")],
        lossy: [field(" v1.13.14 ", "lossy")],
      },
      {
        format: "shadowrocket-proxies",
        direction: "render",
        fields: [field(shadowrocketRevision)],
        raw_only: [field(shadowrocketRevision, "raw_only")],
      },
    ],
  },
};

describe("rendererRevisionFromInspect", () => {
  it("selects the sole trimmed revision from the exact render capability", () => {
    expect(rendererRevisionFromInspect(inspectResponse, "mihomo-proxies")).toBe("v1.19.25");
    expect(rendererRevisionFromInspect(inspectResponse, "sing-box-outbounds")).toBe("v1.13.14");
    expect(rendererRevisionFromInspect(inspectResponse, "shadowrocket-proxies")).toBe(shadowrocketRevision);
  });

  it("does not guess from parse entries, adjacent formats, or blank revisions", () => {
    expect(rendererRevisionFromInspect(inspectResponse, "mihomo")).toBeUndefined();
    expect(rendererRevisionFromInspect({
      capabilities: {
        capabilities: [{
          format: "mihomo-proxies",
          direction: "render",
          fields: [field("  ")],
        }],
      },
    }, "mihomo-proxies")).toBeUndefined();
  });

  it("returns undefined when matching render fields cite multiple revisions", () => {
    expect(rendererRevisionFromInspect({
      capabilities: {
        capabilities: [{
          format: "sing-box-outbounds",
          direction: "render",
          fields: [field("v1.13.14")],
          lossy: [field("v1.14.0", "lossy")],
          raw_only: [field("v1.13.14", "raw_only")],
        }],
      },
    }, "sing-box-outbounds")).toBeUndefined();
  });

  it.each([undefined, null, {}, { capabilities: {} }, { capabilities: { capabilities: {} } }])(
    "returns undefined for a missing capability list (%j)",
    (value) => {
      expect(rendererRevisionFromInspect(value, "mihomo-proxies")).toBeUndefined();
    },
  );
});

function field(revision: unknown, status = "supported") {
  return {
    ir_field: "name",
    protocol: "shadowsocks",
    status,
    source_ref: { revision },
  };
}
