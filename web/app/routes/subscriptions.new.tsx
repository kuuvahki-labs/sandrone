import { useNavigate, useSearchParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useFileResources } from "~/features/files/data/use-file-resources";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { createSubscriptionActions } from "~/features/subscriptions/data/create-subscription-actions";
import { useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import { SubscriptionNewPage } from "~/features/subscriptions/pages/subscription-new-page";
import { useI18n } from "~/shared/i18n/context";
import { remoteInputDefaultsFromSettings } from "~/shared/resources/remote-input-defaults";
import type { SubscriptionCreateType } from "~/shared/routing/paths";
import { subscriptionNewPath } from "~/shared/routing/paths";

export default function NewSubscriptionRoute() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const type = normalizeSubscriptionCreateType(searchParams.get("type"));
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const files = useFileResources(resourcePorts);
  const subscriptionActions = createSubscriptionActions({
    client: app.client,
    closeSheet: shareDialog.close,
    navigate,
    refreshResources: noopRefresh,
    showNotice: app.showNotice,
    t,
  });

  if (subscriptions.loading || files.loading) return <LoadingScreen />;

  return (
    <SubscriptionNewPage
      probeCacheTTLSeconds={app.effectiveSettings.cache_defaults.probe_ttl_seconds}
      probeDefaults={app.effectiveSettings.probe_defaults}
      remoteDefaults={remoteInputDefaultsFromSettings(app.effectiveSettings)}
      scriptFiles={files.items.map(({ name, title }) => ({ name, title }))}
      scriptTimeoutMS={app.effectiveSettings.script_defaults.timeout_ms}
      sources={subscriptions.items}
      type={type}
      onBack={() => navigate("/subscriptions")}
      onCopySource={subscriptionActions.copySubscriptionSource}
      onSave={subscriptionActions.createSubscription}
      onTypeChange={(nextType) => navigate(subscriptionNewPath(nextType), { replace: true })}
    />
  );
}

export function normalizeSubscriptionCreateType(value: string | null | undefined): SubscriptionCreateType {
  if (value === "local" || value === "collection") {
    return value;
  }
  return "remote";
}

async function noopRefresh() {}
