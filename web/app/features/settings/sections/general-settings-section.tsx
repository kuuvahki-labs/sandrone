import { useId, useState } from "react";
import PaletteOutlinedIcon from "@mui/icons-material/PaletteOutlined";
import SaveIcon from "@mui/icons-material/Save";
import StorageOutlinedIcon from "@mui/icons-material/StorageOutlined";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select, { type SelectChangeEvent } from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import { type Locale, supportedLocales } from "~/shared/i18n/locales";
import type { ThemeMode } from "~/shared/storage/preferences";

interface GeneralSettingsSectionProps {
  publicBaseUrl: string;
  themeMode: ThemeMode;
  onSaveBaseUrl: (value: string) => void;
  onThemeMode: (mode: ThemeMode) => void;
}

export function GeneralSettingsSection({
  publicBaseUrl,
  themeMode,
  onSaveBaseUrl,
  onThemeMode,
}: GeneralSettingsSectionProps) {
  const { locale, setLocale, t } = useI18n();
  const [baseUrl, setBaseUrl] = useState(publicBaseUrl);
  const localeLabelId = useId();
  const themeModeLabelId = useId();

  return (
    <>
      <Card component="article" variant="outlined">
        <CardContent>
          <div className="grid gap-4">
            <div className="flex items-center gap-3">
              <PaletteOutlinedIcon aria-hidden color="action" />
              <Typography component="h3" variant="h6">
                {t("settings.appearance.title")}
              </Typography>
            </div>
            <FormControl fullWidth>
              <InputLabel id={themeModeLabelId}>{t("settings.theme.label")}</InputLabel>
              <Select<ThemeMode>
                label={t("settings.theme.label")}
                labelId={themeModeLabelId}
                value={themeMode}
                onChange={(event: SelectChangeEvent<ThemeMode>) => onThemeMode(event.target.value as ThemeMode)}
              >
                <MenuItem value="dark">{t("settings.theme.dark")}</MenuItem>
                <MenuItem value="light">{t("settings.theme.light")}</MenuItem>
                <MenuItem value="system">{t("settings.theme.system")}</MenuItem>
              </Select>
            </FormControl>
            <FormControl fullWidth>
              <InputLabel id={localeLabelId}>{t("settings.language.label")}</InputLabel>
              <Select<Locale>
                label={t("settings.language.label")}
                labelId={localeLabelId}
                value={locale}
                onChange={(event: SelectChangeEvent<Locale>) => setLocale(event.target.value as Locale)}
              >
                {supportedLocales.map((option) => (
                  <MenuItem key={option} value={option}>
                    {option === "zh-CN" ? t("settings.language.optionChinese") : t("settings.language.optionEnglish")}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </div>
        </CardContent>
      </Card>
      <Card component="article" variant="outlined">
        <CardContent>
          <div className="grid gap-4">
            <div className="flex items-center gap-3">
              <StorageOutlinedIcon aria-hidden color="action" />
              <Typography component="h3" variant="h6">
                {t("settings.publicBaseUrl.title")}
              </Typography>
            </div>
            <TextField
              fullWidth
              label="Public Base URL"
              placeholder="https://example.com"
              type="url"
              value={baseUrl}
              onChange={(event) => setBaseUrl(event.target.value)}
            />
            <Button aria-label={t("settings.publicBaseUrl.save")} startIcon={<SaveIcon aria-hidden />} type="button" variant="contained" onClick={() => onSaveBaseUrl(baseUrl)}>
              {t("actions.save")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}
