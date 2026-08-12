import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { enUS } from "~/shared/i18n/translations/en-US";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  defaultSingBoxProcessors,
  singBoxProcessorPreset,
  type SingBoxProcessorPresetID,
  singBoxProcessorPresets,
} from "./processor-presets";

describe("sing-box file processor defaults", () => {
  it("uses sniff and DNS hijack, then traditional NTP direct as the new-file defaults", () => {
    const processors = defaultSingBoxProcessors();

    expect(processors.map((processor) => processor.name)).toEqual([
      "Sniff & DNS Hijack",
      "Traditional NTP Direct",
    ]);
    expect(processors[0]).toMatchObject({
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: { mode: "json_override" },
    });
    expect(processors[1]).toMatchObject({
      name: "Traditional NTP Direct",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "ntp-direct",
          rules_json: JSON.stringify([{ network: "udp", port: 123, outbound: "direct" }]),
        },
      },
    });
    expect(JSON.parse(String(processors[0].params?.content))).toEqual({
      route: {
        "+rules": [
          { action: "sniff" },
          {
            type: "logical",
            mode: "or",
            rules: [{ protocol: "dns" }, { port: 53 }],
            action: "hijack-dns",
          },
        ],
      },
    });
    expect(defaultSingBoxProcessors()[0]).not.toBe(processors[0]);
  });

  it("recognizes only the exact managed JSON override", () => {
    const preset = defaultSingBoxProcessors()[0]!;
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe("sniff");
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...preset,
      params: { ...preset.params, content: `${String(preset.params?.content)}\n` },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...preset,
      params: { ...preset.params, mode: "json_overlay" },
    })).toBeNull();
  });

  it("builds exact STUN and QUIC ordered-rule processors", () => {
    expect(singBoxProcessorPreset("stun-block")).toMatchObject({
      name: "STUN Block",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "stun-block",
          rules_json: JSON.stringify([{ protocol: "stun", action: "reject" }]),
        },
      },
    });
    expect(singBoxProcessorPreset("quic-fallback")).toMatchObject({
      name: "QUIC Fallback",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "quic-fallback",
          rules_json: JSON.stringify([{ protocol: "quic", action: "reject" }]),
        },
      },
    });
  });

  it.each([
    ["ensure-tun", "Ensure TUN", "ensure-tun"],
    ["ipv4-only", "IPv4 Only", "ipv4-only"],
    ["udp-p2p-eim", "UDP/P2P EIM", "udp-p2p-eim"],
    ["linux-tun-acceleration", "Linux/OpenWrt TUN Acceleration", "linux-tun-acceleration"],
    ["mptcp-direct", "MPTCP Direct", "mptcp-direct"],
    ["windows-relaxed-route", "Windows Relaxed Route", "windows-relaxed-route"],
  ] as const)("builds the exact %s structural processor", (id, name, operation) => {
    expect(singBoxProcessorPreset(id)).toEqual({
      name,
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: { operation },
      },
    });
  });

  it("declares the complete dependency, conflict, and default matrix", () => {
    expect(singBoxProcessorPresets.map((preset) => preset.id)).toEqual([
      "sniff",
      "ntp-direct",
      "ensure-tun",
      "stun-block",
      "quic-fallback",
      "ipv4-only",
      "udp-p2p-eim",
      "linux-tun-acceleration",
      "mptcp-direct",
      "windows-relaxed-route",
      "tailscale-native",
      "tailscale-external",
    ]);
    const scenarioIDs = [
      "ensure-tun",
      "stun-block",
      "quic-fallback",
      "ipv4-only",
      "udp-p2p-eim",
      "linux-tun-acceleration",
      "mptcp-direct",
      "windows-relaxed-route",
    ] as const;
    expect(scenarioIDs.map((id) => {
      const preset = presetDescriptor(id);
      return {
        id,
        defaultOn: preset.defaultOn,
        dependencies: preset.dependencies,
        conflicts: preset.conflicts,
      };
    })).toEqual([
      { id: "ensure-tun", defaultOn: false, dependencies: [], conflicts: [] },
      { id: "stun-block", defaultOn: false, dependencies: ["sniff"], conflicts: ["udp-p2p-eim", "tailscale-native", "tailscale-external"] },
      { id: "quic-fallback", defaultOn: false, dependencies: ["sniff"], conflicts: [] },
      { id: "ipv4-only", defaultOn: false, dependencies: [], conflicts: [] },
      { id: "udp-p2p-eim", defaultOn: false, dependencies: [], conflicts: ["stun-block"] },
      { id: "linux-tun-acceleration", defaultOn: false, dependencies: ["ensure-tun"], conflicts: [] },
      { id: "mptcp-direct", defaultOn: false, dependencies: ["linux-tun-acceleration"], conflicts: [] },
      { id: "windows-relaxed-route", defaultOn: false, dependencies: [], conflicts: [] },
    ]);

    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "mptcp-direct", []).addedPresetIDs)
      .toEqual(["ensure-tun", "linux-tun-acceleration", "mptcp-direct"]);
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "stun-block", []).addedPresetIDs)
      .toEqual(["sniff", "stun-block"]);
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "quic-fallback", []).addedPresetIDs)
      .toEqual(["sniff", "quic-fallback"]);
    expect(planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "stun-block",
      [singBoxProcessorPreset("udp-p2p-eim")],
    ).removedPresetIDs).toEqual(["udp-p2p-eim"]);
    expect(planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "udp-p2p-eim",
      [singBoxProcessorPreset("stun-block")],
    ).removedPresetIDs).toEqual(["stun-block"]);
  });

  it("builds exact no-argument native and external Tailscale scripts in native-first order", () => {
    for (const id of ["tailscale-native", "tailscale-external"] as const) {
      const preset = singBoxProcessorPreset(id as SingBoxProcessorPresetID);
      expect(preset).toEqual({
        name: id === "tailscale-native" ? "Tailscale 原生接管" : "Tailscale 共存",
        type: "script",
        stage: "file",
        params: { source: { type: "inline", content: expect.any(String) } },
      });
      expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe(id);
      expect(preset.params).not.toHaveProperty("args");
    }

    expect(presetDescriptor("tailscale-native" as SingBoxProcessorPresetID)).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: ["ensure-tun"],
      conflicts: ["tailscale-external", "stun-block"],
    });
    expect(presetDescriptor("tailscale-external" as SingBoxProcessorPresetID)).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: ["ensure-tun"],
      conflicts: ["tailscale-native", "stun-block"],
    });
  });

  it("applies external Tailscale coexistence atomically and idempotently", () => {
    const ownedDNS = { type: "udp", tag: "ts-dns", server: "100.100.100.100" };
    const ownedDNSRule = {
      domain_suffix: ["ts.net"],
      action: "route",
      server: "ts-dns",
    };
    const original = {
      dns: {
        servers: [
          { type: "local", tag: "dns-local" },
          ownedDNS,
          ownedDNS,
        ],
        rules: [
          { rule_set: ["private"], action: "route", server: "dns-local" },
          ownedDNSRule,
          ownedDNSRule,
        ],
        final: "LockedDNSFinal",
      },
      inbounds: [
        { type: "mixed", tag: "mixed-in" },
        {
          type: "tun",
          tag: "tun-in",
          route_exclude_address: [
            "192.0.2.0/24",
            "100.64.0.0/10",
            "fd7a:115c:a1e0::/48",
            "100.64.0.0/10",
          ],
        },
      ],
      endpoints: [{ type: "wireguard", tag: "keep-ep", address: ["192.0.2.1/32"] }],
      route: { final: "LockedRouteFinal", rules: [{ outbound: "LockedFinal" }] },
    };

    const first = runTailscale("tailscale-external", original);
    expect(first.stringifyCalls).toBe(1);
    expect(first.document).toEqual({
      ...original,
      dns: {
        servers: [{ type: "local", tag: "dns-local" }, ownedDNS],
        rules: [
          { rule_set: ["private"], action: "route", server: "dns-local" },
          ownedDNSRule,
        ],
        final: "LockedDNSFinal",
      },
      inbounds: [
        original.inbounds[0],
        {
          ...original.inbounds[1],
          route_exclude_address: [
            "192.0.2.0/24",
            "100.64.0.0/10",
            "fd7a:115c:a1e0::/48",
          ],
        },
      ],
    });
    expect(first.document.endpoints).toEqual(original.endpoints);
    expect(first.document.route).toEqual(original.route);

    const second = runTailscale("tailscale-external", first.document);
    expect(second.document).toEqual(first.document);
    expect(second.stringifyCalls).toBe(1);
  });

  it("applies native Tailscale with v1.13.14 endpoint, DNS, and route shapes", () => {
    const endpoint = {
      type: "tailscale",
      tag: "ts-ep",
      ephemeral: false,
      accept_routes: false,
    };
    const dnsServer = {
      type: "tailscale",
      tag: "ts-dns",
      endpoint: "ts-ep",
      accept_default_resolvers: false,
    };
    const dnsRule = { ip_accept_any: true, server: "ts-dns" };
    const routeRule = {
      preferred_by: ["ts-ep"],
      action: "route",
      outbound: "ts-ep",
    };
    const original = {
      dns: {
        servers: [{ type: "local", tag: "dns-local" }, dnsServer, dnsServer],
        rules: [{ domain_suffix: ["user.example"], server: "dns-local" }, dnsRule, dnsRule],
        final: "LockedDNSFinal",
      },
      inbounds: [
        { type: "mixed", tag: "mixed-in" },
        {
          type: "tun",
          tag: "tun-in",
          route_exclude_address: [
            "192.0.2.0/24",
            "100.64.0.0/10",
            "fd7a:115c:a1e0::/48",
            "100.64.0.0/10",
          ],
        },
      ],
      endpoints: [
        { type: "wireguard", tag: "keep-ep", address: ["192.0.2.1/32"] },
        endpoint,
        endpoint,
      ],
      route: {
        final: "LockedRouteFinal",
        rules: [
          { domain_suffix: ["user.example"], outbound: "direct" },
          routeRule,
          routeRule,
          { rule_set: ["private"], outbound: "direct" },
          { outbound: "LockedFinal" },
        ],
      },
    };

    const first = runTailscale("tailscale-native", original);
    expect(first.stringifyCalls).toBe(1);
    expect(first.document.endpoints).toEqual([original.endpoints[0], endpoint]);
    expect((first.document.dns as Record<string, unknown>).servers).toEqual([
      { type: "local", tag: "dns-local" },
      dnsServer,
    ]);
    expect((first.document.dns as Record<string, unknown>).rules).toEqual([
      { domain_suffix: ["user.example"], server: "dns-local" },
      dnsRule,
    ]);
    expect(first.document.route).toEqual({
      final: "LockedRouteFinal",
      rules: [
        { domain_suffix: ["user.example"], outbound: "direct" },
        routeRule,
        { rule_set: ["private"], outbound: "direct" },
        { outbound: "LockedFinal" },
      ],
    });
    expect((first.document.inbounds as Array<Record<string, unknown>>)[1]).toEqual({
      type: "tun",
      tag: "tun-in",
      route_exclude_address: ["192.0.2.0/24"],
    });
    expect(first.document.dns).toMatchObject({ final: "LockedDNSFinal" });
    expect(endpoint).toEqual({
      type: "tailscale",
      tag: "ts-ep",
      ephemeral: false,
      accept_routes: false,
    });

    const second = runTailscale("tailscale-native", first.document);
    expect(second.document).toEqual(first.document);
    expect(second.stringifyCalls).toBe(1);
  });

  it("fails closed on ambiguous TUN and incompatible owned endpoint or DNS tags", () => {
    const base = {
      dns: { servers: [], rules: [], final: "LockedDNSFinal" },
      inbounds: [{ type: "tun", tag: "tun-in", route_exclude_address: [] }],
      endpoints: [],
      route: { final: "LockedRouteFinal", rules: [{ outbound: "LockedFinal" }] },
    };
    const cases = [
      {
        id: "tailscale-native" as const,
        document: {
          ...base,
          inbounds: [{ type: "tun", tag: "first" }, { type: "tun", tag: "second" }],
        },
        error: "Sandrone sing-box Tailscale preset found ambiguous TUN inbounds",
      },
      {
        id: "tailscale-native" as const,
        document: {
          ...base,
          endpoints: [{ type: "tailscale", tag: "ts-ep", hostname: "custom" }],
        },
        error: "Sandrone sing-box Tailscale native preset found incompatible endpoint tag ts-ep",
      },
      {
        id: "tailscale-native" as const,
        document: {
          ...base,
          dns: { ...base.dns, servers: [{ type: "udp", tag: "ts-dns", server: "100.100.100.100" }] },
        },
        error: "Sandrone sing-box Tailscale native preset found incompatible DNS server tag ts-dns",
      },
      {
        id: "tailscale-external" as const,
        document: {
          ...base,
          endpoints: [{ type: "tailscale", tag: "ts-ep", ephemeral: false, accept_routes: false }],
        },
        error: "Sandrone sing-box Tailscale external preset found incompatible endpoint tag ts-ep",
      },
      {
        id: "tailscale-external" as const,
        document: {
          ...base,
          dns: { ...base.dns, servers: [{ type: "tailscale", tag: "ts-dns", endpoint: "ts-ep" }] },
        },
        error: "Sandrone sing-box Tailscale external preset found incompatible DNS server tag ts-dns",
      },
      {
        id: "tailscale-native" as const,
        document: {
          ...base,
          outbounds: [{ type: "direct", tag: "ts-ep" }],
        },
        error: "Sandrone sing-box Tailscale native preset found incompatible outbound tag ts-ep",
      },
      {
        id: "tailscale-external" as const,
        document: {
          ...base,
          outbounds: [{ type: "direct", tag: "ts-ep" }],
        },
        error: "Sandrone sing-box Tailscale external preset found incompatible outbound tag ts-ep",
      },
    ];

    for (const test of cases) {
      const execution = prepareTailscale(test.id, test.document);
      const before = execution.input.file.content;
      expect(execution.run).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }
  });

  it("prevalidates every mutated target shape and assigns only after one successful stringify", () => {
    const base = {
      dns: { servers: [], rules: [], final: "LockedDNSFinal" },
      inbounds: [{ type: "tun", tag: "tun-in", route_exclude_address: [] }],
      outbounds: [],
      endpoints: [],
      route: { final: "LockedRouteFinal", rules: [{ outbound: "LockedFinal" }] },
    };
    const cases = [
      {
        id: "tailscale-external" as const,
        document: { ...base, inbounds: "invalid" },
        error: "Sandrone sing-box Tailscale external preset requires inbounds to be an array of objects",
      },
      {
        id: "tailscale-external" as const,
        document: { ...base, dns: [] },
        error: "Sandrone sing-box Tailscale external preset requires dns to be an object",
      },
      {
        id: "tailscale-external" as const,
        document: { ...base, dns: { servers: "invalid", rules: [] } },
        error: "Sandrone sing-box Tailscale external preset requires dns.servers to be an array of objects",
      },
      {
        id: "tailscale-external" as const,
        document: { ...base, endpoints: "invalid" },
        error: "Sandrone sing-box Tailscale external preset requires endpoints to be an array of objects",
      },
      {
        id: "tailscale-native" as const,
        document: { ...base, route: [] },
        error: "Sandrone sing-box Tailscale native preset requires route to be an object",
      },
      {
        id: "tailscale-native" as const,
        document: { ...base, route: { final: "LockedRouteFinal", rules: "invalid" } },
        error: "Sandrone sing-box Tailscale native preset requires route.rules to be an array of objects",
      },
      {
        id: "tailscale-native" as const,
        document: {
          ...base,
          inbounds: [{ type: "tun", tag: "tun-in", route_exclude_address: [false] }],
        },
        error: "Sandrone sing-box Tailscale native preset requires TUN route_exclude_address to be an array of strings",
      },
    ];

    for (const test of cases) {
      const execution = prepareTailscale(test.id, test.document as Record<string, unknown>);
      const before = execution.input.file.content;
      expect(execution.run).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }

    for (const id of ["tailscale-native", "tailscale-external"] as const) {
      const execution = prepareTailscale(id, base, () => {
        throw new Error("stringify failed");
      });
      const before = execution.input.file.content;
      expect(execution.run).toThrowError("stringify failed");
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(1);
    }
  });

  it("replaces Tailscale modes and STUN atomically while preserving edited and ordered processors", () => {
    const customBefore = customProcessor("before");
    const editedSTUN = singBoxProcessorPreset("stun-block");
    editedSTUN.params = { ...editedSTUN.params, args: { preset_id: "stun-block", rules_json: "[]" } };
    const customAfter = customProcessor("after");
    const current = [
      customBefore,
      singBoxProcessorPreset("tailscale-external" as SingBoxProcessorPresetID),
      editedSTUN,
      singBoxProcessorPreset("stun-block"),
      customAfter,
    ];

    const native = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "tailscale-native",
      current,
    );
    expect(native.removedPresetIDs).toEqual(["tailscale-external", "stun-block"]);
    expect(applyPlan(current, native)).toEqual([
      customBefore,
      editedSTUN,
      customAfter,
      singBoxProcessorPreset("ensure-tun"),
      singBoxProcessorPreset("tailscale-native" as SingBoxProcessorPresetID),
    ]);

    const repeated = applyPlan(
      applyPlan(current, native),
      planFileProcessorPresetAddition(
        singBoxProcessorPresets,
        "tailscale-native",
        applyPlan(current, native),
      ),
    );
    expect(repeated).toEqual(applyPlan(current, native));

    const external = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "tailscale-external",
      [singBoxProcessorPreset("tailscale-native")],
    );
    expect(external.removedPresetIDs).toEqual(["tailscale-native"]);
    expect(external.addedPresetIDs).toEqual(["ensure-tun", "tailscale-external"]);

    const stun = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "stun-block",
      [
        customBefore,
        singBoxProcessorPreset("tailscale-native" as SingBoxProcessorPresetID),
        singBoxProcessorPreset("tailscale-external" as SingBoxProcessorPresetID),
        customAfter,
      ],
    );
    expect(stun.removedPresetIDs).toEqual(["tailscale-native", "tailscale-external"]);
    expect(applyPlan([
      customBefore,
      singBoxProcessorPreset("tailscale-native" as SingBoxProcessorPresetID),
      singBoxProcessorPreset("tailscale-external" as SingBoxProcessorPresetID),
      customAfter,
    ], stun).slice(0, 2)).toEqual([customBefore, customAfter]);
  });

  it("uses the exact Chinese STUN warning and explicit scenario risks", () => {
    expect(zhCN["processors.filePreset.singBox.stunBlock.risk"]).toBe(
      "阻止应用通过 STUN 获取公网出口地址；可能导致 WebRTC、语音通话、视频会议或 P2P 连接降级或失效。默认关闭。",
    );
    expect(enUS["processors.filePreset.singBox.udpP2pEim.risk"]).toContain("gVisor");
    expect(enUS["processors.filePreset.singBox.mptcpDirect.risk"]).toContain("cannot transparently proxy MPTCP");
    expect(enUS["processors.filePreset.singBox.mptcpDirect.risk"]).toContain("direct egress");
    expect(enUS["processors.filePreset.singBox.ipv4Only.risk"]).toContain("IPv6-only resources");
    expect(enUS["processors.filePreset.singBox.linuxTunAcceleration.risk"]).toContain("Linux/OpenWrt");
    expect(enUS["processors.filePreset.singBox.windowsRelaxedRoute.risk"]).toContain("Windows");
  });

  it.each([
    "stun-block",
    "quic-fallback",
    "ensure-tun",
    "ipv4-only",
    "udp-p2p-eim",
    "linux-tun-acceleration",
    "mptcp-direct",
    "windows-relaxed-route",
  ] as const)("recognizes only the exact managed processor for %s", (id) => {
    const preset = singBoxProcessorPreset(id);
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe(id);
    const params = preset.params as Record<string, unknown>;
    const source = params.source as Record<string, unknown>;
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...preset,
      params: { ...params, source: { ...source, content: `${String(source.content)}\n// user edit` } },
    })).toBeNull();
  });
});

