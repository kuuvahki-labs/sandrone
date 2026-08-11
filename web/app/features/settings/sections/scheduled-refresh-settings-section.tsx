import Autocomplete from "@mui/material/Autocomplete";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Chip from "@mui/material/Chip";
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

interface ScheduledRefreshSettingsSectionProps {
  resources: ScheduledRefreshResourceChoice[];
  status?: ScheduledRefreshStatus;
  value: SettingsView["scheduled_refresh"];
  onChange: (value: SettingsView["scheduled_refresh"]) => void;
}

export function ScheduledRefreshSettingsSection({
  resources,
  status,
  value,
  onChange,
}: ScheduledRefreshSettingsSectionProps) {
  const { t } = useI18n();
  const options = scheduledRefreshTargetOptions(value.targets, resources);
  const selectedKeys = new Set(value.targets.map(targetKey));
  const selected = options.filter((option) => selectedKeys.has(targetKey(option)));

  return (
    <Card component="article" variant="outlined">
      <CardContent className="grid gap-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <Typography component="h3" variant="h6">{t("settings.scheduledRefresh.title")}</Typography>
            <Typography color="text.secondary" variant="body2">{t("settings.scheduledRefresh.description")}</Typography>
          </div>
          <FormControlLabel
            className="m-0"
            control={<Switch checked={value.enabled} onChange={(event) => onChange({ ...value, enabled: event.target.checked })} />}
            label={t("settings.scheduledRefresh.enabled")}
          />
        </div>
        <TextField
          fullWidth
          helperText={t("settings.scheduledRefresh.timezoneHint")}
          label={t("settings.scheduledRefresh.schedule")}
          placeholder="@every 10m"
          value={value.schedule}
          onChange={(event) => onChange({ ...value, schedule: event.target.value })}
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
          onChange={(_event, next) => onChange({
            ...value,
            targets: next.map(({ kind, name }) => ({ kind, name })),
          })}
        />
        <ScheduledRefreshStatusView status={status} />
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
