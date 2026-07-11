import { describe, expect, it } from "vitest";

import { fileSourceSummary } from "./summary";

describe("file model summary", () => {
  it("summarizes file sources without blank wording", () => {
    expect(fileSourceSummary()).toBe("local");
    expect(fileSourceSummary({ type: "inline", content: "port: 7890" })).toBe("local");
    expect(fileSourceSummary({ type: "remote", remote: { url: "https://example.com/base.yaml" } })).toBe("remote");
    for (const type of ["mihomo", "sing-box", "shadowrocket"]) {
      expect(fileSourceSummary({ type })).toBe(type);
    }
  });
});
