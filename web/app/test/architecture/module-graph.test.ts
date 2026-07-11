import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";

import { describe, expect, it } from "vitest";

import {
  filterModuleGraph,
  findBoundaryViolations,
  findCycles,
  type ModuleGraph,
  type ModuleNode,
  reachableModulePaths,
  readModuleGraph,
} from "./module-graph";

function withFixture<T>(files: Readonly<Record<string, string>>, run: (appDir: string) => T): T {
  const fixtureDir = mkdtempSync(join(tmpdir(), "sandrone-module-graph-"));

  try {
    for (const [relativePath, source] of Object.entries(files)) {
      const filePath = join(fixtureDir, relativePath);
      mkdirSync(dirname(filePath), { recursive: true });
      writeFileSync(filePath, source);
    }
    return run(fixtureDir);
  } finally {
    rmSync(fixtureDir, { recursive: true, force: true });
  }
}

function node(path: string, imports: ModuleNode["imports"]): ModuleNode {
  return { path, imports };
}

describe("readModuleGraph", () => {
  it("retains every supported literal reference and resolves app targets", () => {
    withFixture(
      {
        "entry.ts": `
          import value from "~/feature/value";
          import type { AliasType } from "~/types/alias";
          import { type RelativeType } from "./types";
          import { type MixedType, mixedValue } from "./mixed";
          export { exportedValue } from "./exported";
          export type { ExportedType } from "./exported-type";
          export * from "./export-all";
          import equalValue = require("./equals");
          import type equalType = require("./equals-type");
          type ImportedType = import("./imported-type").ImportedType;
          void import("./dynamic");
          require("./required");
          import React from "react";
          import type { Route } from "./+types/entry";
          void value;
          void mixedValue;
          void exportedValue;
          void equalValue;
          void React;
        `,
        "dynamic.ts": "export {};",
        "equals.ts": "export = {};",
        "equals-type.ts": "declare const EqualType: unique symbol; export = EqualType;",
        "export-all/index.tsx": "export const all = true;",
        "exported-type.ts": "export interface ExportedType {}",
        "exported.ts": "export const exportedValue = true;",
        "feature/value.tsx": "export default 1;",
        "imported-type.ts": "export interface ImportedType {}",
        "mixed.ts": "export interface MixedType {}; export const mixedValue = true;",
        "required/index.ts": "export {};",
        "types.ts": "export interface RelativeType {}",
        "types/alias/index.ts": "export interface AliasType {}",
      },
      (fixtureDir) => {
        const imports = readModuleGraph(fixtureDir).get("entry.ts")?.imports;

        expect(imports).toHaveLength(14);
        expect(imports).toEqual(expect.arrayContaining([
          { source: "./+types/entry", typeOnly: true },
          { source: "./dynamic", target: "dynamic.ts", typeOnly: false },
          { source: "./equals", target: "equals.ts", typeOnly: false },
          { source: "./equals-type", target: "equals-type.ts", typeOnly: true },
          { source: "./export-all", target: "export-all/index.tsx", typeOnly: false },
          { source: "./exported", target: "exported.ts", typeOnly: false },
          { source: "./exported-type", target: "exported-type.ts", typeOnly: true },
          { source: "./imported-type", target: "imported-type.ts", typeOnly: true },
          { source: "./mixed", target: "mixed.ts", typeOnly: false },
          { source: "./required", target: "required/index.ts", typeOnly: false },
          { source: "./types", target: "types.ts", typeOnly: true },
          { source: "~/feature/value", target: "feature/value.tsx", typeOnly: false },
          { source: "~/types/alias", target: "types/alias/index.ts", typeOnly: true },
          { source: "react", typeOnly: false },
        ]));
      },
    );
  });

  it("visits nested import type arguments without duplicating the outer edge", () => {
    withFixture(
      {
        "entry.ts": 'type Result = import("./outer").Outer<import("./inner").Inner>;',
        "inner.ts": "export interface Inner {}",
        "outer.ts": "export interface Outer<T> { value: T }",
      },
      (fixtureDir) => {
        expect(readModuleGraph(fixtureDir).get("entry.ts")?.imports).toEqual([
          { source: "./inner", target: "inner.ts", typeOnly: true },
          { source: "./outer", target: "outer.ts", typeOnly: true },
        ]);
      },
    );
  });

  it("keeps unresolved CSS and generated imports targetless", () => {
    withFixture(
      { "entry.ts": 'import "./styles.css"; import type { Route } from "./+types/entry";' },
      (fixtureDir) => {
        expect(readModuleGraph(fixtureDir).get("entry.ts")?.imports).toEqual([
          { source: "./+types/entry", typeOnly: true },
          { source: "./styles.css", typeOnly: false },
        ]);
      },
    );
  });

  it.each([
    ["dynamic import", "const moduleName = './target'; void import(moduleName);"],
    ["require", "const moduleName = './target'; require(moduleName);"],
  ])("rejects a non-literal %s", (referenceKind, source) => {
    withFixture({ "entry.ts": source }, (fixtureDir) => {
      expect(() => readModuleGraph(fixtureDir)).toThrowError(
        new RegExp(`entry\\.ts.*${referenceKind}.*string literal`, "i"),
      );
    });
  });
});

