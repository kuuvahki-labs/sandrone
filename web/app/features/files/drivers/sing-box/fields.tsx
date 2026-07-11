import TextField from "@mui/material/TextField";

import { isHTTPURL } from "~/features/files/drivers/core/adapter-helpers";
import type {
  GroupFieldsProps,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";

function SingBoxGroupFields({ draft, healthCheck, index, onUpdate }: GroupFieldsProps) {
  const { t } = useI18n();
  if (!healthCheck) return null;
  const invalidURL = Boolean(draft.healthCheckURL) && !isHTTPURL(draft.healthCheckURL);
  return (
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
