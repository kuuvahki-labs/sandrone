import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { enUS } from "~/shared/i18n/translations/en-US";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  defaultMihomoProcessors,
  mihomoProcessorPreset,
  type MihomoProcessorPresetID,
  mihomoProcessorPresets,
} from "./processor-presets";

describe("Mihomo processor presets", () => {
  it("uses Sniffer, TUN, then traditional NTP direct as the new-file default chain", () => {
    expect(defaultMihomoProcessors().map((processor) => processor.name)).toEqual([
      "Sniffer",
      "TUN",
      "Traditional NTP Direct",
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

  it("defines the complete editable YAML override contents", () => {
    const sniffer = presetYAML("sniffer");
    expect(sniffer["sniffer!"]).toMatchObject({
      enable: true,
      "skip-domain": ["Mijia Cloud", "dlg.io.mi.com", "+.push.apple.com"],
    });

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
    expect(tailscale).toMatchObject({
      dns: {
        "fake-ip-filter+": ["+.tailscale.com", "+.ts.net"],
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
          "ntp1.aliyun.com",
          "ntp2.aliyun.com",
          "ntp3.aliyun.com",
          "ntp4.aliyun.com",
          "ntp5.aliyun.com",
          "ntp6.aliyun.com",
          "ntp7.aliyun.com",
          "time1.cloud.tencent.com",
          "time2.cloud.tencent.com",
          "time3.cloud.tencent.com",
          "time4.cloud.tencent.com",
          "time5.cloud.tencent.com",
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
          "localhost.*.weixin.qq.com",
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

  it("builds exact STUN and QUIC ordered-rule processors", () => {
    expect(mihomoProcessorPreset("stun-block")).toMatchObject({
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "stun-block",
          rules_json: JSON.stringify([
            "AND,((NETWORK,UDP),(DST-PORT,3478)),REJECT",
            "AND,((NETWORK,UDP),(DST-PORT,5349)),REJECT",
          ]),
        },
      },
    });
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
      "fake-ip-compat",
      "stun-block",
      "quic-fallback",
      "udp-p2p-eim",
      "linux-tun-acceleration",
      "windows-relaxed-route",
      "tailscale-external",
      "tailnet-share",
    ]);
    expect(presetDescriptor("tailscale-external").dependencies).toEqual(["tun"]);
    expect(presetDescriptor("tailnet-share").dependencies).toEqual(["tun", "tailscale-external"]);
    const scenarioIDs = [
      "stun-block",
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
      { id: "stun-block", defaultOn: false, dependencies: [], conflicts: ["udp-p2p-eim"] },
      { id: "quic-fallback", defaultOn: false, dependencies: [], conflicts: [] },
      { id: "udp-p2p-eim", defaultOn: false, dependencies: [], conflicts: ["stun-block"] },
      { id: "linux-tun-acceleration", defaultOn: false, dependencies: ["tun"], conflicts: [] },
      { id: "windows-relaxed-route", defaultOn: false, dependencies: [], conflicts: [] },
    ]);

    const stunPlan = planFileProcessorPresetAddition(
      mihomoProcessorPresets,
      "stun-block",
      [mihomoProcessorPreset("udp-p2p-eim")],
    );
    expect(stunPlan.removedPresetIDs).toEqual(["udp-p2p-eim"]);
    const eimPlan = planFileProcessorPresetAddition(
      mihomoProcessorPresets,
      "udp-p2p-eim",
      [mihomoProcessorPreset("stun-block")],
    );
    expect(eimPlan.removedPresetIDs).toEqual(["stun-block"]);
  });

  it("has no keepalive preset surface and never disables process lookup", () => {
    const managedSurface = mihomoProcessorPresets.map((preset) => ({
      id: preset.id,
      labelKey: preset.labelKey,
      labels: [enUS[preset.labelKey], zhCN[preset.labelKey]],
      processor: preset.build(),
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

  it.each(["stun-block", "quic-fallback"] as const)(
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

function presetDescriptor(id: MihomoProcessorPresetID) {
  const descriptor = mihomoProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing Mihomo processor preset: ${id}`);
  return descriptor;
}

function presetYAML(id: MihomoProcessorPresetID): Record<string, unknown> {
  return load(presetContent(id)) as Record<string, unknown>;
}

function presetContent(id: MihomoProcessorPresetID): string {
  const content = mihomoProcessorPreset(id).params?.content;
  expect(typeof content).toBe("string");
  return String(content);
}
