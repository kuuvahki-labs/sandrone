import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { createTranslator } from "~/shared/i18n/context";

import legacyOutboundAdapterScript from "./legacy/sing-box-outbound-adapter-reject-empty.js?raw";
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
      "tailscale-external",
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
    const id = "tailscale-external";
    const preset = singBoxProcessorPreset(id);
    expect(preset).toEqual({
      name: "Tailscale coexistence",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: { preset_id: id },
      },
    });
    expect(recognizedFileProcessorPresetID(singBoxProcessorPresets, preset)).toBe(id);
    expect(presetDescriptor(id)).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: [],
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
      route: {
        final: "LockedRouteFinal",
        rules: [
          { domain_suffix: ["tailscale.com"], outbound: "direct" },
          { ip_cidr: ["100.64.0.0/10", "fd7a:115c:a1e0::/48"], outbound: "direct" },
          { outbound: "LockedFinal" },
        ],
      },
    });
    expect(first.document.endpoints).toEqual(original.endpoints);

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
      route: { rules: [{ outbound: "LockedFinal" }] },
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
    expect(first.document.route).toEqual({
      rules: [
        { domain_suffix: ["tailscale.com"], outbound: "direct" },
        { ip_cidr: ["100.64.0.0/10", "fd7a:115c:a1e0::/48"], outbound: "direct" },
        { outbound: "LockedFinal" },
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
      route: { rules: [{ outbound: "LockedFinal" }] },
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
      route: { rules: [{ outbound: "LockedFinal" }] },
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
      route: { rules: [{ outbound: "LockedFinal" }] },
    });
    const before = execution.input.file.content;
    expect(execution.run).toThrowError("requires a tagged real default DNS server");
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(0);
  });

  it("rejects incompatible external Tailscale tags without changing the file", () => {
    const base = {
      dns: { servers: [], rules: [], final: "LockedDNSFinal" },
      inbounds: [{ type: "tun", tag: "tun-in", route_exclude_address: [] }],
      endpoints: [],
      route: { final: "LockedRouteFinal", rules: [{ outbound: "LockedFinal" }] },
    };
    const cases = [
      {
        document: { ...base, inbounds: [{ type: "tun", tag: "first" }, { type: "tun", tag: "second" }] },
        error: "Sandrone sing-box Tailscale preset found ambiguous TUN inbounds",
      },
      {
        document: { ...base, endpoints: [{ type: "tailscale", tag: "ts-ep", ephemeral: false, accept_routes: false }] },
        error: "Sandrone sing-box Tailscale external preset found incompatible endpoint tag ts-ep",
      },
      {
        document: { ...base, dns: { ...base.dns, servers: [{ type: "tailscale", tag: "ts-dns", endpoint: "ts-ep" }] } },
        error: "Sandrone sing-box Tailscale external preset found incompatible DNS server tag ts-dns",
      },
      {
        document: { ...base, outbounds: [{ type: "direct", tag: "ts-ep" }] },
        error: "Sandrone sing-box Tailscale external preset found incompatible outbound tag ts-ep",
      },
    ];

    for (const test of cases) {
      const execution = prepareTailscale("tailscale-external", test.document);
      const before = execution.input.file.content;
      expect(execution.run).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }
  });

  it("validates external Tailscale target shapes before serializing and assigns only after stringify", () => {
    const base = {
      dns: { servers: [], rules: [], final: "LockedDNSFinal" },
      inbounds: [{ type: "tun", tag: "tun-in", route_exclude_address: [] }],
      endpoints: [],
      route: { final: "LockedRouteFinal", rules: [{ outbound: "LockedFinal" }] },
    };
    const cases = [
      { document: { ...base, inbounds: "invalid" }, error: "requires inbounds to be an array of objects" },
      { document: { ...base, dns: [] }, error: "requires dns to be an object" },
      { document: { ...base, dns: { servers: "invalid", rules: [] } }, error: "requires dns.servers to be an array of objects" },
      { document: { ...base, endpoints: "invalid" }, error: "requires endpoints to be an array of objects" },
      { document: { ...base, route: [] }, error: "requires route to be an object" },
      { document: { ...base, route: { final: "LockedRouteFinal", rules: "invalid" } }, error: "requires route.rules to be an array of objects" },
    ];

    for (const test of cases) {
      const execution = prepareTailscale("tailscale-external", test.document as Record<string, unknown>);
      const before = execution.input.file.content;
      expect(execution.run).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }

    const execution = prepareTailscale("tailscale-external", base, () => {
      throw new Error("stringify failed");
    });
    const before = execution.input.file.content;
    expect(execution.run).toThrowError("stringify failed");
    expect(execution.input.file.content).toBe(before);
    expect(execution.stringifyCalls()).toBe(1);
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

  it("recognizes managed scripts with common execution params but not edited sources or invalid business args", () => {
    for (const id of ["tailscale-external", "fakeip-compat", "fakeip-ruleset-geodata"] as const) {
      const preset = singBoxProcessorPreset(id);
      const descriptor = presetDescriptor(id);
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, timeout_ms: 5000 } })).toBe(true);
      const source = preset.params?.source as Record<string, unknown>;
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, source: { ...source, content: `${String(source.content)}\n// edited` } } })).toBe(false);
      expect(descriptor.recognize({ ...preset, params: { ...preset.params, args: { preset_id: id } } })).toBe(id === "tailscale-external");
    }
  });

  it("rejects managed request overrides and wrong execution envelopes before parsing content", () => {
    const managed = [
      ["tailscale-external", "preset_id"],
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

type TailscalePresetID = "tailscale-external";

function runTailscale(
  id: TailscalePresetID,
  document: Record<string, unknown>,
): { document: Record<string, unknown>; stringifyCalls: number } {
  const execution = prepareTailscale(id, document, JSON.stringify);
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
  const input = {
    stage: "file",
    file: { kind: "sing-box", content: JSON.stringify(document) },
    request: { args: {} },
    args: { preset_id: id },
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
  | "tailscale-external"
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
