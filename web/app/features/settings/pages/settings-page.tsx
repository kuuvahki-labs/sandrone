import { useState } from "react";
import AdminPanelSettingsOutlinedIcon from "@mui/icons-material/AdminPanelSettingsOutlined";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import LogoutIcon from "@mui/icons-material/Logout";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import Link from "@mui/material/Link";
import Typography from "@mui/material/Typography";

import { DataSettingsSection } from "~/features/settings/sections/data-settings-section";
import { GeneralSettingsSection } from "~/features/settings/sections/general-settings-section";
import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import type { ThemeMode } from "~/shared/storage/preferences";
import { PageHeader } from "~/shared/ui/page";

export interface SettingsPageProps {
  publicBaseUrl: string;
  runtimeSettings?: RuntimeSettingsInput;
  themeMode: ThemeMode;
  version?: string;
  onDownloadBackup?: () => Promise<void>;
  onRestoreBackup?: (file: Blob) => Promise<void>;
  onSaveBaseUrl: (value: string) => void;
  onSaveRuntimeSettings?: (value: RuntimeSettingsInput) => void;
  onSignOut: () => void;
  onThemeMode: (mode: ThemeMode) => void;
}

export function SettingsPage({
  publicBaseUrl,
  runtimeSettings,
  themeMode,
  version,
  onDownloadBackup,
  onRestoreBackup,
  onSaveBaseUrl,
  onSaveRuntimeSettings,
  onSignOut,
  onThemeMode,
}: SettingsPageProps) {
  const { t } = useI18n();
  const [confirmSignOut, setConfirmSignOut] = useState(false);

  return (
    <section className="grid gap-6">
      <PageHeader
        label={t("nav.settings")}
        title={t("settings.title")}
      />
      <div className="grid gap-4 md:grid-cols-2">
        <GeneralSettingsSection
          publicBaseUrl={publicBaseUrl}
          themeMode={themeMode}
          onSaveBaseUrl={onSaveBaseUrl}
          onThemeMode={onThemeMode}
        />
        <RuntimeSettingsSection
          runtimeSettings={runtimeSettings}
          onSaveRuntimeSettings={onSaveRuntimeSettings}
        />
        <DataSettingsSection
          onDownloadBackup={onDownloadBackup}
          onRestoreBackup={onRestoreBackup}
        />
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-4">
              <div className="flex items-center gap-3">
                <AdminPanelSettingsOutlinedIcon aria-hidden color="action" />
                <Typography component="h3" variant="h6">
                  {t("settings.adminToken.title")}
                </Typography>
              </div>
              <Typography color="text.secondary">{t("settings.adminToken.description")}</Typography>
              <Button color="error" startIcon={<LogoutIcon aria-hidden />} type="button" onClick={() => setConfirmSignOut(true)}>
                {t("settings.signOut.action")}
              </Button>
            </div>
          </CardContent>
        </Card>
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-4">
              <div className="flex items-center gap-3">
                <InfoOutlinedIcon aria-hidden color="action" />
                <Typography component="h3" variant="h6">
                  {t("settings.about.title")}
                </Typography>
              </div>
              <Typography className="break-words" color="text.secondary">
                {t("settings.about.description")}
              </Typography>
              <div className="flex items-center justify-between gap-4">
                <Typography color="text.secondary" variant="body2">
                  {t("settings.about.version")}
                </Typography>
                <Typography variant="body2">
                  {version ? `v${version}` : t("settings.about.versionUnavailable")}
                </Typography>
              </div>
              <Typography color="text.secondary" variant="body2">
                <Link href="https://github.com/kuuvahki-labs/sandrone" rel="noreferrer" target="_blank" underline="hover">
                  GitHub
                </Link>
              </Typography>
            </div>
          </CardContent>
        </Card>
      </div>

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
    </section>
  );
}
