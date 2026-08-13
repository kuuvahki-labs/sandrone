import type { FileProcessorPreset, FileProcessorPresetCategory } from "~/features/files/drivers/core/processor-presets";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import mihomoTailscaleNativeScript from "~/features/files/processors/scripts/mihomo-tailscale-native.js?raw";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

import fakeIPCompatContent from "./preset-content/fake-ip-compat.yaml?raw";
import linuxTunAccelerationContent from "./preset-content/linux-tun-acceleration.yaml?raw";
import snifferContent from "./preset-content/sniffer.yaml?raw";
import tailnetShareContent from "./preset-content/tailnet-share.yaml?raw";
import tailscaleExternalContent from "./preset-content/tailscale-external.yaml?raw";
import tunContent from "./preset-content/tun.yaml?raw";
import udpP2PEIMContent from "./preset-content/udp-p2p-eim.yaml?raw";
import windowsRelaxedRouteContent from "./preset-content/windows-relaxed-route.yaml?raw";

export type MihomoProcessorPresetID =
  | "sniffer"
  | "tun"
  | "ntp-direct"
  | "fake-ip-compat"
  | "quic-fallback"
  | "udp-p2p-eim"
  | "linux-tun-acceleration"
  | "windows-relaxed-route"
  | "tailscale-native"
  | "tailscale-external"
  | "tailnet-share";
type MihomoOrderedRuleProcessorPresetID = "ntp-direct" | "quic-fallback";
type MihomoNativeProcessorPresetID = "tailscale-native";
type MihomoMergeProcessorPresetID = Exclude<
  MihomoProcessorPresetID,
  MihomoOrderedRuleProcessorPresetID | MihomoNativeProcessorPresetID
>;

const PRESET_CONTENT: Record<MihomoMergeProcessorPresetID, string> = {
  sniffer: withoutTrailingNewline(snifferContent),
  tun: withoutTrailingNewline(tunContent),
  "fake-ip-compat": withoutTrailingNewline(fakeIPCompatContent),
  "udp-p2p-eim": withoutTrailingNewline(udpP2PEIMContent),
  "linux-tun-acceleration": withoutTrailingNewline(linuxTunAccelerationContent),
  "windows-relaxed-route": withoutTrailingNewline(windowsRelaxedRouteContent),
  "tailscale-external": withoutTrailingNewline(tailscaleExternalContent),
  "tailnet-share": withoutTrailingNewline(tailnetShareContent),
};

function withoutTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

const ORDERED_RULE_PRESETS: Record<MihomoOrderedRuleProcessorPresetID, OrderedRuleProcessorPresetOptions> = {
  "ntp-direct": {
    id: "ntp-direct",
    kind: "mihomo",
    rules: ["AND,((NETWORK,UDP),(DST-PORT,123)),DIRECT"],
  },
  "quic-fallback": {
    id: "quic-fallback",
    kind: "mihomo",
    rules: ["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"],
  },
};

export function mihomoProcessorPreset(id: MihomoProcessorPresetID, name: string): ProcessorDetail {
  if (isOrderedRulePresetID(id)) return orderedRuleProcessorPreset(ORDERED_RULE_PRESETS[id], name);
  if (id === "tailscale-native") return mihomoTailscaleNativeProcessor(name);
  return {
    name,
    type: "merge",
    stage: "file",
    params: { mode: "yaml_override", content: PRESET_CONTENT[id] },
  };
}

