import type { FileProcessorPreset, FileProcessorPresetCategory } from "~/features/files/drivers/core/processor-presets";
import { githubRuleSourceMirrorPreset } from "~/features/files/processors/github-rule-source-mirror-preset";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

import fakeIPCompatContent from "./preset-content/fake-ip-compat.yaml?raw";
import fakeIPOpenClashContent from "./preset-content/fake-ip-openclash.yaml?raw";
import fakeIPShellCrashContent from "./preset-content/fake-ip-shellcrash.yaml?raw";
import tailscaleExternalContent from "./preset-content/tailscale-external.yaml?raw";

export type MihomoProcessorPresetID =
  | "fake-ip-compat"
  | "fake-ip-openclash"
  | "fake-ip-shellcrash"
  | "quic-fallback"
  | "tailscale-external";
type MihomoOrderedRuleProcessorPresetID = "quic-fallback";
type MihomoMergeProcessorPresetID = Exclude<
  MihomoProcessorPresetID,
  MihomoOrderedRuleProcessorPresetID
>;

const PRESET_CONTENT: Record<MihomoMergeProcessorPresetID, string> = {
  "fake-ip-compat": withoutTrailingNewline(fakeIPCompatContent),
  "fake-ip-openclash": withoutTrailingNewline(fakeIPOpenClashContent),
  "fake-ip-shellcrash": withoutTrailingNewline(fakeIPShellCrashContent),
  "tailscale-external": withoutTrailingNewline(tailscaleExternalContent),
};
function withoutTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

const ORDERED_RULE_PRESETS: Record<MihomoOrderedRuleProcessorPresetID, OrderedRuleProcessorPresetOptions> = {
  "quic-fallback": {
    id: "quic-fallback",
    kind: "mihomo",
    rules: ["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"],
  },
};

export function mihomoProcessorPreset(id: MihomoProcessorPresetID, name: string): ProcessorDetail {
  if (isOrderedRulePresetID(id)) return orderedRuleProcessorPreset(ORDERED_RULE_PRESETS[id], name);
  return {
    name,
    type: "merge",
    stage: "file",
    params: { mode: "yaml_override", content: PRESET_CONTENT[id] },
  };
}

export const mihomoProcessorPresets: readonly FileProcessorPreset[] = [
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
    "tailscale-external",
    "tailscale",
    "processor.mihomoPreset.tailscale",
    false,
    [],
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

function isOrderedRulePresetID(id: MihomoProcessorPresetID): id is MihomoOrderedRuleProcessorPresetID {
  return id === "quic-fallback";
}
