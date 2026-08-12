import type {
  FileProcessorPreset,
  FileProcessorPresetCategory,
} from "~/features/files/drivers/core/processor-presets";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import singBoxTailscaleExternalScript from "~/features/files/processors/scripts/sing-box-tailscale-external.js?raw";
import singBoxTailscaleNativeScript from "~/features/files/processors/scripts/sing-box-tailscale-native.js?raw";
import {
  recognizeSingBoxStructureProcessorPreset,
  singBoxStructureProcessorPreset,
  type SingBoxStructureProcessorPresetOptions,
} from "~/features/files/processors/sing-box-structure-preset";
import type { ProcessorDetail } from "~/shared/resources/types";

export type SingBoxProcessorPresetID =
  | "sniff"
  | "ntp-direct"
  | "ensure-tun"
  | "stun-block"
  | "quic-fallback"
  | "ipv4-only"
  | "udp-p2p-eim"
  | "linux-tun-acceleration"
  | "mptcp-direct"
  | "windows-relaxed-route"
  | "tailscale-native"
  | "tailscale-external";
type SingBoxOrderedRuleProcessorPresetID = "ntp-direct" | "stun-block" | "quic-fallback";
type SingBoxTailscaleProcessorPresetID = "tailscale-native" | "tailscale-external";
type SingBoxStructureProcessorPresetID = Exclude<
  SingBoxProcessorPresetID,
  "sniff" | SingBoxOrderedRuleProcessorPresetID | SingBoxTailscaleProcessorPresetID
>;

const SNIFF_AND_DNS_HIJACK_CONTENT = JSON.stringify({
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
}, null, 2);

const NTP_DIRECT_PRESET: OrderedRuleProcessorPresetOptions = {
  id: "ntp-direct",
  kind: "sing-box",
  name: "Traditional NTP Direct",
  rules: [{ network: "udp", port: 123, outbound: "direct" }],
};

const PRESET_NAMES: Record<SingBoxProcessorPresetID, string> = {
  sniff: "Sniff & DNS Hijack",
  "ntp-direct": "Traditional NTP Direct",
  "ensure-tun": "Ensure TUN",
  "stun-block": "STUN Block",
  "quic-fallback": "QUIC Fallback",
  "ipv4-only": "IPv4 Only",
  "udp-p2p-eim": "UDP/P2P EIM",
  "linux-tun-acceleration": "Linux/OpenWrt TUN Acceleration",
  "mptcp-direct": "MPTCP Direct",
  "windows-relaxed-route": "Windows Relaxed Route",
  "tailscale-native": "Tailscale 原生接管",
  "tailscale-external": "Tailscale 共存",
};

const TAILSCALE_SCRIPTS: Readonly<Record<SingBoxTailscaleProcessorPresetID, string>> = {
  "tailscale-native": singBoxTailscaleNativeScript,
  "tailscale-external": singBoxTailscaleExternalScript,
};

const ORDERED_RULE_PRESETS: Record<
  SingBoxOrderedRuleProcessorPresetID,
  OrderedRuleProcessorPresetOptions
> = {
  "ntp-direct": NTP_DIRECT_PRESET,
  "stun-block": {
    id: "stun-block",
    kind: "sing-box",
    name: PRESET_NAMES["stun-block"],
    rules: [{ protocol: "stun", action: "reject" }],
  },
  "quic-fallback": {
    id: "quic-fallback",
    kind: "sing-box",
    name: PRESET_NAMES["quic-fallback"],
    rules: [{ protocol: "quic", action: "reject" }],
  },
};

const STRUCTURE_PRESETS: Record<
  SingBoxStructureProcessorPresetID,
  SingBoxStructureProcessorPresetOptions
> = {
  "ensure-tun": { operation: "ensure-tun", name: PRESET_NAMES["ensure-tun"] },
  "ipv4-only": { operation: "ipv4-only", name: PRESET_NAMES["ipv4-only"] },
  "udp-p2p-eim": { operation: "udp-p2p-eim", name: PRESET_NAMES["udp-p2p-eim"] },
  "linux-tun-acceleration": {
    operation: "linux-tun-acceleration",
    name: PRESET_NAMES["linux-tun-acceleration"],
  },
  "mptcp-direct": { operation: "mptcp-direct", name: PRESET_NAMES["mptcp-direct"] },
  "windows-relaxed-route": {
    operation: "windows-relaxed-route",
    name: PRESET_NAMES["windows-relaxed-route"],
  },
};

export function singBoxProcessorPreset(id: SingBoxProcessorPresetID): ProcessorDetail {
  if (id === "sniff") return sniffAndDNSHijackProcessor();
  if (isOrderedRulePresetID(id)) return orderedRuleProcessorPreset(ORDERED_RULE_PRESETS[id]);
  if (isTailscalePresetID(id)) return tailscaleProcessor(id);
  return singBoxStructureProcessorPreset(STRUCTURE_PRESETS[id]);
}

