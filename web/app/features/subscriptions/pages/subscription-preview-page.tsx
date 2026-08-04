import { useId, useState } from "react";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import RefreshIcon from "@mui/icons-material/Refresh";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardActionArea from "@mui/material/CardActionArea";
import CardContent from "@mui/material/CardContent";
import Collapse from "@mui/material/Collapse";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";

import type { SubscriptionItem, SubscriptionPreview, SubscriptionPreviewNode, SubscriptionPreviewNodeDiff, SubscriptionPreviewStatus } from "~/features/subscriptions/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { WarningList } from "~/shared/resources/warnings";
import { CodeBlock } from "~/shared/ui/code-editor";
import { Metric, PageHeader } from "~/shared/ui/page";

type PreviewFilter = SubscriptionPreviewStatus | "all";
type PreviewDetailMode = "diff" | "meta";
type StatusColor = "success" | "warning" | "error" | "info";

const maxSummaryChanges = 3;

export interface SubscriptionPreviewPageProps {
  backLabel: string;
  failed?: boolean;
  item: SubscriptionItem;
  pending?: boolean;
  preview?: SubscriptionPreview;
  onBack: () => void;
  onRefresh: () => void;
  onShare: () => void;
}

export function SubscriptionPreviewPage({ backLabel, failed = false, pending = false, preview, onBack, onRefresh, onShare }: SubscriptionPreviewPageProps) {
  const { t } = useI18n();
  const [filter, setFilter] = useState<PreviewFilter>("all");
  const nodes = preview?.nodes ?? [];
  const visibleNodes = filter === "all" ? nodes : nodes.filter((node) => node.status === filter);

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: backLabel, onSelect: onBack }}
        label=""
        primaryAction={{ accessibleLabel: t("subscriptions.preview.refresh"), disabled: pending, icon: <RefreshIcon aria-hidden fontSize="small" />, label: t("actions.refresh"), onSelect: onRefresh }}
        secondaryActions={[{ accessibleLabel: t("subscriptions.actions.share"), icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare }]}
        sticky
        title={t("subscriptions.preview.title")}
      />

      <div aria-label={t("subscriptions.preview.summary")} className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Metric label={t("subscriptions.preview.metricBefore")} value={preview?.beforeCount} />
        <Metric label={t("subscriptions.preview.metricAfter")} value={preview?.afterCount} />
        <Metric label={t("subscriptions.preview.metricRemoved")} value={preview?.statusCounts.removed} />
        <Metric label={t("subscriptions.preview.metricWarnings")} value={preview?.warnings.length} />
      </div>

      {pending && !preview ? (
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-2">
              <Typography component="h3" variant="h6">
                {t("subscriptions.preview.loadingTitle")}
              </Typography>
              <Typography color="text.secondary">{t("subscriptions.preview.loadingDescription")}</Typography>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {failed && !preview ? (
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-2">
              <Typography component="h3" variant="h6">
                {t("subscriptions.preview.errorTitle")}
              </Typography>
              <Typography color="text.secondary">{t("subscriptions.preview.errorDescription")}</Typography>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {preview ? (
        <>
          <div aria-label={t("subscriptions.preview.filter")} className="flex flex-wrap gap-2">
            {previewFilters(preview, t).map((option) => (
              <Button
                key={option.value}
                color={filterColor(option.value)}
                type="button"
                variant={filter === option.value ? "contained" : "outlined"}
                onClick={() => setFilter(option.value)}
              >
                {option.label} {option.count}
              </Button>
            ))}
          </div>

          {preview.warnings.length ? (
            <Card component="section" variant="outlined" aria-label={t("subscriptions.preview.warnings")}>
              <CardContent>
                <div className="grid gap-3">
                  <Typography component="h3" variant="h6">
                    {t("common.warning")}
                  </Typography>
                  <WarningList warnings={preview.warnings} />
                </div>
              </CardContent>
            </Card>
          ) : null}

          <List aria-label={t("subscriptions.preview.nodeList")} className="grid gap-3 p-0">
            {visibleNodes.length ? visibleNodes.map((diff) => (
              <PreviewNodeCard diff={diff} key={`${diff.identity}:${diff.status}:${diff.before?.name ?? ""}:${diff.after?.name ?? ""}`} />
            )) : (
              <ListItem className="block" disablePadding>
                <Card component="article" variant="outlined">
                  <CardContent>
                    <Typography component="h2" variant="h6">
                      {t("subscriptions.preview.empty")}
                    </Typography>
                  </CardContent>
                </Card>
              </ListItem>
            )}
          </List>
        </>
      ) : null}
    </section>
  );
}

function PreviewNodeCard({ diff }: { diff: SubscriptionPreviewNodeDiff }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const [detailMode, setDetailMode] = useState<PreviewDetailMode>("diff");
  const detailsId = useId();
  const node = previewNodeForSummary(diff);
  const nodeDiff = formatNodeDiff(diff);
  const detailValue = detailMode === "meta" ? formatNodeMetadataDiff(diff) : nodeDiff;
  const nodeName = node?.name || t("subscriptions.preview.unnamedNode");
  const hasMetadata = Boolean(nodeMetadataPayload(diff));
  const detailModeControl = (
    <ToggleButtonGroup
      aria-label={t("subscriptions.preview.detailDisplay")}
      exclusive
      size="small"
      value={detailMode}
      onChange={(_, value: PreviewDetailMode | null) => {
        if (value) {
          setDetailMode(value);
        }
      }}
    >
      <ToggleButton aria-label={t("subscriptions.preview.diff")} value="diff">{t("subscriptions.preview.diff")}</ToggleButton>
      {hasMetadata ? <ToggleButton aria-label={t("subscriptions.preview.meta")} value="meta">{t("subscriptions.preview.meta")}</ToggleButton> : null}
    </ToggleButtonGroup>
  );

  return (
    <ListItem className="block min-w-0" disablePadding>
      <Card className="min-w-0" component="article" variant="outlined">
        <CardActionArea
          aria-controls={detailsId}
          aria-expanded={expanded}
          aria-label={t("subscriptions.preview.detailWithName", { name: nodeName })}
          className="block text-left"
          onClick={() => setExpanded((value) => !value)}
        >
          <CardContent className="min-w-0 [&:last-child]:pb-4">
            <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
              <PreviewNodeSummary diff={diff} />
              <span className="pt-1 text-text-secondary">
                {expanded ? <KeyboardArrowUpIcon aria-hidden fontSize="small" /> : <KeyboardArrowDownIcon aria-hidden fontSize="small" />}
              </span>
            </div>
          </CardContent>
        </CardActionArea>
        <Collapse id={detailsId} in={expanded} timeout="auto" unmountOnExit>
          <CardContent className="min-w-0 border-t border-divider pt-3">
            <CodeBlock label={t("subscriptions.preview.detailLabel")} language="json-diff" toolbar={detailModeControl} value={detailValue} />
          </CardContent>
        </Collapse>
      </Card>
    </ListItem>
  );
}

function PreviewNodeSummary({ diff }: { diff: SubscriptionPreviewNodeDiff }) {
  const { t } = useI18n();
  const node = previewNodeForSummary(diff);
  const changes = diff.status === "modified" ? nodeFieldChanges(diff) : [];
  const visibleChanges = changes.slice(0, maxSummaryChanges);
  const remainingChanges = changes.length - visibleChanges.length;

  return (
    <div className="grid min-w-0 gap-2">
      <NodeSummaryItem node={node} />
      {visibleChanges.length ? (
        <div aria-label={t("subscriptions.preview.nodeChangeSummary")} className="grid min-w-0 gap-1">
          {visibleChanges.map((change) => (
            <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] gap-2" key={change.field}>
              <Typography className="font-mono" color="text.secondary" component="span" variant="body2">
                {change.field}
              </Typography>
              <Typography className="break-words [overflow-wrap:anywhere]" component="span" variant="body2">
                {formatFieldChange(change, t)}
              </Typography>
            </div>
          ))}
          {remainingChanges > 0 ? (
            <Typography color="text.secondary" variant="body2">
              {t("subscriptions.preview.remainingChanges", { count: remainingChanges })}
            </Typography>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function NodeSummaryItem({ node }: { node?: SubscriptionPreviewNode }) {
  const { t } = useI18n();
  return (
    <div className="grid min-w-0 gap-1">
      <Typography className="break-words font-medium [overflow-wrap:anywhere]" component="p" variant="subtitle1">
        {node?.name || t("subscriptions.preview.unnamedNode")}
      </Typography>
      <Typography className="break-words [overflow-wrap:anywhere]" color="text.secondary" variant="body2">
        {node?.type ?? "-"} · {node?.endpoint ?? "-"}
      </Typography>
    </div>
  );
}

function previewFilters(preview: SubscriptionPreview, t: Translator): Array<{ value: PreviewFilter; label: string; count: number }> {
  return [
    { value: "all", label: t("subscriptions.preview.statusAll"), count: preview.nodes.length },
    { value: "modified", label: t("subscriptions.preview.statusModified"), count: preview.statusCounts.modified },
    { value: "removed", label: t("subscriptions.preview.statusRemoved"), count: preview.statusCounts.removed },
    { value: "added", label: t("subscriptions.preview.statusAdded"), count: preview.statusCounts.added },
    { value: "unchanged", label: t("subscriptions.preview.statusUnchanged"), count: preview.statusCounts.unchanged },
  ];
}

function filterColor(status: PreviewFilter) {
  return status === "all" ? "primary" : statusColor(status);
}

function statusColor(status: SubscriptionPreviewStatus): StatusColor {
  switch (status) {
    case "added":
      return "success";
    case "modified":
      return "warning";
    case "removed":
      return "error";
    case "unchanged":
      return "info";
  }
}

function formatNodeDiff(diff: SubscriptionPreviewNodeDiff): string {
  const before = nodeDiffPayload(diff.before);
  const after = nodeDiffPayload(diff.after);

  if (diff.status === "added") {
    return prefixedLines(after, "+");
  }
  if (diff.status === "removed") {
    return prefixedLines(before, "-");
  }

  return fullJsonDiff(before, after);
}

function previewNodeForSummary(diff: SubscriptionPreviewNodeDiff): SubscriptionPreviewNode | undefined {
  return diff.after ?? diff.before;
}

interface NodeFieldChange {
  after: unknown;
  before: unknown;
  field: string;
}

function nodeFieldChanges(diff: SubscriptionPreviewNodeDiff): NodeFieldChange[] {
  const before = nodeDiffPayload(diff.before) ?? {};
  const after = nodeDiffPayload(diff.after) ?? {};
  const fields = Array.from(new Set([...Object.keys(before), ...Object.keys(after)])).sort(compareNodeField);
  return fields
    .filter((field) => !stableEqual(before[field], after[field]))
    .map((field) => ({ after: after[field], before: before[field], field }));
}

function compareNodeField(left: string, right: string): number {
  const leftRank = left === "name" ? 0 : 1;
  const rightRank = right === "name" ? 0 : 1;
  return leftRank - rightRank || left.localeCompare(right);
}

function stableEqual(left: unknown, right: unknown): boolean {
  return JSON.stringify(stableValue(left)) === JSON.stringify(stableValue(right));
}

function fieldDiffLines(change: NodeFieldChange): string[] {
  const lines: string[] = [];
  if (change.before !== undefined) {
    lines.push(`- ${formatFieldEntry(change.field, change.before)}`);
  }
  if (change.after !== undefined) {
    lines.push(`+ ${formatFieldEntry(change.field, change.after)}`);
  }
  return lines;
}

function formatFieldEntry(field: string, value: unknown): string {
  return `${JSON.stringify(field)}: ${formatJsonValue(value)}`;
}

function formatJsonValue(value: unknown): string {
  return JSON.stringify(stableValue(value)) ?? String(value);
}

function formatFieldChange(change: NodeFieldChange, t?: Translator): string {
  if (change.before === undefined) {
    return t ? t("subscriptions.preview.changeAdded") : "added";
  }
  if (change.after === undefined) {
    return t ? t("subscriptions.preview.changeRemoved") : "removed";
  }
  if (isStructuredValue(change.before) || isStructuredValue(change.after)) {
    return t ? t("subscriptions.preview.changeUpdated") : "updated";
  }
  return `${formatSummaryValue(change.before)} -> ${formatSummaryValue(change.after)}`;
}

function formatSummaryValue(value: unknown): string {
  if (value === null) {
    return "null";
  }
  const text = typeof value === "string" ? value : String(value);
  return text.length > 72 ? `${text.slice(0, 69)}...` : text;
}

function isStructuredValue(value: unknown): boolean {
  return Boolean(value) && typeof value === "object";
}

function prefixedLines(value: unknown, prefix: "+" | "-"): string {
  return stableStringify(value).split("\n").map((line) => `${prefix} ${line}`).join("\n");
}

function fullJsonDiff(before: Record<string, unknown> | undefined, after: Record<string, unknown> | undefined): string {
  if (!before && !after) {
    return stableStringify({});
  }
  if (!before) {
    return prefixedLines(after, "+");
  }
  if (!after) {
    return prefixedLines(before, "-");
  }
  const beforeText = stableStringify(before, { nameFirst: true });
  const afterText = stableStringify(after, { nameFirst: true });
  if (beforeText === afterText) {
    return afterText;
  }
  return lineDiff(beforeText.split("\n"), afterText.split("\n")).join("\n");
}

type LineDiffStep = {
  line: string;
  type: "added" | "removed" | "unchanged";
};

function lineDiff(before: string[], after: string[]): string[] {
  const lengths = Array.from({ length: before.length + 1 }, () => Array(after.length + 1).fill(0) as number[]);
  for (let i = before.length - 1; i >= 0; i--) {
    for (let j = after.length - 1; j >= 0; j--) {
      lengths[i][j] = before[i] === after[j] ? lengths[i + 1][j + 1] + 1 : Math.max(lengths[i + 1][j], lengths[i][j + 1]);
    }
  }
  const steps: LineDiffStep[] = [];
  let i = 0;
  let j = 0;
  while (i < before.length || j < after.length) {
    if (i < before.length && j < after.length && before[i] === after[j]) {
      steps.push({ type: "unchanged", line: before[i] });
      i++;
      j++;
    } else if (i < before.length && (j >= after.length || lengths[i + 1][j] >= lengths[i][j + 1])) {
      steps.push({ type: "removed", line: before[i] });
      i++;
    } else if (j < after.length) {
      steps.push({ type: "added", line: after[j] });
      j++;
    }
  }
  return steps.map((step) => step.type === "unchanged" ? step.line : `${step.type === "added" ? "+" : "-"} ${step.line}`);
}

function nodeDiffPayload(node?: SubscriptionPreviewNode): Record<string, unknown> | undefined {
  if (!node) {
    return undefined;
  }
  return stableObject({
    ...nodeRawWithoutMeta(node),
    name: node.name,
    type: node.type,
    server: node.server,
    port: node.port,
    endpoint: node.endpoint,
  });
}

function nodeRawWithoutMeta(node: SubscriptionPreviewNode): Record<string, unknown> {
  return Object.fromEntries(Object.entries(node.raw ?? {}).filter(([key]) => key !== "meta"));
}

function nodeMetadataPayload(diff: SubscriptionPreviewNodeDiff): Record<string, unknown> | undefined {
  const before = nodeMetadata(diff.before);
  const after = nodeMetadata(diff.after);
  if (!before && !after) {
    return undefined;
  }
  return stableObject({
    before,
    after,
  });
}

function nodeMetadata(node?: SubscriptionPreviewNode): Record<string, unknown> | undefined {
  const meta = node?.raw?.meta;
  if (!isRecord(meta) || Object.keys(meta).length === 0) {
    return undefined;
  }
  return stableObject(meta);
}

function formatNodeMetadataDiff(diff: SubscriptionPreviewNodeDiff): string {
  const before = nodeMetadata(diff.before);
  const after = nodeMetadata(diff.after);
  if (before && !after) {
    return prefixedLines(before, "-");
  }
  if (!before && after) {
    return prefixedLines(after, "+");
  }
  if (!before && !after) {
    return stableStringify({});
  }
  if (stableEqual(before, after)) {
    return stableStringify(after ?? before);
  }
  const fields = Array.from(new Set([...Object.keys(before ?? {}), ...Object.keys(after ?? {})])).sort();
  const lines = fields
    .filter((field) => !stableEqual(before?.[field], after?.[field]))
    .flatMap((field) => fieldDiffLines({ before: before?.[field], after: after?.[field], field }));
  return lines.join("\n") || stableStringify(after ?? before);
}

function stableStringify(value: unknown, options: { nameFirst?: boolean } = {}): string {
  return JSON.stringify(stableValue(value, options.nameFirst), null, 2);
}

function stableValue(value: unknown, nameFirst = false): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => stableValue(item));
  }
  if (isRecord(value)) {
    return stableObject(value, nameFirst);
  }
  return value;
}

function stableObject(value: Record<string, unknown>, nameFirst = false): Record<string, unknown> {
  return Object.keys(value).sort((left, right) => {
    if (nameFirst && (left === "name" || right === "name")) {
      return left === right ? 0 : left === "name" ? -1 : 1;
    }
    return left.localeCompare(right);
  }).reduce<Record<string, unknown>>((result, key) => {
    const item = value[key];
    if (item !== undefined) {
      result[key] = stableValue(item);
    }
    return result;
  }, {});
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
