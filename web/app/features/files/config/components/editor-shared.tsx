import { type ReactNode, useState } from "react";
import { KeyboardSensor, PointerSensor, useSensor, useSensors } from "@dnd-kit/core";
import { sortableKeyboardCoordinates } from "@dnd-kit/sortable";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowRightIcon from "@mui/icons-material/KeyboardArrowRight";
import Alert from "@mui/material/Alert";
import IconButton from "@mui/material/IconButton";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import type { ConfigValidationIssue } from "~/features/files/config/model/relations";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { ActionMenu } from "~/shared/ui/resource-list";

import { ConfigWorkbenchSection, type ConfigWorkbenchSectionProps, type ConfigWorkbenchSectionSeverity } from "./workbench-section";

type WorkbenchGroupSectionProps = ConfigWorkbenchSectionProps & { keepMounted?: boolean };

export const configEditorPanelClassName = "grid gap-3 border-t border-divider bg-action-hover px-3 py-3 sm:px-4";

export function WorkbenchGroupSection({ children, collapsible = true, defaultExpanded = false, expanded, keepMounted = false, label, onExpandedChange, ...props }: WorkbenchGroupSectionProps) {
  const [localExpanded, setLocalExpanded] = useState(defaultExpanded);
  const isExpanded = expanded ?? localExpanded;
  return (
    <div aria-label={label} className="min-w-0" role="group">
      <ConfigWorkbenchSection
        collapsible={collapsible}
        expanded={isExpanded}
        label={label}
        onExpandedChange={(next) => {
          if (expanded === undefined) setLocalExpanded(next);
          onExpandedChange?.(next);
        }}
        {...props}
      >
        {!collapsible || keepMounted || isExpanded ? children : null}
      </ConfigWorkbenchSection>
    </div>
  );
}

export function SectionIssues({ issues }: { issues: ConfigValidationIssue[] }) {
  const { t } = useI18n();
  if (!issues.length) return null;
  const severity = issues.some((issue) => issue.severity === "error") ? "error" : "warning";
  return (
    <Alert role="alert" severity={severity} variant="outlined">
      <ul className="grid list-disc gap-1 pl-4">
        {issues.map((issue, index) => <li key={`${issue.code}-${issue.itemId}-${index}`}>{configIssueMessage(issue, t)}</li>)}
      </ul>
    </Alert>
  );
}

export function severityForIssues(issues: ConfigValidationIssue[]): ConfigWorkbenchSectionSeverity {
  if (issues.some((issue) => issue.severity === "error")) return "error";
  if (issues.some((issue) => issue.severity === "warning")) return "warning";
  return "success";
}

export function issueSummary(issues: ConfigValidationIssue[], t: Translator, fallback?: string): string | undefined {
  const errors = issues.filter((issue) => issue.severity === "error").length;
  const warnings = issues.length - errors;
  if (errors) return t("files.config.issueCountError", { count: errors });
  if (warnings) return t("files.config.issueCountWarning", { count: warnings });
  return fallback;
}

export function ConfigRowSummary({ primary, secondary = [] }: { primary: ReactNode; secondary?: ReactNode[] }) {
  return (
    <span className="grid min-w-0 flex-1 gap-0.5 py-0.5">
      <Typography className="min-w-0 break-words font-semibold whitespace-normal [overflow-wrap:anywhere]" component="span" variant="body2">
        {primary}
      </Typography>
      {secondary.length ? (
        <Typography className="min-w-0 break-words whitespace-normal [overflow-wrap:anywhere]" color="text.secondary" component="span" variant="caption">
          {secondary.map((item, index) => (
            <span className="[overflow-wrap:anywhere]" key={index}>
              {index > 0 ? <span> · </span> : null}
              {item}
            </span>
          ))}
        </Typography>
      ) : null}
    </span>
  );
}

export function DenseConfigRow({ children }: { children: ReactNode }) {
  return <div className="flex min-h-10 min-w-0 items-center gap-2 px-3 py-1.5">{children}</div>;
}

export function ConfigRowDisclosure({ children, contentID, expanded, label, onToggle }: {
  children: ReactNode;
  contentID: string;
  expanded: boolean;
  label: string;
  onToggle: () => void;
}) {
  return (
    <button
      aria-controls={contentID}
      aria-expanded={expanded}
      aria-label={label}
      className="flex min-h-10 min-w-0 flex-1 cursor-pointer items-center gap-2 text-left hover:bg-action-hover focus-visible:outline-2 focus-visible:outline-offset-[-2px]"
      type="button"
      onClick={onToggle}
    >
      {expanded
        ? <KeyboardArrowDownIcon aria-hidden fontSize="small" />
        : <KeyboardArrowRightIcon aria-hidden fontSize="small" />}
      {children}
    </button>
  );
}

