import { runInNewContext } from "node:vm";

import { dump, load } from "js-yaml";
import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { createTranslator } from "~/shared/i18n/context";
import { enUS } from "~/shared/i18n/translations/en-US";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  defaultMihomoProcessors,
  mihomoProcessorPreset as buildMihomoProcessorPreset,
  type MihomoProcessorPresetID,
  mihomoProcessorPresets,
} from "./processor-presets";

const en = createTranslator("en-US");
const zh = createTranslator("zh-CN");

describe("Mihomo processor presets", () => {
  it("uses Sniffer, traditional NTP direct, and the rule source mirror as the new-file default chain", () => {
    expect(defaultMihomoProcessors(en).map((processor) => processor.name)).toEqual([
      "Sniffer",
      "Traditional NTP Direct",
      "GitHub acceleration",
    ]);
    expect(mihomoProcessorPreset("ntp-direct")).toMatchObject({
      name: "Traditional NTP Direct",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "ntp-direct",
          rules_json: JSON.stringify(["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"]),
        },
      },
    });
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of mihomoProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
  });

  it("defines the complete editable YAML override contents", () => {
    const sniffer = presetYAML("sniffer");
    expect(sniffer["sniffer!"]).toMatchObject({
      enable: true,
      "override-destination": false,
      "skip-domain": ["Mijia Cloud", "dlg.io.mi.com", "+.push.apple.com"],
      sniff: {
        HTTP: { ports: [80, 8080, 8880] },
      },
    });
    expect(sniffer["sniffer!"]).not.toHaveProperty("force-dns-mapping");
    expect(sniffer["sniffer!"]).not.toHaveProperty("parse-pure-ip");

    const tun = presetYAML("tun")["tun!"] as Record<string, unknown>;
    expect(tun).toMatchObject({
      enable: true,
      stack: "mixed",
      "auto-route": true,
      "strict-route": true,
      "auto-detect-interface": true,
      "dns-hijack": ["any:53", "tcp://any:53"],
      "route-exclude-address": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "fe80::/10", "fc00::/7", "224.0.0.251/32", "ff02::fb/128"],
    });
    expect(tun).not.toHaveProperty("device");

    const tailscale = presetYAML("tailscale-external");
    expect(tailscale).toEqual({
      dns: {
        "fake-ip-filter+": ["+.ts.net"],
        "nameserver-policy": { "<+.ts.net>": "100.100.100.100" },
      },
      tun: { "route-exclude-address+": ["100.64.0.0/10", "fd7a:115c:a1e0::/48"] },
    });
    expect(presetYAML("tailnet-share")).toEqual({
      "lan-allowed-ips+": ["100.64.0.0/10", "fd7a:115c:a1e0::/48"],
    });
    expect(presetYAML("fake-ip-compat")).toEqual({
      dns: {
        "fake-ip-filter+": [
          "time-ios.apple.com",
          "time.*.gov",
          "time.*.edu.cn",
          "time.*.apple.com",
          "time1.*.com",
          "time2.*.com",
          "time3.*.com",
          "time4.*.com",
          "time5.*.com",
          "time6.*.com",
          "time7.*.com",
          "ntp1.*.com",
          "ntp2.*.com",
          "ntp3.*.com",
          "ntp4.*.com",
          "ntp5.*.com",
          "ntp6.*.com",
          "ntp7.*.com",
          "*.time.edu.cn",
          "*.ntp.org.cn",
          "ntp.ntsc.ac.cn",
          "mesu.apple.com",
          "swscan.apple.com",
          "swquery.apple.com",
          "swdownload.apple.com",
          "swcdn.apple.com",
          "swdist.apple.com",
          "music.163.com",
          "*.music.163.com",
          "y.qq.com",
          "*.y.qq.com",
          "streamoc.music.tc.qq.com",
          "mobileoc.music.tc.qq.com",
          "isure.stream.qqmusic.qq.com",
          "dl.stream.qqmusic.qq.com",
          "aqqmusic.tc.qq.com",
          "amobile.music.tc.qq.com",
          "songsearch.kugou.com",
          "trackercdn.kugou.com",
          "*.kuwo.cn",
          "music.migu.cn",
          "*.music.migu.cn",
          "*.mcdn.bilivideo.cn",
          "+.cmbchina.com",
          "+.cmbimg.com",
          "+.sandai.net",
          "+.n0808.com",
          "+.uu.163.com",
          "ps.res.netease.com",
          "+.oray.com",
          "+.orayimg.com",
        ],
      },
    });
    expect(presetYAML("fake-ip-openclash")).toEqual({
      "rule-providers": {
        "sandrone-fakeip-openclash": {
          type: "http",
          behavior: "domain",
          format: "text",
          path: "./ruleset/sandrone-fakeip-openclash.list",
          url: "https://cdn.jsdelivr.net/gh/vernesong/OpenClash@master/luci-app-openclash/root/etc/openclash/custom/openclash_custom_fake_filter.list",
          interval: 86400,
        },
      },
      dns: { "fake-ip-filter+": ["rule-set:sandrone-fakeip-openclash"] },
    });
    expect(presetYAML("fake-ip-shellcrash")).toEqual({
      "rule-providers": {
        "sandrone-fakeip-shellcrash": {
          type: "http",
          behavior: "domain",
          format: "text",
          path: "./ruleset/sandrone-fakeip-shellcrash.list",
          url: "https://cdn.jsdelivr.net/gh/juewuy/ShellCrash@dev/public/fake_ip_filter.list",
          interval: 86400,
        },
      },
      dns: { "fake-ip-filter+": ["rule-set:sandrone-fakeip-shellcrash"] },
    });

    expect(presetContent("udp-p2p-eim")).toBe(`# sandrone:mihomo-preset=udp-p2p-eim
tun:
  endpoint-independent-nat: true`);
    expect(presetYAML("udp-p2p-eim")).toEqual({
      tun: { "endpoint-independent-nat": true },
    });

    expect(presetContent("linux-tun-acceleration")).toBe(`# sandrone:mihomo-preset=linux-tun-acceleration
find-process-mode: strict
tun:
  auto-route: true
  auto-redirect: true`);
    expect(presetYAML("linux-tun-acceleration")).toEqual({
      "find-process-mode": "strict",
      tun: { "auto-route": true, "auto-redirect": true },
    });

    expect(presetContent("windows-relaxed-route")).toBe(`# sandrone:mihomo-preset=windows-relaxed-route
tun:
  strict-route: false`);
    expect(presetYAML("windows-relaxed-route")).toEqual({
      tun: { "strict-route": false },
    });
  });

  it("builds native Tailscale with editable auth_key and recognizes its managed params", () => {
    const id = "tailscale-native";
    const preset = mihomoProcessorPreset(id);

    expect(preset).toEqual({
      name: "Native Tailscale",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: { auth_key: "" },
      },
    });
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, preset)).toBe(id);

    const params = preset.params as Record<string, unknown>;
    const source = params.source as Record<string, unknown>;
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
      ...preset,
      params: { ...params, source: { ...source, content: `${String(source.content)}\n// user edit` } },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
      ...preset,
      params: { ...params, args: {} },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
      ...preset,
      params: { ...params, args: { auth_key: "tskey-auth-test" } },
    })).toBe(id);
  });

  it("applies native Tailscale atomically, exactly once, and before the safe generic anchor", () => {
    const original = {
      proxies: [
        { name: "TAILSCALE", type: "tailscale", ephemeral: false, udp: true, "accept-routes": false },
        { name: "Node", type: "ss", server: "example.com", port: 8388 },
        { name: "TAILSCALE", type: "tailscale", ephemeral: false, udp: true, "accept-routes": false },
      ],
      rules: [
        "DOMAIN,user.example,DIRECT",
        "DOMAIN-SUFFIX,ts.net,TAILSCALE",
        "DOMAIN-SUFFIX,ts.net,TAILSCALE",
        "RULE-SET,private,DIRECT",
        "MATCH,LockedFinal",
      ],
      dns: {
        "fake-ip-filter": ["+.ts.net", "base", "+.ts.net"],
        "nameserver-policy": { "existing.example": "system" },
      },
      tun: {
        "route-exclude-address": [
          "192.0.2.0/24",
          "100.64.0.0/10",
          "fd7a:115c:a1e0::/48",
          "100.64.0.0/10",
        ],
      },
    };

    const first = runNativeTailscale(original, "tskey-auth-test");
    expect(first.stringifyCalls).toBe(1);
    expect(first.document.proxies).toEqual([
      { name: "TAILSCALE", type: "tailscale", "auth-key": "tskey-auth-test", ephemeral: false, udp: true, "accept-routes": false },
      original.proxies[1],
    ]);
    expect(first.document.rules).toEqual([
      "DOMAIN,user.example,DIRECT",
      "DOMAIN-SUFFIX,ts.net,TAILSCALE",
      "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
      "IP-CIDR6,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
      "RULE-SET,private,DIRECT",
      "MATCH,LockedFinal",
    ]);
    expect(first.document.dns).toEqual({
      "fake-ip-filter": ["+.ts.net", "base"],
      "nameserver-policy": {
        "existing.example": "system",
        "+.ts.net": "100.100.100.100",
      },
    });
    expect(first.document.tun).toEqual({
      "route-exclude-address": ["192.0.2.0/24"],
    });

    const second = runNativeTailscale(first.document, "tskey-auth-test");
    expect(second.document).toEqual(first.document);
    expect(second.stringifyCalls).toBe(1);
  });

  it("rejects incompatible native structures before stringify or content assignment", () => {
    const cases = [
      {
        name: "invalid proxies",
        document: { proxies: "invalid", rules: ["MATCH,LockedFinal"], dns: {}, tun: {} },
        error: "Sandrone Mihomo Tailscale native preset requires proxies to be an array of objects",
      },
      {
        name: "invalid rules",
        document: { proxies: [], rules: ["MATCH,LockedFinal", { custom: true }], dns: {}, tun: {} },
        error: "Sandrone Mihomo Tailscale native preset requires rules to be an array of strings",
      },
      {
        name: "invalid DNS",
        document: { proxies: [], rules: ["MATCH,LockedFinal"], dns: [], tun: {} },
        error: "Sandrone Mihomo Tailscale native preset requires dns to be an object",
      },
      {
        name: "invalid TUN",
        document: { proxies: [], rules: ["MATCH,LockedFinal"], dns: {}, tun: [] },
        error: "Sandrone Mihomo Tailscale native preset requires tun to be an object",
      },
      {
        name: "same-name proxy with extra fields",
        document: {
          proxies: [{ name: "TAILSCALE", type: "tailscale", ephemeral: false, udp: true, "accept-routes": false, hostname: "custom" }],
          rules: ["MATCH,LockedFinal"],
          dns: {},
          tun: {},
        },
        error: "Sandrone Mihomo Tailscale native preset found incompatible proxy named TAILSCALE",
      },
      {
        name: "invalid DNS filter",
        document: {
          proxies: [],
          rules: ["MATCH,LockedFinal"],
          dns: { "fake-ip-filter": "invalid" },
          tun: {},
        },
        error: "Sandrone Mihomo Tailscale native preset requires dns.fake-ip-filter to be an array of strings",
      },
      {
        name: "invalid DNS policy",
        document: {
          proxies: [],
          rules: ["MATCH,LockedFinal"],
          dns: { "nameserver-policy": [] },
          tun: {},
        },
        error: "Sandrone Mihomo Tailscale native preset requires dns.nameserver-policy to be an object",
      },
      {
        name: "invalid route exclusions",
        document: {
          proxies: [],
          rules: ["MATCH,LockedFinal"],
          dns: {},
          tun: { "route-exclude-address": ["192.0.2.0/24", false] },
        },
        error: "Sandrone Mihomo Tailscale native preset requires tun.route-exclude-address to be an array of strings",
      },
      {
        name: "missing safe rule anchor",
        document: {
          proxies: [],
          rules: ["DOMAIN,user.example,DIRECT"],
          dns: {},
          tun: {},
        },
        error: "Sandrone preset tailscale-native cannot find a safe mihomo rule anchor",
      },
    ];

    for (const test of cases) {
      const execution = prepareNativeTailscale(test.document);
      const before = execution.input.file.content;
      expect(execution.run, test.name).toThrowError(test.error);
      expect(execution.input.file.content).toBe(before);
      expect(execution.stringifyCalls()).toBe(0);
    }

    const stringifyFailure = prepareNativeTailscale(
      { proxies: [], rules: ["MATCH,LockedFinal"], dns: {}, tun: {} },
      () => {
        throw new Error("stringify failed");
      },
    );
    const before = stringifyFailure.input.file.content;
    expect(stringifyFailure.run).toThrowError("stringify failed");
    expect(stringifyFailure.input.file.content).toBe(before);
    expect(stringifyFailure.stringifyCalls()).toBe(1);
  });

  it("builds the exact QUIC ordered-rule processor", () => {
    expect(mihomoProcessorPreset("quic-fallback")).toMatchObject({
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "quic-fallback",
          rules_json: JSON.stringify([
            "AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
          ]),
        },
      },
    });
  });

  it("declares the complete dependency, conflict, and default matrix", () => {
    expect(mihomoProcessorPresets.map((preset) => preset.id)).toEqual([
      "sniffer",
      "tun",
      "ntp-direct",
      "github-rule-source-mirror",
      "fake-ip-compat",
      "fake-ip-openclash",
      "fake-ip-shellcrash",
      "quic-fallback",
      "udp-p2p-eim",
      "linux-tun-acceleration",
      "windows-relaxed-route",
      "tailscale-native",
      "tailscale-external",
      "tailnet-share",
    ]);
    expect(presetDescriptor("tailscale-native")).toMatchObject({
      defaultOn: false,
      dependencies: ["tun"],
      conflicts: ["tailscale-external"],
    });
    expect(presetDescriptor("github-rule-source-mirror")).toMatchObject({
      defaultOn: true,
      dependencies: [],
      conflicts: [],
    });
    expect(presetDescriptor("tailscale-external")).toMatchObject({
      defaultOn: false,
      dependencies: ["tun"],
      conflicts: ["tailscale-native"],
    });
    expect(presetDescriptor("tailscale-external").dependencies).toEqual(["tun"]);
    expect(presetDescriptor("tailnet-share").dependencies).toEqual(["tun", "tailscale-external"]);
    expect([
      "fake-ip-compat",
      "fake-ip-openclash",
      "fake-ip-shellcrash",
    ].map((id) => {
      const preset = presetDescriptor(id as MihomoProcessorPresetID);
      return { id, defaultOn: preset.defaultOn, conflicts: preset.conflicts };
    })).toEqual([
      {
        id: "fake-ip-compat",
        defaultOn: false,
        conflicts: ["fake-ip-openclash", "fake-ip-shellcrash"],
      },
      {
        id: "fake-ip-openclash",
        defaultOn: false,
        conflicts: ["fake-ip-compat", "fake-ip-shellcrash"],
      },
      {
        id: "fake-ip-shellcrash",
        defaultOn: false,
        conflicts: ["fake-ip-compat", "fake-ip-openclash"],
      },
    ]);
    const scenarioIDs = [
      "quic-fallback",
      "udp-p2p-eim",
      "linux-tun-acceleration",
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
      { id: "quic-fallback", defaultOn: false, dependencies: [], conflicts: [] },
      { id: "udp-p2p-eim", defaultOn: false, dependencies: [], conflicts: [] },
      { id: "linux-tun-acceleration", defaultOn: false, dependencies: ["tun"], conflicts: [] },
      { id: "windows-relaxed-route", defaultOn: false, dependencies: [], conflicts: [] },
    ]);

    const editedTailnetShare = mihomoProcessorPreset("tailnet-share");
    editedTailnetShare.params = {
      ...editedTailnetShare.params,
      content: `${String(editedTailnetShare.params?.content)}\n# user edit`,
    };
    const nativeCurrent = [
      mihomoProcessorPreset("tailscale-external"),
      mihomoProcessorPreset("tailnet-share"),
      editedTailnetShare,
    ];
    const nativePlan = planFileProcessorPresetAddition(
      mihomoProcessorPresets,
      "tailscale-native",
      nativeCurrent,
      en,
    );
    expect(nativePlan.removeIndices).toEqual([0, 1]);
    expect(nativePlan.removedPresetIDs).toEqual(["tailscale-external", "tailnet-share"]);
    const nativeRemovals = new Set(nativePlan.removeIndices);
    const nativeSurvivors = nativeCurrent.filter((_, index) => !nativeRemovals.has(index));
    expect(nativeSurvivors).toEqual([editedTailnetShare]);
    expect(nativeSurvivors[0]).toBe(editedTailnetShare);
  });

  it("has no keepalive preset surface and never disables process lookup", () => {
    const managedSurface = mihomoProcessorPresets.map((preset) => ({
      id: preset.id,
      labelKey: preset.labelKey,
      labels: [enUS[preset.labelKey], zhCN[preset.labelKey]],
      processor: preset.build(en),
    }));
    const serialized = JSON.stringify(managedSurface).toLowerCase();

    expect(serialized).not.toContain("keepalive");
    expect(serialized).not.toContain("find-process-mode: off");
  });

  it.each([
    "tailscale-external",
    "udp-p2p-eim",
    "linux-tun-acceleration",
    "windows-relaxed-route",
  ] as const)("recognizes only exact YAML override content for %s", (id) => {
    const preset = mihomoProcessorPreset(id);
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, preset)).toBe(id);
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
      ...preset,
      params: { ...preset.params, content: `${String(preset.params?.content)}\n# user edit` },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, { ...preset, type: "script" })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, { ...preset, params: { ...preset.params, mode: "yaml_overlay" } })).toBeNull();
  });

  it.each([
    "fake-ip-compat",
    "fake-ip-openclash",
    "fake-ip-shellcrash",
  ] as const)("recognizes and explicitly refreshes an older managed %s snapshot", (id) => {
    const preset = mihomoProcessorPreset(id);
    const marker = `# sandrone:mihomo-preset=${id}`;
    const stale = {
      ...preset,
      params: { mode: "yaml_override", content: `${marker}\ndns:\n  fake-ip-filter+: []` },
    };

    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, stale)).toBe(id);
    const plan = planFileProcessorPresetAddition(mihomoProcessorPresets, id, [stale], en);
    expect(plan.removeIndices).toEqual([0]);
    expect(plan.updatedPresetIDs).toEqual([id]);
    expect(plan.removedPresetIDs).toEqual([]);
    expect(plan.additions).toMatchObject([{ presetID: id, beforeIndex: 0 }]);
    expect(plan.additions[0]?.processor).toEqual(preset);
  });

  it("switches Fake-IP sources atomically while preserving unrelated processors", () => {
    const before = mihomoProcessorPreset("sniffer");
    const stable = mihomoProcessorPreset("fake-ip-compat");
    const after = mihomoProcessorPreset("tun");
    const plan = planFileProcessorPresetAddition(
      mihomoProcessorPresets,
      "fake-ip-shellcrash",
      [before, stable, after],
      en,
    );

    expect(plan.removeIndices).toEqual([1]);
    expect(plan.removedPresetIDs).toEqual(["fake-ip-compat"]);
    expect(plan.updatedPresetIDs).toEqual([]);
    expect(plan.addedPresetIDs).toEqual(["fake-ip-shellcrash"]);
    expect(plan.additions[0]?.beforeIndex).toBe(1);
  });

  it.each(["quic-fallback"] as const)(
    "recognizes only the exact ordered-rule processor for %s",
    (id) => {
      const preset = mihomoProcessorPreset(id);
      expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, preset)).toBe(id);
      const params = preset.params as Record<string, unknown>;
      const args = params.args as Record<string, unknown>;
      expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
        ...preset,
        params: { ...params, args: { ...args, rules_json: "[]" } },
      })).toBeNull();
    },
  );
});

