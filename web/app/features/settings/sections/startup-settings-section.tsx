import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Switch from "@mui/material/Switch";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { SettingsView } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

interface StartupSettingsSectionProps {
  value: SettingsView;
  overrides: Record<string, string>;
  token: string | undefined;
  onChange: (value: SettingsView) => void;
  onTokenChange: (value: string | undefined) => void;
}

export function StartupSettingsSection({
  value,
  overrides,
  token,
  onChange,
  onTokenChange,
}: StartupSettingsSectionProps) {
  const { t } = useI18n();
  const source = (path: string) => overrides[path]
    ? t("settings.startup.overridden", { source: overrides[path] })
    : undefined;

  return (
    <Card component="article" variant="outlined">
      <CardContent className="grid gap-4">
        <Typography component="h3" variant="h6">{t("settings.startup.title")}</Typography>
        <div className="grid gap-4 md:grid-cols-2">
          <TextField
            fullWidth
            helperText={source("http.listen")}
            label={t("settings.startup.httpListen")}
            value={value.http.listen}
            onChange={(event) => onChange({ ...value, http: { ...value.http, listen: event.target.value } })}
          />
          <TextField
            fullWidth
            helperText={source("http.token") ?? t("settings.startup.tokenHelp")}
            label={t("settings.startup.token")}
            type="password"
            value={token ?? ""}
            onChange={(event) => onTokenChange(event.target.value || undefined)}
          />
          <FormControlLabel
            control={<Switch checked={value.http.token_required} onChange={(event) => onChange({ ...value, http: { ...value.http, token_required: event.target.checked } })} />}
            label={t("settings.startup.tokenRequired")}
          />
          <FormControl fullWidth>
            <InputLabel>{t("settings.startup.mcpTransport")}</InputLabel>
            <Select
              label={t("settings.startup.mcpTransport")}
              value={value.mcp.transport}
              onChange={(event) => onChange({ ...value, mcp: { ...value.mcp, transport: event.target.value as SettingsView["mcp"]["transport"] } })}
            >
              <MenuItem value="stdio">stdio</MenuItem>
              <MenuItem value="streamable-http">streamable-http</MenuItem>
            </Select>
          </FormControl>
          <TextField
            fullWidth
            helperText={source("mcp.path")}
            label={t("settings.startup.mcpPath")}
            value={value.mcp.path}
            onChange={(event) => onChange({ ...value, mcp: { ...value.mcp, path: event.target.value } })}
          />
          <TextField
            fullWidth
            label={t("settings.startup.mcpMaxOutput")}
            type="number"
            value={value.mcp.max_output_bytes}
            onChange={(event) => onChange({ ...value, mcp: { ...value.mcp, max_output_bytes: Number(event.target.value) || 0 } })}
          />
          <FormControlLabel
            control={<Switch checked={value.mcp.allow_management_tools} onChange={(event) => onChange({ ...value, mcp: { ...value.mcp, allow_management_tools: event.target.checked } })} />}
            label={t("settings.startup.mcpManagement")}
          />
          <TextField
            fullWidth
            helperText={source("webui.static_dir")}
            label={t("settings.startup.webuiDir")}
            value={value.webui.static_dir}
            onChange={(event) => onChange({ ...value, webui: { static_dir: event.target.value } })}
          />
          <FormControl fullWidth>
            <InputLabel>{t("settings.startup.logLevel")}</InputLabel>
            <Select
              label={t("settings.startup.logLevel")}
              value={value.log.level}
              onChange={(event) => onChange({ ...value, log: { level: event.target.value as SettingsView["log"]["level"] } })}
            >
              {(["debug", "info", "warn", "error"] as const).map((level) => <MenuItem key={level} value={level}>{level}</MenuItem>)}
            </Select>
          </FormControl>
        </div>
        <FormControlLabel
          control={<Switch checked={token === ""} onChange={(event) => onTokenChange(event.target.checked ? "" : undefined)} />}
          label={t("settings.startup.clearToken")}
        />
      </CardContent>
    </Card>
  );
}
