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
import type { SubscriptionInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { subscriptionEditPath, subscriptionNewPath, subscriptionPreviewPath } from "~/shared/routing/paths";
import { ResourceImportIcon, useResourceTransferController } from "~/shared/ui/resource-transfer-controller";

export function SubscriptionsRoute() {
  const navigate = useNavigate();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const { loadSubscriptionTraffic } = useSubscriptionDetailsResource(resourcePorts);
  const autoLoadTraffic = app.settingsLoaded && app.effectiveSettings.subscriptions.auto_load_traffic;
  const trafficByKey = useSubscriptionTrafficByKey(
    subscriptions.items,
    autoLoadTraffic,
    loadSubscriptionTraffic,
  );
  const transfer = useResourceTransferController({
    existingNames: subscriptions.items.map((item) => item.name),
    loadResource: (name) => app.client.getSubscription(name),
    onSaved: async (resource) => {
      await subscriptions.reload();
      const type = transferredSubscriptionType(resource.type);
      navigate(type ? subscriptionEditPath(type, String(resource.name)) : "/subscriptions");
    },
    resourceType: "subscription",
    saveResource: (resource) => app.client.createSubscription(resource as unknown as SubscriptionInput),
    showNotice: app.showNotice,
    t,
  });

  if (subscriptions.loading) return <LoadingScreen />;

  return (
    <>
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
          {
            ariaLabel: t("resourceTransfer.importSubscription"),
            icon: <ResourceImportIcon />,
            label: t("actions.import"),
            onSelect: () => { void transfer.importResource(); },
          },
        ]}
        getTrafficKey={subscriptionTrafficKey}
        items={subscriptions.items}
        loaded={subscriptions.loaded}
        trafficByKey={autoLoadTraffic ? trafficByKey : undefined}
        onCopy={(item) => transfer.copyResource(item.name)}
        onDelete={(item) => app.requestDelete({ kind: "subscriptions", name: item.name, label: t("nav.subscriptions"), onDeleted: subscriptions.reload })}
        onEdit={(item) => navigate(subscriptionEditPath(item.kind, item.name))}
        onExport={(item) => { void transfer.exportResource(item.name); }}
        onPreview={(item) => navigate(subscriptionPreviewPath(item.kind, item.name, "list"))}
        onShare={(item) => shareDialog.open({ kind: "subscription", name: item.name })}
      />
      {transfer.dialogs}
    </>
  );
}

function subscriptionTrafficKey(item: SubscriptionItem): string {
  return `${item.kind}:${item.name}`;
}

function transferredSubscriptionType(value: unknown) {
  return value === "remote" || value === "local" || value === "collection" ? value : null;
}
