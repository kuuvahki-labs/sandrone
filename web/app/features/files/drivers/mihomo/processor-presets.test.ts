import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import { recognizedFileProcessorPresetID } from "~/features/files/drivers/core/processor-presets";

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
  });

  it("declares the existing dependency chain in the managed catalog", () => {
    expect(mihomoProcessorPresets.map((preset) => preset.id)).toEqual([
      "sniffer",
      "tun",
      "ntp-direct",
      "fake-ip-compat",
      "tailscale-external",
      "tailnet-share",
    ]);
    expect(presetDescriptor("tailscale-external").dependencies).toEqual(["tun"]);
    expect(presetDescriptor("tailnet-share").dependencies).toEqual(["tun", "tailscale-external"]);
  });

  it("recognizes only exact YAML override content, not a surviving marker", () => {
    const preset = mihomoProcessorPreset("tailscale-external");
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, preset)).toBe("tailscale-external");
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, {
      ...preset,
      params: { ...preset.params, content: `${String(preset.params?.content)}\n# user edit` },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, { ...preset, type: "script" })).toBeNull();
    expect(recognizedFileProcessorPresetID(mihomoProcessorPresets, { ...preset, params: { ...preset.params, mode: "yaml_overlay" } })).toBeNull();
  });
});

function presetDescriptor(id: MihomoProcessorPresetID) {
  const descriptor = mihomoProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing Mihomo processor preset: ${id}`);
  return descriptor;
}

function presetYAML(id: MihomoProcessorPresetID): Record<string, unknown> {
  const content = mihomoProcessorPreset(id).params?.content;
  expect(typeof content).toBe("string");
  return load(String(content)) as Record<string, unknown>;
}
