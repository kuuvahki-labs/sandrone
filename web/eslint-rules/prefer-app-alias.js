import path from "node:path";

const parentRelativeImport = "../";

const preferAppAliasRule = {
  meta: {
    type: "layout",
    docs: {
      description: "Prefer the ~/ alias for parent relative imports within app.",
    },
    fixable: "code",
    messages: {
      preferAlias: "Use the ~/ alias for parent relative imports within app.",
    },
    schema: [],
  },
  create(context) {
    function checkNode(node) {
      const importPath = node.source?.value;
      if (typeof importPath !== "string" || !importPath.startsWith(parentRelativeImport)) {
        return;
      }

      const aliasPath = toAppAlias(context.filename, importPath);
      if (!aliasPath) {
        return;
      }

      context.report({
        node: node.source,
        messageId: "preferAlias",
        fix(fixer) {
          const rawSource = context.sourceCode.getText(node.source);
          const quote = rawSource.startsWith("'") ? "'" : '"';
          return fixer.replaceText(node.source, `${quote}${aliasPath}${quote}`);
        },
      });
    }

    return {
      ExportAllDeclaration: checkNode,
      ExportNamedDeclaration: checkNode,
      ImportDeclaration: checkNode,
    };
  },
};

export default preferAppAliasRule;

function toAppAlias(filename, importPath) {
  const appRoot = appRootFromFilename(filename);
  if (!appRoot) {
    return "";
  }

  const resolvedPath = path.resolve(path.dirname(filename), importPath);
  const relativeToApp = path.relative(appRoot, resolvedPath);
  if (!relativeToApp || relativeToApp.startsWith("..") || path.isAbsolute(relativeToApp)) {
    return "";
  }

  return `~/${relativeToApp.split(path.sep).join("/")}`;
}

function appRootFromFilename(filename) {
  const normalized = path.resolve(filename);
  const marker = `${path.sep}app${path.sep}`;
  const markerIndex = normalized.lastIndexOf(marker);
  if (markerIndex === -1) {
    return "";
  }
  return normalized.slice(0, markerIndex + marker.length - 1);
}
