import { Children, type ReactNode, useEffect, useId, useRef, useState } from "react";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import Box from "@mui/material/Box";
import Button from "@mui/material/Button";
import ButtonBase from "@mui/material/ButtonBase";
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

export function Metric({
  actionLabel,
  label,
  onSelect,
  selected = false,
  value,
}: {
  actionLabel?: string;
  label: string;
  onSelect?: () => void;
  selected?: boolean;
  value?: string | number | null;
}) {
  const content = (
    <div className="grid gap-1">
      <Typography
        className="break-words"
        color={selected ? "primary.main" : undefined}
        component="strong"
        variant="h5"
        sx={{ "@media (max-width:819px)": { fontSize: "1.25rem", lineHeight: 1.35 } }}
      >
        {value ?? "-"}
      </Typography>
      <Typography
        color={selected ? "primary.main" : "text.secondary"}
        variant="body2"
        sx={{ "@media (max-width:819px)": { fontSize: "0.75rem", lineHeight: 1.35 } }}
      >
        {label}
      </Typography>
    </div>
  );
  return (
    <Paper
      variant="outlined"
      className="min-w-0"
      sx={{
        bgcolor: selected ? "action.selected" : undefined,
        borderColor: selected ? "primary.main" : undefined,
        height: "100%",
        "@media (max-width:819px)": {
          border: 0,
          borderBottom: selected ? 2 : 0,
          borderBottomColor: selected ? "primary.main" : undefined,
          borderRadius: 0,
          boxShadow: "none",
          textAlign: "center",
        },
      }}
    >
      {onSelect ? (
        <ButtonBase
          aria-label={actionLabel}
          aria-pressed={selected}
          className="h-full w-full"
          type="button"
          sx={{
            display: "block",
            p: 2,
            textAlign: "inherit",
            "&:focus-visible": { outline: "2px solid", outlineColor: "primary.main", outlineOffset: -2 },
            "@media (max-width:819px)": { px: 0.75, py: 1 },
          }}
          onClick={onSelect}
        >
          {content}
        </ButtonBase>
      ) : (
        <Box
          sx={{
            p: 2,
            "@media (max-width:819px)": { px: 0.75, py: 1 },
          }}
        >
          {content}
        </Box>
      )}
    </Paper>
  );
}

export function MetricGroup({ children, label }: { children: ReactNode; label: string }) {
  const metrics = Children.toArray(children);
  return (
    <Box
      aria-label={label}
      sx={{
        display: "grid",
        gap: 2,
        gridTemplateColumns: `repeat(${metrics.length}, minmax(0, 1fr))`,
        "@media (max-width:819px)": { gap: 0 },
      }}
    >
      {metrics.map((metric, index) => (
        <Box
          key={index}
          sx={index === 0 ? undefined : {
            "@media (max-width:819px)": {
              borderColor: "divider",
              borderLeftStyle: "solid",
              borderLeftWidth: 1,
            },
          }}
        >
          {metric}
        </Box>
      ))}
    </Box>
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

  const mobileVisibleActions = secondaryActions.filter((action) => action.mobileVisible);
  const compactVisibleActions = mobile
    ? mobileVisibleActions.length
      ? mobileVisibleActions
      : secondaryActions.length === 1
        ? secondaryActions
        : []
    : secondaryActions;
  const compactOverflowActions = mobile
    ? secondaryActions.filter((action) => !compactVisibleActions.includes(action)).map(toDestinationListAction)
    : [];
  const actionMenu = (useCompactLayout || mobile) && compactOverflowActions.length ? (
    <ActionMenu actions={compactOverflowActions} buttonSize="medium" label={t("actions.more")} />
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
              <div className="flex items-center gap-1">
                {compactVisibleActions.map((action) => <PageHeaderActionButton action={action} compact key={action.accessibleLabel ?? action.label} />)}
              </div>
              {primaryAction ? <PageHeaderActionButton action={primaryAction} compact /> : null}
              {actionMenu}
            </div>
          </div>
        ) : (
          <div className={`grid gap-4 p-4 sm:p-5 ${backAction ? "pl-0 sm:pl-1" : ""}`}>
            <div className="flex min-w-0 items-center justify-between gap-3 min-[820px]:gap-4">
              <div className="flex min-w-0 flex-1 items-start gap-2">
                {backAction ? (
                  <Button className="mt-0.5 shrink-0 px-2 sm:px-3" startIcon={<ArrowBackIcon aria-hidden fontSize="small" />} type="button" onClick={backAction.onSelect}>
                    {backAction.label}
                  </Button>
                ) : null}
                <div className="min-w-0">
                  {label ? <Typography color="text.secondary" variant="overline">{label}</Typography> : null}
                  <Typography
                    className="break-words [overflow-wrap:anywhere]"
                    component="h2"
                    variant="h4"
                    sx={{ "@media (max-width:819px)": { fontSize: "1.5rem", lineHeight: 1.35 } }}
                  >
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
                <div className="flex min-w-0 shrink-0 flex-wrap items-center justify-end gap-2">
                  {badge}
                  <div className="flex items-center gap-2">
                    {(mobile ? compactVisibleActions : secondaryActions).map((action) => <PageHeaderActionButton action={action} compact={mobile} key={action.accessibleLabel ?? action.label} />)}
                  </div>
                  {primaryAction ? <PageHeaderActionButton action={primaryAction} compact={mobile} /> : null}
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
  mobileIconOnly?: boolean;
  mobileVisible?: boolean;
  onSelect?: () => void;
  tone?: "default" | "danger";
  type?: "button" | "submit";
  variant?: "contained" | "outlined" | "text";
}

function PageHeaderActionButton({ action, compact = false }: { action: PageHeaderAction; compact?: boolean }) {
  const reasonId = useId();
  const iconOnly = compact && action.mobileIconOnly && action.icon !== undefined && action.icon !== null;
  const button = iconOnly ? (
    <IconButton
      aria-hidden={action.disabled && action.disabledReason ? true : undefined}
      aria-describedby={action.disabledReason ? reasonId : undefined}
      aria-label={action.accessibleLabel ?? action.label}
      color={action.tone === "danger" ? "error" : "primary"}
      disabled={action.disabled}
      size="small"
      type={action.type ?? "button"}
      onClick={action.onSelect}
    >
      {action.icon}
    </IconButton>
  ) : (
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
    return iconOnly ? <Tooltip title={action.label}>{button}</Tooltip> : button;
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
