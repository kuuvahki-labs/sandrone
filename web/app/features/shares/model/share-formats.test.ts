import { describe, expect, it } from "vitest";

import { shareCopyFormats, shareUrlWithFormat } from "./share-formats";

describe("share formats", () => {
  it("exposes the four client-oriented copy formats while keeping json-nodes hidden", () => {
    expect(shareCopyFormats.map((entry) => entry.value)).toEqual([
      "uri-list",
      "mihomo-proxies",
      "sing-box-outbounds",
      "shadowrocket-proxies",
    ]);
  });

  it("adds a format to an absolute URL", () => {
    expect(shareUrlWithFormat("https://example.com/s/share", "uri-list")).toBe(
      "https://example.com/s/share?format=uri-list",
    );
  });

  it("replaces an existing format without leaving duplicates", () => {
    expect(shareUrlWithFormat(
      "https://example.com/s/share?format=json-nodes&token=abc&format=uri-list",
      "mihomo-proxies",
    )).toBe("https://example.com/s/share?format=mihomo-proxies&token=abc");
  });

  it("keeps a relative URL relative", () => {
    expect(shareUrlWithFormat("/s/share", "sing-box-outbounds")).toBe(
      "/s/share?format=sing-box-outbounds",
    );
  });

  it("preserves unrelated query parameters and a fragment", () => {
    expect(shareUrlWithFormat("/s/share?token=abc&format=json-nodes#install", "shadowrocket-proxies")).toBe(
      "/s/share?token=abc&format=shadowrocket-proxies#install",
    );
  });
});
