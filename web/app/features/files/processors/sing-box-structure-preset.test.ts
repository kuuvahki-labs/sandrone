import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import {
  recognizeSingBoxStructureProcessorPreset,
  type SingBoxStructureOperation,
  singBoxStructureProcessorPreset as buildSingBoxStructureProcessorPreset,
  type SingBoxStructureProcessorPresetOptions,
} from "./sing-box-structure-preset";

const OPTIONS: Readonly<Record<SingBoxStructureOperation, SingBoxStructureProcessorPresetOptions>> = {
  "udp-p2p-eim": { operation: "udp-p2p-eim" },
  "linux-tun-acceleration": {
    operation: "linux-tun-acceleration",
  },
  "mptcp-direct": { operation: "mptcp-direct" },
  "windows-relaxed-route": {
    operation: "windows-relaxed-route",
  },
};

const NAMES: Readonly<Record<SingBoxStructureOperation, string>> = {
  "udp-p2p-eim": "UDP/P2P 兼容",
  "linux-tun-acceleration": "Linux/OpenWrt TUN 加速",
  "mptcp-direct": "MPTCP 直连",
  "windows-relaxed-route": "Windows 宽松路由",
};

describe("sing-box structure processor presets", () => {
  it("documents the managed operation parameter in the script header", () => {
    const header = inlineSource(singBoxStructureProcessorPreset(OPTIONS["udp-p2p-eim"]))
      .split("function main")[0];

    expect(header).toContain("// Parameters:");
    expect(header).toContain("// - operation:");
  });

  it.each(Object.values(OPTIONS))("builds exact editable inline params for $operation", (options) => {
    const processor = singBoxStructureProcessorPreset(options);

    expect(processor).toEqual({
      name: NAMES[options.operation],
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: { operation: options.operation },
      },
    });
    expect(inlineSource(processor)).toContain("cannot be overridden by request args");
    expect(recognizeSingBoxStructureProcessorPreset(processor, options)).toBe(true);
  });

  it("requires the exact raw source, operation, source shape, and args shape", () => {
    const options = OPTIONS["udp-p2p-eim"];
    const processor = singBoxStructureProcessorPreset(options);
    const params = processor.params as Record<string, unknown>;
    const source = params.source as Record<string, unknown>;
    const args = params.args as Record<string, unknown>;

    expect(recognizeSingBoxStructureProcessorPreset({
      ...processor,
      params: { ...params, source: { ...source, content: `${String(source.content)}\n// edited` } },
    }, options)).toBe(false);
    expect(recognizeSingBoxStructureProcessorPreset({
      ...processor,
      params: { ...params, args: { operation: "mptcp-direct" } },
    }, options)).toBe(false);
    expect(recognizeSingBoxStructureProcessorPreset({
      ...processor,
      params: { ...params, args: { ...args, extra: true } },
    }, options)).toBe(false);
    expect(recognizeSingBoxStructureProcessorPreset({
      ...processor,
      params: { ...params, source: { ...source, id: "edited" } },
    }, options)).toBe(false);
  });

  it.each([
    ["udp-p2p-eim", { endpoint_independent_nat: true }],
    ["linux-tun-acceleration", { auto_route: true, auto_redirect: true }],
    ["mptcp-direct", { exclude_mptcp: true }],
    ["windows-relaxed-route", { strict_route: false }],
  ] as const)("applies %s only to the exact tagged TUN", (operation, expected) => {
    const unrelatedTun = {
      type: "tun",
      tag: "other-tun",
      address: ["192.0.2.1/30", "2001:db8:1::1/126"],
      strict_route: true,
    };
    const mixed = { type: "mixed", tag: "mixed-in", listen: "::1", listen_port: 2080 };
    const custom = { type: "redirect", tag: "custom-in", metadata: { ipv6: "2001:db8::9" } };
    const selected = {
      type: "tun",
      tag: "tun-in",
      address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
      strict_route: true,
      custom: { keep: ["::1", "value"] },
    };
    const original = {
      dns: {
        strategy: "prefer_ipv4",
        servers: [{ type: "udp", server: "2001:4860:4860::8888" }],
      },
      inbounds: [mixed, unrelatedTun, custom, selected],
      experimental: { ipv6: "2001:db8::10" },
    };

    const result = runOperation(operation, original);
    const inbounds = result.document.inbounds as Array<Record<string, unknown>>;

    expect(inbounds).toHaveLength(4);
    expect(inbounds[0]).toEqual(mixed);
    expect(inbounds[1]).toEqual(unrelatedTun);
    expect(inbounds[2]).toEqual(custom);
    expect(result.document.experimental).toEqual(original.experimental);
    expect((result.document.dns as Record<string, unknown>).servers).toEqual(original.dns.servers);
    expect(inbounds[3]).toMatchObject(expected);
    expect(result.stringifyCalls).toBe(1);
  });

  it("falls back to exactly one untagged TUN", () => {
    const result = runOperation("udp-p2p-eim", {
      inbounds: [
        { type: "mixed", tag: "mixed-in" },
        { type: "tun", tag: "custom-tun", address: ["10.0.0.1/30"] },
      ],
    });

    expect(result.document.inbounds).toEqual([
      { type: "mixed", tag: "mixed-in" },
      {
        type: "tun",
        tag: "custom-tun",
        address: ["10.0.0.1/30"],
        endpoint_independent_nat: true,
      },
    ]);
  });

  it.each(["udp-p2p-eim", "linux-tun-acceleration", "mptcp-direct", "windows-relaxed-route"] as const)(
    "rejects a missing TUN for %s without stringifying or assigning partial content",
    (operation) => {
      const execution = prepareOperation(operation, { inbounds: [{ type: "mixed", tag: "mixed-in" }] });
      const before = execution.input.file.content;

      expect(execution.run).toThrowError("Sandrone sing-box structure preset requires a TUN inbound");
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    },
  );

  it("rejects ambiguous TUN selection without mutation", () => {
    const execution = prepareOperation("udp-p2p-eim", {
      inbounds: [
        { type: "tun", tag: "first" },
        { type: "tun", tag: "second" },
      ],
    });
    const before = execution.input.file.content;

    expect(execution.run).toThrowError("Sandrone sing-box structure preset found ambiguous TUN inbounds");
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
  });

  it("fails closed when the exact tun-in tag belongs to a non-TUN inbound", () => {
    const execution = prepareOperation("udp-p2p-eim", {
      inbounds: [
        { type: "mixed", tag: "tun-in" },
        { type: "tun", tag: "custom-tun" },
      ],
    });
    const before = execution.input.file.content;

    expect(execution.run).toThrowError(
      "Sandrone sing-box structure preset tag tun-in is not a TUN inbound",
    );
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
  });

  it("rejects request-owned operation even when it matches the managed value", () => {
    const execution = prepareOperation(
      "mptcp-direct",
      { inbounds: [{ type: "tun", tag: "tun-in" }] },
      { operation: "mptcp-direct" },
    );
    const before = execution.input.file.content;

    expect(execution.run).toThrowError(
      "Sandrone sing-box structure preset operation cannot be overridden by request args",
    );
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
  });

});

