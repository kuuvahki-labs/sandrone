import type { ProcessorDetail } from "~/shared/resources/types";

import insertMihomoRulesScript from "./scripts/insert-mihomo-rules.js?raw";
import insertShadowrocketRulesScript from "./scripts/insert-shadowrocket-rules.js?raw";
import insertSingBoxRulesScript from "./scripts/insert-sing-box-rules.js?raw";

export type OrderedRuleProcessorKind = "mihomo" | "sing-box" | "shadowrocket";
export type OrderedRuleProcessorInsertMode = "anchor" | "top";

export interface OrderedRuleProcessorPresetOptions {
  readonly id: string;
  readonly kind: OrderedRuleProcessorKind;
  readonly rules: readonly unknown[];
  readonly insertMode?: OrderedRuleProcessorInsertMode;
}

const SCRIPT_BY_KIND: Readonly<Record<OrderedRuleProcessorKind, string>> = {
  mihomo: insertMihomoRulesScript,
  "sing-box": insertSingBoxRulesScript,
  shadowrocket: insertShadowrocketRulesScript,
};

export function orderedRuleProcessorPreset(
  options: OrderedRuleProcessorPresetOptions,
  name: string,
): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: expectedParams(options),
  };
}

export function recognizeOrderedRuleProcessorPreset(
  processor: Pick<ProcessorDetail, "type" | "params">,
  options: OrderedRuleProcessorPresetOptions,
): boolean {
  if (processor.type !== "script") return false;
  const params = processor.params;
  if (!isExactRecord(params, ["args", "source"])) return false;
  const source = params.source;
  const args = params.args;
  if (!isExactRecord(source, ["content", "type"])) return false;
  const expectedArgs = options.insertMode
    ? ["insert_mode", "preset_id", "rules_json"]
    : ["preset_id", "rules_json"];
  const legacyTopArgs = options.insertMode === "top"
    && isExactRecord(args, ["preset_id", "rules_json"]);
  if (!legacyTopArgs && !isExactRecord(args, expectedArgs)) return false;
  return source.type === "inline"
    && source.content === SCRIPT_BY_KIND[options.kind]
    && args.preset_id === options.id
    && args.rules_json === JSON.stringify(options.rules)
    && (legacyTopArgs || !options.insertMode || args.insert_mode === options.insertMode);
}

function expectedParams(options: OrderedRuleProcessorPresetOptions): Record<string, unknown> {
  return {
    source: {
      type: "inline",
      content: SCRIPT_BY_KIND[options.kind],
    },
    args: {
      preset_id: options.id,
      rules_json: JSON.stringify(options.rules),
      ...(options.insertMode ? { insert_mode: options.insertMode } : {}),
    },
  };
}

function isExactRecord(value: unknown, expectedKeys: readonly string[]): value is Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return false;
  const keys = Object.keys(value).sort();
  return keys.length === expectedKeys.length
    && keys.every((key, index) => key === expectedKeys[index]);
}
