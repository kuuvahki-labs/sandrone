import type {
  FileProcessorPreset,
  FileProcessorPresetCategory,
} from "~/features/files/drivers/core/processor-presets";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import type { ProcessorDetail } from "~/shared/resources/types";

type ShadowrocketINIOverridePresetID =
  | "webrtc-privacy"
  | "disable-ipv6"
  | "udp-unsupported-direct"
  | "restricted-network-dns-fallback";

const PRESET_CONTENT: Record<ShadowrocketINIOverridePresetID, string> = {
  "webrtc-privacy": `# sandrone:shadowrocket-preset=webrtc-privacy
[General]
stun-response-ip = 1.1.1.1
stun-response-ipv6 = ::1`,
  "disable-ipv6": `# sandrone:shadowrocket-preset=disable-ipv6
[General]
ipv6 = false
prefer-ipv6 = false`,
  "udp-unsupported-direct": `# sandrone:shadowrocket-preset=udp-unsupported-direct
[General]
udp-policy-not-supported-behaviour = DIRECT`,
  "restricted-network-dns-fallback": `# sandrone:shadowrocket-preset=restricted-network-dns-fallback
[General]
dns-direct-fallback-proxy = true`,
};

const PRESET_NAMES: Record<ShadowrocketINIOverridePresetID, string> = {
  "webrtc-privacy": "WebRTC Privacy",
  "disable-ipv6": "Disable IPv6",
  "udp-unsupported-direct": "UDP Unsupported Direct",
  "restricted-network-dns-fallback": "Restricted Network DNS Fallback",
};

const NTP_DIRECT_PRESET: OrderedRuleProcessorPresetOptions = {
  id: "ntp-direct",
  kind: "shadowrocket",
  name: "Traditional NTP Direct",
  rules: ["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"],
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
    build: () => orderedRuleProcessorPreset(NTP_DIRECT_PRESET),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, NTP_DIRECT_PRESET),
  },
  iniOverrideDescriptor(
    "webrtc-privacy",
    "privacy",
    "processors.filePreset.shadowrocket.webrtcPrivacy.label",
    "processors.filePreset.shadowrocket.webrtcPrivacy.description",
    "processors.filePreset.shadowrocket.webrtcPrivacy.risk",
  ),
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
];

export function defaultShadowrocketProcessors(): ProcessorDetail[] {
  return shadowrocketProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build());
}

function iniOverrideDescriptor(
  id: ShadowrocketINIOverridePresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
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
    conflicts: [],
    build: () => ({
      name: PRESET_NAMES[id],
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
