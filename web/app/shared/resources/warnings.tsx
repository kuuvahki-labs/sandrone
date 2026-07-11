import { useId, useState } from "react";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import ButtonBase from "@mui/material/ButtonBase";
import Collapse from "@mui/material/Collapse";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import type { PreviewWarning } from "~/shared/resources/types";
import { CodeBlock } from "~/shared/ui/code-editor";

export function WarningList({ className, warnings }: { className?: string; warnings: readonly PreviewWarning[] }) {
  return (
    <List className={["grid gap-2 p-0", className].filter(Boolean).join(" ")}>
      {warnings.map((warning, index) => (
        <WarningListItem key={`${warning.code}:${warning.node ?? warning.source ?? warning.field ?? ""}:${index}`} warning={warning} />
      ))}
    </List>
  );
}

function WarningListItem({ warning }: { warning: PreviewWarning }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const detailsId = useId();
  const location = warningLocation(warning, t);
  const title = warningSubtitle(warning) || location;
  const context = warningContext(warning);
  const detailTitle = [location, title].filter(Boolean).join(" · ");

  return (
    <ListItem className="block min-w-0" disablePadding>
      <Paper className="min-w-0 overflow-hidden" variant="outlined">
        <ButtonBase
          aria-controls={detailsId}
          aria-expanded={expanded}
          aria-label={t("warnings.detailWithTitle", { title: detailTitle })}
          className="block w-full p-3 text-left"
          type="button"
          onClick={() => setExpanded((value) => !value)}
        >
          <span className="grid min-w-0 w-full grid-cols-[minmax(0,1fr)_auto] items-start gap-3">
            <span className="grid min-w-0 gap-1">
              <Typography className="break-words font-medium [overflow-wrap:anywhere]" component="h4" variant="subtitle1">
                {title}
              </Typography>
              {context.length ? (
                <Typography className="break-words [overflow-wrap:anywhere]" color="text.secondary" component="span" variant="caption">
                  {context.map(([key, value], index) => (
                    <span key={key}>
                      {index ? " · " : null}
                      {key}: <span>{value}</span>
                    </span>
                  ))}
                </Typography>
              ) : null}
            </span>
            <span className="pt-1 text-text-secondary">
              {expanded ? <KeyboardArrowUpIcon aria-hidden fontSize="small" /> : <KeyboardArrowDownIcon aria-hidden fontSize="small" />}
            </span>
          </span>
        </ButtonBase>
        <Collapse id={detailsId} in={expanded} timeout="auto" unmountOnExit>
          <div className="min-w-0 border-t border-divider p-3">
            <CodeBlock label={t("warnings.detailLabel")} language="json" value={stableStringify(warning)} />
          </div>
        </Collapse>
      </Paper>
    </ListItem>
  );
}

function warningLocation(warning: PreviewWarning, t: ReturnType<typeof useI18n>["t"]): string {
  const context = isRecord(warning.node_context) ? warning.node_context : undefined;
  const node = stringValue(warning.node) || stringValue(context?.name);
  if (node) {
    return node;
  }
  const source = stringValue(warning.source) || stringValue(warning.target) || stringValue(warning.field);
  if (source) {
    return source;
  }
  const line = numberValue(warning.line) ?? numberValue(context?.line);
  if (line !== undefined) {
    return t("warnings.lineTitle", { line });
  }
  return warning.code || t("warnings.unnamed");
}

function warningSubtitle(warning: PreviewWarning): string {
  return [warning.code, warning.message].filter(Boolean).join(" · ");
}

function warningContext(warning: PreviewWarning): [string, string][] {
  return [
    ["node", stringValue(warning.node) || stringValue(isRecord(warning.node_context) ? warning.node_context.name : undefined)],
    ["source", stringValue(warning.source)],
    ["target", stringValue(warning.target)],
    ["field", stringValue(warning.field)],
  ].filter((entry): entry is [string, string] => Boolean(entry[1]));
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function numberValue(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function stableStringify(value: unknown): string {
  return JSON.stringify(stableValue(value), null, 2);
}

function stableValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(stableValue);
  }
  if (isRecord(value)) {
    return Object.keys(value).sort().reduce<Record<string, unknown>>((result, key) => {
      const item = value[key];
      if (item !== undefined) {
        result[key] = stableValue(item);
      }
      return result;
    }, {});
  }
  return value;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
