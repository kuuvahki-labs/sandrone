import type {
  FileProcessorPreset,
} from "~/features/files/drivers/core/processor-presets";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

export type ShadowrocketProcessorPresetID =
  | "ntp-direct"
  | "tailscale-native";

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
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: (t) => orderedRuleProcessorPreset(NTP_DIRECT_PRESET, t("processors.filePreset.ntpDirect.label")),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, NTP_DIRECT_PRESET),
  },
  githubRuleSourceMirrorPreset,
  {
    id: TAILSCALE_NATIVE_PRESET.id,
    category: "tailscale",
    labelKey: "processors.filePreset.shadowrocket.tailscaleNative.label",
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
