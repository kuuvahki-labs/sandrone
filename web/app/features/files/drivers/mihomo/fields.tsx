import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";

import { isHTTPURL } from "~/features/files/drivers/core/adapter-helpers";
import type {
  GroupFieldsProps,
  RuleFieldsProps,
  RuleSetFieldsProps,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";
import { SelectField } from "~/shared/ui/form-fields";

function MihomoGroupFields({ draft, healthCheck, index, onUpdate }: GroupFieldsProps) {
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
        type="number"
        value={draft.healthCheckInterval}
        onChange={(event) => onUpdate({ ...draft, healthCheckInterval: event.target.value })}
      />
    </div>
  );
}

function MihomoRuleSetFields({ behaviorOptions, draft, onUpdate }: RuleSetFieldsProps) {
  const { t } = useI18n();
  return (
    <SelectField
      label={t("files.config.behavior")}
      options={[...behaviorOptions]}
      size="small"
      value={draft.behavior}
      onChange={(behavior) => onUpdate({ ...draft, behavior })}
    />
  );
}

function MihomoRuleFields({ draft, onUpdate, supportsNoResolve }: RuleFieldsProps) {
  if (!supportsNoResolve) return null;
  return (
    <FormControlLabel
      className="m-0 w-fit whitespace-nowrap"
      control={<Checkbox checked={Boolean(draft.noResolve)} size="small" onChange={(event) => onUpdate({ ...draft, noResolve: event.target.checked })} />}
      label="no-resolve"
    />
  );
}

export const mihomoConfigurationFields = {
  GroupFields: MihomoGroupFields,
  RuleFields: MihomoRuleFields,
  RuleSetFields: MihomoRuleSetFields,
  ruleSetPresentation: {
    headerLayout: "name-fields-source",
    intervalInputType: "number",
    remoteFields: "format-interval",
    sourceMode: "switchable",
    summaryFields: ["behavior", "format"],
  },
} satisfies StructuredConfigurationFieldSlots;
