import { type ReactNode, useEffect, useId, useRef, useState } from "react";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import useMediaQuery from "@mui/material/useMediaQuery";

import { useI18n } from "~/shared/i18n/context";
import {
  ActionMenu,
  type DestinationListAction,
} from "~/shared/ui/resource-list";

export function Metric({ label, value }: { label: string; value?: string | number | null }) {
  return (
    <Paper variant="outlined" className="min-w-0 p-4">
      <div className="grid gap-1">
        <Typography className="break-words" component="strong" variant="h5">
          {value ?? "-"}
        </Typography>
        <Typography color="text.secondary" variant="body2">
          {label}
        </Typography>
      </div>
    </Paper>
  );
}

export function PageHeader({
  backAction,
  badge,
  description,
  label,
  metrics,
  primaryAction,
  secondaryActions = [],
  sticky = false,
  title,
}: {
  backAction?: PageHeaderBackAction;
  badge?: ReactNode;
  description?: ReactNode;
  label: string;
  metrics?: ReactNode;
  primaryAction?: PageHeaderAction;
  secondaryActions?: PageHeaderAction[];
  sticky?: boolean;
  title: string;
}) {
  const { t } = useI18n();
  const mobile = useMediaQuery("(max-width:819px)");
  const sentinelRef = useRef<HTMLDivElement>(null);
  const [compact, setCompact] = useState(false);
  const useCompactLayout = compact || (sticky && mobile);

  useEffect(() => {
    if (!sticky) return;
    const updateCompactState = () => {
      setCompact((sentinelRef.current?.getBoundingClientRect().bottom ?? 0) < 0);
    };
    updateCompactState();
    window.addEventListener("scroll", updateCompactState, { passive: true });
    window.addEventListener("resize", updateCompactState);
    return () => {
      window.removeEventListener("scroll", updateCompactState);
      window.removeEventListener("resize", updateCompactState);
    };
  }, [sticky]);

  const overflowActions = secondaryActions.map(toDestinationListAction);
  const actionMenu = useCompactLayout && overflowActions.length > 1 ? (
    <div className="min-[820px]:hidden">
      <ActionMenu actions={overflowActions} buttonSize="medium" label={t("actions.more")} />
    </div>
  ) : null;

  return (
    <>
      {sticky ? <div aria-hidden className="-mb-px h-px" ref={sentinelRef} /> : null}
      <Paper
        component="header"
        data-page-header-compact={compact ? "true" : "false"}
        variant="outlined"
        className={sticky ? "sticky top-0 z-[1000]" : undefined}
        sx={useCompactLayout ? { borderLeft: 0, borderRadius: 0, borderRight: 0, boxShadow: 2 } : undefined}
      >
        {useCompactLayout ? (
          <div className="flex min-h-14 min-w-0 items-center gap-1 px-1.5 py-1 sm:gap-2 sm:px-3">
            {backAction ? (
              <IconButton aria-label={backAction.label} className="shrink-0" type="button" onClick={backAction.onSelect}>
                <ArrowBackIcon aria-hidden />
              </IconButton>
            ) : null}
            <Typography className="min-w-0 flex-1 truncate" component="h2" variant="h6">
              {title}
            </Typography>
            <div className="flex shrink-0 items-center gap-1">
              <div className={`items-center gap-1 min-[820px]:flex ${secondaryActions.length === 1 ? "flex" : "hidden"}`}>
                {secondaryActions.map((action) => <PageHeaderActionButton action={action} compact key={action.accessibleLabel ?? action.label} />)}
              </div>
              {primaryAction ? <PageHeaderActionButton action={primaryAction} compact /> : null}
              {actionMenu}
            </div>
          </div>
        ) : (
          <div className={`grid gap-4 p-4 sm:p-5 ${backAction ? "pl-0 sm:pl-1" : ""}`}>
            <div className="flex flex-col items-stretch justify-between gap-4 sm:flex-row sm:items-center">
              <div className="flex min-w-0 items-start gap-2">
                {backAction ? (
                  <Button className="mt-0.5 shrink-0 px-2 sm:px-3" startIcon={<ArrowBackIcon aria-hidden fontSize="small" />} type="button" onClick={backAction.onSelect}>
                    {backAction.label}
                  </Button>
                ) : null}
                <div className="min-w-0">
                  {label ? <Typography color="text.secondary" variant="overline">{label}</Typography> : null}
                  <Typography className="break-words [overflow-wrap:anywhere]" component="h2" variant="h4">
                    {title}
                  </Typography>
                  {description ? (
                    <Typography className="break-words [overflow-wrap:anywhere]" color="text.secondary" component="div">
                      {description}
                    </Typography>
                  ) : null}
                </div>
              </div>
              {badge || primaryAction || secondaryActions.length ? (
                <div className="flex min-w-0 flex-wrap items-center justify-end gap-2">
                  {badge}
                  <div className="flex items-center gap-2">
                    {secondaryActions.map((action) => <PageHeaderActionButton action={action} key={action.accessibleLabel ?? action.label} />)}
                  </div>
                  {primaryAction ? <PageHeaderActionButton action={primaryAction} /> : null}
                  {actionMenu}
                </div>
              ) : null}
            </div>
            {metrics}
          </div>
        )}
      </Paper>
    </>
  );
}

export interface PageHeaderBackAction {
  label: string;
  onSelect: () => void;
}

export interface PageHeaderAction {
  accessibleLabel?: string;
  disabled?: boolean;
  disabledReason?: string;
  icon?: ReactNode;
  label: string;
  onSelect?: () => void;
  tone?: "default" | "danger";
  type?: "button" | "submit";
  variant?: "contained" | "outlined" | "text";
}

function PageHeaderActionButton({ action, compact = false }: { action: PageHeaderAction; compact?: boolean }) {
  const reasonId = useId();
  const button = (
    <Button
      aria-hidden={action.disabled && action.disabledReason ? true : undefined}
      aria-describedby={action.disabledReason ? reasonId : undefined}
      aria-label={action.accessibleLabel}
      color={action.tone === "danger" ? "error" : "primary"}
      disabled={action.disabled}
      size={compact ? "small" : "medium"}
      startIcon={action.icon}
      type={action.type ?? "button"}
      variant={action.variant ?? "outlined"}
      onClick={action.onSelect}
    >
      {action.label}
    </Button>
  );
  if (!action.disabled || !action.disabledReason) {
    return button;
  }
  return (
    <>
      <Tooltip describeChild title={action.disabledReason}>
        <span
          aria-describedby={reasonId}
          aria-disabled="true"
          aria-label={action.accessibleLabel ?? action.label}
          className="inline-flex"
          role="button"
          tabIndex={0}
        >
          {button}
        </span>
      </Tooltip>
      <span className="sr-only" id={reasonId}>{action.disabledReason}</span>
    </>
  );
}

function toDestinationListAction(action: PageHeaderAction): DestinationListAction {
  return {
    accessibleLabel: action.accessibleLabel,
    disabled: action.disabled,
    disabledReason: action.disabledReason,
    icon: action.icon,
    label: action.label,
    onSelect: action.onSelect ?? (() => undefined),
    tone: action.tone,
  };
}
