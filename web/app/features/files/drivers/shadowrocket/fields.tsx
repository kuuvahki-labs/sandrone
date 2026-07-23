import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";

import type {
  GroupFieldsProps,
  RuleFieldsProps,
  RuleSetFieldsProps,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";
import { SelectField } from "~/shared/ui/form-fields";

function ShadowrocketGroupFields({ draft, healthCheck, index, onUpdate }: GroupFieldsProps) {
  const { t } = useI18n();
  return (
    <>
      {healthCheck ? (
        <div className="grid gap-3 sm:grid-cols-3">
          <TextField
            fullWidth
            label={t("files.config.groupIntervalWithIndex", { index: index + 1 })}
            size="small"
            slotProps={{ htmlInput: { min: 1, step: 1 } }}
            type="number"
            value={draft.healthCheckInterval}
            onChange={(event) => onUpdate({ ...draft, healthCheckInterval: event.target.value })}
          />
          <TextField
            fullWidth
            label={t("files.config.groupTimeout")}
            size="small"
            slotProps={{ htmlInput: { min: 1, step: 1 } }}
            type="number"
            value={optionalValue(draft.healthCheckTimeout)}
            onChange={(event) => onUpdate({
              ...draft,
              healthCheckTimeout: optionalNumber(event.target.value),
            })}
          />
          {draft.type === "url-test" || draft.type === "load-balance" ? (
            <TextField
              fullWidth
              label={t("files.config.groupTolerance")}
              size="small"
              slotProps={{ htmlInput: { min: 0, step: 1 } }}
              type="number"
              value={optionalValue(draft.healthCheckTolerance)}
              onChange={(event) => onUpdate({
                ...draft,
                healthCheckTolerance: optionalNumber(event.target.value),
              })}
            />
          ) : null}
        </div>
      ) : null}
    </>
  );
}

function ShadowrocketRuleSetFields({ behaviorOptions, draft, onUpdate }: RuleSetFieldsProps) {
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

function ShadowrocketRuleFields({ draft, onUpdate, supportsNoResolve }: RuleFieldsProps) {
  if (!supportsNoResolve) return null;
  return (
    <FormControlLabel
      className="m-0 w-fit whitespace-nowrap"
      control={<Checkbox checked={Boolean(draft.noResolve)} size="small" onChange={(event) => onUpdate({ ...draft, noResolve: event.target.checked })} />}
      label="no-resolve"
    />
  );
}

function optionalNumber(value: string): number | undefined {
  return value ? Number(value) : undefined;
}

function optionalValue(value: number | undefined): string {
  return value === undefined ? "" : String(value);
}

export const shadowrocketConfigurationFields = {
  GroupFields: ShadowrocketGroupFields,
  RuleFields: ShadowrocketRuleFields,
  RuleSetFields: ShadowrocketRuleSetFields,
  ruleSetPresentation: {
    headerLayout: "name-fields",
    intervalInputType: "text",
    remoteFields: "url-only",
    sourceMode: "remote-only",
    summaryFields: ["behavior"],
  },
} satisfies StructuredConfigurationFieldSlots;
