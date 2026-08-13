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
  build(t: Translator): ProcessorDetail;
  recognize(processor: Pick<ProcessorDetail, "type" | "params">): boolean;
}

export interface FileProcessorPresetPlan {
  readonly additions: readonly FileProcessorPresetAddition[];
  readonly addedPresetIDs: readonly string[];
  readonly dependencyPresetIDs: readonly string[];
  readonly removeIndices: readonly number[];
  readonly removedPresetIDs: readonly string[];
  readonly requestedPresetID: string;
}

export interface FileProcessorPresetAddition {
  readonly presetID: string;
  readonly processor: ProcessorDetail;
  /** Original `current` index to insert before, or `null` to append. */
  readonly beforeIndex: number | null;
}

export function planFileProcessorPresetAddition(
  catalog: readonly FileProcessorPreset[],
  requestedPresetID: string,
  current: readonly ProcessorDetail[],
  t: Translator,
): FileProcessorPresetPlan {
  validateFileProcessorPresetCatalog(catalog);
  const byID = new Map(catalog.map((preset) => [preset.id, preset]));
  const requested = byID.get(requestedPresetID);
  if (!requested) throw new Error(`unknown file processor preset: ${requestedPresetID}`);

  const ordered = dependencyOrder(byID, requested);
  const dependencyPresetIDs = ordered.slice(0, -1).map((preset) => preset.id);
  const requestedClosureIDs = new Set(ordered.map((preset) => preset.id));
  const conflictIDs = new Set(ordered.flatMap((preset) => preset.conflicts));
  const recognizedCurrent = current.map((processor) => (
    recognizedFileProcessorPresetID(catalog, processor)
  ));
  const removalIndices = new Set<number>();

  recognizedCurrent.forEach((presetID, index) => {
    if (presetID !== null && conflictIDs.has(presetID)) removalIndices.add(index);
  });
  cascadeRemovedDependents(
    catalog,
    recognizedCurrent,
    requestedClosureIDs,
    removalIndices,
  );

  const removeIndices = current.flatMap((_, index) => removalIndices.has(index) ? [index] : []);
  const removedPresetIDs: string[] = [];
  const seenRemovedPresetIDs = new Set<string>();
  removeIndices.forEach((index) => {
    const presetID = recognizedCurrent[index];
    if (presetID === null || seenRemovedPresetIDs.has(presetID)) return;
    seenRemovedPresetIDs.add(presetID);
    removedPresetIDs.push(presetID);
  });
  const removedIndices = new Set(removeIndices);
  const survivingPresetIDs = new Set(recognizedCurrent.flatMap((presetID, index) => (
    presetID !== null && !removedIndices.has(index) ? [presetID] : []
  )));
  const additions: FileProcessorPresetAddition[] = [];
  const addedPresetIDs: string[] = [];
  for (const preset of ordered) {
    if (survivingPresetIDs.has(preset.id)) continue;
    additions.push({
      presetID: preset.id,
      processor: preset.build(t),
      beforeIndex: earliestSurvivingConsumerIndex(
        preset.id,
        byID,
        recognizedCurrent,
        removedIndices,
      ),
    });
    addedPresetIDs.push(preset.id);
    survivingPresetIDs.add(preset.id);
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

function cascadeRemovedDependents(
  catalog: readonly FileProcessorPreset[],
  recognizedCurrent: readonly (string | null)[],
  requestedClosureIDs: ReadonlySet<string>,
  removalIndices: Set<number>,
): void {
  const reverseDependencies = new Map<string, string[]>();
  for (const preset of catalog) {
    for (const dependencyID of preset.dependencies) {
      const dependents = reverseDependencies.get(dependencyID) ?? [];
      dependents.push(preset.id);
      reverseDependencies.set(dependencyID, dependents);
    }
  }

  const initiallyRemovedIDs = new Set(recognizedCurrent.flatMap((presetID, index) => (
    presetID !== null && removalIndices.has(index) ? [presetID] : []
  )));
  const unavailableIDs = new Set<string>();
  const queue: string[] = [];
  for (const presetID of initiallyRemovedIDs) {
    if (requestedClosureIDs.has(presetID)) continue;
    if (hasSurvivingPreset(presetID, recognizedCurrent, removalIndices)) continue;
    unavailableIDs.add(presetID);
    queue.push(presetID);
  }

  for (let queueIndex = 0; queueIndex < queue.length; queueIndex += 1) {
    const unavailableID = queue[queueIndex];
    for (const dependentID of reverseDependencies.get(unavailableID) ?? []) {
      if (requestedClosureIDs.has(dependentID)) continue;
      let removedDependent = false;
      recognizedCurrent.forEach((presetID, index) => {
        if (presetID !== dependentID || removalIndices.has(index)) return;
        removalIndices.add(index);
        removedDependent = true;
      });
      if (!removedDependent || unavailableIDs.has(dependentID)) continue;
      if (hasSurvivingPreset(dependentID, recognizedCurrent, removalIndices)) continue;
      unavailableIDs.add(dependentID);
      queue.push(dependentID);
    }
  }
}

function hasSurvivingPreset(
  presetID: string,
  recognizedCurrent: readonly (string | null)[],
  removalIndices: ReadonlySet<number>,
): boolean {
  return recognizedCurrent.some((recognizedID, index) => (
    recognizedID === presetID && !removalIndices.has(index)
  ));
}

function earliestSurvivingConsumerIndex(
  dependencyID: string,
  byID: ReadonlyMap<string, FileProcessorPreset>,
  recognizedCurrent: readonly (string | null)[],
  removedIndices: ReadonlySet<number>,
): number | null {
  for (let index = 0; index < recognizedCurrent.length; index += 1) {
    const consumerID = recognizedCurrent[index];
    if (consumerID === null || removedIndices.has(index)) continue;
    if (presetTransitivelyDependsOn(consumerID, dependencyID, byID)) return index;
  }
  return null;
}

function presetTransitivelyDependsOn(
  presetID: string,
  dependencyID: string,
  byID: ReadonlyMap<string, FileProcessorPreset>,
): boolean {
  const visited = new Set<string>();
  const visit = (candidateID: string): boolean => {
    if (visited.has(candidateID)) return false;
    visited.add(candidateID);
    const candidate = byID.get(candidateID)!;
    return candidate.dependencies.some((directDependencyID) => (
      directDependencyID === dependencyID || visit(directDependencyID)
    ));
  };
  return visit(presetID);
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
