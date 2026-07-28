import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { useRuntimeSettings } from "~/features/settings/data/use-runtime-settings";
import { SettingsRuntimePage } from "~/features/settings/pages/settings-runtime-page";
import { useI18n } from "~/shared/i18n/context";

export default function SettingsRuntimeRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const { t } = useI18n();
  const settings = useRuntimeSettings({
    client: app.client,
    showNotice: app.showNotice,
    t,
  });

  return (
    <SettingsRuntimePage
      autoLoadSubscriptionTraffic={app.autoLoadSubscriptionTraffic}
      runtimeSettings={settings.runtimeSettings}
      onBack={() => navigate("/settings")}
      onAutoLoadSubscriptionTraffic={app.updateAutoLoadSubscriptionTraffic}
      onSaveRuntimeSettings={settings.saveRuntimeSettings}
    />
  );
}
