import { useSandrone } from "~/core/provider/context";
import { useRuntimeSettings } from "~/features/settings/data/use-runtime-settings";
import { SettingsPage } from "~/features/settings/pages/settings-page";
import { useI18n } from "~/shared/i18n/context";

export default function SettingsRoute() {
  const app = useSandrone();
  const { t } = useI18n();
  const settings = useRuntimeSettings({
    client: app.client,
    showNotice: app.showNotice,
    t,
  });

  return (
    <SettingsPage
      publicBaseUrl={app.publicBaseUrl}
      revision={settings.revision}
      runtimeSettings={settings.runtimeSettings}
      themeMode={app.themeMode}
      version={settings.version}
      onDownloadBackup={settings.downloadBackup}
      onRestoreBackup={settings.restoreBackup}
      onSaveBaseUrl={app.saveBaseUrl}
      onSaveRuntimeSettings={settings.saveRuntimeSettings}
      onSignOut={app.signOut}
      onThemeMode={app.updateThemeMode}
    />
  );
}