export const mihomoProcessorPresets: readonly FileProcessorPreset[] = [
  descriptor(
    "sniffer",
    "network",
    "processor.mihomoPreset.sniffer",
    "processors.filePreset.mihomo.sniffer.description",
    "processors.filePreset.mihomo.sniffer.risk",
    true,
  ),
  descriptor(
    "tun",
    "network",
    "processor.mihomoPreset.tun",
    "processors.filePreset.mihomo.tun.description",
    "processors.filePreset.mihomo.tun.risk",
    true,
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["ntp-direct"],
    "network",
    "processors.filePreset.ntpDirect.label",
    "processors.filePreset.ntpDirect.description",
    "processors.filePreset.ntpDirect.risk",
    true,
  ),
  githubRuleSourceMirrorPreset,
  descriptor(
    "fake-ip-compat",
    "network",
    "processor.mihomoPreset.fakeIpCompat",
    "processors.filePreset.mihomo.fakeIpCompat.description",
    "processors.filePreset.mihomo.fakeIpCompat.risk",
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["quic-fallback"],
    "network",
    "processors.filePreset.mihomo.quicFallback.label",
    "processors.filePreset.mihomo.quicFallback.description",
    "processors.filePreset.mihomo.quicFallback.risk",
  ),
  descriptor(
    "udp-p2p-eim",
    "network",
    "processors.filePreset.mihomo.udpP2pEim.label",
    "processors.filePreset.mihomo.udpP2pEim.description",
    "processors.filePreset.mihomo.udpP2pEim.risk",
    false,
    [],
  ),
  descriptor(
    "linux-tun-acceleration",
    "platform",
    "processors.filePreset.mihomo.linuxTunAcceleration.label",
    "processors.filePreset.mihomo.linuxTunAcceleration.description",
    "processors.filePreset.mihomo.linuxTunAcceleration.risk",
    false,
    ["tun"],
  ),
  descriptor(
    "windows-relaxed-route",
    "platform",
    "processors.filePreset.mihomo.windowsRelaxedRoute.label",
    "processors.filePreset.mihomo.windowsRelaxedRoute.description",
    "processors.filePreset.mihomo.windowsRelaxedRoute.risk",
  ),
  mihomoTailscaleNativeDescriptor(),
  descriptor(
    "tailscale-external",
    "tailscale",
    "processor.mihomoPreset.tailscale",
    "processors.filePreset.mihomo.tailscaleExternal.description",
    "processors.filePreset.mihomo.tailscaleExternal.risk",
    false,
    ["tun"],
    ["tailscale-native"],
  ),
  descriptor(
    "tailnet-share",
    "tailscale",
    "processor.mihomoPreset.tailnetShare",
    "processors.filePreset.mihomo.tailnetShare.description",
    "processors.filePreset.mihomo.tailnetShare.risk",
    false,
    ["tun", "tailscale-external"],
  ),
];

export function defaultMihomoProcessors(t: Translator): ProcessorDetail[] {
  return mihomoProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build(t));
}

function descriptor(
  id: MihomoMergeProcessorPresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  defaultOn = false,
  dependencies: readonly MihomoProcessorPresetID[] = [],
  conflicts: readonly MihomoProcessorPresetID[] = [],
): FileProcessorPreset {
  const content = PRESET_CONTENT[id];
  return {
    id,
    category,
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn,
    dependencies,
    conflicts,
    build: (t) => mihomoProcessorPreset(id, t(labelKey)),
    recognize: (processor) => (
      processor.type === "merge"
      && processor.params?.mode === "yaml_override"
      && processor.params.content === content
    ),
  };
}

function orderedRuleDescriptor(
  options: OrderedRuleProcessorPresetOptions,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  descriptionKey: FileProcessorPreset["descriptionKey"],
  riskKey: NonNullable<FileProcessorPreset["riskKey"]>,
  defaultOn = false,
  dependencies: readonly MihomoProcessorPresetID[] = [],
  conflicts: readonly MihomoProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id: options.id,
    category,
    labelKey,
    descriptionKey,
    riskKey,
    defaultOn,
    dependencies,
    conflicts,
    build: (t) => orderedRuleProcessorPreset(options, t(labelKey)),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, options),
  };
}

function mihomoTailscaleNativeProcessor(name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: mihomoTailscaleNativeScript,
      },
    },
  };
}

function mihomoTailscaleNativeDescriptor(): FileProcessorPreset {
  return {
    id: "tailscale-native",
    category: "tailscale",
    labelKey: "processors.filePreset.mihomo.tailscaleNative.label",
    descriptionKey: "processors.filePreset.mihomo.tailscaleNative.description",
    riskKey: "processors.filePreset.mihomo.tailscaleNative.risk",
    defaultOn: false,
    dependencies: ["tun"],
    conflicts: ["tailscale-external"],
    build: (t) => mihomoTailscaleNativeProcessor(t("processors.filePreset.mihomo.tailscaleNative.label")),
    recognize: (processor) => {
      if (processor.type !== "script") return false;
      if (!isExactRecord(processor.params, ["source"])) return false;
      const source = processor.params.source;
      return isExactRecord(source, ["content", "type"])
        && source.type === "inline"
        && source.content === mihomoTailscaleNativeScript;
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

function isOrderedRulePresetID(id: MihomoProcessorPresetID): id is MihomoOrderedRuleProcessorPresetID {
  return id === "ntp-direct" || id === "quic-fallback";
}