function presetDescriptor(id: SingBoxProcessorPresetID) {
  const descriptor = singBoxProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing sing-box processor preset: ${id}`);
  return descriptor;
}

type TailscalePresetID = "tailscale-native" | "tailscale-external";

function runTailscale(
  id: TailscalePresetID,
  document: Record<string, unknown>,
): { document: Record<string, unknown>; stringifyCalls: number } {
  const execution = prepareTailscale(id, document);
  execution.run();
  return {
    document: JSON.parse(execution.input.file.content) as Record<string, unknown>,
    stringifyCalls: execution.stringifyCalls(),
  };
}

function prepareTailscale(
  id: TailscalePresetID,
  document: Record<string, unknown>,
  stringify: (value: unknown) => string = JSON.stringify,
) {
  const preset = singBoxProcessorPreset(id as SingBoxProcessorPresetID);
  const source = (preset.params?.source as Record<string, unknown> | undefined)?.content;
  expect(typeof source).toBe("string");
  const input = { file: { content: JSON.stringify(document) } };
  let stringifyCalls = 0;
  const api = {
    json: {
      parse: JSON.parse,
      stringify: (value: unknown) => {
        stringifyCalls += 1;
        return stringify(value);
      },
    },
  };
  const context: { input: typeof input; api: typeof api; output?: typeof input } = { input, api };
  return {
    input,
    run: () => runInNewContext(`${String(source)}\nglobalThis.output = main(input, api);`, context),
    stringifyCalls: () => stringifyCalls,
  };
}

function customProcessor(name: string) {
  return {
    name,
    type: "script",
    stage: "file",
    params: { source: { type: "inline", content: `// ${name}` } },
  } as const;
}

function applyPlan(
  current: readonly ReturnType<typeof singBoxProcessorPreset>[],
  plan: ReturnType<typeof planFileProcessorPresetAddition>,
) {
  const removals = new Set(plan.removeIndices);
  return [...current.filter((_, index) => !removals.has(index)), ...plan.additions];
}
