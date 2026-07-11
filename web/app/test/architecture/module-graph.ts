import { readdirSync, readFileSync } from "node:fs";
import { dirname, extname, join, relative, resolve } from "node:path";

import ts from "typescript";

export interface ModuleImport {
  readonly source: string;
  readonly target?: string;
  readonly typeOnly: boolean;
}

export interface ModuleNode {
  readonly path: string;
  readonly imports: readonly ModuleImport[];
}

export type ModuleGraph = ReadonlyMap<string, ModuleNode>;

export interface BoundaryPolicy {
  readonly allow: (from: string, to: string) => boolean;
}

export interface BoundaryViolation {
  readonly from: string;
  readonly to: string;
}

interface ModuleReference {
  readonly source: string;
  readonly typeOnly: boolean;
}

export function readModuleGraph(appDir: string): ModuleGraph {
  const root = resolve(appDir);
  const modules = collectTypeScriptModules(root);
  const modulePathByAbsolutePath = new Map(
    modules.map((absolutePath) => [absolutePath, appRelativePath(root, absolutePath)]),
  );
  const graph = new Map<string, ModuleNode>();

  for (const absolutePath of modules) {
    const path = modulePathByAbsolutePath.get(absolutePath);
    if (!path) continue;

    const references = readModuleReferences(path, readFileSync(absolutePath, "utf8"));
    const imports = references
      .map((reference): ModuleImport => {
        const targetPath = resolveAppModule(
          root,
          absolutePath,
          reference.source,
          modulePathByAbsolutePath,
        );
        return targetPath
          ? { ...reference, target: targetPath }
          : reference;
      })
      .sort(compareModuleImports);

    graph.set(path, { path, imports });
  }

  return graph;
}

export function findCycles(graph: ModuleGraph, includeTypeOnly: boolean): string[][] {
  const paths = [...graph.keys()].sort(compareStrings);
  const adjacency = new Map(
    paths.map((path) => [path, moduleTargets(graph, path, includeTypeOnly)]),
  );
  const indices = new Map<string, number>();
  const lowLinks = new Map<string, number>();
  const stack: string[] = [];
  const onStack = new Set<string>();
  const cycles: string[][] = [];
  let nextIndex = 0;

  function connect(path: string) {
    const index = nextIndex;
    nextIndex += 1;
    indices.set(path, index);
    lowLinks.set(path, index);
    stack.push(path);
    onStack.add(path);

    for (const target of adjacency.get(path) ?? []) {
      if (!indices.has(target)) {
        connect(target);
        lowLinks.set(path, Math.min(required(lowLinks, path), required(lowLinks, target)));
      } else if (onStack.has(target)) {
        lowLinks.set(path, Math.min(required(lowLinks, path), required(indices, target)));
      }
    }

    if (required(lowLinks, path) !== required(indices, path)) return;

    const component: string[] = [];
    let member: string | undefined;
    do {
      member = stack.pop();
      if (!member) throw new Error("Tarjan stack was exhausted before its component root");
      onStack.delete(member);
      component.push(member);
    } while (member !== path);

    component.sort(compareStrings);
    const selfCycle = component.length === 1
      && (adjacency.get(component[0]) ?? []).includes(component[0]);
    if (component.length > 1 || selfCycle) cycles.push(component);
  }

  for (const path of paths) {
    if (!indices.has(path)) connect(path);
  }

  return cycles.sort(compareStringArrays);
}

export function filterModuleGraph(
  graph: ModuleGraph,
  include: (path: string) => boolean,
): ModuleGraph {
  const includedPaths = [...graph.keys()].filter(include).sort(compareStrings);
  const filtered = new Map<string, ModuleNode>();

  for (const path of includedPaths) {
    const module = graph.get(path);
    if (!module) continue;
    filtered.set(path, {
      path,
      imports: [...module.imports].sort(compareModuleImports),
    });
  }

  return filtered;
}

export function reachableModulePaths(
  graph: ModuleGraph,
  roots: readonly string[],
  includeTypeOnly: boolean,
): string[] {
  const reachable = new Set<string>();
  const pending = [...new Set(roots.filter((root) => graph.has(root)))];

  while (pending.length > 0) {
    const path = pending.pop();
    if (!path || reachable.has(path)) continue;
    reachable.add(path);
    for (const target of moduleTargets(graph, path, includeTypeOnly)) {
      if (!reachable.has(target)) pending.push(target);
    }
  }

  return [...reachable].sort(compareStrings);
}

export function findBoundaryViolations(
  graph: ModuleGraph,
  policy: BoundaryPolicy,
): BoundaryViolation[] {
  const violations = new Map<string, BoundaryViolation>();

  for (const module of graph.values()) {
    for (const moduleImport of module.imports) {
      const { target } = moduleImport;
      if (!target || policy.allow(module.path, target)) continue;
      violations.set(`${module.path}\0${target}`, { from: module.path, to: target });
    }
  }

  return [...violations.values()].sort((left, right) =>
    compareStrings(left.from, right.from) || compareStrings(left.to, right.to));
}

function collectTypeScriptModules(root: string): string[] {
  const modules: string[] = [];

  function collect(directory: string) {
    const entries = readdirSync(directory, { withFileTypes: true })
      .sort((left, right) => compareStrings(left.name, right.name));
    for (const entry of entries) {
      const absolutePath = join(directory, entry.name);
      if (entry.isDirectory()) {
        collect(absolutePath);
      } else if (entry.isFile() && /\.(?:ts|tsx)$/u.test(entry.name)) {
        modules.push(absolutePath);
      }
    }
  }

  collect(root);
  return modules;
}

