import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { useVersionInfo } from "~/features/settings/data/use-version-info";
import { SettingsPage } from "~/features/settings/pages/settings-page";

export default function SettingsRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const version = useVersionInfo({ client: app.client });

  return (
    <SettingsPage
      publicBaseUrl={app.publicBaseUrl}
      revision={version.revision}
      themeMode={app.themeMode}
      version={version.version}
      onOpenData={() => navigate("/settings/data")}
      onOpenRuntime={() => navigate("/settings/runtime")}
      onSaveBaseUrl={app.saveBaseUrl}
      onSignOut={app.signOut}
      onThemeMode={app.updateThemeMode}
    />
  );
}
