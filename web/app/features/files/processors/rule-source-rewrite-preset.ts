import type { ProcessorDetail } from "~/shared/resources/types";

import presetScript from "./scripts/github-rule-source-rewrite.js?raw";

export const RULE_SOURCE_REWRITE_PRESET_OPTION = "file-preset:github-rule-source-rewrite";

const PRESET_MARKER = "// sandrone:file-preset=github-rule-source-rewrite";

export function ruleSourceRewriteProcessorPreset(): ProcessorDetail {
  return {
    name: "GitHub Rule Source Rewrite",
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: withoutTrailingNewline(presetScript),
      },
    },
  };
}

function withoutTrailingNewline(content: string): string {
  return content.endsWith("\n") ? content.slice(0, -1) : content;
}

export function recognizeRuleSourceRewriteProcessorPreset(
  processor: Pick<ProcessorDetail, "type" | "params">,
): boolean {
  if (processor.type !== "script") return false;
  const source = processor.params?.source;
  if (typeof source !== "object" || source === null || Array.isArray(source)) return false;
  const record = source as Record<string, unknown>;
  return record.type === "inline"
    && typeof record.content === "string"
    && record.content.includes(PRESET_MARKER);
}