export const singBoxProcessorPresets: readonly FileProcessorPreset[] = [
  {
    id: "sniff",
    category: "network",
    labelKey: "processors.filePreset.singBox.sniff.label",
    descriptionKey: "processors.filePreset.singBox.sniff.description",
    riskKey: "processors.filePreset.singBox.sniff.risk",
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: () => singBoxProcessorPreset("sniff"),
    recognize: (processor) => (
      processor.type === "merge"
      && processor.params?.mode === "json_override"
      && processor.params.content === SNIFF_AND_DNS_HIJACK_CONTENT
    ),
  },
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
  structureDescriptor(
    "ensure-tun",
    "network",
    "processors.filePreset.singBox.ensureTun.label",
    "processors.filePreset.singBox.ensureTun.description",
    "processors.filePreset.singBox.ensureTun.risk",
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["stun-block"],
    "privacy",
    "processors.filePreset.singBox.stunBlock.label",
    "processors.filePreset.singBox.stunBlock.description",
    "processors.filePreset.singBox.stunBlock.risk",
    ["sniff"],
    ["udp-p2p-eim", "tailscale-native", "tailscale-external"],
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["quic-fallback"],
    "network",
    "processors.filePreset.singBox.quicFallback.label",
    "processors.filePreset.singBox.quicFallback.description",
    "processors.filePreset.singBox.quicFallback.risk",
    ["sniff"],
  ),
  structureDescriptor(
    "ipv4-only",
    "network",
    "processors.filePreset.singBox.ipv4Only.label",
    "processors.filePreset.singBox.ipv4Only.description",
    "processors.filePreset.singBox.ipv4Only.risk",
  ),
  structureDescriptor(
    "udp-p2p-eim",
    "network",
    "processors.filePreset.singBox.udpP2pEim.label",
    "processors.filePreset.singBox.udpP2pEim.description",
    "processors.filePreset.singBox.udpP2pEim.risk",
    [],
    ["stun-block"],
  ),
  structureDescriptor(
    "linux-tun-acceleration",
    "platform",
    "processors.filePreset.singBox.linuxTunAcceleration.label",
    "processors.filePreset.singBox.linuxTunAcceleration.description",
    "processors.filePreset.singBox.linuxTunAcceleration.risk",
    ["ensure-tun"],
  ),
  structureDescriptor(
    "mptcp-direct",
    "platform",
    "processors.filePreset.singBox.mptcpDirect.label",
    "processors.filePreset.singBox.mptcpDirect.description",
    "processors.filePreset.singBox.mptcpDirect.risk",
    ["linux-tun-acceleration"],
  ),
  structureDescriptor(
    "windows-relaxed-route",
    "platform",
    "processors.filePreset.singBox.windowsRelaxedRoute.label",
    "processors.filePreset.singBox.windowsRelaxedRoute.description",
    "processors.filePreset.singBox.windowsRelaxedRoute.risk",
  ),
  tailscaleDescriptor(
    "tailscale-native",
    "processors.filePreset.singBox.tailscaleNative.label",
    "processors.filePreset.singBox.tailscaleNative.description",
    "processors.filePreset.singBox.tailscaleNative.risk",
    ["tailscale-external", "stun-block"],
  ),
  tailscaleDescriptor(
    "tailscale-external",
    "processors.filePreset.singBox.tailscaleExternal.label",
    "processors.filePreset.singBox.tailscaleExternal.description",
    "processors.filePreset.singBox.tailscaleExternal.risk",
    ["tailscale-native", "stun-block"],
  ),
];

export function defaultSingBoxProcessors(): ProcessorDetail[] {
  return singBoxProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build());
}

function sniffAndDNSHijackProcessor(): ProcessorDetail {
  return {
    name: "Sniff & DNS Hijack",
    type: "merge",
    stage: "file",
    params: {
      mode: "json_override",
      content: SNIFF_AND_DNS_HIJACK_CONTENT,
    },
  };
}

function orderedRuleDescriptor(
  options: OrderedRuleProcessorPresetOptions,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  dependencies: readonly SingBoxProcessorPresetID[] = [],
  conflicts: readonly SingBoxProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id: options.id,
    category,
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn: false,
    dependencies,
    conflicts,
    build: () => orderedRuleProcessorPreset(options),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, options),
  };
}

function structureDescriptor(
  id: SingBoxStructureProcessorPresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  dependencies: readonly SingBoxProcessorPresetID[] = [],
  conflicts: readonly SingBoxProcessorPresetID[] = [],
): FileProcessorPreset {
  const options = STRUCTURE_PRESETS[id];
  return {
    id,
    category,
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn: false,
    dependencies,
    conflicts,
    build: () => singBoxStructureProcessorPreset(options),
    recognize: (processor) => recognizeSingBoxStructureProcessorPreset(processor, options),
  };
}

function tailscaleProcessor(id: SingBoxTailscaleProcessorPresetID): ProcessorDetail {
  return {
    name: PRESET_NAMES[id],
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: TAILSCALE_SCRIPTS[id],
      },
    },
  };
}

function tailscaleDescriptor(
  id: SingBoxTailscaleProcessorPresetID,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  conflicts: readonly SingBoxProcessorPresetID[],
): FileProcessorPreset {
  return {
    id,
    category: "tailscale",
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn: false,
    dependencies: ["ensure-tun"],
    conflicts,
    build: () => tailscaleProcessor(id),
    recognize: (processor) => {
      if (processor.type !== "script") return false;
      if (!isExactRecord(processor.params, ["source"])) return false;
      const source = processor.params.source;
      return isExactRecord(source, ["content", "type"])
        && source.type === "inline"
        && source.content === TAILSCALE_SCRIPTS[id];
    },
  };
}

function isExactRecord(
  value: unknown,
  expectedKeys: readonly string[],
): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const keys = Object.keys(value).sort();
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index]);
}

function isOrderedRulePresetID(
  id: SingBoxProcessorPresetID,
): id is SingBoxOrderedRuleProcessorPresetID {
  return id === "ntp-direct" || id === "stun-block" || id === "quic-fallback";
}

function isTailscalePresetID(
  id: SingBoxProcessorPresetID,
): id is SingBoxTailscaleProcessorPresetID {
  return id === "tailscale-native" || id === "tailscale-external";
}
