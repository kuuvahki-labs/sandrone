import type { FileProcessorPreset } from "~/features/files/drivers/core/processor-presets";
import type { ProcessorDetail } from "~/shared/resources/types";

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

export const singBoxProcessorPresets: readonly FileProcessorPreset[] = [{
  id: "sniff",
  category: "network",
  labelKey: "processors.filePreset.singBox.sniff.label",
  descriptionKey: "processors.filePreset.singBox.sniff.description",
  riskKey: "processors.filePreset.singBox.sniff.risk",
  defaultOn: true,
  dependencies: [],
  conflicts: [],
  build: sniffAndDNSHijackProcessor,
  recognize: (processor) => (
    processor.type === "merge"
    && processor.params?.mode === "json_override"
    && processor.params.content === SNIFF_AND_DNS_HIJACK_CONTENT
  ),
}];

export function defaultSingBoxProcessors(): ProcessorDetail[] {
  return singBoxProcessorPresets
    .filter((preset) => preset.defaultOn)
    .map((preset) => preset.build());
}

function sniffAndDNSHijackProcessor(): ProcessorDetail {
  return {
    name: "Sniff & DNS Hijack",
    type: "merge",
    stage: "file",
    params: {
      mode: "json_override",
      content: SNIFF_AND_DNS_HIJACK_CONTENT,
    },
  };
}
