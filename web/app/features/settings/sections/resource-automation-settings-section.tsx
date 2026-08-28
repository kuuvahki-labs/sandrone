import Autocomplete from "@mui/material/Autocomplete";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Chip from "@mui/material/Chip";
import Collapse from "@mui/material/Collapse";
import Divider from "@mui/material/Divider";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import {
  type ScheduledRefreshResourceChoice,
  type ScheduledRefreshTargetOption,
  scheduledRefreshTargetOptions,
  targetKey,
} from "~/features/settings/model/scheduled-refresh-targets";
import type { ScheduledRefreshStatus, SettingsView } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

interface ResourceAutomationSettingsSectionProps {
  scheduledRefreshEnabled: boolean;
  scheduledRefreshResources: ScheduledRefreshResourceChoice[];
  scheduledRefreshStatus?: ScheduledRefreshStatus;
  scheduledRefreshValue: SettingsView["scheduled_refresh"];
  subscriptionTrafficValue: boolean;
  onScheduledRefreshChange: (value: SettingsView["scheduled_refresh"]) => void;
  onSubscriptionTrafficChange: (enabled: boolean) => void;
}

export function ResourceAutomationSettingsSection({
  scheduledRefreshEnabled,
  scheduledRefreshResources,
  scheduledRefreshStatus,
  scheduledRefreshValue,
  subscriptionTrafficValue,
  onScheduledRefreshChange,
  onSubscriptionTrafficChange,
}: ResourceAutomationSettingsSectionProps) {
  const { t } = useI18n();
  const options = scheduledRefreshTargetOptions(scheduledRefreshValue.targets, scheduledRefreshResources);
  const selectedKeys = new Set(scheduledRefreshValue.targets.map(targetKey));
  const selected = options.filter((option) => selectedKeys.has(targetKey(option)));

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
                  <Autocomplete<ScheduledRefreshTargetOption, true, false, false>
                    disableCloseOnSelect
                    filterSelectedOptions
                    multiple
                    options={options}
                    value={selected}
                    getOptionLabel={(option) => optionLabel(option, t("settings.scheduledRefresh.missing"))}
                    groupBy={(option) => option.kind === "subscription"
                      ? t("settings.scheduledRefresh.groupSubscriptions")
                      : t("settings.scheduledRefresh.groupFiles")}
                    isOptionEqualToValue={(option, current) => targetKey(option) === targetKey(current)}
                    renderInput={(params) => <TextField {...params} label={t("settings.scheduledRefresh.targets")} />}
                    renderValue={(selectedOptions, getItemProps) => selectedOptions.map((option, index) => {
                      const { key, ...itemProps } = getItemProps({ index });
                      return (
                        <Chip
                          {...itemProps}
                          color={option.missing ? "warning" : "default"}
                          key={key}
                          label={optionLabel(option, t("settings.scheduledRefresh.missing"))}
                          size="small"
                        />
                      );
                    })}
                    onChange={(_event, next) => onScheduledRefreshChange({
                      ...scheduledRefreshValue,
                      targets: next.map(({ kind, name }) => ({ kind, name })),
                    })}
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

function optionLabel(option: ScheduledRefreshTargetOption, missingLabel: string): string {
  return option.missing ? `${option.label} (${missingLabel})` : option.label;
}

function formatDate(value?: string): string {
  return value ? new Date(value).toLocaleString() : "-";
}
