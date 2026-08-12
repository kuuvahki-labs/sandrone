import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

export type FileProcessorPresetCategory = "privacy" | "network" | "platform" | "tailscale";

export interface FileProcessorPreset {
  readonly id: string;
  readonly category: FileProcessorPresetCategory;
  readonly labelKey: Parameters<Translator>[0];
  readonly descriptionKey: Parameters<Translator>[0];
  readonly riskKey?: Parameters<Translator>[0];
  readonly defaultOn: boolean;
  readonly dependencies: readonly string[];
  readonly conflicts: readonly string[];
  build(): ProcessorDetail;
  recognize(processor: Pick<ProcessorDetail, "type" | "params">): boolean;
}

export interface FileProcessorPresetPlan {
  readonly additions: readonly ProcessorDetail[];
  readonly addedPresetIDs: readonly string[];
  readonly dependencyPresetIDs: readonly string[];
  readonly removeIndices: readonly number[];
  readonly removedPresetIDs: readonly string[];
  readonly requestedPresetID: string;
}

export function planFileProcessorPresetAddition(
  catalog: readonly FileProcessorPreset[],
  requestedPresetID: string,
  current: readonly ProcessorDetail[],
): FileProcessorPresetPlan {
  validateFileProcessorPresetCatalog(catalog);
  const byID = new Map(catalog.map((preset) => [preset.id, preset]));
  const requested = byID.get(requestedPresetID);
  if (!requested) throw new Error(`unknown file processor preset: ${requestedPresetID}`);

  const ordered = dependencyOrder(byID, requested);
  const dependencyPresetIDs = ordered.slice(0, -1).map((preset) => preset.id);
  const conflictIDs = new Set(ordered.flatMap((preset) => preset.conflicts));
  const removeIndices: number[] = [];
  const removedPresetIDs: string[] = [];
  const seenRemovedPresetIDs = new Set<string>();

  current.forEach((processor, index) => {
    const conflict = catalog.find((preset) => (
      conflictIDs.has(preset.id) && preset.recognize(processor)
    ));
    if (!conflict) return;
    removeIndices.push(index);
    if (!seenRemovedPresetIDs.has(conflict.id)) {
      seenRemovedPresetIDs.add(conflict.id);
      removedPresetIDs.push(conflict.id);
    }
  });

  const removedIndices = new Set(removeIndices);
  const remaining = current.filter((_, index) => !removedIndices.has(index));
  const additions: ProcessorDetail[] = [];
  const addedPresetIDs: string[] = [];
  for (const preset of ordered) {
    if (remaining.some((processor) => preset.recognize(processor))) continue;
    const addition = preset.build();
    additions.push(addition);
    addedPresetIDs.push(preset.id);
    remaining.push(addition);
  }

  return {
    additions,
    addedPresetIDs,
    dependencyPresetIDs,
    removeIndices,
    removedPresetIDs,
    requestedPresetID,
  };
}

export function recognizedFileProcessorPresetID(
  catalog: readonly FileProcessorPreset[],
  processor: Pick<ProcessorDetail, "type" | "params">,
): string | null {
  return catalog.find((preset) => preset.recognize(processor))?.id ?? null;
}

export function filterForeignManagedProcessors(
  targetCatalog: readonly FileProcessorPreset[],
  allCatalogs: readonly (readonly FileProcessorPreset[])[],
  current: readonly ProcessorDetail[],
): ProcessorDetail[] {
  return current.filter((processor) => (
    recognizedFileProcessorPresetID(targetCatalog, processor) !== null
    || !allCatalogs.some((catalog) => recognizedFileProcessorPresetID(catalog, processor) !== null)
  ));
}

export function validateFileProcessorPresetCatalog(
  catalog: readonly FileProcessorPreset[],
): void {
  const byID = new Map<string, FileProcessorPreset>();
  for (const preset of catalog) {
    if (!preset.id.trim()) throw new Error("file processor preset id must not be empty");
    if (byID.has(preset.id)) throw new Error(`duplicate file processor preset id: ${preset.id}`);
    byID.set(preset.id, preset);
  }

  for (const preset of catalog) {
    for (const dependencyID of preset.dependencies) {
      if (dependencyID === preset.id) {
        throw new Error(`file processor preset ${preset.id} must not depend on itself`);
      }
      if (!byID.has(dependencyID)) {
        throw new Error(`file processor preset ${preset.id} has unknown dependency: ${dependencyID}`);
      }
    }
    for (const conflictID of preset.conflicts) {
      if (conflictID === preset.id) {
        throw new Error(`file processor preset ${preset.id} must not conflict with itself`);
      }
      if (!byID.has(conflictID)) {
        throw new Error(`file processor preset ${preset.id} has unknown conflict: ${conflictID}`);
      }
    }
  }

  const visited = new Set<string>();
  const visiting = new Set<string>();
  const path: string[] = [];
  const visit = (preset: FileProcessorPreset) => {
    if (visited.has(preset.id)) return;
    visiting.add(preset.id);
    path.push(preset.id);
    for (const dependencyID of preset.dependencies) {
      if (visiting.has(dependencyID)) {
        const cycleStart = path.indexOf(dependencyID);
        const cycle = [...path.slice(cycleStart), dependencyID];
        throw new Error(`file processor preset dependency cycle: ${cycle.join(" -> ")}`);
      }
      visit(byID.get(dependencyID)!);
    }
    path.pop();
    visiting.delete(preset.id);
    visited.add(preset.id);
  };
  catalog.forEach(visit);
}

function dependencyOrder(
  byID: ReadonlyMap<string, FileProcessorPreset>,
  requested: FileProcessorPreset,
): FileProcessorPreset[] {
  const ordered: FileProcessorPreset[] = [];
  const included = new Set<string>();
  const visit = (preset: FileProcessorPreset) => {
    if (included.has(preset.id)) return;
    preset.dependencies.forEach((id) => visit(byID.get(id)!));
    included.add(preset.id);
    ordered.push(preset);
  };
  visit(requested);
  return ordered;
}
