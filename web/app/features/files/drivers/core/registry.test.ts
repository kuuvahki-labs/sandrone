import { describe, expect, it } from "vitest";

import type {
  FileCreatePreset,
  FileDriverDefinition,
  StructuredFileConfigurationAdapter,
} from "./file-driver";
import type { FileProcessorPreset } from "./processor-presets";
import { createFileDriverRegistry } from "./registry";

describe("file driver core registry", () => {
  it("sorts definitions and presets while preserving lookup identity", () => {
    const late = rawOnlyDriver("late-client", "late-client", 30);
    const early = rawOnlyDriver("early-client", "early-client", 10);
    const registry = createFileDriverRegistry([late, early]);

    expect(registry.drivers.map((driver) => driver.kind)).toEqual(["early-client", "late-client"]);
    expect(registry.createPresets.map((preset) => preset.source)).toEqual(["early-client", "late-client"]);
    expect(registry.get("early-client")).toBe(registry.drivers[0]);
    expect(registry.resolveCreatePreset("late-client")).toBe(registry.createPresets[1]);
    expect(registry.get("")).toBeUndefined();
    expect(registry.get(undefined)).toBeUndefined();
    expect(registry.resolveCreatePreset(null)).toBeUndefined();
  });

  it("registers a raw-only future driver without requiring UI or a structured adapter", () => {
    const registry = createFileDriverRegistry([rawOnlyDriver()]);

    expect(registry.get("future-client")).toMatchObject({
      kind: "future-client",
      configuration: { mode: "raw" },
    });
    expect(registry.resolveCreatePreset("future-client")).toEqual({
      kind: "future-client",
      source: "future-client",
      sourceType: "inline",
      order: 100,
      initialName: "future.json",
    });
  });

  it("deep-freezes definitions, structured adapters, and nested option arrays", () => {
    const registry = createFileDriverRegistry([structuredDriver()]);
    const driver = registry.get("future-client");
    if (driver?.configuration.mode !== "structured") throw new Error("expected structured fixture");

    expect(Object.isFrozen(registry)).toBe(true);
    expect(Object.isFrozen(registry.drivers)).toBe(true);
    expect(Object.isFrozen(registry.createPresets)).toBe(true);
    expect(Object.isFrozen(driver)).toBe(true);
    expect(Object.isFrozen(driver.presentation)).toBe(true);
    expect(Object.isFrozen(driver.configuration)).toBe(true);
    expect(Object.isFrozen(driver.createPresets)).toBe(true);
    expect(Object.isFrozen(driver.createPresets[0])).toBe(true);
    expect(Object.isFrozen(driver.source)).toBe(true);
    expect(Object.isFrozen(driver.processors)).toBe(true);
    expect(Object.isFrozen(driver.processors.mergeModes)).toBe(true);
    expect(Object.isFrozen(driver.processors.presets)).toBe(true);
    expect(Object.isFrozen(driver.processors.presets[0])).toBe(true);
    expect(Object.isFrozen(driver.processors.presets[0].dependencies)).toBe(true);
    expect(Object.isFrozen(driver.processors.presets[0].conflicts)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.adaptive)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.adaptive.typeOptions)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.groups)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.groups.typeOptions)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.ruleSets)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.ruleSets.formatOptions)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.rules)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.preview)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.references)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.references.groupBuiltins)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.references.rulePolicyBuiltins)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.relations)).toBe(true);
    expect(Object.isFrozen(driver.configuration.adapter.templates)).toBe(true);
    expect(driver.configuration.adapter).not.toHaveProperty("ui");
  });

  it("rejects an empty kind with the existing exact error", () => {
    expect(errorMessage(() => createFileDriverRegistry([rawOnlyDriver("", "empty")])))
      .toBe("file kind must not be empty");
  });

  it("rejects duplicate kind and create-source registrations with exact errors", () => {
    const driver = rawOnlyDriver();

    expect(errorMessage(() => createFileDriverRegistry([driver, driver])))
      .toBe("duplicate file kind: future-client");
    expect(errorMessage(() => createFileDriverRegistry([
      driver,
      rawOnlyDriver("another-client", "future-client"),
    ]))).toBe("duplicate file create source: future-client");
  });

  it("rejects a structured driver without its required adapter", () => {
    const malformed = {
      ...rawOnlyDriver(),
      configuration: { mode: "structured" },
    } as unknown as FileDriverDefinition;

    expect(errorMessage(() => createFileDriverRegistry([malformed])))
      .toBe("file kind future-client requires a configuration adapter");
  });

  it("rejects a creation preset owned by a different driver", () => {
    const malformed: FileDriverDefinition = {
      ...rawOnlyDriver(),
      createPresets: [{
        ...rawOnlyDriver().createPresets[0],
        kind: "another-client",
      }],
    };

    expect(errorMessage(() => createFileDriverRegistry([malformed])))
      .toBe("file creation preset future-client kind must match its driver kind future-client");
  });

  it("retains the defensive unknown-preset-kind validation", () => {
    let kindReads = 0;
    const preset = {
      ...rawOnlyDriver().createPresets[0],
      get kind() {
        kindReads += 1;
        return kindReads === 1 ? "future-client" : "missing-client";
      },
    } as FileCreatePreset;
    const malformed: FileDriverDefinition = {
      ...rawOnlyDriver(),
      createPresets: [preset],
    };

    expect(errorMessage(() => createFileDriverRegistry([malformed])))
      .toBe("unknown file kind for create source future-client: missing-client");
  });

  it("rejects a structured adapter owned by a different driver", () => {
    const malformed: FileDriverDefinition = {
      ...structuredDriver(),
      configuration: {
        mode: "structured",
        adapter: structuredAdapterFixture("another-client"),
      },
    };

    expect(errorMessage(() => createFileDriverRegistry([malformed])))
      .toBe("file configuration adapter kind must match its driver kind future-client");
  });

  it("rejects empty and duplicate processor preset IDs during registry construction", () => {
    expect(errorMessage(() => registryWithPresets([processorPreset("")])))
      .toBe("file processor preset id must not be empty");
    expect(errorMessage(() => registryWithPresets([
      processorPreset("duplicate"),
      processorPreset("duplicate"),
    ]))).toBe("duplicate file processor preset id: duplicate");
  });

  it("rejects missing processor preset dependency and conflict IDs during registry construction", () => {
    expect(errorMessage(() => registryWithPresets([
      processorPreset("dependent", { dependencies: ["missing"] }),
    ]))).toBe("file processor preset dependent has unknown dependency: missing");
    expect(errorMessage(() => registryWithPresets([
      processorPreset("exclusive", { conflicts: ["missing"] }),
    ]))).toBe("file processor preset exclusive has unknown conflict: missing");
  });

  it("rejects processor preset dependency cycles during registry construction", () => {
    expect(errorMessage(() => registryWithPresets([
      processorPreset("first", { dependencies: ["second"] }),
      processorPreset("second", { dependencies: ["first"] }),
    ]))).toBe("file processor preset dependency cycle: first -> second -> first");
  });

  it("rejects processor preset self-dependencies during registry construction", () => {
    expect(errorMessage(() => registryWithPresets([
      processorPreset("self", { dependencies: ["self"] }),
    ]))).toBe("file processor preset self must not depend on itself");
  });
});

