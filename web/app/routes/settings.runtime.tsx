import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { useFileResources } from "~/features/files/data/use-file-resources";
import { useScheduledRefreshStatus } from "~/features/settings/data/use-scheduled-refresh-status";
import { useVersionInfo } from "~/features/settings/data/use-version-info";
import { SettingsRuntimePage } from "~/features/settings/pages/settings-runtime-page";
import { useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import { useI18n } from "~/shared/i18n/context";
import { resourceOptionText } from "~/shared/resources/labels";

export default function SettingsRuntimeRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const { t } = useI18n();
  const ports = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(ports);
  const files = useFileResources(ports);
  const scheduledRefreshStatus = useScheduledRefreshStatus(app.client);
  const version = useVersionInfo({ client: app.client });
  const defaultUserAgent = version.name && version.version ? `${version.name}/${version.version}` : "";
  const scheduledRefreshResources = [
    ...subscriptions.items.map((item) => ({ kind: "subscription" as const, name: item.name, label: resourceOptionText(item) })),
    ...files.items.map((item) => ({ kind: "file" as const, name: item.name, label: resourceOptionText(item) })),
  ];

  return (
    <SettingsRuntimePage
      defaultUserAgent={defaultUserAgent}
      overrides={app.settingsOverrides}
      restartRequired={app.restartRequired}
      scheduledRefreshResources={scheduledRefreshResources}
      scheduledRefreshStatus={scheduledRefreshStatus}
      settings={app.settings}
      onBack={() => navigate("/settings")}
      onSave={app.updateSettings}
    />
  );
}