describe("filterModuleGraph", () => {
  it("keeps selected nodes, targetless imports, and excluded-target evidence deterministically", () => {
    const graph: ModuleGraph = new Map([
      ["test/fixture.ts", node("test/fixture.ts", [])],
      [
        "shared/model.ts",
        node("shared/model.ts", [
          { source: "~/shared/value", target: "shared/value.ts", typeOnly: false },
          { source: "~/test/fixture", target: "test/fixture.ts", typeOnly: true },
          { source: "react", typeOnly: false },
        ]),
      ],
      ["shared/value.ts", node("shared/value.ts", [])],
    ]);

    expect([...filterModuleGraph(graph, (path) => path.startsWith("shared/")).entries()])
      .toEqual([
        [
          "shared/model.ts",
          node("shared/model.ts", [
            { source: "react", typeOnly: false },
            { source: "~/shared/value", target: "shared/value.ts", typeOnly: false },
            { source: "~/test/fixture", target: "test/fixture.ts", typeOnly: true },
          ]),
        ],
        ["shared/value.ts", node("shared/value.ts", [])],
      ]);
  });

  it("retains a production-to-test violation without traversing the excluded cycle", () => {
    const graph: ModuleGraph = new Map([
      [
        "features/files/page.ts",
        node("features/files/page.ts", [
          { source: "~/test/helper", target: "test/helper.ts", typeOnly: false },
        ]),
      ],
      [
        "test/helper.ts",
        node("test/helper.ts", [
          { source: "~/features/files/page", target: "features/files/page.ts", typeOnly: false },
        ]),
      ],
    ]);
    const production = filterModuleGraph(graph, (path) => !path.startsWith("test/"));

    expect(production.get("features/files/page.ts")?.imports).toEqual([
      { source: "~/test/helper", target: "test/helper.ts", typeOnly: false },
    ]);
    expect(findBoundaryViolations(production, {
      allow: (_from, to) => to.startsWith("features/files/"),
    })).toEqual([
      { from: "features/files/page.ts", to: "test/helper.ts" },
    ]);
    expect(findCycles(production, false)).toEqual([]);
    expect(findCycles(production, true)).toEqual([]);
  });

  it("does not report an excluded edge when production imports stay in production", () => {
    const graph: ModuleGraph = new Map([
      [
        "features/files/page.ts",
        node("features/files/page.ts", [
          { source: "~/shared/model", target: "shared/model.ts", typeOnly: false },
        ]),
      ],
      ["shared/model.ts", node("shared/model.ts", [])],
      ["test/helper.ts", node("test/helper.ts", [])],
    ]);
    const production = filterModuleGraph(graph, (path) => !path.startsWith("test/"));

    expect(findBoundaryViolations(production, {
      allow: (_from, to) => to.startsWith("features/") || to.startsWith("shared/"),
    })).toEqual([]);
    expect(findCycles(production, true)).toEqual([]);
  });
});

describe("reachableModulePaths", () => {
  it("follows resolved runtime or type-inclusive edges and returns sorted unique paths", () => {
    const graph: ModuleGraph = new Map([
      [
        "root.ts",
        node("root.ts", [
          { source: "./runtime", target: "runtime.ts", typeOnly: false },
          { source: "./types", target: "types.ts", typeOnly: true },
          { source: "react", typeOnly: false },
        ]),
      ],
      ["runtime.ts", node("runtime.ts", [{ source: "./leaf", target: "leaf.ts", typeOnly: false }])],
      ["types.ts", node("types.ts", [{ source: "./leaf", target: "leaf.ts", typeOnly: true }])],
      ["leaf.ts", node("leaf.ts", [])],
    ]);

    expect(reachableModulePaths(graph, ["missing.ts", "root.ts", "root.ts"], false))
      .toEqual(["leaf.ts", "root.ts", "runtime.ts"]);
    expect(reachableModulePaths(graph, ["root.ts"], true))
      .toEqual(["leaf.ts", "root.ts", "runtime.ts", "types.ts"]);
  });

  it("exposes external React and MUI imports reached through a type-only shared edge", () => {
    withFixture(
      {
        "shared/processors/model.ts": 'import type { FieldProps } from "~/shared/ui/form-fields"; export type Processor = FieldProps;',
        "shared/ui/form-fields.tsx": 'import type { ReactNode } from "react"; import Button from "@mui/material/Button"; export interface FieldProps { child?: ReactNode }; void Button;',
      },
      (fixtureDir) => {
        const graph = readModuleGraph(fixtureDir);

        expect(reachableModulePaths(graph, ["shared/processors/model.ts"], false))
          .toEqual(["shared/processors/model.ts"]);
        expect(reachableModulePaths(graph, ["shared/processors/model.ts"], true))
          .toEqual(["shared/processors/model.ts", "shared/ui/form-fields.tsx"]);
        expect(graph.get("shared/ui/form-fields.tsx")?.imports).toEqual(expect.arrayContaining([
          { source: "@mui/material/Button", typeOnly: false },
          { source: "react", typeOnly: true },
        ]));
      },
    );
  });
});

