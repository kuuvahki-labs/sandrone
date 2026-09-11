import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import script from "./sing-box-outbound-adapter.js?raw";

interface FileInput {
  file: { content: string; name: string };
  args?: Record<string, unknown>;
  request?: { args?: Record<string, unknown> };
}

function run(input: FileInput): FileInput {
  return runInNewContext(`${script}\nmain(input, api)`, {
    input,
    api: { json: { parse: JSON.parse, stringify: JSON.stringify } },
  });
}

function filter(document: unknown) {
  return JSON.parse(run({ file: { name: "config.json", content: JSON.stringify(document) } }).file.content);
}

describe("sing-box outbound adaptation script", () => {
  it("filters selectors and URL tests in order, keeps endpoints and node definitions, and consumes fields", () => {
    const nodes = [
      { type: "direct", tag: "direct" },
      { type: "shadowsocks", tag: "HK-2", server: "192.0.2.1" },
      { type: "shadowsocks", tag: "HK-old", server: "192.0.2.2" },
    ];
    const endpoints = [{ type: "wireguard", tag: "香港-1" }];
    const fixed = { type: "selector", tag: "固定", outbounds: ["direct", "香港-1"] };
    const result = filter({
      outbounds: [
        ...nodes,
        fixed,
        { type: "selector", tag: "香港", filter: "HK|香港", "exclude-filter": "old", default: "香港-1", outbounds: ["香港-1", "HK-2", "direct", "HK-old", "HK-2"] },
        { type: "urltest", tag: "自动", filter: "HK", "exclude-filter": "old", outbounds: ["HK-2", "HK-old"], interval: "3m" },
      ],
      endpoints,
      route: { final: "香港" },
    });
    expect(result).toEqual({
      outbounds: [
        ...nodes,
        fixed,
        { type: "selector", tag: "香港", default: "香港-1", outbounds: ["香港-1", "HK-2"] },
        { type: "urltest", tag: "自动", outbounds: ["HK-2"], interval: "3m" },
      ],
      endpoints,
      route: { final: "香港" },
    });
  });

  it.each([
    ["^HK", undefined, ["HK-1", "HK-old"]],
    ["(?i)^HK", "(?i)OLD", ["hk-2", "HK-1"]],
    ["香港", undefined, ["香港-1"]],
    [" HK", undefined, [" HK-3"]],
  ])("matches pattern %s with exclusion %s", (pattern, exclusion, expected) => {
    const result = filter({ outbounds: [{
      type: "selector", tag: "地区", filter: pattern,
      ...(exclusion === undefined ? {} : { "exclude-filter": exclusion }),
      outbounds: ["hk-2", "HK-1", "HK-old", "香港-1", " HK-3"],
    }] });
    expect(result.outbounds[0].outbounds).toEqual(expected);
  });

  it.each([
    [{ filter: "[" }, "invalid filter"],
    [{ "exclude-filter": "[" }, "invalid exclude-filter"],
    [{ filter: "JP" }, "matched no nodes"],
    [{ "exclude-filter": ".*" }, "matched no nodes"],
    [{ outbounds: [] }, "matched no nodes"],
    [{ default: "JP-1" }, "default is not a member"],
    [{ default: "" }, "default is not a member"],
    [{ default: null }, "default is not a member"],
    [{ filter: "" }, "non-empty string"],
    [{ filter: "   " }, "non-empty string"],
    [{ filter: "(?i)" }, "non-empty string"],
    [{ filter: "(?i)  " }, "non-empty string"],
    [{ filter: 1 }, "non-empty string"],
    [{ "exclude-filter": "" }, "non-empty string"],
    [{ "exclude-filter": null }, "non-empty string"],
    [{ outbounds: null }, "array of node names"],
    [{ outbounds: [1] }, "array of node names"],
  ])("rejects invalid group %j atomically", (overrides, message) => {
    const input = { file: { name: "config.json", content: JSON.stringify({ outbounds: [
      { type: "selector", tag: "成功组", filter: "HK", outbounds: ["HK-1"] },
      { type: "urltest", tag: "失败组", filter: "HK", outbounds: ["HK-1"], ...overrides },
    ] }) } };
    const before = input.file.content;
    expect(() => run(input)).toThrow(new RegExp(`\\[失败组\\]: .*${message}`));
    expect(input.file.content).toBe(before);
  });

  it("requires inclusion when a group only supplies exclusion", () => {
    expect(() => filter({ outbounds: [{ type: "selector", tag: "缺少包含", "exclude-filter": "old", outbounds: ["HK-1"] }] }))
      .toThrow("[缺少包含]: filter must be a non-empty string");
  });

  it("leaves a second execution and fixed configurations byte-identical", () => {
    const input = { file: { name: "config.json", content: JSON.stringify({ outbounds: [{
      type: "selector", tag: "香港", filter: "HK", outbounds: ["HK-1"],
    }] }) } };
    run(input);
    const processed = input.file.content;
    expect(run(input)).toBe(input);
    expect(input.file.content).toBe(processed);
    const fixed = { file: { name: "config.json", content: '{\n  "outbounds": [{"type":"selector","tag":"fixed","outbounds":[]}]\n}' } };
    const original = fixed.file.content;
    expect(run(fixed).file.content).toBe(original);
  });

  it("uses the member list at its execution position without re-reading node definitions", () => {
    const result = filter({ outbounds: [
      { type: "direct", tag: "HK-definition-only" },
      { type: "selector", tag: "香港", filter: "HK", outbounds: ["HK-added-by-earlier-script"] },
    ] });
    expect(result.outbounds[1].outbounds).toEqual(["HK-added-by-earlier-script"]);
  });
});


