import { useId } from "react";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select, { type SelectChangeEvent } from "@mui/material/Select";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import { supportedLocales } from "~/shared/i18n/locales";
import type { LocaleMode, ThemeMode } from "~/shared/storage/preferences";

interface AppearanceSettingsSectionProps {
  localeMode: LocaleMode;
  themeMode: ThemeMode;
  onLocaleMode: (mode: LocaleMode) => void;
  onThemeMode: (mode: ThemeMode) => void;
}

export function AppearanceSettingsSection({
  localeMode,
  themeMode,
  onLocaleMode,
  onThemeMode,
}: AppearanceSettingsSectionProps) {
  const { t } = useI18n();
  const localeLabelId = useId();
  const themeModeLabelId = useId();

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-4">
          <Typography component="h3" variant="h6">
            {t("settings.appearance.title")}
          </Typography>
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
            <Select<LocaleMode>
              label={t("settings.language.label")}
              labelId={localeLabelId}
              value={localeMode}
              onChange={(event: SelectChangeEvent<LocaleMode>) => onLocaleMode(event.target.value as LocaleMode)}
            >
              <MenuItem value="auto">{t("settings.language.optionAuto")}</MenuItem>
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
  );
}
