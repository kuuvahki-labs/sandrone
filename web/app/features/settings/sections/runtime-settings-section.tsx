import { type ReactNode, useId } from "react";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { SettingsView } from "~/shared/api/client";
import { useUICapabilities } from "~/shared/capabilities/context";
import { useI18n } from "~/shared/i18n/context";
import { ProbeURLField } from "~/shared/ui/probe-url-field";

interface RuntimeSettingsSectionProps {
  defaultUserAgent?: string;
  value: Pick<SettingsView, "remote_defaults" | "probe_defaults" | "cache_defaults">;
  onChange: (value: Pick<SettingsView, "remote_defaults" | "probe_defaults" | "cache_defaults">) => void;
}

export function RuntimeSettingsSection({
  defaultUserAgent,
  value,
  onChange,
}: RuntimeSettingsSectionProps) {
  const { t } = useI18n();
  const { hasFeature } = useUICapabilities();
  const probeMethodLabelId = useId();

  function updateRemoteDefaults(patch: Partial<SettingsView["remote_defaults"]>) {
    onChange({
      ...value,
      remote_defaults: { ...value.remote_defaults, ...patch },
    });
  }

  function updateProbeDefaults(patch: Partial<SettingsView["probe_defaults"]>) {
    onChange({
      ...value,
      probe_defaults: { ...value.probe_defaults, ...patch },
    });
  }

  function updateCacheDefaults(patch: Partial<SettingsView["cache_defaults"]>) {
    onChange({
      ...value,
      cache_defaults: { ...value.cache_defaults, ...patch },
    });
  }

  return (
    <section aria-labelledby="runtime-defaults-heading" className="grid gap-3">
      <div className="grid gap-1">
        <Typography component="h3" id="runtime-defaults-heading" variant="h6">
          {t("settings.runtime.title")}
        </Typography>
        <Typography color="text.secondary" variant="body2">
          {t("settings.runtime.description")}
        </Typography>
      </div>
      <div className="grid gap-3">
        <RuntimeSettingsGroup
          id="runtime-remote-defaults"
          layoutClassName="md:grid-cols-2 lg:grid-cols-4"
          title={t("settings.runtime.group.remote")}
        >
          <TextField
            fullWidth
            label={t("settings.runtime.remoteUserAgent")}
            placeholder={defaultUserAgent}
            value={value.remote_defaults.user_agent ?? ""}
            onChange={(event) => updateRemoteDefaults({ user_agent: event.target.value })}
          />
          <TextField
            fullWidth
            label={t("settings.runtime.remoteProxy")}
            placeholder="http://127.0.0.1:7890"
            value={value.remote_defaults.proxy ?? ""}
            onChange={(event) => updateRemoteDefaults({ proxy: event.target.value })}
          />
          <TextField
            fullWidth
            label={t("settings.runtime.remoteTimeoutMs")}
            type="number"
            value={numberInputValue(value.remote_defaults.timeout_ms)}
            onChange={(event) => updateRemoteDefaults({ timeout_ms: numberOrZero(event.target.value) })}
          />
          <TextField
            fullWidth
            helperText={t("settings.runtime.cacheZeroHint")}
            label={t("settings.runtime.remoteFetchCacheTTLSeconds")}
            type="number"
            value={numberInputValue(value.cache_defaults.remote_fetch_ttl_seconds)}
            onChange={(event) => updateCacheDefaults({ remote_fetch_ttl_seconds: numberOrZero(event.target.value) })}
          />
        </RuntimeSettingsGroup>
        {hasFeature("probe.enabled") ? (
          <RuntimeSettingsGroup id="runtime-probe-defaults" title={t("settings.runtime.group.probe")}>
            <FormControl fullWidth>
              <InputLabel id={probeMethodLabelId}>{t("settings.runtime.probeMethod")}</InputLabel>
              <Select
                label={t("settings.runtime.probeMethod")}
                labelId={probeMethodLabelId}
                value={value.probe_defaults.method}
                onChange={(event) => updateProbeDefaults({ method: event.target.value as SettingsView["probe_defaults"]["method"] })}
              >
                <MenuItem value="tcp_connect">{t("settings.runtime.probeMethodTcp")}</MenuItem>
                <MenuItem value="udp_ntp">{t("settings.runtime.probeMethodNtp")}</MenuItem>
                <MenuItem value="url_test">{t("settings.runtime.probeMethodUrl")}</MenuItem>
              </Select>
            </FormControl>
            {value.probe_defaults.method === "url_test" ? (
              <ProbeURLField
                className="md:col-span-2"
                label={t("settings.runtime.probeUrl")}
                value={value.probe_defaults.url}
                onChange={(url) => updateProbeDefaults({ url })}
              />
            ) : null}
            {value.probe_defaults.method === "udp_ntp" ? (
              <TextField
                fullWidth
                label={t("settings.runtime.probeNtpServer")}
                value={value.probe_defaults.ntp_server}
                onChange={(event) => updateProbeDefaults({ ntp_server: event.target.value })}
              />
            ) : null}
            <TextField
              fullWidth
              label={t("settings.runtime.probeTimeoutMs")}
              type="number"
              value={numberInputValue(value.probe_defaults.timeout_ms)}
              onChange={(event) => updateProbeDefaults({ timeout_ms: numberOrZero(event.target.value) })}
            />
            <TextField
              fullWidth
              label={t("settings.runtime.probeAttempts")}
              type="number"
              value={numberInputValue(value.probe_defaults.attempts)}
              onChange={(event) => updateProbeDefaults({ attempts: numberOrZero(event.target.value) })}
            />
            <TextField
              fullWidth
              label={t("settings.runtime.probeConcurrency")}
              type="number"
              value={numberInputValue(value.probe_defaults.concurrency)}
              onChange={(event) => updateProbeDefaults({ concurrency: numberOrZero(event.target.value) })}
            />
            <TextField
              fullWidth
              helperText={t("settings.runtime.cacheZeroHint")}
              label={t("settings.runtime.probeResultCacheTTLSeconds")}
              type="number"
              value={numberInputValue(value.cache_defaults.probe_ttl_seconds)}
              onChange={(event) => updateCacheDefaults({ probe_ttl_seconds: numberOrZero(event.target.value) })}
            />
          </RuntimeSettingsGroup>
        ) : null}
        <RuntimeSettingsGroup id="runtime-cache-defaults" title={t("settings.runtime.group.cache")}>
          <Typography className="md:col-span-3" color="text.secondary" variant="body2">
            {t("settings.runtime.cacheDescription")}
          </Typography>
          <TextField
            fullWidth
            label={t("settings.runtime.subscriptionTrafficCacheTTLSeconds")}
            type="number"
            value={numberInputValue(value.cache_defaults.subscription_traffic_ttl_seconds)}
            onChange={(event) => updateCacheDefaults({ subscription_traffic_ttl_seconds: numberOrZero(event.target.value) })}
          />
          <TextField
            fullWidth
            label={t("settings.runtime.subscriptionRenderCacheTTLSeconds")}
            type="number"
            value={numberInputValue(value.cache_defaults.subscription_render_ttl_seconds)}
            onChange={(event) => updateCacheDefaults({ subscription_render_ttl_seconds: numberOrZero(event.target.value) })}
          />
          <TextField
            fullWidth
            label={t("settings.runtime.fileRenderCacheTTLSeconds")}
            type="number"
            value={numberInputValue(value.cache_defaults.file_render_ttl_seconds)}
            onChange={(event) => updateCacheDefaults({ file_render_ttl_seconds: numberOrZero(event.target.value) })}
          />
        </RuntimeSettingsGroup>
      </div>
    </section>
  );
}

interface RuntimeSettingsGroupProps {
  children: ReactNode;
  id: string;
  layoutClassName?: string;
  title: string;
}

function RuntimeSettingsGroup({
  children,
  id,
  layoutClassName = "md:grid-cols-3",
  title,
}: RuntimeSettingsGroupProps) {
  const headingId = `${id}-heading`;
  return (
    <Paper aria-labelledby={headingId} component="section" variant="outlined">
      <div className="grid gap-4 p-4 sm:p-5">
        <Typography component="h4" id={headingId} variant="subtitle1">
          {title}
        </Typography>
        <div className={`grid gap-4 ${layoutClassName}`}>
          {children}
        </div>
      </div>
    </Paper>
  );
}

function numberInputValue(value: number | undefined): string {
  return value === undefined ? "" : String(value);
}

function numberOrZero(value: string): number {
  const trimmed = value.trim();
  if (!trimmed) {
    return 0;
  }
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : 0;
}
