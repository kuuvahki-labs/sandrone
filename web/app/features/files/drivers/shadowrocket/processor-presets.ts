import type {
  FileProcessorPreset,
} from "~/features/files/drivers/core/processor-presets";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import shadowrocketTailscaleExternalScript from "~/features/files/processors/scripts/shadowrocket-tailscale-external.js?raw";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

export type ShadowrocketProcessorPresetID =
  | "tailscale-native"
  | "tailscale-external";

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
  githubRuleSourceMirrorPreset,
  {
    id: TAILSCALE_NATIVE_PRESET.id,
    category: "tailscale",
    labelKey: "processors.filePreset.shadowrocket.tailscaleNative.label",
    defaultOn: false,
    dependencies: [],
    conflicts: ["tailscale-external"],
    build: (t) => orderedRuleProcessorPreset(TAILSCALE_NATIVE_PRESET, t("processors.filePreset.shadowrocket.tailscaleNative.label")),
    recognize: (processor) => (
      recognizeOrderedRuleProcessorPreset(processor, TAILSCALE_NATIVE_PRESET)
    ),
  },
  {
    id: "tailscale-external",
    category: "tailscale",
    labelKey: "processors.filePreset.shadowrocket.tailscaleExternal.label",
    defaultOn: false,
    dependencies: [],
    conflicts: ["tailscale-native"],
    build: (t) => tailscaleExternalProcessor(t("processors.filePreset.shadowrocket.tailscaleExternal.label")),
    recognize: recognizeTailscaleExternalProcessor,
  },
];

export function defaultShadowrocketProcessors(t: Translator): ProcessorDetail[] {
  return shadowrocketProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build(t));
}

function tailscaleExternalProcessor(name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: shadowrocketTailscaleExternalScript,
      },
    },
  };
}

function recognizeTailscaleExternalProcessor(
  processor: Pick<ProcessorDetail, "type" | "params">,
): boolean {
  if (processor.type !== "script" || !isExactRecord(processor.params, ["source"])) return false;
  const source = processor.params.source;
  return isExactRecord(source, ["content", "type"])
    && source.type === "inline"
    && source.content === shadowrocketTailscaleExternalScript;
}

function isExactRecord(value: unknown, expectedKeys: readonly string[]): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const keys = Object.keys(value).sort();
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index]);
}
