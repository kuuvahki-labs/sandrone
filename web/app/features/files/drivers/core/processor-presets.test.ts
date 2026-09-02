import { describe, expect, it } from "vitest";

import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

import {
  type FileProcessorPreset,
  type FileProcessorPresetPlan,
  filterForeignManagedProcessors,
  planFileProcessorPresetAddition as buildPresetPlan,
  recognizedFileProcessorPresetID,
} from "./processor-presets";

describe("file processor preset planner", () => {
  it("adds dependencies once in topological order", () => {
    const plan = planFileProcessorPresetAddition(catalog, "mptcp", []);

    expect(plan.addedPresetIDs).toEqual(["tun", "linux-acceleration", "mptcp"]);
    expect(plan.dependencyPresetIDs).toEqual(["tun", "linux-acceleration"]);
    expect(plan.additions.map((item) => item.processor.name)).toEqual(["TUN", "Linux", "MPTCP"]);
    expect(plan.additions.map((item) => item.beforeIndex)).toEqual([null, null, null]);
  });

  it("inserts missing dependencies before an existing managed consumer", () => {
    const before = custom("before");
    const native = built("tailscale-native");
    const after = custom("after");
    const current = [before, native, after];

    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);

    expect(plan.additions).toEqual([{
      presetID: "native-base",
      processor: built("native-base"),
      beforeIndex: 1,
    }]);
    expect(plan.removeIndices).toEqual([]);
    const applied = applyPlan(current, plan);
    expect(applied.map(nameOf)).toEqual(["before", "Native Base", "Tailscale Native", "after"]);
    expect(applied[0]).toBe(before);
    expect(applied[2]).toBe(native);
    expect(applied[3]).toBe(after);
  });

  it("keeps same-slot additions in stable topological order", () => {
    const mptcp = built("mptcp");

    const plan = planFileProcessorPresetAddition(catalog, "mptcp", [mptcp]);

    expect(plan.additions.map(({ presetID, beforeIndex }) => ({ presetID, beforeIndex })))
      .toEqual([
        { presetID: "tun", beforeIndex: 0 },
        { presetID: "linux-acceleration", beforeIndex: 0 },
      ]);
    expect(applyPlan([mptcp], plan).map(nameOf)).toEqual(["TUN", "Linux", "MPTCP"]);
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

  it("cascades conflict removal through exact managed reverse dependents", () => {
    const transitive = built("tailnet-access");
    const external = built("tailscale-external");
    const share = built("tailnet-share");
    const current = [transitive, custom("middle"), external, share];

    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);

    expect(plan.removeIndices).toEqual([0, 2, 3]);
    expect(plan.removedPresetIDs).toEqual([
      "tailnet-access",
      "tailscale-external",
      "tailnet-share",
    ]);
  });

  it("preserves edited and unrecognized reverse dependents", () => {
    const editedShare = {
      ...built("tailnet-share"),
      params: { mode: "yaml_override", content: "edited share" },
    };
    const unknown = custom("unknown");
    const current = [built("tailscale-external"), editedShare, unknown];

    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);

    expect(plan.removeIndices).toEqual([0]);
    const applied = applyPlan(current, plan);
    expect(applied.filter((processor) => processor === editedShare || processor === unknown))
      .toEqual([editedShare, unknown]);
  });

  it("does not cascade a dependent when the requested closure restores its dependency", () => {
    const native = built("tailscale-native");

    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", [native]);

    expect(plan.removeIndices).toEqual([]);
    expect(plan.addedPresetIDs).toEqual(["native-base"]);
    expect(applyPlan([native], plan)).toEqual([built("native-base"), native]);
  });

  it("preserves every surviving current processor by identity and relative order", () => {
    const before = custom("before");
    const unrelated = built("unrelated");
    const editedShare = {
      ...built("tailnet-share"),
      params: { mode: "yaml_override", content: "edited share" },
    };
    const after = custom("after");
    const current = [before, built("tailscale-external"), unrelated, editedShare, after];

    const plan = planFileProcessorPresetAddition(catalog, "tailscale-native", current);
    const applied = applyPlan(current, plan);
    const survivingCurrent = applied.filter((processor) => current.includes(processor));

    expect(survivingCurrent).toEqual([before, unrelated, editedShare, after]);
    expect(survivingCurrent[0]).toBe(before);
    expect(survivingCurrent[1]).toBe(unrelated);
    expect(survivingCurrent[2]).toBe(editedShare);
    expect(survivingCurrent[3]).toBe(after);
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
      updatedPresetIDs: [],
      requestedPresetID: "mptcp",
    });
    expect(current).toEqual(before);
    current.forEach((processor, index) => expect(processor).toBe(before[index]));
  });

  it("replaces a recognized stale preset in place when explicitly requested", () => {
    const versioned = versionedPreset("versioned", "Versioned");
    const versionedCatalog = [...catalog, versioned];
    const stale = {
      ...versioned.build(t),
      params: { mode: "yaml_override", content: "# preset:versioned\nold" },
    };
    const before = custom("before");
    const after = custom("after");

    const plan = planFileProcessorPresetAddition(versionedCatalog, "versioned", [before, stale, after]);

    expect(plan.removeIndices).toEqual([1]);
    expect(plan.removedPresetIDs).toEqual([]);
    expect(plan.updatedPresetIDs).toEqual(["versioned"]);
    expect(plan.additions).toEqual([{
      presetID: "versioned",
      processor: versioned.build(t),
      beforeIndex: 1,
    }]);
    expect(applyPlan([before, stale, after], plan).map(nameOf))
      .toEqual(["before", "Versioned", "after"]);
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
    const target = targetCatalog[0].build(t);
    const foreign = foreignCatalog[0].build(t);
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
  preset("native-base", "Native Base"),
  preset("tailscale-external", "Tailscale External"),
  preset("tailnet-share", "Tailnet Share", { dependencies: ["tailscale-external"] }),
  preset("tailnet-access", "Tailnet Access", { dependencies: ["tailnet-share"] }),
  preset("stun", "STUN"),
  preset("tailscale-native", "Tailscale Native", {
    dependencies: ["native-base"],
    conflicts: ["tailscale-external", "stun"],
  }),
  preset("unrelated", "Unrelated"),
];

const t: Translator = (_key, params) => String(params?.presetName ?? "");

function planFileProcessorPresetAddition(
  presetCatalog: readonly FileProcessorPreset[],
  requestedPresetID: string,
  current: readonly ProcessorDetail[],
): FileProcessorPresetPlan {
  return buildPresetPlan(presetCatalog, requestedPresetID, current, t);
}

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
    defaultOn: false,
    dependencies: edges.dependencies ?? [],
    conflicts: edges.conflicts ?? [],
    build: (translator) => builtProcessor(
      id,
      translator("files.kind.static", { presetName: name }),
    ),
    recognize: (processor) => (
      processor.type === expected.type
      && exactParams(processor.params, expected.params)
    ),
  };
}