export function ConfigListActions({ children }: { children: ReactNode }) {
  return (
    <div
      className="flex flex-wrap justify-end gap-2 border-t border-divider bg-action-hover px-3 py-2"
      data-slot="config-list-actions"
    >
      {children}
    </div>
  );
}

export function RowOrderActions({ deleteLabel, downDisabled, downLabel, mobileMenuLabel, onDelete, onDown, onUp, upDisabled, upLabel }: {
  deleteLabel: string;
  downDisabled: boolean;
  downLabel: string;
  mobileMenuLabel?: string;
  onDelete: () => void;
  onDown: () => void;
  onUp: () => void;
  upDisabled: boolean;
  upLabel: string;
}) {
  const { t } = useI18n();
  return (
    <>
      <div className={`shrink-0 items-center ${mobileMenuLabel ? "hidden sm:flex" : "flex"}`}>
        <Tooltip title={upLabel}><span><IconButton aria-label={upLabel} disabled={upDisabled} size="small" type="button" onClick={onUp}><ArrowUpwardIcon aria-hidden fontSize="small" /></IconButton></span></Tooltip>
        <Tooltip title={downLabel}><span><IconButton aria-label={downLabel} disabled={downDisabled} size="small" type="button" onClick={onDown}><ArrowDownwardIcon aria-hidden fontSize="small" /></IconButton></span></Tooltip>
        <Tooltip title={deleteLabel}><IconButton aria-label={deleteLabel} size="small" type="button" onClick={onDelete}><DeleteOutlinedIcon aria-hidden fontSize="small" /></IconButton></Tooltip>
      </div>
      {mobileMenuLabel ? (
        <div className="shrink-0 sm:hidden">
          <ActionMenu
            actions={[
              { accessibleLabel: upLabel, disabled: upDisabled, icon: <ArrowUpwardIcon aria-hidden className="mr-2" fontSize="small" />, label: t("actions.moveUp"), onSelect: onUp },
              { accessibleLabel: downLabel, disabled: downDisabled, icon: <ArrowDownwardIcon aria-hidden className="mr-2" fontSize="small" />, label: t("actions.moveDown"), onSelect: onDown },
              { accessibleLabel: deleteLabel, icon: <DeleteOutlinedIcon aria-hidden className="mr-2" fontSize="small" />, label: t("actions.delete"), onSelect: onDelete, tone: "danger" },
            ]}
            buttonSize="small"
            label={mobileMenuLabel}
          />
        </div>
      ) : null}
    </>
  );
}

export function replaceAt<T>(values: T[], index: number, value: T): T[] {
  return values.map((entry, currentIndex) => currentIndex === index ? value : entry);
}

export function removeAt<T>(values: T[], index: number): T[] {
  return values.filter((_, currentIndex) => currentIndex !== index);
}

export function useSortableSensors() {
  return useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 6 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
}

function configIssueMessage(issue: ConfigValidationIssue, t: Translator): string {
  if (issue.messageKey) return t(issue.messageKey, issue.messageParams);
  const index = Number.parseInt(issue.itemId.split("-").at(-1) ?? "", 10) + 1;
  const params = {
    index: Number.isFinite(index) ? index : "?",
    reference: issue.reference ?? "",
  };
  switch (issue.code) {
    case "group_name_empty":
      return t("files.config.issueGroupNameEmpty", params);
    case "group_name_duplicate":
      return t("files.config.issueGroupNameDuplicate", params);
    case "group_members_empty":
      return t("files.config.issueGroupMembersEmpty", params);
    case "group_filter_invalid":
      return t("files.config.issueGroupFilterInvalid", params);
    case "group_url_invalid":
      return t("files.config.issueGroupURLInvalid", params);
    case "group_interval_invalid":
      return t("files.config.issueGroupIntervalInvalid", params);
    case "rule_set_name_empty":
      return t("files.config.issueRuleSetNameEmpty", params);
    case "rule_set_name_duplicate":
      return t("files.config.issueRuleSetNameDuplicate", params);
    case "rule_set_url_invalid":
      return t("files.config.issueRuleSetURLInvalid", params);
    case "rule_set_interval_invalid":
      return t("files.config.issueRuleSetIntervalInvalid", params);
    case "rule_set_payload_empty":
      return t("files.config.issueRuleSetPayloadEmpty", params);
    case "rule_set_reference_empty":
      return t("files.config.issueRuleSetEmpty", params);
    case "unknown_rule_set":
      return t("files.config.issueRuleSetMissing", params);
    case "rule_policy_empty":
      return t("files.config.issuePolicyEmpty", params);
    case "rule_value_empty":
      return t("files.config.issueRuleValueEmpty", params);
    case "unknown_rule_policy":
      return t("files.config.issuePolicyMissing", params);
    case "group_reference_cycle":
      return t("files.config.issueGroupCycle", params);
    case "final_rule_not_last":
      return t("files.config.issueFinalRuleNotLast", params);
    default:
      return issue.message;
  }
}