describe("findCycles", () => {
  it("finds self, ordinary, and optionally type-only cycles", () => {
    const graph: ModuleGraph = new Map([
      ["types/b.ts", node("types/b.ts", [{ source: "./a", target: "types/a.ts", typeOnly: true }])],
      ["ordinary/c.ts", node("ordinary/c.ts", [{ source: "./a", target: "ordinary/a.ts", typeOnly: false }])],
      ["self.ts", node("self.ts", [{ source: "./self", target: "self.ts", typeOnly: false }])],
      ["ordinary/a.ts", node("ordinary/a.ts", [{ source: "./b", target: "ordinary/b.ts", typeOnly: false }])],
      ["types/a.ts", node("types/a.ts", [{ source: "./b", target: "types/b.ts", typeOnly: true }])],
      ["ordinary/b.ts", node("ordinary/b.ts", [{ source: "./c", target: "ordinary/c.ts", typeOnly: false }])],
      ["acyclic.ts", node("acyclic.ts", [])],
    ]);

    expect(findCycles(graph, false)).toEqual([
      ["ordinary/a.ts", "ordinary/b.ts", "ordinary/c.ts"],
      ["self.ts"],
    ]);
    expect(findCycles(graph, true)).toEqual([
      ["ordinary/a.ts", "ordinary/b.ts", "ordinary/c.ts"],
      ["self.ts"],
      ["types/a.ts", "types/b.ts"],
    ]);
  });

  it("returns deterministically sorted components regardless of insertion order", () => {
    const modules: Array<readonly [string, ModuleNode]> = [
      ["z.ts", node("z.ts", [{ source: "./y", target: "y.ts", typeOnly: false }])],
      ["y.ts", node("y.ts", [{ source: "./z", target: "z.ts", typeOnly: false }])],
      ["b.ts", node("b.ts", [{ source: "./a", target: "a.ts", typeOnly: false }])],
      ["a.ts", node("a.ts", [{ source: "./b", target: "b.ts", typeOnly: false }])],
    ];

    expect(findCycles(new Map(modules), false)).toEqual([["a.ts", "b.ts"], ["y.ts", "z.ts"]]);
    expect(findCycles(new Map([...modules].reverse()), false)).toEqual([["a.ts", "b.ts"], ["y.ts", "z.ts"]]);
  });
});

describe("findBoundaryViolations", () => {
  it("reports sorted unique resolved edges rejected by policy and ignores externals", () => {
    const graph: ModuleGraph = new Map([
      [
        "features/files/page.ts",
        node("features/files/page.ts", [
          { source: "~/shared/ui/page", target: "shared/ui/page.ts", typeOnly: false },
          { source: "~/features/files/model", target: "features/files/model.ts", typeOnly: true },
          { source: "~/features/shares/model", target: "features/shares/model.ts", typeOnly: false },
          { source: "~/features/shares/model", target: "features/shares/model.ts", typeOnly: true },
          { source: "react", typeOnly: false },
        ]),
      ],
      ["features/files/model.ts", node("features/files/model.ts", [])],
      ["features/shares/model.ts", node("features/shares/model.ts", [])],
      ["shared/ui/page.ts", node("shared/ui/page.ts", [])],
    ]);

    expect(findBoundaryViolations(graph, {
      allow: (from, to) => {
        if (!from.startsWith("features/")) return true;
        const feature = from.split("/")[1];
        return to.startsWith("shared/") || to.startsWith(`features/${feature}/`);
      },
    })).toEqual([
      { from: "features/files/page.ts", to: "features/shares/model.ts" },
    ]);
  });
});
