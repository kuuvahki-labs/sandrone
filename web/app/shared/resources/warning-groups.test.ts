import { describe, expect, it } from "vitest";

import type { PreviewWarning } from "~/shared/resources/types";
import { groupPreviewWarnings } from "~/shared/resources/warning-groups";

describe("groupPreviewWarnings", () => {
  it("groups by the public diagnostic fingerprint without using node context", () => {
    const first = warning({ node: "node-a", node_index: 0 });
    const differentField = warning({ field: "uri.query.spx", node: "node-c" });
    const second = warning({
      node: "node-b",
      node_context: { name: "sensitive-name", raw: { token: "private" } },
      node_index: 1,
    });
    const differentSource = warning({ node: "node-d", source: "mihomo" });
    const differentTarget = warning({ node: "node-e", target: "sing-box" });

    const groups = groupPreviewWarnings([first, differentField, second, differentSource, differentTarget]);

    expect(groups).toHaveLength(4);
    expect(groups.map((group) => group.warning)).toEqual([first, differentField, differentSource, differentTarget]);
    expect(groups[0].warnings).toEqual([first, second]);
  });

  it("preserves the first server occurrence order inside and across groups", () => {
    const warnings = [
      warning({ message: "second fingerprint", node: "first occurrence" }),
      warning({ message: "first fingerprint", node: "middle occurrence" }),
      warning({ message: "second fingerprint", node: "last occurrence" }),
    ];

    const groups = groupPreviewWarnings(warnings);

    expect(groups.map((group) => group.warning.message)).toEqual(["second fingerprint", "first fingerprint"]);
    expect(groups[0].warnings.map((item) => item.node)).toEqual(["first occurrence", "last occurrence"]);
  });
});

function warning(overrides: Partial<PreviewWarning>): PreviewWarning {
  return {
    code: "parse_unknown_field",
    field: "uri.query.mode",
    message: "field preserved in NodeIR Raw",
    source: "uri-list",
    target: "mihomo",
    ...overrides,
  };
}
