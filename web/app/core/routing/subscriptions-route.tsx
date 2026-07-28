import { useNavigate } from "react-router";
import AccountTreeOutlinedIcon from "@mui/icons-material/AccountTreeOutlined";
import CloudDownloadOutlinedIcon from "@mui/icons-material/CloudDownloadOutlined";
import LinkOutlinedIcon from "@mui/icons-material/LinkOutlined";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useSubscriptionDetailsResource, useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import { useSubscriptionTrafficByKey } from "~/features/subscriptions/data/use-subscription-traffic";
import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import { SubscriptionsPage } from "~/features/subscriptions/pages/subscriptions-page";
import { useI18n } from "~/shared/i18n/context";
import { subscriptionEditPath, subscriptionNewPath } from "~/shared/routing/paths";

export function SubscriptionsRoute() {
  const navigate = useNavigate();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const { loadSubscriptionTraffic } = useSubscriptionDetailsResource(resourcePorts);
  const trafficByKey = useSubscriptionTrafficByKey(
    subscriptions.items,
    app.autoLoadSubscriptionTraffic,
    loadSubscriptionTraffic,
  );

  if (subscriptions.loading) return <LoadingScreen />;

  return (
    <SubscriptionsPage
      createActions={[
        {
          ariaLabel: t("subscriptions.create.remote"),
          icon: <CloudDownloadOutlinedIcon aria-hidden />,
          label: t("model.subscription.remoteShort"),
          onSelect: () => navigate(subscriptionNewPath("remote")),
        },
        {
          ariaLabel: t("subscriptions.create.local"),
          icon: <LinkOutlinedIcon aria-hidden />,
          label: t("model.subscription.localShort"),
          onSelect: () => navigate(subscriptionNewPath("local")),
        },
        {
          ariaLabel: t("subscriptions.create.collection"),
          icon: <AccountTreeOutlinedIcon aria-hidden />,
          label: t("model.subscription.collectionShort"),
          onSelect: () => navigate(subscriptionNewPath("collection")),
        },
      ]}
      getTrafficKey={subscriptionTrafficKey}
      items={subscriptions.items}
      trafficByKey={app.autoLoadSubscriptionTraffic ? trafficByKey : undefined}
      onDelete={(item) => app.requestDelete({ kind: "subscriptions", name: item.name, label: t("nav.subscriptions"), onDeleted: subscriptions.reload })}
      onEdit={(item) => navigate(subscriptionEditPath(item.kind, item.name))}
      onShare={(item) => shareDialog.open({ kind: "subscription", name: item.name })}
    />
  );
}

function subscriptionTrafficKey(item: SubscriptionItem): string {
  return `${item.kind}:${item.name}`;
}
