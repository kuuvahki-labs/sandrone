import { useEffect, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Paper from "@mui/material/Paper";

import { settingsUpdateFromView } from "~/features/settings/model/project-settings";
import type { ScheduledRefreshResourceChoice } from "~/features/settings/model/scheduled-refresh-targets";
import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import { ScheduledRefreshSettingsSection } from "~/features/settings/sections/scheduled-refresh-settings-section";
import { StartupSettingsSection } from "~/features/settings/sections/startup-settings-section";
import { SubscriptionTrafficSettingsSection } from "~/features/settings/sections/subscription-traffic-settings-section";
import type { ScheduledRefreshStatus, SettingsUpdate, SettingsView } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsRuntimePageProps {
  settings: SettingsView;
  overrides: Record<string, string>;
  restartRequired: string[];
  scheduledRefreshResources: ScheduledRefreshResourceChoice[];
  scheduledRefreshStatus?: ScheduledRefreshStatus;
  onBack: () => void;
  onSave: (value: SettingsUpdate) => Promise<unknown> | unknown;
}

export function SettingsRuntimePage({
  settings,
  overrides,
  restartRequired,
  scheduledRefreshResources,
  scheduledRefreshStatus,
  onBack,
  onSave,
}: SettingsRuntimePageProps) {
  const { t } = useI18n();
  const [draft, setDraft] = useState(settings);

  useEffect(() => {
    setDraft(settings);
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
        value={draft}
        onChange={setDraft}
      />
      <SubscriptionTrafficSettingsSection
        value={draft.subscriptions.auto_load_traffic}
        onChange={(enabled) => setDraft((current) => ({
          ...current,
          subscriptions: { auto_load_traffic: enabled },
        }))}
      />
      <ScheduledRefreshSettingsSection
        resources={scheduledRefreshResources}
        status={scheduledRefreshStatus}
        value={draft.scheduled_refresh}
        onChange={(scheduledRefresh) => setDraft((current) => ({ ...current, scheduled_refresh: scheduledRefresh }))}
      />
      <RuntimeSettingsSection
        value={draft}
        onChange={(runtime) => setDraft((current) => ({ ...current, ...runtime }))}
      />
      <Paper className="sticky bottom-14 z-10 flex justify-end p-3 min-[820px]:bottom-0" component="footer" elevation={4}>
        <Button
          aria-label={t("settings.save")}
          startIcon={<SaveIcon aria-hidden />}
          type="button"
          variant="contained"
          onClick={() => void onSave(settingsUpdateFromView(draft))}
        >
          {t("actions.save")}
        </Button>
      </Paper>
    </section>
  );
}
