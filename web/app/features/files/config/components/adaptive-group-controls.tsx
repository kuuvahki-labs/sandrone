import { useEffect, useState } from "react";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Collapse from "@mui/material/Collapse";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type {
  AdaptiveGroupCandidate,
  AdaptiveGroupOptions,
  AdaptiveGroupType,
  AdaptiveGroupWarning,
} from "~/features/files/config/model/adaptive-groups";
import { ADAPTIVE_REGION_IDS, DEFAULT_ADAPTIVE_REGION_IDS } from "~/features/files/config/model/adaptive-groups";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { SelectField } from "~/shared/ui/form-fields";

import { WorkbenchGroupSection } from "./editor-shared";

export interface ConfigAdaptiveGroupControlsProps {
  candidates: readonly AdaptiveGroupCandidate[];
  disabledReason?: string;
  generatedCount: number;
  onOptionsChange: (options: AdaptiveGroupOptions) => void;
  onGenerate: (options: AdaptiveGroupOptions) => void;
  options: AdaptiveGroupOptions;
  typeOptions: readonly { label: string; value: string }[];
  warnings: readonly AdaptiveGroupWarning[];
}

export function ConfigAdaptiveGroupControls({
  candidates,
  disabledReason,
  generatedCount,
  onOptionsChange,
  onGenerate,
  options,
  typeOptions,
  warnings,
}: ConfigAdaptiveGroupControlsProps) {
  const { t } = useI18n();
  const [minimumText, setMinimumText] = useState(String(options.minimumNodeCount));
  const [scopeExpanded, setScopeExpanded] = useState(false);
  const minimumNodeCount = Number(minimumText);
  const minimumValid = Number.isInteger(minimumNodeCount) && minimumNodeCount >= 1;
  const reason = disabledReason ?? (!minimumValid ? t("files.config.adaptiveMinimumInvalid") : undefined);
  const enabledRegionIds = options.enabledRegionIds ?? DEFAULT_ADAPTIVE_REGION_IDS;
  const enabledRegionSet = new Set(enabledRegionIds);
  const warningMessages = warnings.flatMap((warning) => {
    const message = adaptiveWarningMessage(warning, t);
    return message ? [message] : [];
  });

  useEffect(() => setMinimumText(String(options.minimumNodeCount)), [options.minimumNodeCount]);

  function changeOptions(update: Partial<AdaptiveGroupOptions>) {
    onOptionsChange({
      ...options,
      enabledRegionIds: [...enabledRegionIds],
      ...update,
    });
  }

  const generateButton = (
    <Button
      aria-label={t(generatedCount > 0 ? "files.config.adaptiveRegenerate" : "files.config.adaptiveGenerate")}
      disabled={Boolean(reason)}
      type="button"
      variant="contained"
      onClick={() => {
        if (reason) return;
        onGenerate({
          ...options,
          enabledRegionIds: [...enabledRegionIds],
          minimumNodeCount,
        });
      }}
    >
      {t(generatedCount > 0 ? "actions.regenerate" : "actions.generate")}
    </Button>
  );

  return (
    <WorkbenchGroupSection
      collapsible={false}
      headerActions={generateButton}
      id="config-adaptive-groups"
      label={t("files.config.adaptiveGroups")}
    >
      <div className="grid gap-3" onChange={(event) => event.stopPropagation()}>
        <Typography color="text.secondary" variant="body2">
          {t("files.config.adaptiveDescription")}
        </Typography>
        <div className="grid gap-3 sm:grid-cols-2">
          <SelectField
            label={t("files.config.adaptiveGroupType")}
            options={[...typeOptions]}
            size="small"
            value={options.type}
            onChange={(value) => changeOptions({ type: value as AdaptiveGroupType })}
          />
          <TextField
            error={!minimumValid}
            fullWidth
            label={t("files.config.adaptiveMinimumNodeCount")}
            size="small"
            slotProps={{ htmlInput: { min: 1, step: 1 } }}
            type="number"
            value={minimumText}
            onChange={(event) => {
              const value = event.target.value;
              setMinimumText(value);
              const nextMinimum = Number(value);
              if (Number.isInteger(nextMinimum) && nextMinimum >= 1) {
                changeOptions({ minimumNodeCount: nextMinimum });
              }
            }}
          />
        </div>
        <div className="grid rounded-md border border-divider">
          <Button
            aria-controls="adaptive-group-scope"
            aria-expanded={scopeExpanded}
            className="justify-between rounded-md px-3 py-2 text-left normal-case"
            endIcon={scopeExpanded ? <ExpandLessIcon aria-hidden /> : <ExpandMoreIcon aria-hidden />}
            type="button"
            onClick={() => setScopeExpanded((current) => !current)}
          >
            <span className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-2">
              <Typography className="font-semibold" component="span" variant="body2">
                {t("files.config.adaptiveGroupsToGenerate")}
              </Typography>
              <Typography color="text.secondary" component="span" variant="caption">
                {t("files.config.adaptiveRegionsSelected", { count: enabledRegionSet.size })}
              </Typography>
            </span>
          </Button>
          <Collapse id="adaptive-group-scope" in={scopeExpanded} timeout="auto" unmountOnExit>
            <div className="grid gap-2 border-t border-divider px-3 py-2">
              <div className="flex flex-wrap justify-end gap-1">
              <Button
                size="small"
                type="button"
                onClick={() => changeOptions({ enabledRegionIds: [...ADAPTIVE_REGION_IDS] })}
              >
                {t("actions.selectAll")}
              </Button>
              <Button size="small" type="button" onClick={() => changeOptions({ enabledRegionIds: [] })}>
                {t("actions.clear")}
              </Button>
              </div>
              <div className="grid max-h-[min(40vh,20rem)] grid-cols-1 gap-x-3 overflow-y-auto rounded-md border border-divider px-2 py-1 sm:grid-cols-2 lg:grid-cols-3">
                {candidates.map((candidate) => (
                  <FormControlLabel
                    control={(
                      <Checkbox
                        checked={enabledRegionSet.has(candidate.id)}
                        onChange={(_, checked) => {
                          const next = new Set(enabledRegionSet);
                          if (checked) next.add(candidate.id);
                          else next.delete(candidate.id);
                          changeOptions({ enabledRegionIds: ADAPTIVE_REGION_IDS.filter((id) => next.has(id)) });
                        }}
                      />
                    )}
                    key={candidate.id}
                    label={<CandidateLabel candidate={candidate} />}
                  />
                ))}
              </div>
            </div>
          </Collapse>
        </div>
        {reason ? <Typography color="warning.main" variant="body2">{reason}</Typography> : null}
        {warningMessages.length ? (
          <Alert severity="warning" variant="outlined">
            <ul className="grid list-disc gap-1 pl-4">
              {warningMessages.map((message, index) => <li key={`${message}:${index}`}>{message}</li>)}
            </ul>
          </Alert>
        ) : null}
      </div>
    </WorkbenchGroupSection>
  );
}

function CandidateLabel({ candidate, fallback = "" }: { candidate?: AdaptiveGroupCandidate; fallback?: string }) {
  const { t } = useI18n();
  const name = candidate?.name ?? fallback;
  const count = candidate?.matchedNodeCount ?? 0;
  const active = candidate?.active ?? false;
  return (
    <span className="flex min-w-0 items-baseline gap-1">
      <Typography component="span" variant="body2">{name}</Typography>
      <Typography color={active ? "text.secondary" : "text.disabled"} component="span" variant="caption">
        {t("files.config.adaptiveMatchedNodes", { count })}
      </Typography>
    </span>
  );
}

function adaptiveWarningMessage(warning: AdaptiveGroupWarning, t: Translator): string | null {
  switch (warning.code) {
    case "group_name_conflict":
      return t("files.config.adaptiveNameConflict", { name: warning.groupName });
    case "node_name_conflict":
      return t("files.config.adaptiveNodeNameConflict", { name: warning.groupName });
    case "referenced_stale_group":
      return t("files.config.adaptiveReferencedPreserved", { name: warning.groupName });
    default:
      return null;
  }
}