function rawOnlyDriver(
  kind = "future-client",
  source = kind,
  order = 100,
): FileDriverDefinition {
  return {
    kind,
    presentation: {
      labelKey: "files.kind.static",
      icon: "file",
    },
    configuration: { mode: "raw" },
    createPresets: [{
      kind,
      source,
      sourceType: "inline",
      order,
      initialName: "future.json",
    }],
    source: {
      defaultBase: () => "{}",
      basePlaceholder: "{}",
      remoteURLPlaceholder: "https://example.com/future.json",
      syntax: "json",
      strategy: "optional-base",
      validate: () => null,
    },
    processors: {
      defaults: () => [],
      mergeModes: ["json_overlay", "json_override"],
      presets: [],
      validate: () => [],
    },
  };
}

function structuredDriver(kind = "future-client"): FileDriverDefinition {
  const raw = rawOnlyDriver(kind);
  return {
    ...raw,
    configuration: {
      mode: "structured",
      adapter: structuredAdapterFixture(kind),
    },
    processors: {
      ...raw.processors,
      presets: [processorPreset("base")],
    },
  };
}

function registryWithPresets(presets: readonly FileProcessorPreset[]) {
  const driver = rawOnlyDriver();
  return createFileDriverRegistry([{
    ...driver,
    processors: { ...driver.processors, presets },
  }]);
}

function processorPreset(
  id: string,
  edges: {
    dependencies?: readonly string[];
    conflicts?: readonly string[];
  } = {},
): FileProcessorPreset {
  return {
    id,
    category: "network",
    labelKey: "files.kind.static",
    descriptionKey: "files.kind.static",
    defaultOn: false,
    dependencies: edges.dependencies ?? [],
    conflicts: edges.conflicts ?? [],
    build: () => ({ type: "merge", params: { content: id } }),
    recognize: (processor) => processor.type === "merge" && processor.params?.content === id,
  };
}

function structuredAdapterFixture(kind: string): StructuredFileConfigurationAdapter {
  return {
    adaptive: {
      typeOptions: [{ label: "Select", value: "select" }],
    },
    groups: {
      typeOptions: [{ label: "Select", value: "select" }],
    },
    kind,
    preview: {},
    references: {
      groupBuiltins: ["DIRECT"],
      rulePolicyBuiltins: ["DIRECT", "REJECT"],
    },
    relations: {},
    ruleSets: {
      formatOptions: [{ label: "YAML", value: "yaml" }],
    },
    rules: {},
    templates: {},
  } as unknown as StructuredFileConfigurationAdapter;
}

function errorMessage(action: () => unknown): string | undefined {
  try {
    action();
  } catch (error) {
    expect(error).toBeInstanceOf(Error);
    return (error as Error).message;
  }
  return undefined;
}
