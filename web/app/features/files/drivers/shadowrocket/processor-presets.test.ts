import { describe, expect, it } from "vitest";

import {
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "~/features/files/drivers/core/processor-presets";
import { enUS } from "~/shared/i18n/translations/en-US";
import { zhCN } from "~/shared/i18n/translations/zh-CN";

import {
  defaultShadowrocketProcessors,
  shadowrocketProcessorPresets,
} from "./processor-presets";

describe("Shadowrocket processor presets", () => {
  it("uses traditional NTP direct as the only new-file default", () => {
    const processors = defaultShadowrocketProcessors();

    expect(processors.map((processor) => processor.name)).toEqual(["Traditional NTP Direct"]);
    expect(processors[0]).toMatchObject({
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "ntp-direct",
          rules_json: JSON.stringify(["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"]),
        },
      },
    });
  });

  it("recognizes only the exact managed NTP script", () => {
    const preset = defaultShadowrocketProcessors()[0]!;

    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, preset)).toBe("ntp-direct");
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      params: {
        ...preset.params,
        args: { preset_id: "ntp-direct", rules_json: "[]" },
      },
    })).toBeNull();
  });

  it("builds the exact optional INI override scenarios", () => {
    expect(Object.fromEntries(SCENARIOS.map(({ id }) => {
      const processor = presetDescriptor(id).build();
      return [id, {
        type: processor.type,
        stage: processor.stage,
        params: processor.params,
      }];
    }))).toEqual(Object.fromEntries(SCENARIOS.map(({ id, content }) => [id, {
      type: "merge",
      stage: "file",
      params: { mode: "ini_override", content },
    }])));
  });

  it("declares every optional scenario default off without dependencies or conflicts", () => {
    expect(shadowrocketProcessorPresets.map((descriptor) => descriptor.id)).toEqual([
      "ntp-direct",
      "webrtc-privacy",
      "disable-ipv6",
      "udp-unsupported-direct",
      "restricted-network-dns-fallback",
      "tailscale-native",
    ]);
    expect(SCENARIOS.map(({ id }) => {
      const descriptor = presetDescriptor(id);
      return {
        id,
        defaultOn: descriptor.defaultOn,
        dependencies: descriptor.dependencies,
        conflicts: descriptor.conflicts,
      };
    })).toEqual(SCENARIOS.map(({ id }) => ({
      id,
      defaultOn: false,
      dependencies: [],
      conflicts: id === "webrtc-privacy" ? ["tailscale-native"] : [],
    })));
    expect(defaultShadowrocketProcessors().map((processor) => processor.name)).toEqual([
      "Traditional NTP Direct",
    ]);
  });

  it("builds the exact native Tailscale ordered rules without a module warning", () => {
    const descriptor = presetDescriptor("tailscale-native");
    const processor = descriptor.build();

    expect(processor).toEqual({
      name: "Tailscale 原生接管",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline", content: expect.any(String) },
        args: {
          preset_id: "tailscale-native",
          rules_json: JSON.stringify([
            "DOMAIN-SUFFIX,ts.net,TAILSCALE",
            "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
            "IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
          ]),
        },
      },
    });
    expect(descriptor).toMatchObject({
      category: "tailscale",
      defaultOn: false,
      dependencies: [],
      conflicts: ["webrtc-privacy"],
    });
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, processor))
      .toBe("tailscale-native");
    expect(`${enUS[descriptor.descriptionKey]} ${enUS[descriptor.riskKey!]}`.toLowerCase())
      .not.toContain("module");
    expect(`${zhCN[descriptor.descriptionKey]} ${zhCN[descriptor.riskKey!]}`)
      .not.toContain("模块");
  });

  it("replaces managed WebRTC privacy and Tailscale reciprocally but preserves edits", () => {
    const tailscale = presetDescriptor("tailscale-native").build();
    const privacy = presetDescriptor("webrtc-privacy").build();
    const editedPrivacy = {
      ...privacy,
      params: { ...privacy.params, content: `${String(privacy.params?.content)}\n# user edit` },
    };
    expect(presetDescriptor("webrtc-privacy").conflicts).toEqual(["tailscale-native"]);

    const nativePlan = planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "tailscale-native",
      [editedPrivacy, privacy],
    );
    expect(nativePlan.removeIndices).toEqual([1]);
    expect(nativePlan.removedPresetIDs).toEqual(["webrtc-privacy"]);
    expect(nativePlan.additions).toEqual([tailscale]);

    const stunPlan = planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "webrtc-privacy",
      [tailscale],
    );
    expect(stunPlan.removedPresetIDs).toEqual(["tailscale-native"]);

    expect(planFileProcessorPresetAddition(
      shadowrocketProcessorPresets,
      "tailscale-native",
      [tailscale],
    ).additions).toEqual([]);
  });

  it.each(SCENARIOS)("recognizes only the exact INI override for $id", ({ id }) => {
    const preset = presetDescriptor(id).build();

    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, preset)).toBe(id);
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      params: { ...preset.params, content: `${String(preset.params?.content)}\n# user edit` },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      params: { ...preset.params, mode: "append" },
    })).toBeNull();
    expect(recognizedFileProcessorPresetID(shadowrocketProcessorPresets, {
      ...preset,
      type: "script",
    })).toBeNull();
  });

  it("uses the fixed risk copy and exposes no Shadowrocket QUIC preset surface", () => {
    expect(zhCN["processors.filePreset.shadowrocket.webrtcPrivacy.risk"]).toBe(
      "可能导致语音通话、视频会议、WebRTC 或 P2P 连接降级或失效。",
    );
    expect(enUS["processors.filePreset.shadowrocket.disableIPv6.risk"]).toBe(
      "This controls only the IPv6 behavior expressible in Shadowrocket configuration and does not guarantee that underlying node transport never uses IPv6.",
    );
    expect(enUS["processors.filePreset.shadowrocket.udpUnsupportedDirect.risk"]).toBe(
      "Matching traffic bypasses the proxy, so the real egress address, carrier path, and local DNS may be exposed.",
    );
    expect(enUS["processors.filePreset.shadowrocket.restrictedNetworkDNSFallback.risk"]).toBe(
      "Domains intended for direct resolution may instead be resolved through the proxy.",
    );

    const surface = shadowrocketProcessorPresets.map((preset) => ({
      id: preset.id,
      labelKey: preset.labelKey,
      labelEN: enUS[preset.labelKey],
      labelZH: zhCN[preset.labelKey],
      processor: preset.build(),
    }));
    expect(JSON.stringify(surface).toLowerCase()).not.toContain("quic");
  });
});

const SCENARIOS = [
  {
    id: "webrtc-privacy",
    content: `# sandrone:shadowrocket-preset=webrtc-privacy
[General]
stun-response-ip = 1.1.1.1
stun-response-ipv6 = ::1`,
  },
  {
    id: "disable-ipv6",
    content: `# sandrone:shadowrocket-preset=disable-ipv6
[General]
ipv6 = false
prefer-ipv6 = false`,
  },
  {
    id: "udp-unsupported-direct",
    content: `# sandrone:shadowrocket-preset=udp-unsupported-direct
[General]
udp-policy-not-supported-behaviour = DIRECT`,
  },
  {
    id: "restricted-network-dns-fallback",
    content: `# sandrone:shadowrocket-preset=restricted-network-dns-fallback
[General]
dns-direct-fallback-proxy = true`,
  },
] as const;

function presetDescriptor(id: string) {
  const descriptor = shadowrocketProcessorPresets.find((preset) => preset.id === id);
  if (!descriptor) throw new Error(`missing Shadowrocket processor preset: ${id}`);
  return descriptor;
}
