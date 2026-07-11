import { type ReactNode, useState } from "react";
import CheckCircleOutlinedIcon from "@mui/icons-material/CheckCircleOutlined";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import WarningAmberIcon from "@mui/icons-material/WarningAmber";
import Collapse from "@mui/material/Collapse";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Typography from "@mui/material/Typography";

export type ConfigWorkbenchSectionSeverity = "default" | "success" | "warning" | "error";

export interface ConfigWorkbenchSectionProps {
  children: ReactNode;
  collapsible?: boolean;
  count?: number;
  defaultExpanded?: boolean;
  expanded?: boolean;
  headerActions?: ReactNode;
  id: string;
  label: string;
  onExpandedChange?: (expanded: boolean) => void;
  severity?: ConfigWorkbenchSectionSeverity;
  severityLabel?: string;
  summary?: number | string;
}

const severityClassName: Record<ConfigWorkbenchSectionSeverity, string> = {
  default: "text-text-secondary",
  error: "text-error",
  success: "text-success",
  warning: "text-warning",
};

export function ConfigWorkbenchSection({
  children,
  collapsible = true,
  count,
  defaultExpanded = false,
  expanded,
  headerActions,
  id,
  label,
  onExpandedChange,
  severity = "default",
  severityLabel,
  summary,
}: ConfigWorkbenchSectionProps) {
  const [uncontrolledExpanded, setUncontrolledExpanded] = useState(defaultExpanded);
  const isExpanded = expanded ?? uncontrolledExpanded;
  const headerId = `${id}-header`;
  const contentId = `${id}-content`;

  function toggleExpanded() {
    const nextExpanded = !isExpanded;
    if (expanded === undefined) {
      setUncontrolledExpanded(nextExpanded);
    }
    onExpandedChange?.(nextExpanded);
  }

  const headerContent = (
    <div className="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-0.5" data-slot="section-info">
      <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
        <Typography className="min-w-0 font-semibold" component="span" variant="body2">
          {label}
        </Typography>
        {count !== undefined ? (
          <Typography className="shrink-0 rounded-full bg-action-hover px-2 py-0.5" color="text.secondary" component="span" variant="caption">
            {count}
          </Typography>
        ) : null}
        {summary !== undefined && summary !== null ? (
          <Typography className="min-w-0 truncate max-sm:basis-full" color="text.secondary" component="span" variant="caption">
            {summary}
          </Typography>
        ) : null}
      </span>
      {severity !== "default" ? (
        <span className={`flex min-w-0 items-center gap-1 ${severityClassName[severity]}`} data-slot="section-status">
          <SeverityIcon severity={severity} />
          <Typography className="min-w-0 truncate max-sm:sr-only" color="inherit" component="span" variant="caption">
            {severityLabel ?? severity}
          </Typography>
        </span>
      ) : null}
    </div>
  );

  return (
    <Paper
      className="min-w-0 overflow-hidden rounded-md"
      component="section"
      data-severity={severity}
      variant="outlined"
    >
      <div className="flex min-h-11 min-w-0 items-center gap-2 px-3 py-2 text-left">
        {headerContent}
        {headerActions ? (
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-2" data-slot="section-actions">
            {headerActions}
          </div>
        ) : null}
        {collapsible ? (
          <IconButton
            aria-controls={contentId}
            aria-expanded={isExpanded}
            aria-label={label}
            className="shrink-0 text-text-secondary"
            id={headerId}
            size="small"
            type="button"
            onClick={toggleExpanded}
          >
            {isExpanded ? <ExpandLessIcon aria-hidden fontSize="small" /> : <ExpandMoreIcon aria-hidden fontSize="small" />}
          </IconButton>
        ) : <span className="sr-only" id={headerId}>{label}</span>}
      </div>
      {collapsible ? (
        <Collapse aria-labelledby={headerId} id={contentId} in={isExpanded} role="region" timeout="auto">
          <div className="grid min-w-0 gap-3 border-t border-divider p-3">
            {children}
          </div>
        </Collapse>
      ) : (
        <div aria-labelledby={headerId} className="grid min-w-0 gap-3 border-t border-divider p-3" id={contentId} role="region">
          {children}
        </div>
      )}
    </Paper>
  );
}

function SeverityIcon({ severity }: { severity: ConfigWorkbenchSectionSeverity }) {
  switch (severity) {
    case "success":
      return <CheckCircleOutlinedIcon aria-hidden fontSize="small" />;
    case "warning":
      return <WarningAmberIcon aria-hidden fontSize="small" />;
    case "error":
      return <ErrorOutlinedIcon aria-hidden fontSize="small" />;
    default:
      return <InfoOutlinedIcon aria-hidden fontSize="small" />;
  }
}
