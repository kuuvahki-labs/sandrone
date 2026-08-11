import { describe, expect, it } from "vitest";

import { scheduledRefreshTargetOptions } from "./scheduled-refresh-targets";

describe("scheduled refresh target options", () => {
  it("preserves configured resources that are no longer present", () => {
    const options = scheduledRefreshTargetOptions(
      [
        { kind: "subscription", name: "missing-provider" },
        { kind: "file", name: "client.yaml" },
      ],
      [
        { kind: "subscription", name: "provider", label: "Provider" },
        { kind: "file", name: "client.yaml", label: "Client config" },
      ],
    );

    expect(options).toContainEqual({
      kind: "subscription",
      name: "missing-provider",
      label: "missing-provider",
      missing: true,
    });
    expect(options).toContainEqual({
      kind: "file",
      name: "client.yaml",
      label: "Client config",
      missing: false,
    });
  });
});
