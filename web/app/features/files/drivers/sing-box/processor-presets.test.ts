import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { createTranslator } from "~/shared/i18n/context";

import legacyOutboundAdapterScript from "./legacy/sing-box-outbound-adapter-reject-empty.js?raw";
import legacyTailscaleExternalScript from "./legacy/sing-box-tailscale-external.js?raw";
import legacyTailscaleNativeScript from "./legacy/sing-box-tailscale-native.js?raw";
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
      "GitHub acceleration",
    ]);
    expect(processors[0]).toMatchObject({ params: { args: { default_outbound: "Proxy" } } });
    expect(defaultSingBoxProcessors(en, { namingLocale: "zh-CN" })[0])
      .toMatchObject({ params: { args: { default_outbound: "🚀 节点选择" } } });
    expect(defaultSingBoxProcessors(en, { namingLocale: "en-US" })[0]).not.toBe(processors[0]);
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of singBoxProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
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
    expect(descriptor.isCurrent?.(processor)).toBe(true);
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

  it("recognizes the exact reject-empty adapter and preserves its saved parameters when explicitly refreshed", () => {
    const descriptor = presetDescriptor("outbound-adapter");
    const legacy = {
      ...buildSingBoxProcessorPreset("outbound-adapter", "My outbound adapter"),
      enabled: false,
      params: {
        source: { type: "inline", content: legacyOutboundAdapterScript },
        timeout_ms: 10000,
        args: { default_outbound: "Manual" },
      },
    };

    expect(descriptor.recognize(legacy)).toBe(true);
    expect(descriptor.isCurrent?.(legacy)).toBe(false);
    const plan = planFileProcessorPresetAddition(singBoxProcessorPresets, "outbound-adapter", [legacy], en);
    expect(plan.updatedPresetIDs).toEqual(["outbound-adapter"]);
    expect(plan.removeIndices).toEqual([0]);
    expect(plan.additions).toHaveLength(1);
    expect(plan.additions[0]).toMatchObject({
      presetID: "outbound-adapter",
      beforeIndex: 0,
      processor: {
        name: "My outbound adapter",
        enabled: true,
        type: "script",
        stage: "file",
        params: { timeout_ms: 10000, args: { default_outbound: "Manual" } },
      },
    });
    expect((plan.additions[0].processor.params?.source as Record<string, unknown>).content)
      .not.toBe(legacyOutboundAdapterScript);
    expect(descriptor.isCurrent?.(plan.additions[0].processor)).toBe(true);
    expect(descriptor.recognize({
      ...legacy,
      params: { ...legacy.params, source: { type: "inline", content: `${legacyOutboundAdapterScript}\n` } },
    })).toBe(false);
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
      "github-rule-source-mirror",
      "quic-fallback",
      "tailscale-native",
      "tailscale-external",
      "tailnet-share",
      "fakeip-compat",
      "fakeip-ruleset-geodata",
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
      { id: "quic-fallback", defaultOn: false, dependencies: [], conflicts: [] },
    ]);

    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "quic-fallback", [], en).addedPresetIDs)
      .toEqual(["quic-fallback"]);
  });

  it("builds Tailscale processors with stable preset markers", () => {
    for (const id of ["tailscale-native", "tailscale-external"] as const) {
      const preset = singBoxProcessorPreset(id as SingBoxProcessorPresetID);
      expect(preset).toEqual({
        name: id === "tailscale-native" ? "Native Tailscale" : "Tailscale coexistence",
        type: "script",
        stage: "file",
        params: {
          source: { type: "inline", content: expect.any(String) },
          args: id === "tailscale-native"
            ? { preset_id: id, auth_key: "" }
            : { preset_id: id },
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

  it("routes Tailscale DNS before catch-all FakeIP rules", () => {
    const fakeServer = { type: "fakeip", tag: "fake", inet4_range: "198.18.0.0/15" };
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

  it("applies native Tailscale with v1.14.0 endpoint, DNS, and route shapes", () => {
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
    const dnsRule = { preferred_by: "ts-dns", action: "route", server: "ts-dns" };
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

  it("switches FakeIP list modes in place while preserving edited processors", () => {
    const customBefore = customProcessor("before");
    const customAfter = customProcessor("after");
    const stable = singBoxProcessorPreset("fakeip-compat");
    const upstream = singBoxProcessorPreset("fakeip-ruleset-geodata");
    const current = [customBefore, stable, customAfter];

    expect(presetDescriptor("fakeip-compat")).toMatchObject({
      defaultOn: false,
      dependencies: [],
      conflicts: ["fakeip-ruleset-geodata"],
      replaceConflictsInPlace: true,
    });
    expect(presetDescriptor("fakeip-ruleset-geodata")).toMatchObject({
      defaultOn: false,
      dependencies: [],
      conflicts: ["fakeip-compat"],
      replaceConflictsInPlace: true,
    });

    const upstreamPlan = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "fakeip-ruleset-geodata",
      current,
      en,
    );
    expect(upstreamPlan.removedPresetIDs).toEqual(["fakeip-compat"]);
    expect(applyPlan(current, upstreamPlan)).toEqual([customBefore, upstream, customAfter]);
    expect(planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "fakeip-ruleset-geodata",
      [upstream],
      en,
    )).toMatchObject({ additions: [], removeIndices: [] });

    const stablePlan = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "fakeip-compat",
      [customBefore, upstream, customAfter],
      en,
    );
    expect(stablePlan.removedPresetIDs).toEqual(["fakeip-ruleset-geodata"]);
    expect(applyPlan([customBefore, upstream, customAfter], stablePlan)).toEqual(current);

    const editedStable = {
      ...stable,
      params: {
        ...stable.params,
        source: {
          ...(stable.params?.source as Record<string, unknown>),
          content: `${String((stable.params?.source as Record<string, unknown>).content)}\n// edited`,
        },
      },
    };
    const withEdited = [editedStable, upstream];
    const editedPlan = planFileProcessorPresetAddition(
      singBoxProcessorPresets,
      "fakeip-compat",
      withEdited,
      en,
    );
    expect(editedPlan.removeIndices).toEqual([1]);
    expect(applyPlan(withEdited, editedPlan)).toEqual([editedStable, stable]);
  });

  it("builds the managed presets with editable typed arguments", () => {
    expect(singBoxProcessorPreset("tailnet-share")).toMatchObject({
      params: { args: {
        preset_id: "tailnet-share",
        listen_addresses: [],
        listen_port: 2081,
        username: "",
        password: "",
      } },
    });
    const fakeIPArgs = singBoxProcessorPreset("fakeip-compat").params?.args as Record<string, unknown>;
    expect(fakeIPArgs).toEqual({
      preset_id: "fakeip-compat",
      server: "",
      domain: [
        "time-ios.apple.com", "ntp.ntsc.ac.cn", "mesu.apple.com", "swscan.apple.com",
        "swquery.apple.com", "swdownload.apple.com", "swcdn.apple.com", "swdist.apple.com",
        "music.163.com", "y.qq.com", "streamoc.music.tc.qq.com", "mobileoc.music.tc.qq.com",
        "isure.stream.qqmusic.qq.com", "dl.stream.qqmusic.qq.com", "aqqmusic.tc.qq.com",
        "amobile.music.tc.qq.com", "songsearch.kugou.com", "trackercdn.kugou.com",
        "music.migu.cn", "ps.res.netease.com",
      ],
      domain_suffix: ["cmbchina.com", "cmbimg.com", "sandai.net", "n0808.com", "uu.163.com", "oray.com", "orayimg.com"],
      domain_regex: [
        "^time\\.[^.]+\\.gov$", "^time\\.[^.]+\\.edu\\.cn$", "^time\\.[^.]+\\.apple\\.com$",
        "^time1\\.[^.]+\\.com$", "^time2\\.[^.]+\\.com$", "^time3\\.[^.]+\\.com$",
        "^time4\\.[^.]+\\.com$", "^time5\\.[^.]+\\.com$", "^time6\\.[^.]+\\.com$",
        "^time7\\.[^.]+\\.com$", "^ntp1\\.[^.]+\\.com$", "^ntp2\\.[^.]+\\.com$",
        "^ntp3\\.[^.]+\\.com$", "^ntp4\\.[^.]+\\.com$", "^ntp5\\.[^.]+\\.com$",
        "^ntp6\\.[^.]+\\.com$", "^ntp7\\.[^.]+\\.com$", "^[^.]+\\.time\\.edu\\.cn$",
        "^[^.]+\\.ntp\\.org\\.cn$", "^[^.]+\\.music\\.163\\.com$", "^[^.]+\\.y\\.qq\\.com$",
        "^[^.]+\\.kuwo\\.cn$", "^[^.]+\\.music\\.migu\\.cn$", "^[^.]+\\.mcdn\\.bilivideo\\.cn$",
      ],
    });
    expect((fakeIPArgs.domain_regex as string[]).some((value) => value.includes("*"))).toBe(false);
    expect(singBoxProcessorPreset("fakeip-ruleset-geodata")).toMatchObject({
      params: { args: { preset_id: "fakeip-ruleset-geodata", server: "" } },
    });
  });

  it("shares exact Tailnet IPs with optional authentication and replaces owned listeners", () => {
    const base = { inbounds: [{ type: "tun", tag: "tun-in" }, { type: "mixed", tag: "mixed-in", listen: "127.0.0.1", listen_port: 2080 }] };
    const first = runManaged("tailnet-share", base, {
      listen_addresses: ["100.64.0.7", "fd7a:115c:a1e0::7"],
      listen_port: 2443,
      username: "alice",
      password: "secret",
    });
    expect(first.document.inbounds).toEqual([
      ...base.inbounds,
      { type: "mixed", tag: "tailnet-share-v4", listen: "100.64.0.7", listen_port: 2443, users: [{ username: "alice", password: "secret" }] },
      { type: "mixed", tag: "tailnet-share-v6", listen: "fd7a:115c:a1e0::7", listen_port: 2443, users: [{ username: "alice", password: "secret" }] },
    ]);
    expect(runManaged("tailnet-share", first.document, {
      listen_addresses: ["100.64.0.7", "fd7a:115c:a1e0::7"], listen_port: 2443, username: "alice", password: "secret",
    }).document).toEqual(first.document);
    expect(runManaged("tailnet-share", first.document, {
      listen_addresses: ["100.127.255.254"], listen_port: 2081, username: "", password: "",
    }).document.inbounds).toEqual([
      ...base.inbounds,
      { type: "mixed", tag: "tailnet-share-v4", listen: "100.127.255.254", listen_port: 2081 },
    ]);
    expect(() => runManaged("tailnet-share", base, {
      listen_addresses: ["100.63.255.255"], listen_port: 2443, username: "", password: "",
    })).toThrowError("requires a unique exact IP in the Tailnet ranges");
    expect(() => runManaged("tailnet-share", {
      inbounds: [...base.inbounds, { type: "mixed", tag: "wild", listen: "0.0.0.0", listen_port: 2443 }],
    }, {
      listen_addresses: ["100.64.0.7"], listen_port: 2443, username: "", password: "",
    })).toThrowError("conflicting listener socket");
    expect(() => runManaged("tailnet-share", {
      inbounds: [...base.inbounds, { type: "mixed", tag: "wild-default", listen_port: 2443 }],
    }, {
      listen_addresses: ["100.64.0.7"], listen_port: 2443, username: "", password: "",
    })).toThrowError("conflicting listener socket");
    expect(() => runManaged("tailnet-share", {
      inbounds: [...base.inbounds, { type: "mixed", tag: "same-ip", listen: "fd7a:115c:a1e0:0:0:0:0:7", listen_port: 2443 }],
    }, {
      listen_addresses: ["fd7a:115c:a1e0::7"], listen_port: 2443, username: "", password: "",
    })).toThrowError("conflicting listener socket");
  });

  it("rejects invalid Tailnet share arguments and owned-tag collisions", () => {
    const base = { inbounds: [{ type: "tun", tag: "tun-in" }] };
    const invalidCases = [
      { args: { listen_addresses: [], listen_port: 2080, username: "", password: "" }, error: "at least one exact Tailnet IP address" },
      { args: { listen_addresses: ["100.64.0.7/32"], listen_port: 2080, username: "", password: "" }, error: "unique exact IP in the Tailnet ranges" },
      { args: { listen_addresses: ["fd7a:115c:a1e1::7"], listen_port: 2080, username: "", password: "" }, error: "unique exact IP in the Tailnet ranges" },
      { args: { listen_addresses: ["100.64.0.7", "100.65.0.8"], listen_port: 2080, username: "", password: "" }, error: "unique exact IP in the Tailnet ranges" },
      { args: { listen_addresses: ["100.64.0.7"], listen_port: 2080, username: "alice", password: "" }, error: "requires username and password together" },
      { args: { listen_addresses: ["100.64.0.7"], listen_port: 0, username: "", password: "" }, error: "requires a valid listen_port" },
    ];
    for (const test of invalidCases) {
      expect(() => runManaged("tailnet-share", base, test.args)).toThrowError(test.error);
    }
    const compatible = { type: "mixed", tag: "tailnet-share-v4", listen: "100.64.0.9", listen_port: 2080 };
    expect(() => runManaged("tailnet-share", { inbounds: [base.inbounds[0], compatible, compatible] }, {
      listen_addresses: ["100.64.0.7"], listen_port: 2080, username: "", password: "",
    })).toThrowError("duplicate inbound tag tailnet-share-v4");
    expect(() => runManaged("tailnet-share", {
      inbounds: [base.inbounds[0], { type: "http", tag: "tailnet-share-v4", listen: "100.64.0.9", listen_port: 2080 }],
    }, {
      listen_addresses: ["100.64.0.7"], listen_port: 2080, username: "", password: "",
    })).toThrowError("incompatible inbound tag tailnet-share-v4");
  });

  it("adds one inline FakeIP compatibility rule-set before any FakeIP DNS route", () => {
    const original = {
      dns: {
        servers: [
          { type: "https", tag: "dns-remote", server: "1.1.1.1" },
          { type: "fakeip", tag: "dns-fakeip" },
        ],
        rules: [
          { domain_suffix: ["before.example"], server: "dns-remote" },
          { query_type: ["A", "AAAA"], action: "route", server: "dns-fakeip" },
        ],
        final: "dns-remote",
      },
      route: { rule_set: [{ type: "inline", tag: "private", rules: [] }], rules: [] },
    };
    const first = runManaged("fakeip-compat", original, {
      domain: ["exact.example"],
      domain_suffix: ["suffix.example"],
      domain_regex: ["^[^.]+\\.wild\\.example$"],
      server: "",
    });
    expect(first.document).toMatchObject({
      dns: { rules: [
        original.dns.rules[0],
        { rule_set: ["sandrone-fakeip-compat"], action: "route", server: "dns-remote" },
        original.dns.rules[1],
      ] },
      route: { rule_set: [
        original.route.rule_set[0],
        { type: "inline", tag: "sandrone-fakeip-compat", rules: [{
          domain: ["exact.example"],
          domain_suffix: ["suffix.example"],
          domain_regex: ["^[^.]+\\.wild\\.example$"],
        }] },
      ] },
    });
    expect(runManaged("fakeip-compat", first.document, {
      domain: ["exact.example"], domain_suffix: ["suffix.example"], domain_regex: ["^[^.]+\\.wild\\.example$"], server: "",
    }).document).toEqual(first.document);
    const changed = runManaged("fakeip-compat", first.document, {
      domain: ["changed.example"], domain_suffix: [], domain_regex: [], server: "dns-remote",
    }).document;
    expect(changed).toMatchObject({
      route: { rule_set: expect.arrayContaining([
        { type: "inline", tag: "sandrone-fakeip-compat", rules: [{ domain: ["changed.example"] }] },
      ]) },
    });
    expect(runManaged("fakeip-compat", changed, {
      domain: ["changed.example"], domain_suffix: [], domain_regex: [], server: "dns-remote",
    }).document).toEqual(changed);
    expect(() => runManaged("fakeip-compat", {
      ...original,
      route: { ...original.route, rule_set: [{ type: "remote", tag: "sandrone-fakeip-compat", url: "https://example.com" }] },
    }, { domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("found incompatible route rule-set tag sandrone-fakeip-compat");
    expect(() => runManaged("fakeip-compat", {
      ...original,
      dns: {
        ...original.dns,
        servers: [{ type: "udp", tag: "magic", server: "100.100.100.100" }, original.dns.servers[1]],
        final: "magic",
      },
    }, { domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("requires a tagged real DNS resolver");
  });

  it("selects an explicit real resolver and inserts before the earliest of multiple FakeIP routes", () => {
    const document = {
      dns: {
        servers: [
          { type: "https", tag: "dns-remote", server: "1.1.1.1" },
          { type: "tls", tag: "dns-explicit", server: "8.8.8.8" },
          { type: "fakeip", tag: "fake-one" },
          { type: "fakeip", tag: "fake-two" },
        ],
        rules: [
          { domain: ["before.example"], server: "dns-remote" },
          { domain: ["first-fake.example"], server: "fake-two" },
          { domain: ["middle.example"], server: "dns-remote" },
          { domain: ["second-fake.example"], server: "fake-one" },
        ],
        final: "dns-remote",
      },
      route: { rule_set: [], rules: [] },
    };
    const result = runManaged("fakeip-compat", document, {
      domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "dns-explicit",
    }).document;
    expect((result.dns as { rules: unknown[] }).rules).toEqual([
      document.dns.rules[0],
      { rule_set: ["sandrone-fakeip-compat"], action: "route", server: "dns-explicit" },
      ...document.dns.rules.slice(1),
    ]);
  });

  it("rejects incomplete FakeIP inputs and reserved-tag collisions", () => {
    const base = {
      dns: {
        servers: [{ type: "local", tag: "real" }, { type: "fakeip", tag: "fake" }],
        rules: [{ server: "fake" }],
        final: "real",
      },
      route: { rule_set: [], rules: [] },
    };
    expect(() => runManaged("fakeip-compat", base, { domain: [], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("requires at least one compatibility domain");
    expect(() => runManaged("fakeip-compat", {
      ...base, dns: { ...base.dns, servers: [{ type: "local", tag: "real" }] },
    }, { domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("requires a tagged FakeIP DNS server");
    const managed = { type: "inline", tag: "sandrone-fakeip-compat", rules: [{ domain: ["old.example"] }] };
    expect(() => runManaged("fakeip-compat", {
      ...base, route: { ...base.route, rule_set: [managed, managed] },
    }, { domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("duplicate route rule-set tag sandrone-fakeip-compat");
    expect(() => runManaged("fakeip-compat", {
      ...base,
      dns: { ...base.dns, rules: [{ rule_set: ["sandrone-fakeip-compat"], action: "reject", server: "real" }, ...base.dns.rules] },
    }, { domain: ["exact.example"], domain_suffix: [], domain_regex: [], server: "" }))
      .toThrowError("incompatible DNS rule for sandrone-fakeip-compat");
  });

  it("adds the managed DustinWin rule-set before the earliest FakeIP DNS route", () => {
    const original = {
      http_clients: [{ tag: "rule-set-direct" }, { tag: "rule-set-proxy" }],
      dns: {
        servers: [
          { type: "https", tag: "dns-remote", server: "1.1.1.1" },
          { type: "tls", tag: "dns-explicit", server: "8.8.8.8" },
          { type: "fakeip", tag: "fake-one" },
          { type: "fakeip", tag: "fake-two" },
        ],
        rules: [
          { domain: ["before.example"], server: "dns-remote" },
          { domain: ["first-fake.example"], server: "fake-two" },
          { domain: ["middle.example"], server: "dns-remote" },
          { domain: ["second-fake.example"], server: "fake-one" },
        ],
        final: "dns-remote",
      },
      route: {
        default_http_client: "rule-set-direct",
        final: "LockedRouteFinal",
        rule_set: [{ type: "inline", tag: "private", rules: [] }],
        rules: [{ outbound: "LockedFinal" }],
      },
    };
    const first = runManaged("fakeip-ruleset-geodata", original, { server: "dns-explicit" });
    expect(first.stringifyCalls).toBe(1);
    expect(first.document).toEqual({
      ...original,
      dns: {
        ...original.dns,
        rules: [
          original.dns.rules[0],
          { rule_set: ["sandrone-fakeip-ruleset-geodata"], action: "route", server: "dns-explicit" },
          ...original.dns.rules.slice(1),
        ],
      },
      route: {
        ...original.route,
        rule_set: [
          original.route.rule_set[0],
          {
            type: "remote",
            tag: "sandrone-fakeip-ruleset-geodata",
            format: "binary",
            url: "https://cdn.jsdelivr.net/gh/DustinWin/ruleset_geodata@sing-box-ruleset/fakeip-filter.srs",
            http_client: "rule-set-direct",
            update_interval: "1d",
          },
        ],
      },
    });
    expect(runManaged("fakeip-ruleset-geodata", first.document, { server: "dns-explicit" }).document)
      .toEqual(first.document);
  });

  it("uses dns.final and a sole HTTP client when explicit defaults are omitted", () => {
    const original = {
      http_clients: [{ tag: "only-client" }],
      dns: {
        servers: [{ type: "local", tag: "real" }, { type: "fakeip", tag: "fake" }],
        rules: [{ server: "fake" }],
        final: "real",
      },
      route: { rules: [{ outbound: "LockedFinal" }] },
    };
    const result = runManaged("fakeip-ruleset-geodata", original).document;
    expect((result.dns as { rules: unknown[] }).rules[0]).toEqual({
      rule_set: ["sandrone-fakeip-ruleset-geodata"],
      action: "route",
      server: "real",
    });
    expect((result.route as { rule_set: unknown[] }).rule_set[0]).toMatchObject({
      tag: "sandrone-fakeip-ruleset-geodata",
      http_client: "only-client",
    });
  });

  it("rejects invalid DustinWin rule-set inputs and collisions atomically", () => {
    const managedRuleSet = {
      type: "remote",
      tag: "sandrone-fakeip-ruleset-geodata",
      format: "binary",
      url: "https://cdn.jsdelivr.net/gh/DustinWin/ruleset_geodata@sing-box-ruleset/fakeip-filter.srs",
      http_client: "rule-set-direct",
      update_interval: "1d",
    };
    const base = {
      http_clients: [{ tag: "rule-set-direct" }],
      dns: {
        servers: [{ type: "local", tag: "real" }, { type: "fakeip", tag: "fake" }],
        rules: [{ server: "fake" }],
        final: "real",
      },
      route: { default_http_client: "rule-set-direct", rule_set: [], rules: [] },
    };
    const cases = [
      {
        document: { ...base, dns: { ...base.dns, servers: [{ type: "local", tag: "real" }] } },
        error: "requires a tagged FakeIP DNS server",
      },
      {
        document: { ...base, dns: { ...base.dns, rules: [{ server: "real" }] } },
        error: "requires a DNS rule routed to FakeIP",
      },
      {
        document: { ...base, dns: { ...base.dns, final: "fake" } },
        error: "requires a tagged real DNS resolver",
      },
      {
        document: { ...base, http_clients: [] },
        error: "requires one configured default HTTP client",
      },
      {
        document: {
          ...base,
          http_clients: [{ tag: "one" }, { tag: "two" }],
          route: { ...base.route, default_http_client: "" },
        },
        error: "requires one configured default HTTP client",
      },
      {
        document: {
          ...base,
          route: { ...base.route, rule_set: [{ ...managedRuleSet, url: "https://example.com/other.srs" }] },
        },
        error: "found incompatible route rule-set tag sandrone-fakeip-ruleset-geodata",
      },
      {
        document: { ...base, route: { ...base.route, rule_set: [managedRuleSet, managedRuleSet] } },
        error: "found duplicate route rule-set tag sandrone-fakeip-ruleset-geodata",
      },
      {
        document: {
          ...base,
          dns: {
            ...base.dns,
            rules: [
              { rule_set: ["sandrone-fakeip-ruleset-geodata", "private"], action: "route", server: "real" },
              ...base.dns.rules,
            ],
          },
        },
        error: "found incompatible DNS rule for sandrone-fakeip-ruleset-geodata",
      },
    ];

    for (const test of cases) {
      const execution = prepareManaged("fakeip-ruleset-geodata", test.document);
      const before = execution.input.file.content;
      expect(execution.run).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }
  });

  it("declares the full Tailscale dependency chain and cascades dependents", () => {
    expect(presetDescriptor("tailnet-share")).toMatchObject({ dependencies: ["tailscale-external"], conflicts: [] });
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "tailnet-share", [], en).addedPresetIDs)
      .toEqual(["tailscale-external", "tailnet-share"]);
    const current = [singBoxProcessorPreset("tailscale-external"), singBoxProcessorPreset("tailnet-share")];
    const native = planFileProcessorPresetAddition(singBoxProcessorPresets, "tailscale-native", current, en);
    expect(native.removedPresetIDs).toEqual(["tailscale-external", "tailnet-share"]);
    expect(native.addedPresetIDs).toEqual(["tailscale-native"]);
  });

  it("recognizes managed scripts with common execution params but not edited sources or invalid business args", () => {
    for (const id of ["tailscale-native", "tailscale-external", "tailnet-share", "fakeip-compat", "fakeip-ruleset-geodata"] as const) {
      const preset = singBoxProcessorPreset(id);
      const descriptor = presetDescriptor(id);
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, timeout_ms: 5000 } })).toBe(true);
      const source = preset.params?.source as Record<string, unknown>;
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, source: { ...source, content: `${String(source.content)}\n// edited` } } })).toBe(false);
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, args: { preset_id: id } } })).toBe(id === "tailscale-external");
    }
  });

  it("recognizes exact legacy Tailscale sources and refreshes them only when reselected", () => {
    const legacyNative = {
      ...singBoxProcessorPreset("tailscale-native"),
      params: {
        source: { type: "inline", content: legacyTailscaleNativeScript },
        timeout_ms: 5000,
        args: { auth_key: "legacy-secret" },
      },
    };
    const legacyExternal = {
      ...singBoxProcessorPreset("tailscale-external"),
      params: { source: { type: "inline", content: legacyTailscaleExternalScript }, timeout_ms: 5000 },
    };
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, legacyNative)).toBe("tailscale-native");
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, legacyExternal)).toBe("tailscale-external");
    expect(presetDescriptor("tailscale-native").isCurrent?.(legacyNative)).toBe(false);
    expect(presetDescriptor("tailscale-external").isCurrent?.(legacyExternal)).toBe(false);

    const nativePlan = planFileProcessorPresetAddition(singBoxProcessorPresets, "tailscale-native", [legacyNative], en);
    expect(nativePlan.updatedPresetIDs).toEqual(["tailscale-native"]);
    expect(nativePlan.addedPresetIDs).toEqual(["tailscale-native"]);
    expect((nativePlan.additions.at(-1)?.processor.params?.source as Record<string, unknown>).content)
      .not.toBe(legacyTailscaleNativeScript);
    expect(planFileProcessorPresetAddition(singBoxProcessorPresets, "tailnet-share", [legacyExternal], en).updatedPresetIDs)
      .toEqual(["tailscale-external"]);

    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, {
      ...legacyNative,
      params: { ...legacyNative.params, source: { type: "inline", content: `${legacyTailscaleNativeScript}\n` } },
    })).toBeNull();
  });

  it("rejects managed request overrides and wrong execution envelopes before parsing content", () => {
    const managed = [
      ["tailscale-native", "auth_key"],
      ["tailscale-external", "preset_id"],
      ["tailnet-share", "listen_port"],
      ["fakeip-compat", "domain"],
      ["fakeip-ruleset-geodata", "server"],
    ] as const;
    for (const [id] of managed) {
      expect(prepareManaged(id, {}, {}, { stage: "nodes" }).run).toThrowError("requires sing-box file-stage input");
      expect(prepareManaged(id, {}, {}, { kind: "mihomo" }).run).toThrowError("requires sing-box file-stage input");
    }
    for (const [id, key] of managed) {
      const overridden = prepareManaged(id, {}, {}, { requestArgs: { [key]: "request-controlled" } });
      expect(overridden.run).toThrowError("Sandrone preset arguments cannot be overridden by request args");
      expect(overridden.stringifyCalls()).toBe(0);
    }
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
    stage: "file",
    file: { kind: "sing-box", content: JSON.stringify(document) },
    request: { args: {} },
    args: id === "tailscale-native"
      ? { preset_id: id, auth_key: authKey }
      : { preset_id: id },
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

type ManagedScriptPresetID =
  | "tailscale-native"
  | "tailscale-external"
  | "tailnet-share"
  | "fakeip-compat"
  | "fakeip-ruleset-geodata";

function runManaged(
  id: ManagedScriptPresetID,
  document: Record<string, unknown>,
  args: Record<string, unknown> = {},
): { document: Record<string, unknown>; stringifyCalls: number } {
  const execution = prepareManaged(id, document, args);
  execution.run();
  return {
    document: JSON.parse(execution.input.file.content) as Record<string, unknown>,
    stringifyCalls: execution.stringifyCalls(),
  };
}

function prepareManaged(
  id: ManagedScriptPresetID,
  document: Record<string, unknown>,
  args: Record<string, unknown> = {},
  overrides: { stage?: string; kind?: string; requestArgs?: Record<string, unknown> } = {},
) {
  const preset = singBoxProcessorPreset(id);
  const source = (preset.params?.source as Record<string, unknown> | undefined)?.content;
  expect(typeof source).toBe("string");
  const defaultArgs = preset.params?.args as Record<string, unknown>;
  const input = {
    stage: overrides.stage ?? "file",
    file: { kind: overrides.kind ?? "sing-box", content: JSON.stringify(document) },
    request: { args: overrides.requestArgs ?? {} },
    args: { ...defaultArgs, ...args },
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
