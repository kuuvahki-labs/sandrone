import { describe, expect, it } from "vitest";

import { validateJSONConfigSource, validateJSONMergeProcessors } from "./input-validation";

describe("sing-box file input validation", () => {
  it.each([
    { content: "[]", error: "source_json_object_required" },
    { content: "null", error: "source_json_object_required" },
    { content: "{", error: "source_json_invalid" },
  ])("rejects an invalid inline base: $content", ({ content, error }) => {
    expect(validateJSONConfigSource({ type: "inline", content })).toBe(error);
  });

  it("accepts a sing-box inline JSON object", () => {
    expect(validateJSONConfigSource({ type: "inline", content: "{\"log\":{}}" })).toBeNull();
  });

  it("accepts an omitted sing-box source that uses the service base", () => {
    expect(validateJSONConfigSource({})).toBeNull();
  });

  it.each(["", "ftp://example.com/base.json", "not a url"])("rejects a non-HTTP remote base: %s", (url) => {
    expect(validateJSONConfigSource({ type: "remote", remote: { url } })).toBe("source_remote_url_invalid");
  });

  it("accepts HTTPS remote bases", () => {
    expect(validateJSONConfigSource({ type: "remote", remote: { url: "https://example.com/base.json" } })).toBeNull();
  });

  it("rejects malformed JSON override content", () => {
    expect(validateJSONMergeProcessors([{
      type: "merge",
      params: { mode: "json_override", content: "{" },
    }])).toEqual([expect.objectContaining({ code: "processor_json_invalid", index: 0 })]);
  });

  it("does not block saving malformed content for a disabled processor", () => {
    expect(validateJSONMergeProcessors([{
      type: "merge",
      enabled: false,
      params: { mode: "json_override", content: "{" },
    }])).toEqual([]);
  });

  it.each([
    { content: '{"route":{"+rules":{}}}', path: "route.+rules" },
    { content: '{"route":{"rules+":"dns"}}', path: "route.rules+" },
  ])("rejects non-array JSON override operands at $path", ({ content, path }) => {
    expect(validateJSONMergeProcessors([{
      type: "merge",
      params: { mode: "json_override", content },
    }])).toEqual([expect.objectContaining({
      code: "processor_json_override_array_required",
      index: 0,
      path,
    })]);
  });

  it("accepts nested JSON override operators", () => {
    expect(validateJSONMergeProcessors([{
      type: "merge",
      params: {
        mode: "json_override",
        content: '{"route":{"+rules":[{"action":"sniff"}],"<rules!>":[]}}',
      },
    }])).toEqual([]);
  });

  it("accepts null deletion for JSON override operator keys", () => {
    expect(validateJSONMergeProcessors([{
      type: "merge",
      params: {
        mode: "json_override",
        content: '{"route":{"+rules":null,"rule_set+":null}}',
      },
    }])).toEqual([]);
  });
});
