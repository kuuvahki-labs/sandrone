import { spawnSync } from "node:child_process";
import { mkdirSync, mkdtempSync, readdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

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

type ReachableModulePaths = (
  graph: ModuleGraph,
  roots: readonly string[],
  includeTypeOnly: boolean,
) => string[];

const architectureDir = dirname(fileURLToPath(import.meta.url));
const appDir = resolve(architectureDir, "../..");
const repoDir = resolve(appDir, "../..");
const globalDataExclude = join(architectureDir, "global-data.exclude");
const repositoryIgnoreFiles = trackedRepositoryIgnoreFiles();
const appGraph = readModuleGraph(appDir);

function trackedRepositoryIgnoreFiles(): Set<string> {
  const result = spawnSync(
    "git",
    ["ls-files", "--cached", "-z", "--", "*.gitignore"],
    { cwd: repoDir, encoding: "utf8" },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`git ls-files failed while reading repository ignore files: ${result.stderr}`);
  }
  return new Set(
    result.stdout
      .split("\0")
      .filter(Boolean)
      .map((path) => resolve(repoDir, path)),
  );
}

function gitIgnores(
  path: string,
  env: typeof process.env = process.env,
): boolean {
  const result = spawnSync(
    "git",
    ["check-ignore", "--no-index", "--stdin", "--verbose", "-z"],
    { cwd: repoDir, encoding: "utf8", env, input: `${path}\0` },
  );
  if (result.error) throw result.error;
  if (result.status === 1) return false;
  if (result.status === 0) {
    const [source, , pattern] = result.stdout.split("\0");
    return Boolean(source)
      && Boolean(pattern)
      && repositoryIgnoreFiles.has(resolve(repoDir, source))
      && !pattern.startsWith("!");
  }
  throw new Error(`git check-ignore failed for ${path}: ${result.stderr}`);
}

function isProductionModule(path: string): boolean {
  return !path.startsWith("test/")
    && !path.includes(".test.")
    && !path.includes(".dom.test.")
    && !/(?:^|\/)test-data\.tsx$/u.test(path);
}

function productionImportsExcludedModules(graph: ModuleGraph): Array<{ from: string; to: string }> {
  return [...graph.values()]
    .filter((module) => isProductionModule(module.path))
    .flatMap((module) => module.imports
      .filter((moduleImport) => moduleImport.target && !isProductionModule(moduleImport.target))
      .map((moduleImport) => ({ from: module.path, to: moduleImport.target! })))
    .sort((left, right) => left.from.localeCompare(right.from) || left.to.localeCompare(right.to));
}

function allowedLayerDependency(from: string, to: string): boolean {
  if (from.startsWith("shared/")) return to.startsWith("shared/");
  if (from.startsWith("features/")) {
    const feature = from.split("/")[1];
    return to.startsWith("shared/") || to.startsWith(`features/${feature}/`);
  }
  if (from.startsWith("core/")) {
    return to.startsWith("shared/") || to.startsWith("features/") || to.startsWith("core/");
  }
  if (from.startsWith("routes/")) {
    return to.startsWith("shared/") || to.startsWith("features/") || to.startsWith("core/");
  }
  return true;
}

function node(path: string, targets: readonly string[]): ModuleNode {
  return {
    path,
    imports: targets.map((target) => ({ source: `~/${target}`, target, typeOnly: false })),
  };
}

function isReactOrMui(source: string): boolean {
  return source === "react"
    || source.startsWith("react/")
    || source === "@mui/material"
    || source === "@mui/icons-material"
    || source.startsWith("@mui/");
}

function pureClosureViolations(
  graph: ModuleGraph,
  root: string,
  reach: ReachableModulePaths,
): string[] {
  const violations: string[] = [];
  for (const modulePath of reach(graph, [root], true)) {
    if (!modulePath.startsWith("shared/")) violations.push(`${root} -> ${modulePath}`);
    for (const moduleImport of graph.get(modulePath)?.imports ?? []) {
      if (isReactOrMui(moduleImport.source)) {
        violations.push(`${modulePath} -> ${moduleImport.source}`);
      }
    }
  }
  return violations.sort();
}

function isDriverUI(path: string): boolean {
  return path.startsWith("features/files/editor/")
    || path.startsWith("features/files/config/components/")
    || /^features\/files\/drivers\/[^/]+\/fields(?:\/|\.tsx?$)/u.test(path);
}

