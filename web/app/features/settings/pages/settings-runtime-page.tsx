import { useEffect, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";

import { settingsUpdateFromView } from "~/features/settings/model/project-settings";
import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import { StartupSettingsSection } from "~/features/settings/sections/startup-settings-section";
import { SubscriptionTrafficSettingsSection } from "~/features/settings/sections/subscription-traffic-settings-section";
import type { SettingsUpdate, SettingsView } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsRuntimePageProps {
  settings: SettingsView;
  overrides: Record<string, string>;
  restartRequired: string[];
  onBack: () => void;
  onSave: (value: SettingsUpdate) => Promise<unknown> | unknown;
}

export function SettingsRuntimePage({
  settings,
  overrides,
  restartRequired,
  onBack,
  onSave,
}: SettingsRuntimePageProps) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(settings);
  const [token, setToken] = useState<string | undefined>();

  useEffect(() => {
    setDraft(settings);
    setToken(undefined);
  }, [settings]);

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        title={t("settings.advanced.title")}
      />
      {restartRequired.length > 0 ? (
        <Alert severity="warning">{t("settings.restartRequired", { fields: restartRequired.join(", ") })}</Alert>
      ) : null}
      <StartupSettingsSection
        overrides={overrides}
        token={token}
        value={draft}
        onChange={setDraft}
        onTokenChange={setToken}
      />
      <SubscriptionTrafficSettingsSection
        value={draft.subscriptions.auto_load_traffic}
        onChange={(enabled) => setDraft((current) => ({
          ...current,
          subscriptions: { auto_load_traffic: enabled },
        }))}
      />
      <RuntimeSettingsSection
        value={draft}
        onChange={(runtime) => setDraft((current) => ({ ...current, ...runtime }))}
      />
      <Button
        aria-label={t("settings.save")}
        className="justify-self-end"
        startIcon={<SaveIcon aria-hidden />}
        type="button"
        variant="contained"
        onClick={() => void onSave(settingsUpdateFromView(draft, token))}
      >
        {t("actions.save")}
      </Button>
    </section>
  );
}