function presetDescriptor(id: MihomoProcessorPresetID | "github-rule-source-mirror") {
  const descriptor = mihomoProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing Mihomo processor preset: ${id}`);
  return descriptor;
}

function mihomoProcessorPreset(id: MihomoProcessorPresetID) {
  const preset = presetDescriptor(id);
  return buildMihomoProcessorPreset(id, en(preset.labelKey));
}

function presetYAML(id: MihomoProcessorPresetID): Record<string, unknown> {
  return load(presetContent(id)) as Record<string, unknown>;
}

function presetContent(id: MihomoProcessorPresetID): string {
  const content = mihomoProcessorPreset(id).params?.content;
  expect(typeof content).toBe("string");
  return String(content);
}

function runNativeTailscale(document: Record<string, unknown>, authKey = "") {
  const execution = prepareNativeTailscale(document, undefined, authKey);
  execution.run();
  return {
    document: load(execution.input.file.content) as Record<string, unknown>,
    stringifyCalls: execution.stringifyCalls(),
  };
}

function prepareNativeTailscale(
  document: Record<string, unknown>,
  stringify = (value: unknown) => dump(JSON.parse(JSON.stringify(value))),
  authKey = "",
) {
  const preset = mihomoProcessorPreset("tailscale-native");
  const source = (preset.params?.source as Record<string, unknown> | undefined)?.content;
  expect(typeof source).toBe("string");
  const input = { file: { content: dump(document) }, args: { auth_key: authKey } };
  let stringifyCalls = 0;
  const api = {
    yaml: {
      parse: load,
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
