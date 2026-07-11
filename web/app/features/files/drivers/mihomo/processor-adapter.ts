import type { FileProcessorAdapter } from "~/features/files/drivers/core/file-driver";
import type { Translator } from "~/shared/i18n/context";
import { cleanParams, type ProcessorDraft } from "~/shared/processors/model";
import type { ProcessorDetail } from "~/shared/resources/types";

import {
  mihomoProcessorPreset,
  type MihomoProcessorPresetID,
  recognizeMihomoProcessorPreset,
} from "./processor-presets";

const PRESET_OPTION_PREFIX = "mihomo-preset:";

export const mihomoProcessorAdapter: FileProcessorAdapter = {
  options: (t: Translator) => [
    { value: `${PRESET_OPTION_PREFIX}sniffer`, label: t("processor.mihomoPreset.sniffer") },
    { value: `${PRESET_OPTION_PREFIX}tun`, label: t("processor.mihomoPreset.tun") },
    { value: `${PRESET_OPTION_PREFIX}tailscale`, label: t("processor.mihomoPreset.tailscale") },
    { value: `${PRESET_OPTION_PREFIX}tailnet-share`, label: t("processor.mihomoPreset.tailnetShare") },
  ],
  addPreset(type, current) {
    if (!type.startsWith(PRESET_OPTION_PREFIX)) return undefined;
    const requested = type.slice(PRESET_OPTION_PREFIX.length) as MihomoProcessorPresetID;
    const dependencies: Record<MihomoProcessorPresetID, MihomoProcessorPresetID[]> = {
      sniffer: ["sniffer"],
      tun: ["tun"],
      tailscale: ["tun", "tailscale"],
      "tailnet-share": ["tun", "tailscale", "tailnet-share"],
    };
    const requestedDependencies = dependencies[requested];
    if (!requestedDependencies) return undefined;
    const existing = new Set(current.map((draft) => recognizeMihomoProcessorPreset(draft)).filter(Boolean));
    const additions = requestedDependencies
      .filter((id) => !existing.has(id))
      .map((id, index) => processorDraft(mihomoProcessorPreset(id), index));
    return [...current, ...additions];
  },
  normalize: (processors) => processors,
};

export function processorsWithoutMihomoPresets(processors: ProcessorDetail[]): ProcessorDetail[] {
  return processors.filter((processor) => recognizeMihomoProcessorPreset(processor) === null);
}

function processorDraft(processor: ProcessorDetail, index: number): ProcessorDraft {
  return {
    id: `mihomo-preset-${processor.name ?? processor.type}-${index}`,
    name: typeof processor.name === "string" ? processor.name : "",
    type: processor.type,
    params: cleanParams(processor.params ?? {}),
  };
}
