import { describe, expect, it } from "vitest";

import { sharesFromShareList } from "./codec";

describe("share model codec", () => {
  it("maps share target kinds and public URLs", () => {
    expect(sharesFromShareList({ shares: [
      { id: "sh_123", name: "mobile", target_kind: "file", target_name: "default.yaml" },
      { id: "sh_nodes", name: "nodes", target_kind: "subscription", target_name: "provider", target_format: "mihomo-proxies" },
    ] }, "https://example.com")).toEqual([
      expect.objectContaining({ id: "sh_123", targetKind: "file", status: "valid", publicUrl: "https://example.com/s/sh_123" }),
      expect.objectContaining({
        id: "sh_nodes",
        targetKind: "subscription",
        targetFormat: "mihomo-proxies",
        status: "valid",
        publicUrl: "https://example.com/s/sh_nodes?format=mihomo-proxies",
      }),
    ]);
  });

  it("derives share status from valid time range", () => {
    const future = new Date(Date.now() + 60 * 60 * 1000).toISOString();
    const past = new Date(Date.now() - 60 * 60 * 1000).toISOString();
    const shares = sharesFromShareList({ shares: [
      { id: "future", target_kind: "file", target_name: "client", valid_from: future },
      { id: "expired", target_kind: "file", target_name: "client", valid_until: past },
      { id: "active", target_kind: "file", target_name: "client", valid_from: past, valid_until: future },
    ] });

    expect(shares.map((share) => [share.id, share.status])).toEqual([
      ["future", "upcoming"],
      ["expired", "expired"],
      ["active", "valid"],
    ]);
  });

  it("maps age encryption and exhausted maximum uses", () => {
    const [share] = sharesFromShareList({ shares: [{
      id: "limited",
      target_kind: "file",
      target_name: "client",
      age_recipient: "age1recipient",
      max_uses: 2,
      use_count: 2,
    }] });
    expect(share).toMatchObject({
      ageRecipient: "age1recipient",
      maxUses: 2,
      useCount: 2,
      status: "exhausted",
    });
  });
});
