import Alert from "@mui/material/Alert";
import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";

import { isHTTPURL } from "~/features/files/drivers/core/adapter-helpers";
import type {
  GroupFieldsProps,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";

function SingBoxGroupFields({ draft, healthCheck, index, onUpdate }: GroupFieldsProps) {
  const { t } = useI18n();
  const invalidURL = Boolean(draft.healthCheckURL) && !isHTTPURL(draft.healthCheckURL);
  return (
    <>
      {healthCheck ? (
        <>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_11rem]">
            <TextField
              error={invalidURL}
              fullWidth
              required
              helperText={invalidURL ? t("files.config.invalidHttpUrl") : undefined}
              label={t("files.config.groupUrlWithIndex", { index: index + 1 })}
              size="small"
              slotProps={{ htmlInput: { pattern: "https?://.+" } }}
              type="url"
              value={draft.healthCheckURL}
              onChange={(event) => onUpdate({ ...draft, healthCheckURL: event.target.value })}
            />
            <TextField
              fullWidth
              required
              label={t("files.config.groupIntervalWithIndex", { index: index + 1 })}
              size="small"
              type="text"
              value={draft.healthCheckInterval}
              onChange={(event) => onUpdate({ ...draft, healthCheckInterval: event.target.value })}
            />
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <TextField
              fullWidth
              label={t("files.config.groupTolerance")}
              size="small"
              slotProps={{ htmlInput: { max: 65535, min: 0, step: 1 } }}
              type="number"
              value={optionalNumberValue(draft.healthCheckTolerance)}
              onChange={(event) => onUpdate({
                ...draft,
                healthCheckTolerance: optionalNumber(event.target.value),
              })}
            />
            <TextField
              fullWidth
              helperText={t("files.config.groupIdleTimeoutHint")}
              label={t("files.config.groupIdleTimeout")}
              size="small"
              type="text"
              value={draft.healthCheckIdleTimeout ?? ""}
              onChange={(event) => onUpdate({
                ...draft,
                healthCheckIdleTimeout: event.target.value || undefined,
              })}
            />
          </div>
        </>
      ) : null}
      <FormControlLabel
        className="m-0 w-fit"
        control={(
          <Checkbox
            checked={draft.interruptExistingConnections === true}
            size="small"
            onChange={(event) => onUpdate({ ...draft, interruptExistingConnections: event.target.checked })}
          />
        )}
        label={t("files.config.interruptExistingConnections")}
      />
      {draft.type === "url-test" && draft.interruptExistingConnections === true ? (
        <Alert severity="warning">{t("files.config.interruptExistingConnectionsWarning")}</Alert>
      ) : null}
    </>
  );
}

function optionalNumber(value: string): number | undefined {
  return value === "" ? undefined : Number(value);
}

function optionalNumberValue(value: number | undefined): string {
  return value === undefined ? "" : String(value);
}

function SingBoxRuleSetFields() {
  return null;
}

function SingBoxRuleFields() {
  return null;
}

export const singBoxConfigurationFields = {
  GroupFields: SingBoxGroupFields,
  RuleFields: SingBoxRuleFields,
  RuleSetFields: SingBoxRuleSetFields,
  ruleSetPresentation: {
    headerLayout: "name-source",
    intervalInputType: "text",
    remoteFields: "format-interval",
    sourceMode: "switchable",
    summaryFields: ["format"],
  },
} satisfies StructuredConfigurationFieldSlots;
