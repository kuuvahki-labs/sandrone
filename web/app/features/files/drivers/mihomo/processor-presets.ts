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
import fakeIPOpenClashContent from "./preset-content/fake-ip-openclash.yaml?raw";
import fakeIPShellCrashContent from "./preset-content/fake-ip-shellcrash.yaml?raw";
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
  | "fake-ip-openclash"
  | "fake-ip-shellcrash"
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
  "fake-ip-openclash": withoutTrailingNewline(fakeIPOpenClashContent),
  "fake-ip-shellcrash": withoutTrailingNewline(fakeIPShellCrashContent),
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
    true,
  ),
  descriptor(
    "tun",
    "network",
    "processor.mihomoPreset.tun",
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["ntp-direct"],
    "network",
    "processors.filePreset.ntpDirect.label",
    true,
  ),
  githubRuleSourceMirrorPreset,
  versionedDescriptor(
    "fake-ip-compat",
    "network",
    "processor.mihomoPreset.fakeIpCompat",
    false,
    [],
    ["fake-ip-openclash", "fake-ip-shellcrash"],
  ),
  versionedDescriptor(
    "fake-ip-openclash",
    "network",
    "processors.filePreset.mihomo.fakeIpOpenClash.label",
    false,
    [],
    ["fake-ip-compat", "fake-ip-shellcrash"],
  ),
  versionedDescriptor(
    "fake-ip-shellcrash",
    "network",
    "processors.filePreset.mihomo.fakeIpShellCrash.label",
    false,
    [],
    ["fake-ip-compat", "fake-ip-openclash"],
  ),
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["quic-fallback"],
    "network",
    "processors.filePreset.mihomo.quicFallback.label",
  ),
  descriptor(
    "udp-p2p-eim",
    "network",
    "processors.filePreset.mihomo.udpP2pEim.label",
    false,
    [],
  ),
  descriptor(
    "linux-tun-acceleration",
    "platform",
    "processors.filePreset.mihomo.linuxTunAcceleration.label",
    false,
    ["tun"],
  ),
  descriptor(
    "windows-relaxed-route",
    "platform",
    "processors.filePreset.mihomo.windowsRelaxedRoute.label",
  ),
  mihomoTailscaleNativeDescriptor(),
  descriptor(
    "tailscale-external",
    "tailscale",
    "processor.mihomoPreset.tailscale",
    false,
    ["tun"],
    ["tailscale-native"],
  ),
  descriptor(
    "tailnet-share",
    "tailscale",
    "processor.mihomoPreset.tailnetShare",
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
  defaultOn = false,
  dependencies: readonly MihomoProcessorPresetID[] = [],
  conflicts: readonly MihomoProcessorPresetID[] = [],
): FileProcessorPreset {
  const content = PRESET_CONTENT[id];
  return {
    id,
    category,
    labelKey,
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

function versionedDescriptor(
  id: MihomoMergeProcessorPresetID,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  defaultOn = false,
  dependencies: readonly MihomoProcessorPresetID[] = [],
  conflicts: readonly MihomoProcessorPresetID[] = [],
): FileProcessorPreset {
  const content = PRESET_CONTENT[id];
  const marker = `# sandrone:mihomo-preset=${id}`;
  const recognizesIdentity = (processor: Pick<ProcessorDetail, "type" | "params">) => (
    processor.type === "merge"
    && processor.params?.mode === "yaml_override"
    && typeof processor.params.content === "string"
    && (processor.params.content === marker || processor.params.content.startsWith(`${marker}\n`))
  );
  return {
    id,
    category,
    labelKey,
    defaultOn,
    dependencies,
    conflicts,
    replaceConflictsInPlace: true,
    build: (t) => mihomoProcessorPreset(id, t(labelKey)),
    recognize: recognizesIdentity,
    isCurrent: (processor) => (
      recognizesIdentity(processor)
      && processor.params?.content === content
    ),
  };
}

function orderedRuleDescriptor(
  options: OrderedRuleProcessorPresetOptions,
  category: FileProcessorPresetCategory,
  labelKey: FileProcessorPreset["labelKey"],
  defaultOn = false,
  dependencies: readonly MihomoProcessorPresetID[] = [],
  conflicts: readonly MihomoProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id: options.id,
    category,
    labelKey,
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
      args: { auth_key: "" },
    },
  };
}

function mihomoTailscaleNativeDescriptor(): FileProcessorPreset {
  return {
    id: "tailscale-native",
    category: "tailscale",
    labelKey: "processors.filePreset.mihomo.tailscaleNative.label",
    defaultOn: false,
    dependencies: ["tun"],
    conflicts: ["tailscale-external"],
    build: (t) => mihomoTailscaleNativeProcessor(t("processors.filePreset.mihomo.tailscaleNative.label")),
    recognize: (processor) => {
      if (processor.type !== "script") return false;
      if (!isExactRecord(processor.params, ["source"])
        && !isExactRecord(processor.params, ["args", "source"])) return false;
      const source = processor.params.source;
      if (!isExactRecord(source, ["content", "type"])
        || source.type !== "inline"
        || source.content !== mihomoTailscaleNativeScript) return false;
      if (!("args" in processor.params)) return true;
      const args = processor.params.args;
      return isExactRecord(args, ["auth_key"])
        && typeof args.auth_key === "string";
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
