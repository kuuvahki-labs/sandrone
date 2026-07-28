import { useId } from "react";
import PaletteOutlinedIcon from "@mui/icons-material/PaletteOutlined";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControl from "@mui/material/FormControl";
import MenuItem from "@mui/material/MenuItem";
import Select, { type SelectChangeEvent } from "@mui/material/Select";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import { type Locale, supportedLocales } from "~/shared/i18n/locales";
import type { ThemeMode } from "~/shared/storage/preferences";

interface AppearanceSettingsSectionProps {
  themeMode: ThemeMode;
  onThemeMode: (mode: ThemeMode) => void;
}

export function AppearanceSettingsSection({
  themeMode,
  onThemeMode,
}: AppearanceSettingsSectionProps) {
  const { locale, setLocale, t } = useI18n();
  const localeLabelId = useId();
  const themeModeLabelId = useId();

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-4">
          <div className="flex items-center gap-3">
            <PaletteOutlinedIcon aria-hidden color="action" />
            <Typography component="h3" variant="h6">
              {t("settings.appearance.title")}
            </Typography>
          </div>
          <div aria-labelledby={themeModeLabelId} className="grid min-w-0 gap-2 sm:grid-cols-[minmax(8rem,0.45fr)_minmax(0,1fr)] sm:items-center" role="group">
            <Typography id={themeModeLabelId}>{t("settings.theme.label")}</Typography>
            <FormControl fullWidth>
              <Select<ThemeMode>
                labelId={themeModeLabelId}
                value={themeMode}
                onChange={(event: SelectChangeEvent<ThemeMode>) => onThemeMode(event.target.value as ThemeMode)}
              >
                <MenuItem value="dark">{t("settings.theme.dark")}</MenuItem>
                <MenuItem value="light">{t("settings.theme.light")}</MenuItem>
                <MenuItem value="system">{t("settings.theme.system")}</MenuItem>
              </Select>
            </FormControl>
          </div>
          <div aria-labelledby={localeLabelId} className="grid min-w-0 gap-2 sm:grid-cols-[minmax(8rem,0.45fr)_minmax(0,1fr)] sm:items-center" role="group">
            <Typography id={localeLabelId}>{t("settings.language.label")}</Typography>
            <FormControl fullWidth>
              <Select<Locale>
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
        </div>
      </CardContent>
    </Card>
  );
}