describe("sing-box explicit default outbound", () => {
  function inputFor(document: unknown, args?: Record<string, unknown>): FileInput {
    return { file: { name: "config.json", content: JSON.stringify(document, null, 2) }, args };
  }

  it.each([undefined, {}, { default_outbound: "" }, { default_outbound: " \t\n" }])(
    "preserves existing content and does not validate route when unset: %j", (args) => {
      for (const document of [{}, { route: { final: "missing" } }, { route: null }, { endpoints: "unused" }]) {
        const input = inputFor(document, args);
        const before = input.file.content;
        expect(run(input)).toBe(input);
        expect(input.file.content).toBe(before);
      }
    },
  );

  it.each([
    { outbounds: [{ type: "selector", tag: "Manual", outbounds: ["node"] }, { type: "direct", tag: "node" }] },
    { outbounds: [{ type: "direct", tag: "Manual" }] },
    { outbounds: [], endpoints: [{ type: "wireguard", tag: "Manual" }] },
    { endpoints: [{ type: "wireguard", tag: "Manual" }] },
  ])("selects an exact unique outbound or endpoint and creates route: %j", (document) => {
    const input = inputFor(document, { default_outbound: "Manual" });
    expect(run(input)).toBe(input);
    expect(JSON.parse(input.file.content)).toEqual({ ...document, route: { final: "Manual" } });
    const processed = input.file.content;
    run(input);
    expect(input.file.content).toBe(processed);
  });

  it("overrides final without changing other route options or node definitions", () => {
    const document = { outbounds: [{ type: "direct", tag: "Manual" }], route: { final: "Previous", rules: [] } };
    const input = inputFor(document, { default_outbound: "Manual" });
    run(input);
    expect(JSON.parse(input.file.content)).toEqual({ ...document, route: { final: "Manual", rules: [] } });
  });

  it("preserves bytes when the requested final is already selected", () => {
    const input = inputFor({ outbounds: [{ tag: "Manual" }], route: { final: "Manual" } }, { default_outbound: "Manual" });
    const before = input.file.content;
    run(input);
    expect(input.file.content).toBe(before);
  });

  it("uses the exact parameter, preserving meaningful surrounding whitespace", () => {
    const input = inputFor({ outbounds: [{ tag: " Manual " }] }, { default_outbound: " Manual " });
    run(input);
    expect(JSON.parse(input.file.content).route.final).toBe(" Manual ");
    expect(() => run(inputFor({ outbounds: [{ tag: "Manual" }] }, { default_outbound: " Manual " }))).toThrow("found 0");
  });

  it.each([
    [{}, "found 0"],
    [{ outbounds: [{ tag: "manual" }, { tag: "Manual-extra" }] }, "found 0"],
    [{ outbounds: [{ tag: "Manual" }, { tag: "Manual" }] }, "found 2"],
    [{ endpoints: [{ tag: "Manual" }, { tag: "Manual" }] }, "found 2"],
    [{ outbounds: [{ tag: "Manual" }], endpoints: [{ tag: "Manual" }] }, "found 2"],
    [{ outbounds: null }, "outbounds to be an array"],
    [{ outbounds: {} }, "outbounds to be an array"],
    [{ endpoints: null }, "endpoints to be an array"],
    [{ endpoints: {} }, "endpoints to be an array"],
    [{ outbounds: [{ tag: "Manual" }], route: null }, "route to be an object"],
    [{ outbounds: [{ tag: "Manual" }], route: [] }, "route to be an object"],
    [{ outbounds: [{ tag: "Manual" }], route: "invalid" }, "route to be an object"],
  ])("rejects invalid targets and route atomically: %j", (document, message) => {
    const input = inputFor(document, { default_outbound: "Manual" });
    const before = input.file.content;
    expect(() => run(input)).toThrow(message);
    expect(input.file.content).toBe(before);
  });

  it.each([null, false, 0, 1, [], {}])("rejects non-string default_outbound %j", (value) => {
    const input = inputFor({}, { default_outbound: value });
    expect(() => run(input)).toThrow("default_outbound must be a string");
  });

  it.each(["Manual", "", null, undefined])("rejects request override %j even with saved configuration", (value) => {
    const input = inputFor({ outbounds: [{ tag: "Manual" }] }, { default_outbound: "Manual" });
    input.request = { args: { default_outbound: value } };
    expect(() => run(input)).toThrow("cannot be overridden by request args");
  });

  it("allows unrelated request arguments", () => {
    const input = inputFor({ outbounds: [{ tag: "Manual" }] }, { default_outbound: "Manual" });
    input.request = { args: { unrelated: "value" } };
    run(input);
    expect(JSON.parse(input.file.content).route.final).toBe("Manual");
  });

  it("does not publish successful regex changes if final selection fails", () => {
    const input = inputFor({ outbounds: [{ type: "selector", tag: "Manual", filter: "HK", outbounds: ["HK-1", "JP-1"] }] }, { default_outbound: "Missing" });
    const before = input.file.content;
    expect(() => run(input)).toThrow("found 0");
    expect(input.file.content).toBe(before);
  });

  it("adapts regex groups and final together and repeats without further changes", () => {
    const input = inputFor({ outbounds: [{ type: "selector", tag: "Manual", filter: "HK", outbounds: ["HK-1", "JP-1"] }] }, { default_outbound: "Manual" });
    run(input);
    expect(JSON.parse(input.file.content)).toEqual({ outbounds: [{ type: "selector", tag: "Manual", outbounds: ["HK-1"] }], route: { final: "Manual" } });
    const processed = input.file.content;
    run(input);
    expect(input.file.content).toBe(processed);
  });

  it("evaluates targets at its current execution position", () => {
    const input = inputFor({}, { default_outbound: "Earlier" });
    expect(() => run(input)).toThrow("found 0");
    input.file.content = JSON.stringify({ outbounds: [{ type: "direct", tag: "Earlier" }] });
    run(input);
    expect(JSON.parse(input.file.content).route.final).toBe("Earlier");
  });
});
