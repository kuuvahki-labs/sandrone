import { spawnSync } from "node:child_process";
import { readdirSync, readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import ts from "typescript";
import { describe, expect, it } from "vitest";

import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";

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

const expectedAppEntries = [
  "app.css",
  "core",
  "eslint-config.test.ts",
  "features",
  "root.test.ts",
  "root.tsx",
  "routes",
  "routes.ts",
  "shared",
  "styles",
  "test",
  "vite-config.test.ts",
] as const;

const expectedFeatureDirectories = ["files", "settings", "shares", "subscriptions"] as const;
const expectedFileFeatureEntries = [
  "config",
  "data",
  "drivers",
  "editor",
  "model",
  "pages",
  "processors",
  "test-data.tsx",
] as const;

const featureOwnedModules = [
  "features/files/config/model/adaptive-availability.ts",
  "features/files/config/model/adaptive-groups.ts",
  "features/files/config/model/adaptive-regions.ts",
  "features/files/config/model/editor-model.ts",
  "features/files/config/model/naming.ts",
  "features/files/config/model/node-source.ts",
  "features/files/config/model/preview.ts",
  "features/files/config/model/references.ts",
  "features/files/config/model/relations.ts",
  "features/files/config/model/templates.ts",
  "features/files/data/create-file-actions.ts",
  "features/files/data/use-file-resources.ts",
  "features/files/editor/file-driver-icon.tsx",
  "features/files/editor/file-driver-ui-registry.ts",
  "features/files/editor/file-driver-ui.ts",
  "features/files/editor/file-form.tsx",
  "features/files/editor/raw-config-editor.tsx",
  "features/files/editor/source-editor.tsx",
  "features/files/model/codec.ts",
  "features/files/model/input-validation.ts",
  "features/files/model/summary.ts",
  "features/files/model/types.ts",
  "features/files/pages/files-page.tsx",
  "features/files/pages/file-new-page.tsx",
  "features/files/pages/file-edit-page.tsx",
  "features/files/pages/file-preview-page.tsx",
  "features/files/processors/processor-builder.tsx",
  "features/settings/data/use-backup-operations.ts",
  "features/settings/data/use-version-info.ts",
  "features/settings/model/project-settings.ts",
  "features/settings/pages/settings-data-page.tsx",
  "features/settings/pages/settings-page.tsx",
  "features/settings/pages/settings-runtime-page.tsx",
  "features/settings/sections/appearance-settings-section.tsx",
  "features/settings/sections/data-settings-section.tsx",
  "features/settings/sections/runtime-settings-section.tsx",
  "features/settings/sections/service-connection-section.tsx",
  "features/settings/sections/startup-settings-section.tsx",
  "features/settings/sections/subscription-traffic-settings-section.tsx",
  "features/shares/components/share-dialog-context.tsx",
  "features/shares/components/share-dialog.tsx",
  "features/shares/components/manual-copy-dialog.tsx",
  "features/shares/components/share-url-selection.ts",
  "features/shares/data/create-share-actions.ts",
  "features/shares/data/use-shares-resource.ts",
  "features/shares/model/codec.ts",
  "features/shares/model/share-formats.ts",
  "features/shares/model/types.ts",
  "features/shares/pages/shares-page.tsx",
  "features/subscriptions/components/processor-builder.tsx",
  "features/subscriptions/components/source-multi-select.tsx",
  "features/subscriptions/components/subscription-form.tsx",
  "features/subscriptions/components/subscription-traffic-summary.tsx",
  "features/subscriptions/data/create-subscription-actions.ts",
  "features/subscriptions/data/use-subscription-resources.ts",
  "features/subscriptions/model/codec.ts",
  "features/subscriptions/model/summary.ts",
  "features/subscriptions/model/types.ts",
  "features/subscriptions/pages/subscription-edit-page.tsx",
  "features/subscriptions/pages/subscription-new-page.tsx",
  "features/subscriptions/pages/subscription-preview-page.tsx",
  "features/subscriptions/pages/subscriptions-page.tsx",
] as const;

const genericFileKindModules = [
  "features/files/model/types.ts",
  "features/files/model/codec.ts",
  "features/files/model/summary.ts",
  "features/files/editor/file-form.tsx",
  "features/files/pages/files-page.tsx",
  "features/files/pages/file-new-page.tsx",
  "features/files/pages/file-edit-page.tsx",
  "features/files/pages/file-preview-page.tsx",
  "features/files/editor/source-editor.tsx",
  "features/files/data/create-file-actions.ts",
  "routes/files.tsx",
  "routes/files.new.tsx",
  "routes/files.$name.edit.tsx",
  "routes/files.$name.preview.tsx",
] as const;

const allDriverKinds = FILE_DRIVER_REGISTRY.drivers.map((driver) => driver.kind);
const structuredDriverKinds = FILE_DRIVER_REGISTRY.drivers
  .filter((driver) => driver.configuration.mode === "structured")
  .map((driver) => driver.kind);
const registeredClientLiteral = new RegExp(
  `["'\`][^"'\`]*(?:${allDriverKinds.map(escapeRegExp).join("|")})[^"'\`]*["'\`]`,
  "i",
);
const targetNativeSchemaKey = /\b(?:auto_detect_interface|auto_redirect|auto_route|cache_file|clash_api|default_domain_resolver|domain_suffix|download_detour|fallback_delay|inbound|inbounds|interrupt_exist_connections|ip_cidr|ip_is_private|listen_port|network_strategy|outbound|outbounds|override_address|override_port|proxies|route|route_exclude_address|rule_set|rule_set_ipcidr_match_source|server_port|source_ip_cidr|strict_route|tag|update_interval)\b|(?<![\w-])(?:dialer-proxy|disable-udp|exclude-filter|exclude-type|expected-status|include-all|include-all-proxies|include-all-providers|interface-name|max-failed-times|policy-path|policy-regex-filter|proxy-groups|proxy-providers|routing-mark|rule-providers)(?![\w-])/i;
const targetNativeActionKey = /(?:\.\s*action\b|["'`]action["'`]\s*:|(?<![\w$])action\s*:)/i;

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function directoryEntries(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true })
    .map((entry) => entry.name)
    .sort();
}

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

function featurePagePaths(): string[] {
  return [...appGraph.values()]
    .map((module) => module.path)
    .filter((path) => /^features\/[^/]+\/pages\/.+\.tsx$/u.test(path) && isProductionModule(path))
    .sort();
}

describe("authenticated page layout", () => {
  it("keeps the shell as the sole owner of the centered lg content area", () => {
    const shellSource = readFileSync(join(appDir, "core/components/shell.tsx"), "utf8");

    expect(shellSource).toContain('<Container disableGutters maxWidth="lg">');
  });

  it("does not add private maximum widths or containers to feature pages", () => {
    const violations = featurePagePaths().flatMap((path) => {
      const source = readFileSync(join(appDir, path), "utf8");
      const module = appGraph.get(path);
      const reasons = [
        ...(/\bmax-w-/u.test(source) ? ["private max-width"] : []),
        ...(module?.imports.some((moduleImport) => moduleImport.source === "@mui/material/Container")
          ? ["MUI Container import"]
          : []),
      ];
      return reasons.map((reason) => ({ path, reason }));
    });

    expect(violations).toEqual([]);
  });
});

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

function isForbiddenDriverTarget(path: string): boolean {
  return path.startsWith("features/files/editor/")
    || path.startsWith("features/files/config/components/")
    || /^features\/files\/drivers\/(?:static|mihomo|sing-box|shadowrocket)\//u.test(path);
}

function clientControlFlowLiterals(source: string): string[] {
  const sourceFile = ts.createSourceFile(
    "client-control-flow.tsx",
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const registeredKinds = new Set(allDriverKinds);
  const literals: string[] = [];

  function addLiteral(expression: ts.Expression) {
    if (ts.isStringLiteralLike(expression) && registeredKinds.has(expression.text)) {
      literals.push(expression.text);
    }
  }

  function visit(current: ts.Node) {
    if (ts.isCaseClause(current)) {
      addLiteral(current.expression);
    } else if (ts.isBinaryExpression(current) && [
      ts.SyntaxKind.EqualsEqualsToken,
      ts.SyntaxKind.EqualsEqualsEqualsToken,
      ts.SyntaxKind.ExclamationEqualsToken,
      ts.SyntaxKind.ExclamationEqualsEqualsToken,
    ].includes(current.operatorToken.kind)) {
      addLiteral(current.left);
      addLiteral(current.right);
    }
    ts.forEachChild(current, visit);
  }

  visit(sourceFile);
  return literals;
}

function targetNativeSchemaKeys(source: string): string[] {
  return [source.match(targetNativeSchemaKey)?.[0], source.match(targetNativeActionKey)?.[0]]
    .filter((match): match is string => Boolean(match));
}

function hasStarReExport(source: string): boolean {
  const sourceFile = ts.createSourceFile(
    "barrel-guard.tsx",
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  return sourceFile.statements.some((statement) => (
    ts.isExportDeclaration(statement)
    && (!statement.exportClause || ts.isNamespaceExport(statement.exportClause))
  ));
}

function forbiddenCoreContractTypeNames(source: string): string[] {
  return [...new Set(source.match(/\b(?:ComponentType|ConfigFieldSlotProps|GroupFieldsProps|RuleFieldsProps|RuleSetFieldsProps|RuleSetHeaderLayout|RuleSetPresentation|RuleSetSummaryField|StructuredConfigurationFieldSlots)\b/gu) ?? [])]
    .sort();
}

function unexpectedRegistryImports(
  imports: ModuleNode["imports"],
  expected: ModuleNode["imports"],
): string[] {
  const expectedImports = new Set(expected.map(moduleImportKey));
  return imports
    .filter((moduleImport) => !expectedImports.has(moduleImportKey(moduleImport)))
    .map((moduleImport) => moduleImport.source)
    .sort();
}

function moduleImportKey(moduleImport: ModuleNode["imports"][number]): string {
  return `${moduleImport.source}\0${moduleImport.target ?? ""}\0${String(moduleImport.typeOnly)}`;
}

function exportedDeclarationNames(source: string): string[] {
  const sourceFile = ts.createSourceFile(
    "exports.ts",
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const names: string[] = [];

  function addBindingName(name: ts.BindingName) {
    if (ts.isIdentifier(name)) {
      names.push(name.text);
      return;
    }
    for (const element of name.elements) {
      if (!ts.isOmittedExpression(element)) addBindingName(element.name);
    }
  }

  for (const statement of sourceFile.statements) {
    const modifiers = ts.canHaveModifiers(statement) ? ts.getModifiers(statement) : undefined;
    const exported = modifiers?.some((modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword);
    const defaultExported = modifiers?.some((modifier) => modifier.kind === ts.SyntaxKind.DefaultKeyword);
    if (ts.isExportAssignment(statement)) {
      names.push(statement.isExportEquals ? "export=" : "default");
    } else if (ts.isNamespaceExportDeclaration(statement)) {
      names.push(statement.name.text);
    } else if (ts.isExportDeclaration(statement)) {
      if (!statement.exportClause) names.push("*");
      else if (ts.isNamedExports(statement.exportClause)) {
        names.push(...statement.exportClause.elements.map((element) => element.name.text));
      } else {
        names.push(statement.exportClause.name.text);
      }
    } else if (!exported) {
      continue;
    } else if (defaultExported) {
      names.push("default");
    } else if (ts.isVariableStatement(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        addBindingName(declaration.name);
      }
    } else if (
      ts.isFunctionDeclaration(statement)
      || ts.isClassDeclaration(statement)
      || ts.isInterfaceDeclaration(statement)
      || ts.isTypeAliasDeclaration(statement)
      || ts.isEnumDeclaration(statement)
      || ts.isModuleDeclaration(statement)
      || ts.isImportEqualsDeclaration(statement)
    ) {
      names.push(statement.name?.text ?? "<export>");
    } else {
      names.push("<export>");
    }
  }
  return names.sort();
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

describe("current frontend layout", () => {
  it("has only the current root, feature, and file-feature entries", () => {
    expect(directoryEntries(appDir)).toEqual(expectedAppEntries);
    expect(directoryEntries(join(appDir, "features"))).toEqual(expectedFeatureDirectories);
    expect(directoryEntries(join(appDir, "features/files"))).toEqual(expectedFileFeatureEntries);
  });

  it("ignores runtime data without hiding feature-owned data modules", () => {
    expect(gitIgnores("data/.architecture-probe")).toBe(true);
    expect(gitIgnores("internal/entry/cli/data/.architecture-probe")).toBe(true);
    for (const feature of expectedFeatureDirectories) {
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

  it("owns feature pages, editors, data, and application composition at direct paths", () => {
    for (const modulePath of [
      ...featureOwnedModules,
      "core/app-layout.tsx",
      "core/material-ui.tsx",
      "core/provider/use-project-settings.ts",
      "core/sandrone-provider.tsx",
      "core/routing/subscriptions-route.tsx",
      "shared/api/client.ts",
      "shared/i18n/context.tsx",
      "shared/preview/use-resource-preview.ts",
      "shared/processors/model.ts",
      "shared/ui/page.tsx",
    ]) {
      expect(appGraph.has(modulePath), modulePath).toBe(true);
    }
  });

  it("keeps CSS layers, MUI wiring, and the provider composition root explicit", () => {
    const appStyles = readFileSync(join(appDir, "app.css"), "utf8");
    expect(appStyles).toContain("@layer theme, base, mui, components, utilities;");
    expect(appStyles).toContain('@import "tailwindcss";');
    expect(appStyles).toContain('@import "./styles/base.css" layer(base);');

    const baseStyles = readFileSync(join(appDir, "styles/base.css"), "utf8");
    expect(baseStyles).toContain(".highlighted-textarea-input {");
    expect(baseStyles).toContain("-webkit-text-fill-color: transparent;");
    expect(baseStyles).toContain(".highlighted-textarea-input::selection");

    const muiProvider = readFileSync(join(appDir, "core/material-ui.tsx"), "utf8");
    expect(muiProvider).toContain("StyledEngineProvider");
    expect(muiProvider).toContain("enableCssLayer");

    const provider = readFileSync(join(appDir, "core/sandrone-provider.tsx"), "utf8");
    expect(provider.split("\n").filter((line) => line.trim()).length).toBeLessThanOrEqual(130);
  });

  it("has no production layer indexes, star barrels, or imports targeting indexes", () => {
    const productionModules = [...appGraph.values()]
      .filter((module) => isProductionModule(module.path));
    const layerModules = productionModules
      .filter((module) => /^(?:core|features|shared)\//u.test(module.path));

    expect(layerModules.map((module) => module.path).filter((path) => /(?:^|\/)index\.tsx?$/u.test(path)))
      .toEqual([]);
    expect(layerModules.flatMap((module) => (
      hasStarReExport(readFileSync(join(appDir, module.path), "utf8"))
        ? [module.path]
        : []
    ))).toEqual([]);
    expect(productionModules.flatMap((module) => module.imports
      .filter((moduleImport) => moduleImport.target && /(?:^|\/)index\.tsx?$/u.test(moduleImport.target))
      .map((moduleImport) => ({ from: module.path, to: moduleImport.target })))).toEqual([]);
  });

  it("detects commented and namespace star re-exports without matching template text", () => {
    expect(hasStarReExport('export /* ownership */ * from "./module";')).toBe(true);
    expect(hasStarReExport('export * as namespace from "./module";')).toBe(true);
    expect(hasStarReExport('const example = `\nexport * from "./module";\n`;')).toBe(false);
    expect(hasStarReExport('export { value } from "./module";')).toBe(false);
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

  it("keeps share-dialog composition in the shell, routes, and shared subscriptions core", () => {
    const contextModule = "features/shares/components/share-dialog-context.tsx";
    const consumers = [...appGraph.values()]
      .filter((module) => isProductionModule(module.path))
      .filter((module) => module.imports.some((moduleImport) => moduleImport.target === contextModule))
      .map((module) => module.path)
      .sort();

    expect(consumers).toEqual([
      "core/app-layout.tsx",
      "core/routing/subscriptions-route.tsx",
      "routes/files.$name.edit.tsx",
      "routes/files.new.tsx",
      "routes/files.tsx",
      "routes/subscriptions.$kind.$name.edit.tsx",
      "routes/subscriptions.new.tsx",
    ]);
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

describe("FileDriver boundaries", () => {
  const corePrefix = "features/files/drivers/core/";
  const corePaths = [...appGraph.keys()]
    .filter((path) => isProductionModule(path) && path.startsWith(corePrefix))
    .sort();

  it("keeps every pure core module free of UI, concrete drivers, React, and MUI", () => {
    const violations = corePaths.flatMap((modulePath) => (appGraph.get(modulePath)?.imports ?? [])
      .filter((moduleImport) => isReactOrMui(moduleImport.source)
        || Boolean(moduleImport.target && isForbiddenDriverTarget(moduleImport.target)))
      .map((moduleImport) => `${modulePath} -> ${moduleImport.target ?? moduleImport.source}`));

    expect(violations).toEqual([]);

    const contract = readFileSync(join(appDir, `${corePrefix}file-driver.ts`), "utf8");
    expect(forbiddenCoreContractTypeNames(contract)).toEqual([]);
    expect(contract).not.toMatch(/adapter\s*\.\s*ui/u);
  });

  it("recognizes every editor-owned slot and rule-set presentation contract", () => {
    expect(forbiddenCoreContractTypeNames(`
      type ComponentType = unknown;
      interface ConfigFieldSlotProps<T> { draft: T }
      interface GroupFieldsProps {}
      interface RuleFieldsProps {}
      interface RuleSetFieldsProps {}
      type RuleSetHeaderLayout = "name";
      type RuleSetSummaryField = "format";
      interface RuleSetPresentation {}
      interface StructuredConfigurationFieldSlots {}
    `)).toEqual([
      "ComponentType",
      "ConfigFieldSlotProps",
      "GroupFieldsProps",
      "RuleFieldsProps",
      "RuleSetFieldsProps",
      "RuleSetHeaderLayout",
      "RuleSetPresentation",
      "RuleSetSummaryField",
      "StructuredConfigurationFieldSlots",
    ]);
  });

  it("keeps runtime and type-inclusive core closures within their promised boundary", () => {
    const runtimePaths = reachableModulePaths(appGraph, corePaths, false);
    const typeInclusivePaths = reachableModulePaths(appGraph, corePaths, true);
    expect(runtimePaths.filter(isForbiddenDriverTarget)).toEqual([]);
    expect(typeInclusivePaths.filter(isForbiddenDriverTarget)).toEqual([]);
    expect(runtimePaths.flatMap((modulePath) => (appGraph.get(modulePath)?.imports ?? [])
      .filter((moduleImport) => isReactOrMui(moduleImport.source))
      .map((moduleImport) => `${modulePath} -> ${moduleImport.source}`))).toEqual([]);
  });

  it("keeps the core registry and its fixture tests independent from composition and clients", () => {
    expect(appGraph.get(`${corePrefix}registry.ts`)?.imports).toEqual([
      {
        source: "./file-driver",
        target: `${corePrefix}file-driver.ts`,
        typeOnly: true,
      },
    ]);

    const testTargets = appGraph.get(`${corePrefix}registry.test.ts`)?.imports
      .map((moduleImport) => moduleImport.target)
      .filter((target): target is string => Boolean(target))
      .filter((target) => target.startsWith("features/files/drivers/"))
      .sort();
    expect(testTargets).toEqual([
      `${corePrefix}file-driver.ts`,
      `${corePrefix}registry.ts`,
    ]);
  });

  it("collects every identifier exported through object and array binding patterns", () => {
    expect(exportedDeclarationNames(`
      declare const source: { extra: unknown };
      export const { extra } = source;
    `)).toEqual(["extra"]);
    expect(exportedDeclarationNames(`
      declare const source: readonly [unknown, unknown, { nested: unknown }, ...unknown[]];
      export const [first, , { nested }, ...rest] = source;
    `)).toEqual(["first", "nested", "rest"]);
  });

  it("normalizes every default export form and export-equals to stable markers", () => {
    for (const source of [
      "declare const registry: unknown; export default registry;",
      "export default function NamedFunction() {}",
      "export default function () {}",
      "export default class NamedClass {}",
      "export default class {}",
      "export default interface NamedInterface {}",
      "export default interface {}",
    ]) {
      expect(exportedDeclarationNames(source), source).toEqual(["default"]);
    }
    expect(exportedDeclarationNames("declare const registry: unknown; export = registry;"))
      .toEqual(["export="]);
  });

  it("collects every named declaration and module-style export form", () => {
    expect(exportedDeclarationNames(`
      export const namedValue = 1;
      export function namedFunction() {}
      export class NamedClass {}
      export interface NamedInterface {}
      export type NamedType = string;
      export enum NamedEnum {}
      export module NamedModule {}
      export namespace NamedNamespace {}
      export import NamedImport = require("./named");
    `)).toEqual([
      "NamedClass",
      "NamedEnum",
      "NamedImport",
      "NamedInterface",
      "NamedModule",
      "NamedNamespace",
      "NamedType",
      "namedFunction",
      "namedValue",
    ]);
    expect(exportedDeclarationNames(`
      export { direct, original as renamed } from "./named";
      export * from "./star";
      export * as bundle from "./bundle";
      export as namespace Registry;
    `)).toEqual(["*", "Registry", "bundle", "direct", "renamed"]);
  });

  it("uses one pure four-driver composition registry with the exact public API", () => {
    const registryPath = "features/files/drivers/registry.ts";
    const registry = appGraph.get(registryPath);

    expect(registry?.imports).toEqual([
      { source: "./core/file-driver", target: "features/files/drivers/core/file-driver.ts", typeOnly: true },
      { source: "./core/registry", target: "features/files/drivers/core/registry.ts", typeOnly: false },
      { source: "./mihomo/driver", target: "features/files/drivers/mihomo/driver.ts", typeOnly: false },
      { source: "./shadowrocket/driver", target: "features/files/drivers/shadowrocket/driver.ts", typeOnly: false },
      { source: "./sing-box/driver", target: "features/files/drivers/sing-box/driver.ts", typeOnly: false },
      { source: "./static/driver", target: "features/files/drivers/static/driver.ts", typeOnly: false },
    ]);
    expect(exportedDeclarationNames(readFileSync(join(appDir, registryPath), "utf8"))).toEqual([
      "FILE_DRIVER_REGISTRY",
      "fileDriver",
      "requireFileDriver",
    ]);

    const concreteDefinitions = new Set([
      "features/files/drivers/mihomo/driver.ts",
      "features/files/drivers/shadowrocket/driver.ts",
      "features/files/drivers/sing-box/driver.ts",
      "features/files/drivers/static/driver.ts",
    ]);
    const otherConsumers = [...appGraph.values()]
      .filter((module) => isProductionModule(module.path) && module.path !== registryPath)
      .flatMap((module) => module.imports
        .filter((moduleImport) => moduleImport.target && concreteDefinitions.has(moduleImport.target))
        .map((moduleImport) => ({ from: module.path, to: moduleImport.target })));
    expect(otherConsumers).toEqual([]);
  });

  it("keeps field implementations out of concrete definitions and in the editor UI registry", () => {
    for (const kind of ["static", "mihomo", "sing-box", "shadowrocket"]) {
      const driverPath = `features/files/drivers/${kind}/driver.ts`;
      expect(appGraph.get(driverPath)?.imports.some((moduleImport) => moduleImport.source.includes("fields")), driverPath)
        .toBe(false);
    }

    const uiRegistryPath = "features/files/editor/file-driver-ui-registry.ts";
    expect(appGraph.get(uiRegistryPath)?.imports).toEqual([
      { source: "./file-driver-ui", target: "features/files/editor/file-driver-ui.ts", typeOnly: true },
      { source: "~/features/files/drivers/mihomo/fields", target: "features/files/drivers/mihomo/fields.tsx", typeOnly: false },
      { source: "~/features/files/drivers/shadowrocket/fields", target: "features/files/drivers/shadowrocket/fields.tsx", typeOnly: false },
      { source: "~/features/files/drivers/sing-box/fields", target: "features/files/drivers/sing-box/fields.tsx", typeOnly: false },
    ]);

    const uiConsumers = [...appGraph.values()]
      .filter((module) => isProductionModule(module.path))
      .filter((module) => module.imports.some((moduleImport) => moduleImport.target === uiRegistryPath))
      .map((module) => module.path)
      .sort();
    expect(uiConsumers).toEqual(["features/files/editor/file-form.tsx"]);
  });

  it("detects targetless external and UI registry import bypasses", () => {
    const expected = [
      { source: "./file-driver", target: `${corePrefix}file-driver.ts`, typeOnly: true },
    ];
    expect(unexpectedRegistryImports([
      ...expected,
      { source: "@mui/material", typeOnly: false },
      { source: "react", typeOnly: true },
      { source: "zod", typeOnly: false },
    ], expected)).toEqual(["@mui/material", "react", "zod"]);
  });

  it("keeps config workbench modules independent from concrete fields and the UI registry", () => {
    const configModules = [...appGraph.values()]
      .filter((module) => isProductionModule(module.path))
      .filter((module) => /^features\/files\/config\/(?:model|components)\//u.test(module.path));
    const violations = configModules.flatMap((module) => module.imports
      .filter((moduleImport) => moduleImport.target === "features/files/editor/file-driver-ui-registry.ts"
        || /features\/files\/drivers\/(?:static|mihomo|sing-box|shadowrocket)\/fields\.tsx$/u.test(moduleImport.target ?? ""))
      .map((moduleImport) => `${module.path} -> ${moduleImport.target}`));

    expect(violations).toEqual([]);
  });

  it("keeps shared workbench and field drafts target-neutral", () => {
    const configPaths = [...appGraph.keys()]
      .filter((path) => isProductionModule(path))
      .filter((path) => /^features\/files\/config\/(?:model|components)\//u.test(path));
    for (const modulePath of configPaths) {
      const source = readFileSync(join(appDir, modulePath), "utf8");
      expect(source, `${modulePath} client literal`).not.toMatch(registeredClientLiteral);
      expect(targetNativeSchemaKeys(source), `${modulePath} native key`).toEqual([]);
    }

    for (const kind of structuredDriverKinds) {
      const modulePath = `features/files/drivers/${kind}/fields.tsx`;
      const source = readFileSync(join(appDir, modulePath), "utf8");
      expect(targetNativeSchemaKeys(source), modulePath).toEqual([]);
      expect(source, modulePath).not.toMatch(/\badapterState\b/iu);
    }

    for (const modulePath of genericFileKindModules) {
      expect(clientControlFlowLiterals(readFileSync(join(appDir, modulePath), "utf8")), modulePath)
        .toEqual([]);
    }
  });

  it("detects registered client literals in forbidden control flow and ordinary literals", () => {
    expect(clientControlFlowLiterals(`
      if (kind === "static") useDriver();
      if ("sing-box" !== kind) useFallback();
      switch (kind) { case "mihomo": useMihomo(); }
    `)).toEqual(["static", "sing-box", "mihomo"]);
    expect(clientControlFlowLiterals(`
      const normalized = { kind: driver.kind, groups: [] };
    `)).toEqual([]);
    expect(registeredClientLiteral.test('const target = "shadowrocket";')).toBe(true);
    expect(registeredClientLiteral.test('const target = "future-client";')).toBe(false);
  });

  it("detects target-native schema keys while allowing normalized workbench keys", () => {
    for (const key of [
      "action",
      "outbound",
      "auto_route",
      "proxy-groups",
      "rule-providers",
    ]) {
      expect(targetNativeSchemaKeys(`const native = { "${key}": true };`), key)
        .not.toEqual([]);
    }
    expect(targetNativeSchemaKeys(`
      const normalized = { groups: [], rule_sets: [], rules: [] };
      const groupId = "config-proxy-groups";
    `)).toEqual([]);
  });
});
