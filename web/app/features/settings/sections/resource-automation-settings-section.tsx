import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Checkbox from "@mui/material/Checkbox";
import Collapse from "@mui/material/Collapse";
import Divider from "@mui/material/Divider";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { ScheduledRefreshResourceChoice } from "~/features/settings/model/scheduled-refresh-targets";
import type { ScheduledRefreshStatus, SettingsView } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

import { ScheduledRefreshTargetPicker } from "./scheduled-refresh-target-picker";

interface ResourceAutomationSettingsSectionProps {
  scheduledRefreshEnabled: boolean;
  scheduledRefreshResources: ScheduledRefreshResourceChoice[];
  scheduledRefreshStatus?: ScheduledRefreshStatus;
  scheduledRefreshValue: SettingsView["scheduled_refresh"];
  subscriptionTrafficValue: boolean;
  runScheduledRefreshAfterSave: boolean;
  onRunScheduledRefreshAfterSaveChange: (enabled: boolean) => void;
  onScheduledRefreshChange: (value: SettingsView["scheduled_refresh"]) => void;
  onSubscriptionTrafficChange: (enabled: boolean) => void;
}

export function ResourceAutomationSettingsSection({
  scheduledRefreshEnabled,
  scheduledRefreshResources,
  scheduledRefreshStatus,
  scheduledRefreshValue,
  subscriptionTrafficValue,
  runScheduledRefreshAfterSave,
  onRunScheduledRefreshAfterSaveChange,
  onScheduledRefreshChange,
  onSubscriptionTrafficChange,
}: ResourceAutomationSettingsSectionProps) {
  const { t } = useI18n();

  return (
    <Card component="article" variant="outlined">
      <CardContent className="grid gap-5">
        <Typography component="h3" variant="h6">{t("settings.automation.title")}</Typography>
        <section aria-labelledby="subscription-traffic-heading" className="grid gap-2">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <Typography component="h4" id="subscription-traffic-heading" variant="subtitle1">
              {t("settings.subscriptionTraffic.title")}
            </Typography>
            <FormControlLabel
              className="m-0 shrink-0"
              control={(
                <Switch
                  checked={subscriptionTrafficValue}
                  onChange={(event) => onSubscriptionTrafficChange(event.target.checked)}
                />
              )}
              label={t("settings.subscriptionTraffic.autoLoad.label")}
            />
          </div>
        </section>
        {scheduledRefreshEnabled ? (
          <>
            <Divider />
            <section aria-labelledby="scheduled-refresh-heading" className="grid gap-4">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                <Typography component="h4" id="scheduled-refresh-heading" variant="subtitle1">
                  {t("settings.scheduledRefresh.title")}
                </Typography>
                <FormControlLabel
                  className="m-0 shrink-0"
                  control={(
                    <Switch
                      checked={scheduledRefreshValue.enabled}
                      onChange={(event) => onScheduledRefreshChange({
                        ...scheduledRefreshValue,
                        enabled: event.target.checked,
                      })}
                    />
                  )}
                  label={t("settings.scheduledRefresh.enabled")}
                />
              </div>
              <Collapse in={scheduledRefreshValue.enabled} unmountOnExit>
                <div className="grid gap-4 pt-1">
                  <TextField
                    fullWidth
                    helperText={t("settings.scheduledRefresh.timezoneHint")}
                    label={t("settings.scheduledRefresh.schedule")}
                    placeholder="@every 10m"
                    value={scheduledRefreshValue.schedule}
                    onChange={(event) => onScheduledRefreshChange({
                      ...scheduledRefreshValue,
                      schedule: event.target.value,
                    })}
                  />
                  <ScheduledRefreshTargetPicker
                    resources={scheduledRefreshResources}
                    targets={scheduledRefreshValue.targets}
                    onChange={(targets) => onScheduledRefreshChange({
                      ...scheduledRefreshValue,
                      targets,
                    })}
                  />
                  <FormControlLabel
                    className="m-0"
                    control={(
                      <Checkbox
                        checked={runScheduledRefreshAfterSave}
                        disabled={scheduledRefreshValue.targets.length === 0 || scheduledRefreshStatus?.running === true}
                        onChange={(event) => onRunScheduledRefreshAfterSaveChange(event.target.checked)}
                      />
                    )}
                    label={t("settings.scheduledRefresh.runAfterSave")}
                  />
                  <ScheduledRefreshStatusView status={scheduledRefreshStatus} />
                </div>
              </Collapse>
            </section>
          </>
        ) : null}
      </CardContent>
    </Card>
  );
}

function ScheduledRefreshStatusView({ status }: { status?: ScheduledRefreshStatus }) {
  const { t } = useI18n();
  if (!status) {
    return <Typography color="text.secondary" variant="body2">{t("settings.scheduledRefresh.statusUnavailable")}</Typography>;
  }
  return (
    <div className="grid gap-1 rounded-md border border-solid border-divider p-3 text-sm sm:grid-cols-2">
      <Typography variant="body2">{t("settings.scheduledRefresh.status")}: {status.running ? t("settings.scheduledRefresh.running") : t("settings.scheduledRefresh.idle")}</Typography>
      <Typography variant="body2">{t("settings.scheduledRefresh.nextRun")}: {formatDate(status.next_run_at)}</Typography>
      <Typography variant="body2">{t("settings.scheduledRefresh.lastRun")}: {formatDate(status.last_completed_at)}</Typography>
      <Typography variant="body2">{t("settings.scheduledRefresh.lastCounts", { success: status.last_success_count, failure: status.last_failure_count, skipped: status.skipped_count })}</Typography>
    </div>
  );
}

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleString() : "-";
}
