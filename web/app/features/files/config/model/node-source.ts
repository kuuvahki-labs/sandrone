import type { PreviewWarning } from "~/shared/resources/types";

export interface ConfigNodePreviewInput {
  readonly subscriptionName: string;
  readonly nodes: readonly {
    readonly runtimeId: string;
    readonly after?: {
      readonly name: string;
      readonly type?: string;
      readonly endpoint: string;
    };
    readonly targetNames?: Readonly<Record<string, string>>;
  }[];
  readonly warnings: readonly ConfigNodePreviewWarningInput[];
}

interface ConfigNodePreviewWarningInput {
  readonly code: string;
  readonly message: string;
  readonly field?: string;
  readonly node?: string;
  readonly source?: string;
  readonly target?: string;
}

export interface ConfigNodeSummary {
  endpoint: string;
  key: string;
  name: string;
  type?: string;
}

export type ConfigNodeWarning = PreviewWarning;

export interface ConfigTargetNodeOptions {
  coverage: "complete" | "partial";
  options: ConfigNodeSummary[];
}

export interface ConfigNodePreview {
  duplicateNames: string[];
  nodes: ConfigNodeSummary[];
  options: ConfigNodeSummary[];
  renderCandidates: ConfigNodeSummary[];
  subscriptionName: string;
  targetOptions?: Readonly<Record<string, ConfigTargetNodeOptions>>;
  unnamedCount: number;
  warnings: ConfigNodeWarning[];
}

export function configNodePreviewFromSubscription(
  preview: ConfigNodePreviewInput,
): ConfigNodePreview {
  const nodes: ConfigNodeSummary[] = [];
  const renderCandidates: ConfigNodeSummary[] = [];
  const targetOptions = new Map<string, { options: ConfigNodeSummary[]; presentCount: number }>();
  let unnamedCount = 0;
  for (const diff of preview.nodes) {
    const node = diff.after;
    if (!node) continue;
    const summary: ConfigNodeSummary = {
      key: diff.runtimeId,
      name: node.name,
      ...(node.type ? { type: node.type } : {}),
      endpoint: node.endpoint,
    };
    renderCandidates.push(summary);

    for (const [target, targetName] of Object.entries(diff.targetNames ?? {})) {
      const current = targetOptions.get(target) ?? { options: [], presentCount: 0 };
      current.presentCount += 1;
      if (targetName) current.options.push({ ...summary, name: targetName });
      targetOptions.set(target, current);
    }

    if (!node.name.trim()) {
      unnamedCount += 1;
      continue;
    }
    nodes.push(summary);
  }

  const seen = new Set<string>();
  const duplicateNames = new Set<string>();
  const options = nodes.filter((node) => {
    if (seen.has(node.name)) {
      duplicateNames.add(node.name);
      return false;
    }
    seen.add(node.name);
    return true;
  });
  const realizedOptions = targetOptions.size
    ? Object.fromEntries([...targetOptions].map(([target, realized]) => [
      target,
      {
        coverage: realized.presentCount === renderCandidates.length ? "complete" : "partial",
        options: realized.options,
      } satisfies ConfigTargetNodeOptions,
    ]))
    : undefined;

  return {
    subscriptionName: preview.subscriptionName,
    nodes,
    options,
    renderCandidates,
    ...(realizedOptions ? { targetOptions: realizedOptions } : {}),
    warnings: preview.warnings.map(safeWarning),
    duplicateNames: [...duplicateNames],
    unnamedCount,
  };
}

function safeWarning(warning: ConfigNodePreviewWarningInput): ConfigNodeWarning {
  return {
    code: warning.code,
    message: warning.message,
    ...(warning.field ? { field: warning.field } : {}),
    ...(warning.node ? { node: warning.node } : {}),
    ...(warning.source ? { source: warning.source } : {}),
    ...(warning.target ? { target: warning.target } : {}),
  };
}
