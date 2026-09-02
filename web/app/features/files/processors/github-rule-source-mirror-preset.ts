import type { FileProcessorPreset } from "~/features/files/drivers/core/processor-presets";
import type { ProcessorDetail } from "~/shared/resources/types";

import replaceStringsScript from "./scripts/replace-strings.js?raw";

export const GITHUB_RULE_SOURCE_MIRROR_PRESET_ID = "github-rule-source-mirror";

export const GITHUB_RULE_SOURCE_MIRROR_REPLACEMENTS: readonly (readonly [string, string])[] = [
  [
    "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/",
    "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/",
  ],
  [
    "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/",
    "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/",
  ],
  [
    "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/",
    "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/",
  ],
];

const LEGACY_PRESET_MARKER = "// sandrone:file-preset=github-rule-source-rewrite";

export const githubRuleSourceMirrorPreset: FileProcessorPreset = {
  id: GITHUB_RULE_SOURCE_MIRROR_PRESET_ID,
  category: "network",
  labelKey: "files.processor.githubRuleSourceMirrorPreset",
  defaultOn: true,
  dependencies: [],
  conflicts: [],
  build: (t) => githubRuleSourceMirrorProcessorPreset(t("files.processor.githubRuleSourceMirrorPreset")),
  recognize: recognizeGitHubRuleSourceMirrorProcessorPreset,
};

export function githubRuleSourceMirrorProcessorPreset(name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: withoutTrailingNewline(replaceStringsScript),
      },
      args: {
        preset_id: GITHUB_RULE_SOURCE_MIRROR_PRESET_ID,
        replacements: GITHUB_RULE_SOURCE_MIRROR_REPLACEMENTS.map((pair) => [...pair]),
      },
    },
  };
}

export function recognizeGitHubRuleSourceMirrorProcessorPreset(
  processor: Pick<ProcessorDetail, "type" | "params">,
): boolean {
  if (processor.type !== "script") return false;
  const source = objectValue(processor.params?.source);
  if (source.type !== "inline" || typeof source.content !== "string") return false;
  if (source.content.includes(LEGACY_PRESET_MARKER)) return true;
  if (source.content !== withoutTrailingNewline(replaceStringsScript)) return false;
  const args = objectValue(processor.params?.args);
  return args.preset_id === GITHUB_RULE_SOURCE_MIRROR_PRESET_ID
    && Array.isArray(args.replacements);
}

function withoutTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

function objectValue(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? value as Record<string, unknown>
    : {};
}
