import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { SettingsRuntimePage } from "~/features/settings/pages/settings-runtime-page";

export default function SettingsRuntimeRoute() {
  const app = useSandrone();
  const navigate = useNavigate();

  return (
    <SettingsRuntimePage
      overrides={app.settingsOverrides}
      restartRequired={app.restartRequired}
      settings={app.settings}
      onBack={() => navigate("/settings")}
      onSave={app.updateSettings}
    />
  );
}