function readModuleReferences(path: string, source: string): ModuleReference[] {
  const sourceFile = ts.createSourceFile(
    path,
    source,
    ts.ScriptTarget.Latest,
    true,
    extname(path) === ".tsx" ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const references: ModuleReference[] = [];

  function addLiteralReference(
    expression: ts.Expression | undefined,
    typeOnly: boolean,
    referenceKind: string,
  ) {
    if (!expression || !ts.isStringLiteralLike(expression)) {
      const received = expression?.getText(sourceFile) ?? "no argument";
      throw new Error(
        `${path}: ${referenceKind} module must be a string literal; received ${received}`,
      );
    }
    references.push({ source: expression.text, typeOnly });
  }

  function visit(node: ts.Node) {
    if (ts.isImportDeclaration(node)) {
      addLiteralReference(
        node.moduleSpecifier,
        importDeclarationIsTypeOnly(node),
        "static import",
      );
      return;
    }
    if (ts.isExportDeclaration(node) && node.moduleSpecifier) {
      addLiteralReference(
        node.moduleSpecifier,
        exportDeclarationIsTypeOnly(node),
        "re-export",
      );
      return;
    }
    if (ts.isImportEqualsDeclaration(node)
      && ts.isExternalModuleReference(node.moduleReference)) {
      addLiteralReference(node.moduleReference.expression, node.isTypeOnly, "import equals");
      return;
    }
    if (ts.isImportTypeNode(node)) {
      const argument = node.argument;
      const expression = ts.isLiteralTypeNode(argument) ? argument.literal : undefined;
      addLiteralReference(expression, true, "import type");
    }
    if (ts.isCallExpression(node)) {
      if (node.expression.kind === ts.SyntaxKind.ImportKeyword) {
        addLiteralReference(node.arguments[0], false, "dynamic import");
        return;
      }
      if (ts.isIdentifier(node.expression) && node.expression.text === "require") {
        addLiteralReference(node.arguments[0], false, "require");
        return;
      }
    }
    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return references;
}

function importDeclarationIsTypeOnly(declaration: ts.ImportDeclaration): boolean {
  const clause = declaration.importClause;
  if (!clause) return false;
  if (clause.isTypeOnly) return true;
  if (clause.name || !clause.namedBindings || ts.isNamespaceImport(clause.namedBindings)) {
    return false;
  }
  return clause.namedBindings.elements.length > 0
    && clause.namedBindings.elements.every((specifier) => specifier.isTypeOnly);
}

function exportDeclarationIsTypeOnly(declaration: ts.ExportDeclaration): boolean {
  if (declaration.isTypeOnly) return true;
  const clause = declaration.exportClause;
  return Boolean(clause
    && ts.isNamedExports(clause)
    && clause.elements.length > 0
    && clause.elements.every((specifier) => specifier.isTypeOnly));
}

function resolveAppModule(
  root: string,
  fromAbsolutePath: string,
  source: string,
  modulePathByAbsolutePath: ReadonlyMap<string, string>,
): string | undefined {
  let unresolvedPath: string;
  if (source.startsWith("~/")) {
    unresolvedPath = resolve(root, source.slice(2));
  } else if (source.startsWith("./") || source.startsWith("../")) {
    unresolvedPath = resolve(dirname(fromAbsolutePath), source);
  } else {
    return undefined;
  }

  const candidates = /\.(?:ts|tsx)$/u.test(unresolvedPath)
    ? [unresolvedPath]
    : [
        `${unresolvedPath}.ts`,
        `${unresolvedPath}.tsx`,
        join(unresolvedPath, "index.ts"),
        join(unresolvedPath, "index.tsx"),
      ];

  for (const candidate of candidates) {
    const target = modulePathByAbsolutePath.get(candidate);
    if (target) return target;
  }
  return undefined;
}

function moduleTargets(
  graph: ModuleGraph,
  path: string,
  includeTypeOnly: boolean,
): string[] {
  const targets = new Set<string>();
  for (const moduleImport of graph.get(path)?.imports ?? []) {
    if (moduleImport.target
      && graph.has(moduleImport.target)
      && (includeTypeOnly || !moduleImport.typeOnly)) {
      targets.add(moduleImport.target);
    }
  }
  return [...targets].sort(compareStrings);
}

function appRelativePath(root: string, absolutePath: string): string {
  return relative(root, absolutePath).replaceAll("\\", "/");
}

function required(values: ReadonlyMap<string, number>, key: string): number {
  const value = values.get(key);
  if (value === undefined) throw new Error(`Missing Tarjan state for ${key}`);
  return value;
}

function compareModuleImports(left: ModuleImport, right: ModuleImport): number {
  return compareStrings(left.source, right.source)
    || compareStrings(left.target ?? "", right.target ?? "")
    || Number(left.typeOnly) - Number(right.typeOnly);
}

function compareStringArrays(left: readonly string[], right: readonly string[]): number {
  const length = Math.min(left.length, right.length);
  for (let index = 0; index < length; index += 1) {
    const result = compareStrings(left[index], right[index]);
    if (result) return result;
  }
  return left.length - right.length;
}

function compareStrings(left: string, right: string): number {
  return left < right ? -1 : left > right ? 1 : 0;
}
