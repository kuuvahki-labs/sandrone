import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import {
  type FileProcessorPreset,
  type FileProcessorPresetPlan,
  filterForeignManagedProcessors,
  planFileProcessorPresetAddition,
  recognizedFileProcessorPresetID,
} from "./processor-presets";

describe("file processor preset planner", () => {
  it("adds dependencies once in topological order", () => {
    const plan = planFileProcessorPresetAddition(catalog, "mptcp", []);

    expect(plan.addedPresetIDs).toEqual(["tun", "linux-acceleration", "mptcp"]);
    expect(plan.dependencyPresetIDs).toEqual(["tun", "linux-acceleration"]);
    expect(plan.additions.map((item) => item.name)).toEqual(["TUN", "Linux", "MPTCP"]);
  });

  it("atomically removes recognized conflicts and preserves every other relative position", () => {
    const current = [
      custom("before"),
      built("tailscale-external"),
      custom("middle"),
      built("stun"),
      custom("after"),
    ];
    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);

    expect(plan.removeIndices).toEqual([1, 3]);
    expect(plan.removedPresetIDs).toEqual(["tailscale-external", "stun"]);
    expect(applyPlan(current, plan).filter(isCustom).map(nameOf)).toEqual(["before", "middle", "after"]);
  });

  it("never removes an edited processor that no longer matches exactly", () => {
    const edited = { ...built("stun"), params: { mode: "yaml_override", content: "edited" } };
    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", [edited]);

    expect(plan.removeIndices).toEqual([]);
  });

  it("deduplicates removed IDs in processor order for dependency conflicts", () => {
    const current = [built("stun"), custom("middle"), built("stun")];
    const plan = planFileProcessorPresetAddition(catalog, "mptcp", current);

    expect(plan.removeIndices).toEqual([0, 2]);
    expect(plan.removedPresetIDs).toEqual(["stun"]);
  });

  it("makes duplicate requests no-ops without mutating current processors", () => {
    const current = [built("tun"), built("linux-acceleration"), built("mptcp")];
    const before = [...current];
    const plan = planFileProcessorPresetAddition(catalog, "mptcp", current);

    expect(plan).toMatchObject({
      additions: [],
      addedPresetIDs: [],
      dependencyPresetIDs: ["tun", "linux-acceleration"],
      removeIndices: [],
      removedPresetIDs: [],
      requestedPresetID: "mptcp",
    });
    expect(current).toEqual(before);
    current.forEach((processor, index) => expect(processor).toBe(before[index]));
  });

  it("throws when the requested preset does not exist", () => {
    expect(() => planFileProcessorPresetAddition(catalog, "missing", []))
      .toThrowError("unknown file processor preset: missing");
  });

  it("recognizes the first exact preset match and otherwise returns null", () => {
    expect(recognizedFileProcessorPresetID(catalog, built("tun"))).toBe("tun");
    expect(recognizedFileProcessorPresetID(catalog, custom("unknown"))).toBeNull();
  });

  it("filters foreign managed processors while preserving target and unknown values by identity", () => {
    const targetCatalog = [preset("target", "Target")];
    const foreignCatalog = [preset("foreign", "Foreign")];
    const target = targetCatalog[0].build();
    const foreign = foreignCatalog[0].build();
    const unknown = custom("unknown");

    const filtered = filterForeignManagedProcessors(
      targetCatalog,
      [targetCatalog, foreignCatalog],
      [target, foreign, unknown],
    );

    expect(filtered).toEqual([target, unknown]);
    expect(filtered[0]).toBe(target);
    expect(filtered[1]).toBe(unknown);
  });
});

const catalog: readonly FileProcessorPreset[] = [
  preset("tun", "TUN"),
  preset("linux-acceleration", "Linux", { dependencies: ["tun"], conflicts: ["stun"] }),
  preset("mptcp", "MPTCP", { dependencies: ["linux-acceleration", "tun"] }),
  preset("tailscale-external", "Tailscale External"),
  preset("stun", "STUN"),
  preset("tailscale-native", "Tailscale Native", {
    conflicts: ["tailscale-external", "stun"],
  }),
];

function preset(
  id: string,
  name: string,
  edges: {
    dependencies?: readonly string[];
    conflicts?: readonly string[];
  } = {},
): FileProcessorPreset {
  const expected = builtProcessor(id, name);
  return {
    id,
    category: "network",
    labelKey: "files.kind.static",
    descriptionKey: "files.kind.static",
    defaultOn: false,
    dependencies: edges.dependencies ?? [],
    conflicts: edges.conflicts ?? [],
    build: () => builtProcessor(id, name),
    recognize: (processor) => (
      processor.type === expected.type
      && exactParams(processor.params, expected.params)
    ),
  };
}

function built(id: string): ProcessorDetail {
  const descriptor = catalog.find((candidate) => candidate.id === id);
  if (!descriptor) throw new Error(`missing test preset: ${id}`);
  return descriptor.build();
}

function builtProcessor(id: string, name: string): ProcessorDetail {
  return {
    name,
    type: "merge",
    stage: "file",
    params: { mode: "yaml_override", content: `# preset:${id}` },
  };
}

function exactParams(
  actual: ProcessorDetail["params"],
  expected: ProcessorDetail["params"],
): boolean {
  const actualEntries = Object.entries(actual ?? {});
  const expectedEntries = Object.entries(expected ?? {});
  return actualEntries.length === expectedEntries.length
    && expectedEntries.every(([key, value]) => actual?.[key] === value);
}

function custom(name: string): ProcessorDetail {
  return {
    name,
    type: "script",
    stage: "file",
    params: { content: name },
  };
}

function applyPlan(
  current: readonly ProcessorDetail[],
  plan: FileProcessorPresetPlan,
): ProcessorDetail[] {
  const removals = new Set(plan.removeIndices);
  return [
    ...current.filter((_, index) => !removals.has(index)),
    ...plan.additions,
  ];
}

function isCustom(processor: ProcessorDetail): boolean {
  return processor.type === "script";
}

function nameOf(processor: ProcessorDetail): string | undefined {
  return processor.name;
}
