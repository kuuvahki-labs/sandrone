import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { useVersionInfo } from "~/features/settings/data/use-version-info";
import { settingsUpdateFromView } from "~/features/settings/model/project-settings";
import { SettingsPage } from "~/features/settings/pages/settings-page";

export default function SettingsRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const version = useVersionInfo({ client: app.client });

  return (
    <SettingsPage
      publicBaseUrl={app.publicBaseUrl}
      revision={version.revision}
      localeMode={app.effectiveSettings.appearance.locale}
      themeMode={app.effectiveSettings.appearance.theme_mode}
      version={version.version}
      onOpenData={() => navigate("/settings/data")}
      onOpenService={() => navigate("/settings/service")}
      onSaveBaseUrl={app.saveBaseUrl}
      onSignOut={app.signOut}
      onLocaleMode={(locale) => void app.updateSettings(settingsUpdateFromView({
        ...app.settings,
        appearance: { ...app.settings.appearance, locale },
      }))}
      onThemeMode={(themeMode) => void app.updateSettings(settingsUpdateFromView({
        ...app.settings,
        appearance: { ...app.settings.appearance, theme_mode: themeMode },
      }))}
    />
  );
}
