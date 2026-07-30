import type { ProcessorDetail } from "~/shared/resources/types";

export const RULE_SOURCE_REWRITE_PRESET_OPTION = "file-preset:github-rule-source-rewrite";

const PRESET_MARKER = "// sandrone:file-preset=github-rule-source-rewrite";
const PRESET_SCRIPT = `${PRESET_MARKER}
// Edit the destination values below to use another mirror.
const replacements = [
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

function main(input) {
  if (!input.file || typeof input.file.content !== "string") {
    return input;
  }
  let content = input.file.content;
  for (const [source, destination] of replacements) {
    content = content.split(source).join(destination);
  }
  input.file.content = content;
  return input;
}
`;

export function ruleSourceRewriteProcessorPreset(): ProcessorDetail {
  return {
    name: "GitHub Rule Source Rewrite",
    type: "script",
    stage: "file",
    params: {
      source: {
        type: "inline",
        content: PRESET_SCRIPT,
      },
    },
  };
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
