import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import {
  recognizeSingBoxStructureProcessorPreset,
  type SingBoxStructureOperation,
  singBoxStructureProcessorPreset,
  type SingBoxStructureProcessorPresetOptions,
} from "./sing-box-structure-preset";

const OPTIONS: Readonly<Record<SingBoxStructureOperation, SingBoxStructureProcessorPresetOptions>> = {
  "ensure-tun": { operation: "ensure-tun", name: "Ensure TUN" },
  "ipv4-only": { operation: "ipv4-only", name: "IPv4 Only" },
  "udp-p2p-eim": { operation: "udp-p2p-eim", name: "UDP/P2P EIM" },
  "linux-tun-acceleration": {
    operation: "linux-tun-acceleration",
    name: "Linux/OpenWrt TUN Acceleration",
  },
  "mptcp-direct": { operation: "mptcp-direct", name: "MPTCP Direct" },
  "windows-relaxed-route": {
    operation: "windows-relaxed-route",
    name: "Windows Relaxed Route",
  },
};

const EXPECTED_TUN = {
  type: "tun",
  tag: "tun-in",
  address: ["172.19.0.1/30", "fdfe:dcba:9876::1/126"],
  auto_route: true,
  strict_route: true,
};

describe("sing-box structure processor presets", () => {
  it.each(Object.values(OPTIONS))("builds exact editable inline params for $operation", (options) => {
    const processor = singBoxStructureProcessorPreset(options);

    expect(processor).toEqual({
      name: options.name,
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
    const options = OPTIONS["ipv4-only"];
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
      params: { ...params, args: { operation: "ensure-tun" } },
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

  it("appends the canonical TUN only when no TUN exists", () => {
    const original = {
      dns: { strategy: "prefer_ipv4" },
      inbounds: [
        { type: "mixed", tag: "mixed-in", listen_port: 2080 },
        { type: "redirect", tag: "custom-in", listen: "::1" },
      ],
    };

    const first = runOperation("ensure-tun", original);
    expect(first.document.inbounds).toEqual([
      original.inbounds[0],
      original.inbounds[1],
      EXPECTED_TUN,
    ]);
    expect(first.stringifyCalls).toBe(1);

    const second = runOperation("ensure-tun", first.document);
    expect(second.document.inbounds).toEqual(first.document.inbounds);
    expect(second.document.inbounds).toHaveLength(3);
    expect(second.stringifyCalls).toBe(1);
  });

  it.each([
    ["ipv4-only", { address: ["172.19.0.1/30"], dnsStrategy: "ipv4_only" }],
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
    if ("dnsStrategy" in expected) {
      expect(inbounds[3]).toMatchObject({ address: expected.address });
      expect((result.document.dns as Record<string, unknown>).strategy).toBe(expected.dnsStrategy);
      expect(inbounds[3].address).toEqual(expected.address);
      expect(inbounds[1].address).toEqual(unrelatedTun.address);
      expect(inbounds[3].custom).toEqual(selected.custom);
    } else {
      expect(inbounds[3]).toMatchObject(expected);
    }
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

  it.each(["ipv4-only", "udp-p2p-eim", "linux-tun-acceleration", "mptcp-direct", "windows-relaxed-route"] as const)(
    "rejects a missing TUN for %s without stringifying or assigning partial content",
    (operation) => {
      const execution = prepareOperation(operation, { inbounds: [{ type: "mixed", tag: "mixed-in" }] });
      const before = execution.input.file.content;

      expect(execution.run).toThrowError("Sandrone sing-box structure preset requires a TUN inbound");
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    },
  );

  it("rejects ambiguous TUN selection even for ensure without mutation", () => {
    const execution = prepareOperation("ensure-tun", {
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
    const execution = prepareOperation("ensure-tun", {
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

  it("validates the selected address before cloning the IPv4-only result", () => {
    const execution = prepareOperation("ipv4-only", {
      dns: { strategy: "prefer_ipv4" },
      inbounds: [{ type: "tun", tag: "tun-in", address: "172.19.0.1/30" }],
    });
    const before = execution.input.file.content;

    expect(execution.run).toThrowError(
      "Sandrone sing-box IPv4-only preset requires TUN address to be an array of strings",
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
