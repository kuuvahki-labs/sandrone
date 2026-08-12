import type {
  FileCreatePreset,
  FileDriverDefinition,
  StructuredFileConfigurationAdapter,
} from "./file-driver";
import {
  type FileProcessorPreset,
  validateFileProcessorPresetCatalog,
} from "./processor-presets";

export interface FileDriverRegistry {
  readonly drivers: readonly Readonly<FileDriverDefinition>[];
  readonly createPresets: readonly Readonly<FileCreatePreset>[];
  get(kind: string | null | undefined): Readonly<FileDriverDefinition> | undefined;
  resolveCreatePreset(source: string | null | undefined): Readonly<FileCreatePreset> | undefined;
}

export function createFileDriverRegistry(
  definitions: readonly FileDriverDefinition[],
): FileDriverRegistry {
  const byKind = new Map<string, Readonly<FileDriverDefinition>>();
  const bySource = new Map<string, Readonly<FileCreatePreset>>();
  const ordered = [...definitions]
    .sort((left, right) => (
      firstCreateOrder(left) - firstCreateOrder(right) || left.kind.localeCompare(right.kind)
    ))
    .map((driver) => {
      if (!driver.kind.trim()) throw new Error("file kind must not be empty");
      if (byKind.has(driver.kind)) throw new Error(`duplicate file kind: ${driver.kind}`);
      if (driver.configuration.mode === "structured" && !driver.configuration.adapter) {
        throw new Error(`file kind ${driver.kind} requires a configuration adapter`);
      }
      const mismatchedPreset = driver.createPresets.find((preset) => preset.kind !== driver.kind);
      if (mismatchedPreset) {
        throw new Error(`file creation preset ${mismatchedPreset.source} kind must match its driver kind ${driver.kind}`);
      }
      if (driver.configuration.mode === "structured" && driver.configuration.adapter.kind !== driver.kind) {
        throw new Error(`file configuration adapter kind must match its driver kind ${driver.kind}`);
      }
      const frozen = freezeDriver(driver);
      validateFileProcessorPresetCatalog(frozen.processors.presets);
      byKind.set(frozen.kind, frozen);
      return frozen;
    });
  const createPresets = ordered
    .flatMap((driver) => driver.createPresets)
    .sort((left, right) => left.order - right.order);
  for (const preset of createPresets) {
    if (bySource.has(preset.source)) throw new Error(`duplicate file create source: ${preset.source}`);
    if (!byKind.has(preset.kind)) throw new Error(`unknown file kind for create source ${preset.source}: ${preset.kind}`);
    bySource.set(preset.source, preset);
  }
  const frozenDrivers = Object.freeze(ordered);
  const frozenCreatePresets = Object.freeze(createPresets);
  return Object.freeze({
    drivers: frozenDrivers,
    createPresets: frozenCreatePresets,
    get: (kind: string | null | undefined) => kind ? byKind.get(kind) : undefined,
    resolveCreatePreset: (source: string | null | undefined) => source ? bySource.get(source) : undefined,
  });
}

function freezeDriver(driver: FileDriverDefinition): Readonly<FileDriverDefinition> {
  const createPresets = Object.freeze(driver.createPresets.map((preset) => Object.freeze({ ...preset })));
  const processorPresets = Object.freeze(driver.processors.presets.map(freezeProcessorPreset));
  const configuration = driver.configuration.mode === "structured"
    ? Object.freeze({
      ...driver.configuration,
      adapter: freezeConfigurationAdapter(driver.configuration.adapter),
    })
    : Object.freeze({ ...driver.configuration });
  return Object.freeze({
    ...driver,
    presentation: Object.freeze({ ...driver.presentation }),
    configuration,
    createPresets,
    source: Object.freeze({ ...driver.source }),
    processors: Object.freeze({
      ...driver.processors,
      adapter: driver.processors.adapter ? Object.freeze({ ...driver.processors.adapter }) : undefined,
      mergeModes: Object.freeze([...driver.processors.mergeModes]),
      presets: processorPresets,
    }),
  }) as Readonly<FileDriverDefinition>;
}

function freezeProcessorPreset(preset: FileProcessorPreset): FileProcessorPreset {
  return Object.freeze({
    ...preset,
    dependencies: Object.freeze([...preset.dependencies]),
    conflicts: Object.freeze([...preset.conflicts]),
  });
}

function freezeConfigurationAdapter(
  adapter: StructuredFileConfigurationAdapter,
): StructuredFileConfigurationAdapter {
  return Object.freeze({
    ...adapter,
    adaptive: Object.freeze({
      ...adapter.adaptive,
      typeOptions: Object.freeze([...adapter.adaptive.typeOptions]),
    }),
    groups: Object.freeze({
      ...adapter.groups,
      typeOptions: Object.freeze([...adapter.groups.typeOptions]),
    }),
    ruleSets: Object.freeze({
      ...adapter.ruleSets,
      formatOptions: Object.freeze([...adapter.ruleSets.formatOptions]),
    }),
    rules: Object.freeze({ ...adapter.rules }),
    preview: Object.freeze({ ...adapter.preview }),
    references: Object.freeze({
      ...adapter.references,
      groupBuiltins: Object.freeze([...adapter.references.groupBuiltins]),
      rulePolicyBuiltins: Object.freeze([...adapter.references.rulePolicyBuiltins]),
    }),
    relations: Object.freeze({ ...adapter.relations }),
    templates: Object.freeze({ ...adapter.templates }),
  });
}

function firstCreateOrder(driver: FileDriverDefinition): number {
  return driver.createPresets.reduce(
    (order, preset) => Math.min(order, preset.order),
    Number.POSITIVE_INFINITY,
  );
}
