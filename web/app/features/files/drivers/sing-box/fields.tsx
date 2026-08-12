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
