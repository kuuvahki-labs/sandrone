import { type ReactNode, useEffect, useId, useState } from "react";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import SaveIcon from "@mui/icons-material/Save";
import Accordion from "@mui/material/Accordion";
import AccordionDetails from "@mui/material/AccordionDetails";
import AccordionSummary from "@mui/material/AccordionSummary";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { completeRuntimeSettings, defaultRuntimeSettings } from "~/features/settings/model/runtime-settings";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { ProbeURLField } from "~/shared/ui/probe-url-field";

interface RuntimeSettingsSectionProps {
  runtimeSettings?: RuntimeSettingsInput;
  onSaveRuntimeSettings?: (value: RuntimeSettingsInput) => void;
}

export function RuntimeSettingsSection({
  runtimeSettings = defaultRuntimeSettings,
  onSaveRuntimeSettings,
}: RuntimeSettingsSectionProps) {
  const { t } = useI18n();
  const [runtimeDraft, setRuntimeDraft] = useState(() => completeRuntimeSettings(runtimeSettings));
  const probeMethodLabelId = useId();

  useEffect(() => {
    setRuntimeDraft(completeRuntimeSettings(runtimeSettings));
  }, [runtimeSettings]);

  function updateRemoteDefaults(patch: Partial<RuntimeSettingsInput["remote_defaults"]>) {
    setRuntimeDraft((current) => ({
      ...current,
      remote_defaults: { ...current.remote_defaults, ...patch },
    }));
  }

  function updateProbeDefaults(patch: Partial<RuntimeSettingsInput["probe_defaults"]>) {
    setRuntimeDraft((current) => ({
      ...current,
      probe_defaults: { ...current.probe_defaults, ...patch },
    }));
  }

  function updateCacheDefaults(patch: Partial<RuntimeSettingsInput["cache_defaults"]>) {
    setRuntimeDraft((current) => ({
      ...current,
      cache_defaults: { ...current.cache_defaults, ...patch },
    }));
  }

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-4">
          <Typography component="h3" variant="h6">
            {t("settings.runtime.title")}
          </Typography>
          <div className="grid gap-3">
            <RuntimeSettingsGroup defaultExpanded id="runtime-remote-defaults" title={t("settings.runtime.group.remote")}>
              <TextField
                fullWidth
                label={t("settings.runtime.remoteUserAgent")}
                value={runtimeDraft.remote_defaults.user_agent ?? ""}
                onChange={(event) => updateRemoteDefaults({ user_agent: event.target.value })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.remoteProxy")}
                placeholder="http://127.0.0.1:7890"
                value={runtimeDraft.remote_defaults.proxy ?? ""}
                onChange={(event) => updateRemoteDefaults({ proxy: event.target.value })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.remoteTimeoutMs")}
                type="number"
                value={numberInputValue(runtimeDraft.remote_defaults.timeout_ms)}
                onChange={(event) => updateRemoteDefaults({ timeout_ms: numberOrUndefined(event.target.value) })}
              />
            </RuntimeSettingsGroup>
            <RuntimeSettingsGroup id="runtime-cache-defaults" title={t("settings.runtime.group.cache")}>
              <TextField
                fullWidth
                label={t("settings.runtime.remoteFetchCacheTTLSeconds")}
                type="number"
                value={numberInputValue(runtimeDraft.cache_defaults.remote_fetch_ttl_seconds)}
                onChange={(event) => updateCacheDefaults({ remote_fetch_ttl_seconds: numberOrUndefined(event.target.value) ?? 0 })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.subscriptionTrafficCacheTTLSeconds")}
                type="number"
                value={numberInputValue(runtimeDraft.cache_defaults.subscription_traffic_ttl_seconds)}
                onChange={(event) => updateCacheDefaults({ subscription_traffic_ttl_seconds: numberOrUndefined(event.target.value) ?? 0 })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.subscriptionRenderCacheTTLSeconds")}
                type="number"
                value={numberInputValue(runtimeDraft.cache_defaults.subscription_render_ttl_seconds)}
                onChange={(event) => updateCacheDefaults({ subscription_render_ttl_seconds: numberOrUndefined(event.target.value) ?? 0 })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.fileRenderCacheTTLSeconds")}
                type="number"
                value={numberInputValue(runtimeDraft.cache_defaults.file_render_ttl_seconds)}
                onChange={(event) => updateCacheDefaults({ file_render_ttl_seconds: numberOrUndefined(event.target.value) ?? 0 })}
              />
            </RuntimeSettingsGroup>
            <RuntimeSettingsGroup id="runtime-probe-defaults" title={t("settings.runtime.group.probe")}>
              <FormControl fullWidth>
                <InputLabel id={probeMethodLabelId}>{t("settings.runtime.probeMethod")}</InputLabel>
                <Select
                  label={t("settings.runtime.probeMethod")}
                  labelId={probeMethodLabelId}
                  value={runtimeDraft.probe_defaults.method ?? "url_test"}
                  onChange={(event) => updateProbeDefaults({ method: event.target.value as RuntimeSettingsInput["probe_defaults"]["method"] })}
                >
                  <MenuItem value="tcp_connect">tcp_connect</MenuItem>
                  <MenuItem value="udp_ntp">udp_ntp</MenuItem>
                  <MenuItem value="url_test">url_test</MenuItem>
                </Select>
              </FormControl>
              <ProbeURLField
                className="md:col-span-2"
                label={t("settings.runtime.probeUrl")}
                value={runtimeDraft.probe_defaults.url ?? ""}
                onChange={(url) => updateProbeDefaults({ url })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.probeNtpServer")}
                value={runtimeDraft.probe_defaults.ntp_server ?? ""}
                onChange={(event) => updateProbeDefaults({ ntp_server: event.target.value })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.probeTimeoutMs")}
                type="number"
                value={numberInputValue(runtimeDraft.probe_defaults.timeout_ms)}
                onChange={(event) => updateProbeDefaults({ timeout_ms: numberOrUndefined(event.target.value) })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.probeAttempts")}
                type="number"
                value={numberInputValue(runtimeDraft.probe_defaults.attempts)}
                onChange={(event) => updateProbeDefaults({ attempts: numberOrUndefined(event.target.value) })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.probeConcurrency")}
                type="number"
                value={numberInputValue(runtimeDraft.probe_defaults.concurrency)}
                onChange={(event) => updateProbeDefaults({ concurrency: numberOrUndefined(event.target.value) })}
              />
              <TextField
                fullWidth
                label={t("settings.runtime.probeCacheTTLSeconds")}
                type="number"
                value={numberInputValue(runtimeDraft.probe_defaults.cache_ttl_seconds)}
                onChange={(event) => updateProbeDefaults({ cache_ttl_seconds: numberOrUndefined(event.target.value) ?? 0 })}
              />
            </RuntimeSettingsGroup>
          </div>
          <Button aria-label={t("settings.runtime.save")} className="justify-self-end" startIcon={<SaveIcon aria-hidden />} type="button" variant="contained" onClick={() => onSaveRuntimeSettings?.(runtimeDraft)}>
            {t("actions.save")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}

interface RuntimeSettingsGroupProps {
  children: ReactNode;
  defaultExpanded?: boolean;
  id: string;
  title: string;
}

function RuntimeSettingsGroup({ children, defaultExpanded = false, id, title }: RuntimeSettingsGroupProps) {
  return (
    <Accordion defaultExpanded={defaultExpanded} disableGutters variant="outlined">
      <AccordionSummary
        aria-controls={`${id}-content`}
        expandIcon={<ExpandMoreIcon aria-hidden />}
        id={`${id}-header`}
      >
        <Typography component="h4" variant="subtitle1">
          {title}
        </Typography>
      </AccordionSummary>
      <AccordionDetails>
        <div className="grid gap-4 md:grid-cols-3">
          {children}
        </div>
      </AccordionDetails>
    </Accordion>
  );
}

function numberInputValue(value: number | undefined): string {
  return value === undefined ? "" : String(value);
}

function numberOrUndefined(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) ? parsed : undefined;
}
