import { describe, expect, it } from "vitest";

import { shareFromCreateResponse, sharesFromShareList } from "./codec";

describe("share model codec", () => {
  it("maps a create response to the canonical default URL", () => {
    expect(shareFromCreateResponse({ share: {
      id: "sh_new",
      name: "mobile",
      target_kind: "subscription",
      target_name: "provider",
      target_format: "mihomo-proxies",
      public_filename: "mobile.yaml",
      format_filenames: { "mihomo-proxies": "mobile.yaml" },
    } }, "https://public.example/base/")).toEqual(expect.objectContaining({
      id: "sh_new",
      publicUrl: "https://public.example/base/s/sh_new/mobile.yaml?format=mihomo-proxies",
    }));
  });

  it.each([
    [{ share: { public_filename: "mobile.txt" } }, "id"],
    [{ share: { id: "sh_new" } }, "public_filename"],
  ])("rejects a create response missing %s", (response, _field) => {
    expect(() => shareFromCreateResponse(response, "https://public.example")).toThrow(
      "Invalid create share response",
    );
  });

  it("maps share target kinds and public URLs", () => {
    expect(sharesFromShareList({ shares: [
      {
        id: "sh_123",
        name: "mobile",
        target_kind: "file",
        target_name: "default.yaml",
        public_filename: "shadowrocket.conf",
      },
      {
        id: "sh_nodes",
        name: "nodes",
        target_kind: "subscription",
        target_name: "provider",
        target_format: "mihomo-proxies",
        public_filename: "mobile.yaml",
        format_filenames: {
          "uri-list": "mobile.txt",
          "mihomo-proxies": "mobile.yaml",
          "shadowrocket-proxies": "mobile.conf",
          "sing-box-outbounds": "mobile.json",
          "json-nodes": "mobile.json",
        },
      },
    ] }, "https://example.com")).toEqual([
      expect.objectContaining({
        id: "sh_123",
        targetKind: "file",
        status: "valid",
        publicUrl: "https://example.com/s/sh_123/shadowrocket.conf",
      }),
      expect.objectContaining({
        id: "sh_nodes",
        targetKind: "subscription",
        targetFormat: "mihomo-proxies",
        formatFilenames: {
          "uri-list": "mobile.txt",
          "mihomo-proxies": "mobile.yaml",
          "shadowrocket-proxies": "mobile.conf",
          "sing-box-outbounds": "mobile.json",
          "json-nodes": "mobile.json",
        },
        status: "valid",
        publicUrl: "https://example.com/s/sh_nodes/mobile.yaml?format=mihomo-proxies",
      }),
    ]);
  });

  it("percent-encodes the filename as one path segment", () => {
    const [share] = sharesFromShareList({ shares: [{
      id: "unicode",
      name: "配置",
      target_kind: "file",
      target_name: "client",
      public_filename: "配置 ☁.conf",
    }] });

    expect(share.publicUrl).toBe("/s/unicode/%E9%85%8D%E7%BD%AE%20%E2%98%81.conf");
  });

  it("keeps legacy URLs when an older server omits presentation fields", () => {
    const [share] = sharesFromShareList({ shares: [{
      id: "legacy",
      target_kind: "subscription",
      target_name: "provider",
      target_format: "uri-list",
    }] }, "https://example.com");

    expect(share.publicUrl).toBe("https://example.com/s/legacy?format=uri-list");
    expect(share.formatFilenames).toEqual({});
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
