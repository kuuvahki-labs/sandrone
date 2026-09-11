import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { createTranslator } from "~/shared/i18n/context";

import {
  defaultSingBoxProcessors,
  singBoxProcessorPreset as buildSingBoxProcessorPreset,
  type SingBoxProcessorPresetID,
  singBoxProcessorPresets,
} from "./processor-presets";

const en = createTranslator("en-US");
const zh = createTranslator("zh-CN");

describe("sing-box file processor defaults", () => {
  it("stores the default outbound in the first new-file processor", () => {
    const processors = defaultSingBoxProcessors(en, { namingLocale: "en-US" });

    expect(processors.map((processor) => processor.name)).toEqual([
      "Outbound configuration adaptation",
      "Sniff & DNS Hijack",
      "GitHub acceleration",
    ]);
    expect(processors[0]).toMatchObject({ params: { args: { default_outbound: "Proxy" } } });
    expect(defaultSingBoxProcessors(en, { namingLocale: "zh-CN" })[0])
      .toMatchObject({ params: { args: { default_outbound: "🚀 节点选择" } } });
    expect(processors[1]).toMatchObject({
      name: "Sniff & DNS Hijack",
      type: "merge",
      stage: "file",
      params: { mode: "json_override" },
    });
    expect(JSON.parse(String(processors[1].params?.content))).toEqual({
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
    expect(defaultSingBoxProcessors(en, { namingLocale: "en-US" })[0]).not.toBe(processors[0]);
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of singBoxProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
  });

  it("recognizes only the exact managed JSON override", () => {
    const preset = defaultSingBoxProcessors(en, { namingLocale: "en-US" })[1]!;
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

  it("builds the exact QUIC ordered-rule processor", () => {
    expect(singBoxProcessorPreset("quic-fallback")).toMatchObject({
      name: "Force QUIC fallback",
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

  it("recognizes outbound adaptation by source while preserving editable execution and business parameters", () => {
    const descriptor = presetDescriptor("outbound-adapter");
    const processor = buildSingBoxProcessorPreset("outbound-adapter", "Group filter");
    const source = processor.params!.source as Record<string, unknown>;
    expect(descriptor).toMatchObject({ defaultOn: true, dependencies: [], conflicts: [] });
    expect(processor).toMatchObject({
      name: "Group filter", type: "script", stage: "file",
      params: { source: { type: "inline", content: expect.any(String) } },
    });
    expect(descriptor.recognize(processor)).toBe(true);
    expect(descriptor.recognize({ ...processor, params: { ...processor.params, args: {} } })).toBe(true);
    const configured = { ...processor, name: "My outbound adapter", params: {
      ...processor.params, timeout_ms: 10000, args: { default_outbound: "Manual" },
    } };
    expect(descriptor.recognize(configured)).toBe(true);
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "outbound-adapter", [configured], en).additions).toEqual([]);
    expect(processor.params).not.toHaveProperty("args");
    expect(descriptor.recognize({
      ...processor,
      params: { source: { ...source, content: `${String(source.content)}\n// customized` } },
    })).toBe(false);
    expect(descriptor.recognize({
      ...processor, params: { source: { ...source, type: "file" } },
    })).toBe(false);
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "outbound-adapter", [processor], en).additions)
      .toEqual([]);
  });

  it("reports a default outbound absent from the editable groups without changing the processor", () => {
    const descriptor = presetDescriptor("outbound-adapter");
    const processor = defaultSingBoxProcessors(en, { namingLocale: "en-US" })[0]!;
    expect(descriptor.configurationNotices?.(processor, { groups: [{ tag: "Proxy" }] })).toEqual([]);
    expect(descriptor.configurationNotices?.(processor, { groups: [{ tag: "Manual" }] })).toEqual([{
      messageKey: "files.config.defaultOutboundReferenceMissing", params: { target: "Proxy" },
    }]);
    expect(descriptor.configurationNotices?.(descriptor.build(en), { groups: [] })).toEqual([]);
    expect(processor.params?.args).toEqual({ default_outbound: "Proxy" });
  });

  it("declares the complete dependency, conflict, and default matrix", () => {
    expect(singBoxProcessorPresets.map((preset) => preset.id)).toEqual([
      "outbound-adapter",
      "sniff",
      "github-rule-source-mirror",
      "quic-fallback",
      "tailscale-native",
      "tailscale-external",
    ]);
    const scenarioIDs = [
      "quic-fallback",
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
      { id: "quic-fallback", defaultOn: false, dependencies: ["sniff"], conflicts: [] },
    ]);

    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "quic-fallback", [], en).addedPresetIDs)
      .toEqual(["sniff", "quic-fallback"]);
  });

  it("builds native Tailscale with editable auth_key and external Tailscale without args", () => {
    for (const id of ["tailscale-native", "tailscale-external"] as const) {
      const preset = singBoxProcessorPreset(id as SingBoxProcessorPresetID);
      expect(preset).toEqual({
        name: id === "tailscale-native" ? "Native Tailscale" : "Tailscale coexistence",
        type: "script",
        stage: "file",
        params: {
          source: { type: "inline", content: expect.any(String) },
          ...(id === "tailscale-native" ? { args: { auth_key: "" } } : {}),
        },
      });
      expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe(id);
    }

    expect(presetDescriptor("tailscale-native" as SingBoxProcessorPresetID)).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: ["tailscale-external"],
    });
    expect(presetDescriptor("tailscale-external" as SingBoxProcessorPresetID)).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: ["tailscale-native"],
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
          ownedDNSRule,
          { rule_set: ["private"], action: "route", server: "dns-local" },
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

  it.each([
    { type: "fakeip", tag: "fake", inet4_range: "198.18.0.0/15" },
    { address: "fakeip", tag: "fake" },
  ])("routes Tailscale DNS before catch-all FakeIP rules (%j)", (fakeServer) => {
    const magicDNSRule = { domain_suffix: ["ts.net"], action: "route", server: "ts-dns" };
    const catchAll = { query_type: ["A", "AAAA"], action: "route", server: "fake" };
    const original = {
      inbounds: [{ type: "tun", tag: "tun-in" }],
      dns: {
        servers: [
          { type: "local", tag: "local" },
          fakeServer,
          { type: "https", tag: "chosen-resolver", server: "192.0.2.53", detour: "proxy" },
        ],
        rules: [catchAll, magicDNSRule, magicDNSRule],
        final: "chosen-resolver",
      },
    };

    const first = runTailscale("tailscale-external", original);
    expect(first.document.dns).toEqual({
      ...original.dns,
      servers: [...original.dns.servers, { type: "udp", tag: "ts-dns", server: "100.100.100.100" }],
      rules: [
        magicDNSRule,
        { domain_suffix: ["tailscale.com"], action: "route", server: "chosen-resolver" },
        catchAll,
      ],
    });
    expect(runTailscale("tailscale-external", first.document).document).toEqual(first.document);
  });

  it.each([
    { type: "udp", tag: "first-resolver", server: "192.0.2.53" },
    { tag: "first-resolver", address: "https://dns.example.com/dns-query" },
    { type: "legacy", tag: "first-resolver", address: "https://dns.example.com/dns-query" },
    { type: "", tag: "first-resolver", address: "local" },
    { type: "resolved", tag: "first-resolver", accept_default_resolvers: true },
  ])("uses the first real DNS server when FakeIP is present without dns.final (%j)", (realServer) => {
    const original = {
      inbounds: [{ type: "tun", tag: "tun-in" }],
      dns: {
        servers: [realServer, { type: "fakeip", tag: "fake" }],
        rules: [{ server: "fake" }],
      },
    };
    expect(runTailscale("tailscale-external", original).document.dns).toEqual({
      servers: [...original.dns.servers, { type: "udp", tag: "ts-dns", server: "100.100.100.100" }],
      rules: [
        { domain_suffix: ["ts.net"], action: "route", server: "ts-dns" },
        { domain_suffix: ["tailscale.com"], action: "route", server: "first-resolver" },
        ...original.dns.rules,
      ],
    });
  });

  it.each([
    { type: "fakeip", tag: "default" },
    { type: "hosts", tag: "default" },
    { type: "tailscale", tag: "default" },
    { type: "resolved", tag: "default", accept_default_resolvers: false },
    { type: "udp", tag: "default", server: "100.100.100.100" },
    { tag: "default", address: "rcode://success" },
    { tag: "default", address: "udp://100.100.100.100:53" },
    { type: "local" },
  ])("rejects an unsuitable default DNS atomically when FakeIP is present (%j)", (defaultServer) => {
    const execution = prepareTailscale("tailscale-external", {
      inbounds: [{ type: "tun", tag: "tun-in" }],
      dns: { servers: [defaultServer, { type: "fakeip", tag: "fake" }] },
    });
    const before = execution.input.file.content;
    expect(execution.run).toThrowError("requires a tagged real default DNS server");
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
  });

  it.each(["fake", "missing"])("does not replace an invalid explicit dns.final=%s with another resolver", (final) => {
    const execution = prepareTailscale("tailscale-external", {
      inbounds: [{ type: "tun", tag: "tun-in" }],
      dns: {
        servers: [{ type: "local", tag: "real" }, { type: "fakeip", tag: "fake" }],
        final,
      },
    });
    const before = execution.input.file.content;
    expect(execution.run).toThrowError("requires a tagged real default DNS server");
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
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

    const first = runTailscale("tailscale-native", original, "tskey-auth-test");
    expect(first.stringifyCalls).toBe(1);
    expect(first.document.endpoints).toEqual([
      original.endpoints[0],
      { ...endpoint, auth_key: "tskey-auth-test" },
    ]);
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

    const second = runTailscale("tailscale-native", first.document, "tskey-auth-test");
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

  it("replaces Tailscale modes atomically while preserving edited and ordered processors", () => {
    const customBefore = customProcessor("before");
    const customAfter = customProcessor("after");
    const current = [
      customBefore,
      singBoxProcessorPreset("tailscale-external" as SingBoxProcessorPresetID),
      customAfter,
    ];

    const native = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "tailscale-native",
      current,
      en,
    );
    expect(native.removedPresetIDs).toEqual(["tailscale-external"]);
    expect(applyPlan(current, native)).toEqual([
      customBefore,
      customAfter,
      singBoxProcessorPreset("tailscale-native" as SingBoxProcessorPresetID),
    ]);

    const repeated = applyPlan(
      applyPlan(current, native),
      planFileProcessorPresetAddition(
        singBoxProcessorPresets,
        "tailscale-native",
        applyPlan(current, native),
        en,
      ),
    );
    expect(repeated).toEqual(applyPlan(current, native));

    const external = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "tailscale-external",
      [singBoxProcessorPreset("tailscale-native")],
      en,
    );
    expect(external.removedPresetIDs).toEqual(["tailscale-native"]);
    expect(external.addedPresetIDs).toEqual(["tailscale-external"]);

  });

  it.each([
    "quic-fallback",
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

function singBoxProcessorPreset(id: SingBoxProcessorPresetID) {
  const preset = presetDescriptor(id);
  return buildSingBoxProcessorPreset(id, en(preset.labelKey));
}

type TailscalePresetID = "tailscale-native" | "tailscale-external";

function runTailscale(
  id: TailscalePresetID,
  document: Record<string, unknown>,
  authKey = "",
): { document: Record<string, unknown>; stringifyCalls: number } {
  const execution = prepareTailscale(id, document, JSON.stringify, authKey);
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
  authKey = "",
) {
  const preset = singBoxProcessorPreset(id as SingBoxProcessorPresetID);
  const source = (preset.params?.source as Record<string, unknown> | undefined)?.content;
  expect(typeof source).toBe("string");
  const input = {
    file: { content: JSON.stringify(document) },
    args: id === "tailscale-native" ? { auth_key: authKey } : {},
  };
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
  const additionsByIndex = new Map<number | null, ReturnType<typeof singBoxProcessorPreset>[]>();
  for (const addition of plan.additions) {
    const additions = additionsByIndex.get(addition.beforeIndex) ?? [];
    additions.push(addition.processor);
    additionsByIndex.set(addition.beforeIndex, additions);
  }
  const applied: ReturnType<typeof singBoxProcessorPreset>[] = [];
  current.forEach((processor, index) => {
    applied.push(...(additionsByIndex.get(index) ?? []));
    if (!removals.has(index)) applied.push(processor);
  });
  applied.push(...(additionsByIndex.get(null) ?? []));
  return applied;
}
