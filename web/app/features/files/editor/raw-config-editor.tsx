import { type ReactNode, useEffect, useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import MenuItem from "@mui/material/MenuItem";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { WorkbenchGroupSection } from "~/features/files/config/components/editor-shared";
import type { FileConfigDetail } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import { HighlightedTextarea } from "~/shared/ui/code-editor";

export function RawFileConfigEditor({
  baseEditor,
  configDefault,
  onValidityChange,
  subscriptions,
}: {
  baseEditor: ReactNode;
  configDefault?: FileConfigDetail;
  onValidityChange?: (valid: boolean) => void;
  subscriptions: ResourceOption[];
}) {
  const { t } = useI18n();
  const originalSubscriptions = configDefault?.subscriptions ?? [];
  const multipleSubscriptions = originalSubscriptions.length > 1;
  const [selected, setSelected] = useState(originalSubscriptions.length === 1 ? originalSubscriptions[0] : "");
  const [settingsText, setSettingsText] = useState(() => JSON.stringify(
    configDefault?.settingsPresent ? configDefault.settings : {},
    null,
    2,
  ));
  const [settingsTouched, setSettingsTouched] = useState(false);
  const settings = useMemo(() => parseJSONObject(settingsText), [settingsText]);
  const selectedSubscriptions = multipleSubscriptions ? originalSubscriptions : selected ? [selected] : [];
  const serialized = JSON.stringify({
    ...(selectedSubscriptions.length ? { subscriptions: selectedSubscriptions } : {}),
    ...(configDefault?.settingsPresent || settingsTouched ? {
      settings: settings.value ?? (configDefault?.settingsPresent ? configDefault.settings : {}),
    } : {}),
  });
  const valid = !multipleSubscriptions && settings.value !== undefined;

  useEffect(() => onValidityChange?.(valid), [onValidityChange, valid]);

  return (
    <div className="grid gap-3">
      <input name="config" type="hidden" value={serialized} />
      <Typography className="font-semibold" component="h2" variant="h6">
        {t("files.config.content")}
      </Typography>
      <WorkbenchGroupSection collapsible={false} id="file-config-base" label={t("files.config.baseContent")}>
        {baseEditor}
      </WorkbenchGroupSection>
      <WorkbenchGroupSection collapsible={false} id="file-config-raw-settings" label={t("files.form.config")}>
        <TextField
          disabled={multipleSubscriptions}
          fullWidth
          label={t("files.config.subscription")}
          select
          value={selected}
          onChange={(event) => setSelected(event.target.value)}
        >
          <MenuItem value="">—</MenuItem>
          {subscriptions.map((subscription) => <MenuItem key={subscription.name} value={subscription.name}>{subscription.name}</MenuItem>)}
        </TextField>
        {multipleSubscriptions ? <Alert severity="error">{t("files.config.multipleSubscriptions", { names: originalSubscriptions.join(", ") })}</Alert> : null}
        <HighlightedTextarea
          label={t("files.config.rawSettings")}
          language="json"
          minRows={12}
          showLineNumbers
          value={settingsText}
          onChange={(event) => {
            setSettingsTouched(true);
            setSettingsText(event.target.value);
          }}
        />
        {settings.error ? <Alert severity="error">{t("files.config.rawSettingsInvalid")}</Alert> : null}
      </WorkbenchGroupSection>
    </div>
  );
}

function parseJSONObject(text: string): { error?: string; value?: Record<string, unknown> } {
  try {
    const value = JSON.parse(text) as unknown;
    return typeof value === "object" && value !== null && !Array.isArray(value)
      ? { value: value as Record<string, unknown> }
      : { error: "not-object" };
  } catch {
    return { error: "invalid-json" };
  }
}
