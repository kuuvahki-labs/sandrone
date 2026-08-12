import type { FileProcessorPreset } from "~/features/files/drivers/core/processor-presets";
import {
  orderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "~/features/files/processors/ordered-rule-preset";
import type { ProcessorDetail } from "~/shared/resources/types";

const NTP_DIRECT_PRESET: OrderedRuleProcessorPresetOptions = {
  id: "ntp-direct",
  kind: "shadowrocket",
  name: "Traditional NTP Direct",
  rules: ["AND,((PROTOCOL,UDP),(DST-PORT,123)),DIRECT"],
};

export const shadowrocketProcessorPresets: readonly FileProcessorPreset[] = [{
  id: NTP_DIRECT_PRESET.id,
  category: "network",
  labelKey: "processors.filePreset.ntpDirect.label",
  descriptionKey: "processors.filePreset.ntpDirect.description",
  riskKey: "processors.filePreset.ntpDirect.risk",
  defaultOn: true,
  dependencies: [],
  conflicts: [],
  build: () => orderedRuleProcessorPreset(NTP_DIRECT_PRESET),
  recognize: (processor) => recognizeOrderedRuleProcessorPreset(processor, NTP_DIRECT_PRESET),
}];

export function defaultShadowrocketProcessors(): ProcessorDetail[] {
  return shadowrocketProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build());
}
