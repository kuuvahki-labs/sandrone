import { useEffect, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";

import { settingsUpdateFromView } from "~/features/settings/model/project-settings";
import type { ScheduledRefreshResourceChoice } from "~/features/settings/model/scheduled-refresh-targets";
import { ResourceAutomationSettingsSection } from "~/features/settings/sections/resource-automation-settings-section";
import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import { StartupSettingsSection } from "~/features/settings/sections/startup-settings-section";
import type { ScheduledRefreshStatus, SettingsUpdate, SettingsView } from "~/shared/api/client";
import { useUICapabilities } from "~/shared/capabilities/context";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsServicePageProps {
  defaultUserAgent?: string;
  settings: SettingsView;
  overrides: Record<string, string>;
  restartRequired: string[];
  scheduledRefreshResources: ScheduledRefreshResourceChoice[];
  scheduledRefreshStatus?: ScheduledRefreshStatus;
  onBack: () => void;
  onRunScheduledRefresh: () => Promise<unknown> | unknown;
  onSave: (value: SettingsUpdate) => Promise<unknown> | unknown;
}

export function SettingsServicePage({
  defaultUserAgent,
  settings,
  overrides,
  restartRequired,
  scheduledRefreshResources,
  scheduledRefreshStatus,
  onBack,
  onRunScheduledRefresh,
  onSave,
}: SettingsServicePageProps) {
  const { t } = useI18n();
  const { hasFeature } = useUICapabilities();
  const [draft, setDraft] = useState(settings);
  const [runScheduledRefreshAfterSave, setRunScheduledRefreshAfterSave] = useState(false);

  useEffect(() => {
    setDraft(settings);
  }, [settings]);

  const save = async () => {
    try {
      await onSave(settingsUpdateFromView(draft));
      if (runScheduledRefreshAfterSave) {
        await onRunScheduledRefresh();
        setRunScheduledRefreshAfterSave(false);
      }
    } catch {
      // The settings provider and action handler surface their own notices.
    }
  };

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        primaryAction={{
          accessibleLabel: t("settings.save"),
          icon: <SaveIcon aria-hidden fontSize="small" />,
          label: t("actions.save"),
          onSelect: () => void save(),
          variant: "contained",
        }}
        sticky
        title={t("settings.service.title")}
      />
      {restartRequired.length > 0 ? (
        <Alert severity="warning">{t("settings.restartRequired", { fields: restartRequired.join(", ") })}</Alert>
      ) : null}
      <RuntimeSettingsSection
        defaultUserAgent={defaultUserAgent}
        value={draft}
        onChange={(runtime) => setDraft((current) => ({ ...current, ...runtime }))}
      />
      <ResourceAutomationSettingsSection
        scheduledRefreshEnabled={hasFeature("scheduler.enabled")}
        scheduledRefreshResources={scheduledRefreshResources}
        scheduledRefreshStatus={scheduledRefreshStatus}
        scheduledRefreshValue={draft.scheduled_refresh}
        subscriptionTrafficValue={draft.subscriptions.auto_load_traffic}
        runScheduledRefreshAfterSave={runScheduledRefreshAfterSave}
        onRunScheduledRefreshAfterSaveChange={setRunScheduledRefreshAfterSave}
        onScheduledRefreshChange={(scheduledRefresh) => {
          if (!scheduledRefresh.enabled || scheduledRefresh.targets.length === 0) {
            setRunScheduledRefreshAfterSave(false);
          }
          setDraft((current) => ({ ...current, scheduled_refresh: scheduledRefresh }));
        }}
        onSubscriptionTrafficChange={(enabled) => setDraft((current) => ({
          ...current,
          subscriptions: { auto_load_traffic: enabled },
        }))}
      />
      <StartupSettingsSection
        overrides={overrides}
        value={draft}
        onChange={setDraft}
      />
    </section>
  );
}