function driverUIViolations(
  graph: ModuleGraph,
  roots: readonly string[],
  allowSharedTranslation = false,
): string[] {
  // Type-only references to shared contracts do not load their implementation.
  const runtimePaths = reachableModulePaths(graph, roots, false);
  return [
    ...reachableModulePaths(graph, roots, true).filter(isDriverUI),
    ...runtimePaths.flatMap((path) => (graph.get(path)?.imports ?? [])
      .filter((dependency) => isReactOrMui(dependency.source))
      // Domain configuration uses translate from the existing shared context
      // module. Other indirect UI imports still violate the driver boundary.
      .filter((dependency) => !(allowSharedTranslation
        && path === "shared/i18n/context.tsx" && dependency.source === "react"))
      .map((dependency) => `${path} -> ${dependency.source}`)),
  ].sort();
}

function concreteDriverPaths(graph: ModuleGraph, roots: readonly string[]): string[] {
  return reachableModulePaths(graph, roots, true)
    .filter((path) => /^features\/files\/drivers\/(?!core\/)[^/]+\//u.test(path));
}

function withModules(files: Record<string, string>, run: (graph: ModuleGraph) => void) {
  const fixtureDir = mkdtempSync(join(tmpdir(), "sandrone-boundaries-"));
  try {
    for (const [path, source] of Object.entries(files)) {
      const filename = join(fixtureDir, path);
      mkdirSync(dirname(filename), { recursive: true });
      writeFileSync(filename, source);
    }
    run(readModuleGraph(fixtureDir));
  } finally {
    rmSync(fixtureDir, { recursive: true, force: true });
  }
}

describe("extensible module boundaries", () => {
  it("allows new features, barrels, route re-exports and pure registry helpers", () => {
    withModules({
      "routes/new.tsx": 'export { default } from "../features/new/page";',
      "features/new/page.tsx": 'export { default } from "./components";',
      "features/new/components/index.ts": 'export { default } from "./page";',
      "features/new/components/page.tsx": 'import Button from "@mui/material/Button"; export default Button;',
      "features/files/drivers/registry.ts": 'export * from "./core"; export { driver } from "./new/driver";',
      "features/files/drivers/new/driver.ts": 'export { helper as driver } from "../core";',
      "features/files/drivers/core/index.ts": 'export * from "./helper";',
      "features/files/drivers/core/helper.ts": 'export { helper } from "~/shared/helpers/new";',
      "shared/helpers/new.ts": 'import { z } from "zod"; export const helper = z.string();',
    }, (graph) => {
      expect(findBoundaryViolations(graph, { allow: allowedLayerDependency })).toEqual([]);
      expect(findCycles(graph, true)).toEqual([]);
      expect(driverUIViolations(graph, ["features/files/drivers/registry.ts"])).toEqual([]);
      expect(concreteDriverPaths(graph, ["features/files/drivers/core/index.ts"])).toEqual([]);
    });
  });

  it("detects layer violations and cycles through re-exports", () => {
    withModules({
      "shared/index.ts": 'export * from "../features/new/model";',
      "features/new/model.ts": 'export * from "~/shared";',
    }, (graph) => {
      expect(findBoundaryViolations(graph, { allow: allowedLayerDependency })).toEqual([
        { from: "shared/index.ts", to: "features/new/model.ts" },
      ]);
      expect(findCycles(graph, true)).toEqual([["features/new/model.ts", "shared/index.ts"]]);
    });
  });

  it("detects UI and concrete driver dependencies through pure helper re-exports", () => {
    withModules({
      "features/files/drivers/core/index.ts": 'export * from "~/shared/helper"; export * from "../new/driver";',
      "shared/helper.ts": 'import type { ReactNode } from "react"; export type Props = ReactNode;',
      "features/files/drivers/new/driver.ts": 'export * from "./fields";',
      "features/files/drivers/new/fields.tsx": 'export { default } from "@mui/material/Button";',
    }, (graph) => {
      const roots = ["features/files/drivers/core/index.ts"];
      expect(driverUIViolations(graph, roots)).toEqual([
        "features/files/drivers/new/fields.tsx",
        "features/files/drivers/new/fields.tsx -> @mui/material/Button",
        "shared/helper.ts -> react",
      ]);
      expect(concreteDriverPaths(graph, roots)).toEqual([
        "features/files/drivers/new/driver.ts",
        "features/files/drivers/new/fields.tsx",
      ]);
    });
  });

  it("allows shared translations but rejects UI hidden behind arbitrary driver helpers", () => {
    withModules({
      "features/files/drivers/new/driver.ts": 'export * from "./helpers"; export { translate } from "~/shared/i18n/context";',
      "features/files/drivers/new/helpers.ts": 'export { default } from "./presentation";',
      "features/files/drivers/new/presentation.tsx": 'export { default } from "@mui/material/Button";',
      "shared/i18n/context.tsx": 'import { createContext } from "react"; export const context = createContext(null); export const translate = (key: string) => key;',
    }, (graph) => {
      expect(driverUIViolations(graph, ["shared/i18n/context.tsx"], true)).toEqual([]);
      expect(driverUIViolations(graph, ["features/files/drivers/new/driver.ts"], true)).toEqual([
        "features/files/drivers/new/presentation.tsx -> @mui/material/Button",
      ]);
    });
  });
});

describe("FileDriver boundaries", () => {
  const productionPaths = [...appGraph.keys()].filter(isProductionModule);
  const corePaths = productionPaths.filter((path) => path.startsWith("features/files/drivers/core/"));
  const driverRoots = productionPaths.filter((path) => /^features\/files\/drivers\/[^/]+\/driver\.ts$/u.test(path));

  it("keeps pure core independent from UI and concrete drivers", () => {
    expect(driverUIViolations(appGraph, corePaths)).toEqual([]);
    expect(concreteDriverPaths(appGraph, corePaths)).toEqual([]);
  });

  it("keeps domain drivers and their registry independent from UI", () => {
    const roots = [...driverRoots, "features/files/drivers/registry.ts"];
    expect(driverUIViolations(appGraph, roots, true)).toEqual([]);
  });

  it("keeps the shared workbench independent from concrete fields and UI composition", () => {
    const configPaths = productionPaths
      .filter((path) => /^features\/files\/config\/(?:model|components)\//u.test(path));
    const violations = reachableModulePaths(appGraph, configPaths, true)
      .filter((path) => path === "features/files/editor/file-driver-ui-registry.ts"
        || /^features\/files\/drivers\/[^/]+\/fields(?:\/|\.tsx?$)/u.test(path));
    expect(violations).toEqual([]);
  });
});

describe("repository ignore boundaries", () => {
  it("ignores runtime data without hiding feature-owned data modules", () => {
    expect(gitIgnores("data/.architecture-probe")).toBe(true);
    expect(gitIgnores("internal/entry/cli/data/.architecture-probe")).toBe(true);
    for (const feature of readdirSync(join(appDir, "features"))) {
      expect(gitIgnores(`web/app/features/${feature}/data/.architecture-probe`), feature)
        .toBe(false);
    }
  });

  it("evaluates repository ignore policy independently from global excludes", () => {
    const env = {
      ...process.env,
      GIT_CONFIG_COUNT: "1",
      GIT_CONFIG_KEY_0: "core.excludesFile",
      GIT_CONFIG_VALUE_0: globalDataExclude,
    };

    expect(gitIgnores("web/app/features/files/data/.architecture-probe", env))
      .toBe(false);
  });

  it("uses nested repository ignore files and honors their negated matches", () => {
    expect(gitIgnores("web/app/test/architecture/ignore-fixture/hidden/.architecture-probe"))
      .toBe(true);
    expect(gitIgnores("web/app/test/architecture/ignore-fixture/visible/.architecture-probe"))
      .toBe(false);
  });
});

describe("application dependency graph", () => {
  it("demonstrates every forbidden layer direction in a policy fixture", () => {
    const graph: ModuleGraph = new Map([
      ["shared/a.ts", node("shared/a.ts", ["features/files/a.ts", "core/a.ts", "routes/a.tsx"])],
      ["features/files/a.ts", node("features/files/a.ts", ["features/shares/a.ts", "core/a.ts", "routes/a.tsx"])],
      ["features/shares/a.ts", node("features/shares/a.ts", [])],
      ["core/a.ts", node("core/a.ts", ["routes/a.tsx", "root.tsx"])],
      ["routes/a.tsx", node("routes/a.tsx", ["routes/b.tsx"])],
      ["routes/b.tsx", node("routes/b.tsx", [])],
      ["root.tsx", node("root.tsx", [])],
    ]);

    expect(findBoundaryViolations(graph, { allow: allowedLayerDependency })).toEqual([
      { from: "core/a.ts", to: "root.tsx" },
      { from: "core/a.ts", to: "routes/a.tsx" },
      { from: "features/files/a.ts", to: "core/a.ts" },
      { from: "features/files/a.ts", to: "features/shares/a.ts" },
      { from: "features/files/a.ts", to: "routes/a.tsx" },
      { from: "routes/a.tsx", to: "routes/b.tsx" },
      { from: "shared/a.ts", to: "core/a.ts" },
      { from: "shared/a.ts", to: "features/files/a.ts" },
      { from: "shared/a.ts", to: "routes/a.tsx" },
    ]);
  });

  it("enforces the final policy over production runtime and type-only edges", () => {
    const productionGraph = filterModuleGraph(appGraph, isProductionModule);

    expect(findBoundaryViolations(productionGraph, { allow: allowedLayerDependency })).toEqual([]);
    expect(findCycles(productionGraph, false)).toEqual([]);
    expect(findCycles(productionGraph, true)).toEqual([]);
  });

  it("rejects production imports of every excluded test-module class", () => {
    const graph: ModuleGraph = new Map([
      [
        "features/files/page.ts",
        {
          path: "features/files/page.ts",
          imports: [
            { source: "~/features/files/browser.dom.test", target: "features/files/browser.dom.test.ts", typeOnly: false },
            { source: "~/features/files/model.test", target: "features/files/model.test.ts", typeOnly: true },
            { source: "~/features/files/test-data", target: "features/files/test-data.tsx", typeOnly: false },
            { source: "~/shared/model", target: "shared/model.ts", typeOnly: false },
            { source: "~/test/helper", target: "test/helper.ts", typeOnly: false },
          ],
        },
      ],
      ["features/files/browser.dom.test.ts", { path: "features/files/browser.dom.test.ts", imports: [] }],
      ["features/files/model.test.ts", { path: "features/files/model.test.ts", imports: [] }],
      ["features/files/test-data.tsx", { path: "features/files/test-data.tsx", imports: [] }],
      ["shared/model.ts", { path: "shared/model.ts", imports: [] }],
      ["test/helper.ts", { path: "test/helper.ts", imports: [] }],
    ]);

    expect(productionImportsExcludedModules(graph)).toEqual([
      { from: "features/files/page.ts", to: "features/files/browser.dom.test.ts" },
      { from: "features/files/page.ts", to: "features/files/model.test.ts" },
      { from: "features/files/page.ts", to: "features/files/test-data.tsx" },
      { from: "features/files/page.ts", to: "test/helper.ts" },
    ]);
    expect(productionImportsExcludedModules(new Map([
      [
        "features/files/page.ts",
        {
          path: "features/files/page.ts",
          imports: [{ source: "~/shared/model", target: "shared/model.ts", typeOnly: false }],
        },
      ],
      ["shared/model.ts", { path: "shared/model.ts", imports: [] }],
    ]))).toEqual([]);
    expect(productionImportsExcludedModules(appGraph)).toEqual([]);
  });
});

describe("pure processor model", () => {
  it("detects an indirect type-only path to React and MUI", () => {
    const graph: ModuleGraph = new Map([
      [
        "shared/processors/model.ts",
        {
          path: "shared/processors/model.ts",
          imports: [{ source: "~/shared/ui/form-fields", target: "shared/ui/form-fields.tsx", typeOnly: true }],
        },
      ],
      [
        "shared/ui/form-fields.tsx",
        {
          path: "shared/ui/form-fields.tsx",
          imports: [
            { source: "@mui/material/Button", typeOnly: false },
            { source: "react", typeOnly: true },
          ],
        },
      ],
    ]);

    expect(pureClosureViolations(graph, "shared/processors/model.ts", reachableModulePaths)).toEqual([
      "shared/ui/form-fields.tsx -> @mui/material/Button",
      "shared/ui/form-fields.tsx -> react",
    ]);
  });

  it("is transitively shared-only and independent from React and MUI", () => {
    expect(pureClosureViolations(appGraph, "shared/processors/model.ts", reachableModulePaths))
      .toEqual([]);
  });
});
