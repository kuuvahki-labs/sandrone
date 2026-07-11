import { load } from "js-yaml";
import { describe, expect, it } from "vitest";

import {
  defaultMihomoProcessors,
  mihomoProcessorPreset,
  recognizeMihomoProcessorPreset,
} from "./processor-presets";

describe("Mihomo processor presets", () => {
  it("uses Sniffer then TUN as the new-file default chain", () => {
    expect(defaultMihomoProcessors().map((processor) => processor.name)).toEqual(["Sniffer", "TUN"]);
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
      "auto-detect-interface": true,
      "dns-hijack": ["any:53", "tcp://any:53"],
      "route-exclude-address": ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "fe80::/10", "fc00::/7"],
    });
    expect(tun).not.toHaveProperty("device");

    const tailscale = presetYAML("tailscale");
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
  });

  it("recognizes only marked YAML overrides", () => {
    const preset = mihomoProcessorPreset("tailscale");
    expect(recognizeMihomoProcessorPreset(preset)).toBe("tailscale");
    expect(recognizeMihomoProcessorPreset({
      ...preset,
      params: { ...preset.params, content: "dns:\n  enable: true" },
    })).toBeNull();
    expect(recognizeMihomoProcessorPreset({ ...preset, params: { ...preset.params, mode: "yaml_overlay" } })).toBeNull();
  });
});

function presetYAML(id: "sniffer" | "tun" | "tailscale" | "tailnet-share"): Record<string, unknown> {
  const content = mihomoProcessorPreset(id).params?.content;
  expect(typeof content).toBe("string");
  return load(String(content)) as Record<string, unknown>;
}