function versionedPreset(id: string, name: string): FileProcessorPreset {
  const current = builtProcessor(id, name);
  const marker = `# preset:${id}`;
  const recognize = (processor: Pick<ProcessorDetail, "type" | "params">) => (
    processor.type === "merge"
    && processor.params?.mode === "yaml_override"
    && typeof processor.params.content === "string"
    && processor.params.content.startsWith(marker)
  );
  return {
    id,
    category: "network",
    labelKey: "files.kind.static",
    defaultOn: false,
    dependencies: [],
    conflicts: [],
    build: () => current,
    recognize,
    isCurrent: (processor) => recognize(processor)
      && processor.params?.content === current.params?.content,
  };
}

function built(id: string): ProcessorDetail {
  const descriptor = catalog.find((candidate) => candidate.id === id);
  if (!descriptor) throw new Error(`missing test preset: ${id}`);
  return descriptor.build(t);
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
  const additionsByIndex = new Map<number | null, ProcessorDetail[]>();
  for (const addition of plan.additions) {
    const additions = additionsByIndex.get(addition.beforeIndex) ?? [];
    additions.push(addition.processor);
    additionsByIndex.set(addition.beforeIndex, additions);
  }
  const applied: ProcessorDetail[] = [];
  current.forEach((processor, index) => {
    applied.push(...(additionsByIndex.get(index) ?? []));
    if (!removals.has(index)) applied.push(processor);
  });
  applied.push(...(additionsByIndex.get(null) ?? []));
  return applied;
}

function isCustom(processor: ProcessorDetail): boolean {
  return processor.type === "script";
}

function nameOf(processor: ProcessorDetail): string | undefined {
  return processor.name;
}
