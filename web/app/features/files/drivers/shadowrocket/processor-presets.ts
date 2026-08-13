import type {
  FileProcessorPreset,
  FileProcessorPresetCategory,
} from "~/features/files/drivers/core/processor-presets";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

import disableIPv6Content from "./preset-content/disable-ipv6.ini?raw";
import restrictedNetworkDNSFallbackContent from "./preset-content/restricted-network-dns-fallback.ini?raw";
import udpUnsupportedDirectContent from "./preset-content/udp-unsupported-direct.ini?raw";

type ShadowrocketINIOverridePresetID =
  | "disable-ipv6"
  | "udp-unsupported-direct"
  | "restricted-network-dns-fallback";

export type ShadowrocketProcessorPresetID =
  | "ntp-direct"
  | ShadowrocketINIOverridePresetID
  | "tailscale-native";

const PRESET_CONTENT: Record<ShadowrocketINIOverridePresetID, string> = {
  "disable-ipv6": withoutTrailingNewline(disableIPv6Content),
  "udp-unsupported-direct": withoutTrailingNewline(udpUnsupportedDirectContent),
  "restricted-network-dns-fallback": withoutTrailingNewline(restrictedNetworkDNSFallbackContent),
};

function withoutTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

const NTP_DIRECT_PRESET: OrderedRuleProcessorPresetOptions = {
  id: "ntp-direct",
  kind: "shadowrocket",
  rules: ["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"],
};

const TAILSCALE_NATIVE_PRESET: OrderedRuleProcessorPresetOptions = {
  id: "tailscale-native",
  kind: "shadowrocket",
  insertMode: "top",
  rules: [
    "DOMAIN-SUFFIX,ts.net,TAILSCALE",
    "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
    "IP-CIDR,fd7a:115c:a1e0::/48,TAILSCALE,no-resolve",
  ],
};

export const shadowrocketProcessorPresets: readonly FileProcessorPreset[] = [
  {
    id: NTP_DIRECT_PRESET.id,
    category: "network",
    labelKey: "processors.filePreset.ntpDirect.label",
    descriptionKey: "processors.filePreset.ntpDirect.description",
    riskKey: "processors.filePreset.ntpDirect.risk",
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: (t) => orderedRuleProcessorPreset(NTP_DIRECT_PRESET, t("processors.filePreset.ntpDirect.label")),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, NTP_DIRECT_PRESET),
  },
  githubRuleSourceMirrorPreset,
  iniOverrideDescriptor(
    "disable-ipv6",
    "network",
    "processors.filePreset.shadowrocket.disableIPv6.label",
    "processors.filePreset.shadowrocket.disableIPv6.description",
    "processors.filePreset.shadowrocket.disableIPv6.risk",
  ),
  iniOverrideDescriptor(
    "udp-unsupported-direct",
    "network",
    "processors.filePreset.shadowrocket.udpUnsupportedDirect.label",
    "processors.filePreset.shadowrocket.udpUnsupportedDirect.description",
    "processors.filePreset.shadowrocket.udpUnsupportedDirect.risk",
  ),
  iniOverrideDescriptor(
    "restricted-network-dns-fallback",
    "network",
    "processors.filePreset.shadowrocket.restrictedNetworkDNSFallback.label",
    "processors.filePreset.shadowrocket.restrictedNetworkDNSFallback.description",
    "processors.filePreset.shadowrocket.restrictedNetworkDNSFallback.risk",
  ),
  {
    id: TAILSCALE_NATIVE_PRESET.id,
    category: "tailscale",
    labelKey: "processors.filePreset.shadowrocket.tailscaleNative.label",
    descriptionKey: "processors.filePreset.shadowrocket.tailscaleNative.description",
    riskKey: "processors.filePreset.shadowrocket.tailscaleNative.risk",
    defaultOn: false,
    dependencies: [],
    conflicts: [],
    build: (t) => orderedRuleProcessorPreset(TAILSCALE_NATIVE_PRESET, t("processors.filePreset.shadowrocket.tailscaleNative.label")),
    recognize: (processor) => (
      recognizeOrderedRuleProcessorPreset(processor, TAILSCALE_NATIVE_PRESET)
    ),
  },
];

export function defaultShadowrocketProcessors(t: Translator): ProcessorDetail[] {
  return shadowrocketProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build(t));
}

function iniOverrideDescriptor(
  id: ShadowrocketINIOverridePresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  conflicts: readonly ShadowrocketProcessorPresetID[] = [],
): FileProcessorPreset {
  const content = PRESET_CONTENT[id];
  return {
    id,
    category,
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn: false,
    dependencies: [],
    conflicts,
    build: (t) => ({
      name: t(labelKey),
      type: "merge",
      stage: "file",
      params: { mode: "ini_override", content },
    }),
    recognize: (processor) => (
      processor.type === "merge"
      && processor.params?.mode === "ini_override"
      && processor.params.content === content
    ),
  };
}
