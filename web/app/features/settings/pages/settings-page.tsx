import { useState } from "react";
import ChevronRightIcon from "@mui/icons-material/ChevronRight";
import LogoutIcon from "@mui/icons-material/Logout";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardActionArea from "@mui/material/CardActionArea";
import CardContent from "@mui/material/CardContent";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import Link from "@mui/material/Link";
import Typography from "@mui/material/Typography";

import { AppearanceSettingsSection } from "~/features/settings/sections/appearance-settings-section";
import { ServiceConnectionSection } from "~/features/settings/sections/service-connection-section";
import { useI18n } from "~/shared/i18n/context";
import type { LocaleMode, ThemeMode } from "~/shared/storage/preferences";
import { PageHeader } from "~/shared/ui/page";

export interface SettingsPageProps {
  publicBaseUrl: string;
  revision?: string;
  localeMode: LocaleMode;
  themeMode: ThemeMode;
  version?: string;
  onOpenData: () => void;
  onOpenRuntime: () => void;
  onSaveBaseUrl: (value: string) => void;
  onSignOut: () => void;
  onLocaleMode: (mode: LocaleMode) => void;
  onThemeMode: (mode: ThemeMode) => void;
}

export function SettingsPage({
  publicBaseUrl,
  revision,
  localeMode,
  themeMode,
  version,
  onOpenData,
  onOpenRuntime,
  onSaveBaseUrl,
  onSignOut,
  onLocaleMode,
  onThemeMode,
}: SettingsPageProps) {
  const { t } = useI18n();
  const [confirmSignOut, setConfirmSignOut] = useState(false);

  return (
    <>
      <section className="grid gap-6">
        <PageHeader
          description={t("settings.description")}
          label={t("nav.settings")}
          title={t("settings.title")}
        />
        <AppearanceSettingsSection
          localeMode={localeMode}
          themeMode={themeMode}
          onLocaleMode={onLocaleMode}
          onThemeMode={onThemeMode}
        />
        <ServiceConnectionSection
          publicBaseUrl={publicBaseUrl}
          onSaveBaseUrl={onSaveBaseUrl}
        />
        <Card component="article" variant="outlined">
          <CardActionArea component="div" aria-label={t("settings.advanced.open")} onClick={onOpenRuntime}>
            <CardContent className="flex items-center justify-between gap-4">
              <Typography component="h3" variant="h6">{t("settings.advanced.title")}</Typography>
              <ChevronRightIcon aria-hidden color="action" />
            </CardContent>
          </CardActionArea>
        </Card>
        <Card component="article" variant="outlined">
          <CardContent className="grid gap-3">
            <Typography component="h3" variant="h6">{t("settings.dataAndAccount.title")}</Typography>
            <Button
              aria-label={t("settings.data.open")}
              className="justify-between"
              endIcon={<ChevronRightIcon aria-hidden />}
              onClick={onOpenData}
            >
              {t("settings.data.entry")}
            </Button>
            <div className="flex min-w-0 flex-col items-start gap-3 border-t border-divider pt-3 sm:flex-row sm:items-center sm:justify-between sm:gap-4">
              <div className="min-w-0">
                <Typography>{t("settings.adminToken.title")}</Typography>
                <Typography className="break-words" color="text.secondary" variant="body2">
                  {t("settings.adminToken.description")}
                </Typography>
              </div>
              <Button className="shrink-0" color="error" startIcon={<LogoutIcon aria-hidden />} onClick={() => setConfirmSignOut(true)}>
                {t("settings.signOut.action")}
              </Button>
            </div>
          </CardContent>
        </Card>
        <Card component="article" variant="outlined">
          <CardContent className="grid gap-3">
            <div className="flex items-center justify-between gap-4">
              <Typography component="h3" variant="h6">{t("settings.about.title")}</Typography>
              <Typography color="text.secondary" variant="body2">
                {version
                  ? `${version === "dev" ? "dev" : `v${version}`}${revision ? ` (${revision.slice(0, 12)})` : ""}`
                  : t("settings.about.versionUnavailable")}
              </Typography>
            </div>
            <Link href="https://github.com/kuuvahki-labs/sandrone" rel="noreferrer" target="_blank">
              GitHub
            </Link>
          </CardContent>
        </Card>
      </section>

      <Dialog open={confirmSignOut} aria-labelledby="sign-out-title" onClose={() => setConfirmSignOut(false)}>
        <DialogTitle id="sign-out-title">{t("settings.signOut.confirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t("settings.signOut.confirmBody")}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button type="button" onClick={() => setConfirmSignOut(false)}>{t("actions.cancel")}</Button>
          <Button aria-label={t("settings.signOut.action")} color="error" type="button" variant="contained" onClick={onSignOut}>{t("actions.signOut")}</Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
