import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";
import { describe, expect, it } from "vitest";

import routes from "~/routes";

import { readModuleGraph } from "./module-graph";

const appDir = resolve(dirname(fileURLToPath(import.meta.url)), "../..");
const routesDir = join(appDir, "routes");

const expectedRoutes = [
  { id: "home", index: true, path: null, file: "routes/_index.tsx" },
  { id: "subscriptions", index: false, path: "subscriptions", file: "routes/subscriptions.tsx" },
  { id: "subscriptions-new", index: false, path: "subscriptions/new", file: "routes/subscriptions.new.tsx" },
  { id: "subscriptions-edit", index: false, path: "subscriptions/:kind/:name/edit", file: "routes/subscriptions.$kind.$name.edit.tsx" },
  { id: "subscriptions-preview", index: false, path: "subscriptions/:kind/:name/preview", file: "routes/subscriptions.$kind.$name.preview.tsx" },
  { id: "files", index: false, path: "files", file: "routes/files.tsx" },
  { id: "files-new", index: false, path: "files/new", file: "routes/files.new.tsx" },
  { id: "files-edit", index: false, path: "files/:name/edit", file: "routes/files.$name.edit.tsx" },
  { id: "files-preview", index: false, path: "files/:name/preview", file: "routes/files.$name.preview.tsx" },
  { id: "shares", index: false, path: "shares", file: "routes/shares.tsx" },
  { id: "settings", index: false, path: "settings", file: "routes/settings.tsx" },
  { id: "settings-runtime", index: false, path: "settings/runtime", file: "routes/settings.runtime.tsx" },
  { id: "settings-data", index: false, path: "settings/data", file: "routes/settings.data.tsx" },
] as const;

interface RouteConfigRecord {
  readonly children?: readonly unknown[];
  readonly file?: string;
  readonly id?: string;
  readonly index?: boolean;
  readonly path?: string;
}

function normalizeRoute(entry: RouteConfigRecord) {
  return {
    id: entry.id,
    index: entry.index === true,
    path: entry.path ?? null,
    file: entry.file,
  };
}

function hasRealDefaultComponent(source: string, fileName: string): boolean {
  const sourceFile = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const forwardingDefault = sourceFile.statements.some((statement) => {
    if (!ts.isExportDeclaration(statement) || !statement.moduleSpecifier) return false;
    const clause = statement.exportClause;
    return Boolean(clause
      && ts.isNamedExports(clause)
      && clause.elements.some((element) => element.name.text === "default"));
  });
  if (forwardingDefault) return false;

  return sourceFile.statements.some((statement) => {
    if (ts.isExportAssignment(statement)) return !statement.isExportEquals;
    if (ts.isExportDeclaration(statement)
      && !statement.moduleSpecifier
      && statement.exportClause
      && ts.isNamedExports(statement.exportClause)) {
      return statement.exportClause.elements.some((element) => element.name.text === "default");
    }
    if (!ts.canHaveModifiers(statement)) return false;
    const modifiers = ts.getModifiers(statement) ?? [];
    return modifiers.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword)
      && modifiers.some((modifier) => modifier.kind === ts.SyntaxKind.DefaultKeyword);
  });
}

describe("public React Router modules", () => {
  it("keeps the exact ordered 13-route production contract", async () => {
    const configuredRoutes = await Promise.resolve(routes) as readonly RouteConfigRecord[];

    expect(configuredRoutes.map(normalizeRoute)).toEqual(expectedRoutes);
    expect(configuredRoutes.every((entry) => !entry.children?.length)).toBe(true);
  });

  it("has exactly the configured production route filenames", () => {
    const productionFiles = readdirSync(routesDir, { withFileTypes: true })
      .filter((entry) => entry.isFile() && entry.name.endsWith(".tsx") && !entry.name.includes(".test."))
      .map((entry) => entry.name)
      .sort();

    expect(productionFiles).toEqual(expectedRoutes.map((entry) => entry.file.slice("routes/".length)).sort());
  });

  it("uses real default-exported components rather than forwarding route modules", () => {
    for (const { file } of expectedRoutes) {
      const source = readFileSync(join(appDir, file), "utf8");
      expect(hasRealDefaultComponent(source, file), file).toBe(true);
    }
  });

  it("accepts an imported default identifier but rejects a forwarding re-export", () => {
    expect(hasRealDefaultComponent(`
      import { RouteComponent } from "~/core/routing/route";
      export default RouteComponent;
    `, "imported.tsx")).toBe(true);
    expect(hasRealDefaultComponent(`
      export { default } from "./other-route";
    `, "forwarding.tsx")).toBe(false);
  });

  it("does not import one public route module from another", () => {
    const graph = readModuleGraph(appDir);
    const violations = [...graph.values()]
      .filter((module) => module.path.startsWith("routes/") && !module.path.includes(".test."))
      .flatMap((module) => module.imports
        .filter((moduleImport) => moduleImport.target?.startsWith("routes/"))
        .map((moduleImport) => ({ from: module.path, to: moduleImport.target })))
      .sort((left, right) => left.from.localeCompare(right.from) || left.to!.localeCompare(right.to!));

    expect(violations).toEqual([]);
  });

  it("does not import MUI directly from public route modules", () => {
    const graph = readModuleGraph(appDir);
    const violations = expectedRoutes.flatMap(({ file }) => (graph.get(file)?.imports ?? [])
      .filter((moduleImport) => moduleImport.source === "@mui/material"
        || moduleImport.source === "@mui/icons-material"
        || moduleImport.source.startsWith("@mui/"))
      .map((moduleImport) => ({ file, moduleName: moduleImport.source })));

    expect(violations).toEqual([]);
  });
});
