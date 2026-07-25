import { describe, expect, it } from "vitest";

import { shareCopyFormats, shareUrlWithFormat } from "./share-formats";

describe("share formats", () => {
  it("exposes Base64 first and keeps json-nodes hidden", () => {
    expect(shareCopyFormats.map((entry) => entry.value)).toEqual([
      "base64",
      "uri-list",
      "mihomo-proxies",
      "sing-box-outbounds",
      "shadowrocket-proxies",
    ]);
  });

  it("selects Base64 while preserving the friendly filename and fragment", () => {
    expect(shareUrlWithFormat(
      "https://example.com/s/share/mobile.txt?format=uri-list#install",
      "base64",
      "mobile.txt",
    )).toBe("https://example.com/s/share/mobile.txt?format=base64#install");
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

  it("replaces the friendly filename with the selected format filename", () => {
    expect(shareUrlWithFormat(
      "https://example.com/s/sh_nodes/mobile.yaml?token=abc&format=mihomo-proxies#install",
      "shadowrocket-proxies",
      "mobile.conf",
    )).toBe(
      "https://example.com/s/sh_nodes/mobile.conf?token=abc&format=shadowrocket-proxies#install",
    );
  });

  it("percent-encodes a selected Unicode filename as one segment", () => {
    expect(shareUrlWithFormat(
      "/s/sh_nodes/mobile.yaml",
      "shadowrocket-proxies",
      "配置 ☁.conf",
    )).toBe(
      "/s/sh_nodes/%E9%85%8D%E7%BD%AE%20%E2%98%81.conf?format=shadowrocket-proxies",
    );
  });
});
