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
import singBoxTailscaleExternalScript from "~/features/files/processors/scripts/sing-box-tailscale-external.js?raw";
import singBoxTailscaleNativeScript from "~/features/files/processors/scripts/sing-box-tailscale-native.js?raw";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

export type SingBoxProcessorPresetID =
  | "sniff"
  | "quic-fallback"
  | "tailscale-native"
  | "tailscale-external";
type SingBoxOrderedRuleProcessorPresetID = "quic-fallback";
type SingBoxTailscaleProcessorPresetID = "tailscale-native" | "tailscale-external";

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

const TAILSCALE_SCRIPTS: Readonly<Record<SingBoxTailscaleProcessorPresetID, string>> = {
  "tailscale-native": singBoxTailscaleNativeScript,
  "tailscale-external": singBoxTailscaleExternalScript,
};

const ORDERED_RULE_PRESETS: Record<
  SingBoxOrderedRuleProcessorPresetID,
  OrderedRuleProcessorPresetOptions
> = {
  "quic-fallback": {
    id: "quic-fallback",
    kind: "sing-box",
    rules: [{ protocol: "quic", action: "reject" }],
  },
};

export function singBoxProcessorPreset(id: SingBoxProcessorPresetID, name: string): ProcessorDetail {
  if (id === "sniff") return sniffAndDNSHijackProcessor(name);
  if (isOrderedRulePresetID(id)) return orderedRuleProcessorPreset(ORDERED_RULE_PRESETS[id], name);
  return tailscaleProcessor(id, name);
}

export const singBoxProcessorPresets: readonly FileProcessorPreset[] = [
  {
    id: "sniff",
    category: "network",
    labelKey: "processors.filePreset.singBox.sniff.label",
    defaultOn: true,
    dependencies: [],
    conflicts: [],
    build: (t) => singBoxProcessorPreset("sniff", t("processors.filePreset.singBox.sniff.label")),
    recognize: (processor) => (
      processor.type === "merge"
      && processor.params?.mode === "json_override"
      && processor.params.content === SNIFF_AND_DNS_HIJACK_CONTENT
    ),
  },
  githubRuleSourceMirrorPreset,
  orderedRuleDescriptor(
    ORDERED_RULE_PRESETS["quic-fallback"],
    "network",
    "processors.filePreset.singBox.quicFallback.label",
    ["sniff"],
  ),
  tailscaleDescriptor(
    "tailscale-native",
    "processors.filePreset.singBox.tailscaleNative.label",
    ["tailscale-external"],
  ),
  tailscaleDescriptor(
    "tailscale-external",
    "processors.filePreset.singBox.tailscaleExternal.label",
    ["tailscale-native"],
  ),
];

export function defaultSingBoxProcessors(t: Translator): ProcessorDetail[] {
  return singBoxProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build(t));
}

function sniffAndDNSHijackProcessor(name: string): ProcessorDetail {
  return {
    name,
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
  dependencies: readonly SingBoxProcessorPresetID[] = [],
  conflicts: readonly SingBoxProcessorPresetID[] = [],
): FileProcessorPreset {
  return {
    id: options.id,
    category,
    labelKey,
    defaultOn: false,
    dependencies,
    conflicts,
    build: (t) => orderedRuleProcessorPreset(options, t(labelKey)),
    recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, options),
  };
}

function tailscaleProcessor(id: SingBoxTailscaleProcessorPresetID, name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: TAILSCALE_SCRIPTS[id],
      },
      ...(id === "tailscale-native" ? { args: { auth_key: "" } } : {}),
    },
  };
}

function tailscaleDescriptor(
  id: SingBoxTailscaleProcessorPresetID,
  labelKey: FileProcessorPreset["labelKey"],
  conflicts: readonly SingBoxProcessorPresetID[],
): FileProcessorPreset {
  return {
    id,
    category: "tailscale",
    labelKey,
    defaultOn: false,
    dependencies: [],
    conflicts,
    build: (t) => tailscaleProcessor(id, t(labelKey)),
    recognize: (processor) => {
      if (processor.type !== "script" || !processor.params) return false;
      const expectedKeys = id === "tailscale-native" && "args" in processor.params
        ? ["args", "source"]
        : ["source"];
      if (!isExactRecord(processor.params, expectedKeys)) return false;
      const source = processor.params.source;
      if (!isExactRecord(source, ["content", "type"])
        || source.type !== "inline"
        || source.content !== TAILSCALE_SCRIPTS[id]) return false;
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

function isOrderedRulePresetID(
  id: SingBoxProcessorPresetID,
): id is SingBoxOrderedRuleProcessorPresetID {
  return id === "quic-fallback";
}
