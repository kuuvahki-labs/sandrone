import { describe, expect, it } from "vitest";

import {
  buildConvertLink,
  maxPublicConvertContentBytes,
  validateConvertLinkInput,
} from "./convert-link";

describe("convert link", () => {
  it("encodes a nested remote subscription URL without changing its data", () => {
    const result = buildConvertLink({
      publicBaseUrl: "https://public.example/",
      response: "raw",
      source: { kind: "url", value: "https://subscription.example/nodes?token=a+b&name=HK#primary" },
      toFormat: "base64",
    });

    expect(result).toBe("https://public.example/convert?url=https%3A%2F%2Fsubscription.example%2Fnodes%3Ftoken%3Da%2Bb%26name%3DHK%23primary&to_format=base64");
    const generated = new URL(result);
    expect(generated.searchParams.get("url")).toBe("https://subscription.example/nodes?token=a+b&name=HK#primary");
    expect(generated.searchParams.has("from_format")).toBe(false);
    expect(generated.searchParams.has("response")).toBe(false);
  });

  it("preserves inline content and emits explicit format and JSON options", () => {
    const content = "ss://method:secret@example.com:8388#HK+1\nvmess://example&next";
    const result = buildConvertLink({
      fromFormat: "uri-list",
      publicBaseUrl: "https://public.example",
      response: "json",
      source: { kind: "content", value: content },
      toFormat: "json-nodes",
    });

    const generated = new URL(result);
    expect(generated.searchParams.get("content")).toBe(content);
    expect(generated.searchParams.get("from_format")).toBe("uri-list");
    expect(generated.searchParams.get("to_format")).toBe("json-nodes");
    expect(generated.searchParams.get("response")).toBe("json");
    expect(generated.searchParams.has("url")).toBe(false);
  });

  it("validates source, target, scheme, and the UTF-8 content limit", () => {
    const base = {
      publicBaseUrl: "https://public.example",
      response: "raw" as const,
      toFormat: "base64",
    };
    expect(validateConvertLinkInput({ ...base, source: { kind: "url", value: "" } })).toBe("source_required");
    expect(validateConvertLinkInput({ ...base, source: { kind: "url", value: "not a URL" } })).toBe("url_invalid");
    expect(validateConvertLinkInput({ ...base, source: { kind: "url", value: "ftp://example.com/sub" } })).toBe("url_scheme");
    expect(validateConvertLinkInput({ ...base, source: { kind: "content", value: "😀".repeat((maxPublicConvertContentBytes / 4) + 1) } })).toBe("content_too_large");
    expect(validateConvertLinkInput({ ...base, source: { kind: "content", value: "ss://node" }, toFormat: "" })).toBe("to_format_required");
  });
});
