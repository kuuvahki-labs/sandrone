import { load } from "js-yaml";
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
  it("keeps baseline Sniffer and TUN out of the new-file processor chain", () => {
    expect(defaultMihomoProcessors(en).map((processor) => processor.name)).toEqual([
      "GitHub acceleration",
    ]);
  });

  it.each([["en-US", en], ["zh-CN", zh]] as const)("uses every preset label as its %s processor name", (_locale, t) => {
    for (const preset of mihomoProcessorPresets) {
      expect(preset.build(t).name).toBe(t(preset.labelKey));
    }
  });

  it("defines the complete editable YAML override contents", () => {
    const tailscale = presetYAML("tailscale-external");
    expect(tailscale).toEqual({
      dns: {
        "fake-ip-filter+": ["+.ts.net", "+.tailscale.com"],
        "nameserver-policy": { "<+.ts.net>": "100.100.100.100" },
      },
      tun: { "route-exclude-address+": ["100.64.0.0/10", "fd7a:115c:a1e0::/48"] },
      "+rules": [
        "DOMAIN-SUFFIX,tailscale.com,DIRECT",
        "IP-CIDR,100.64.0.0/10,DIRECT,no-resolve",
        "IP-CIDR6,fd7a:115c:a1e0::/48,DIRECT,no-resolve",
      ],
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
      "github-rule-source-mirror",
      "fake-ip-compat",
      "fake-ip-openclash",
      "fake-ip-shellcrash",
      "quic-fallback",
      "tailscale-external",
    ]);
    expect(presetDescriptor("github-rule-source-mirror")).toMatchObject({
      defaultOn: true,
      dependencies: [],
      conflicts: [],
    });
    expect(presetDescriptor("tailscale-external")).toMatchObject({
      defaultOn: false,
      dependencies: [],
      conflicts: [],
    });
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
    const before = mihomoProcessorPreset("quic-fallback");
    const stable = mihomoProcessorPreset("fake-ip-compat");
    const after = { name: "after", type: "script", stage: "file", params: { source: { type: "inline", content: "// after" } } } as const;
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