function inlineSource(processor: ProcessorDetail): string {
  const source = processor.params?.source as Record<string, unknown> | undefined;
  if (typeof source?.content !== "string") throw new Error("expected inline script content");
  return source.content;
}

function singBoxStructureProcessorPreset(options: SingBoxStructureProcessorPresetOptions): ProcessorDetail {
  return buildSingBoxStructureProcessorPreset(options, NAMES[options.operation]);
}

function runOperation(
  operation: SingBoxStructureOperation,
  document: Record<string, unknown>,
  requestArgs?: Record<string, string>,
): { document: Record<string, unknown>; stringifyCalls: number } {
  const execution = prepareOperation(operation, document, requestArgs);
  execution.run();
  return {
    document: JSON.parse(execution.input.file.content) as Record<string, unknown>,
    stringifyCalls: execution.stringifyCalls(),
  };
}

function prepareOperation(
  operation: SingBoxStructureOperation,
  document: Record<string, unknown>,
  requestArgs?: Record<string, string>,
) {
  const processor = singBoxStructureProcessorPreset(OPTIONS[operation]);
  const input: {
    file: { content: string };
    args: { operation: SingBoxStructureOperation };
    request?: { args: Record<string, string> };
  } = {
    file: { content: JSON.stringify(document) },
    args: { operation },
    ...(requestArgs ? { request: { args: requestArgs } } : {}),
  };
  let stringifyCalls = 0;
  const api = {
    json: {
      parse: JSON.parse,
      stringify: (value: unknown) => {
        stringifyCalls += 1;
        return JSON.stringify(value);
      },
    },
  };
  const context: { input: typeof input; api: typeof api; output?: typeof input } = { input, api };
  return {
    input,
    run: () => runInNewContext(
      `${inlineSource(processor)}\nglobalThis.output = main(input, api);`,
      context,
    ),
    stringifyCalls: () => stringifyCalls,
  };
}
