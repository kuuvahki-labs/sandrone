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
      { id: "stun-block", defaultOn: false, dependencies: ["sniff"], conflicts: ["udp-p2p-eim"] },
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
