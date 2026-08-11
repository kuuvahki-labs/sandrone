import { useCallback } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useSubscriptionDetailsResource, useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import type { SubscriptionPreview } from "~/features/subscriptions/model/types";
import { SubscriptionPreviewPage } from "~/features/subscriptions/pages/subscription-preview-page";
import { useI18n } from "~/shared/i18n/context";
import { useResourcePreview } from "~/shared/preview/use-resource-preview";
import { decodeResourceRouteParam, resourcePreviewOrigin, subscriptionEditPath, subscriptionKind } from "~/shared/routing/paths";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function SubscriptionPreviewRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const [searchParams] = useSearchParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const { loadSubscriptionPreview } = useSubscriptionDetailsResource(resourcePorts);
  const kind = subscriptionKind(params.kind);
  const name = decodeResourceRouteParam(params.name);
  const item = kind && name ? subscriptions.items.find((candidate) => candidate.kind === kind && candidate.name === name) : undefined;
  const backToList = resourcePreviewOrigin(searchParams.get("from")) === "list";
  const loadPreview = useCallback((): Promise<SubscriptionPreview | null> => item ? loadSubscriptionPreview(item.name) : Promise.resolve(null), [item, loadSubscriptionPreview]);
  const refreshPreviewLoader = useCallback((): Promise<SubscriptionPreview | null> => item ? loadSubscriptionPreview(item.name, { refresh: true }) : Promise.resolve(null), [item, loadSubscriptionPreview]);
  const { failed, pending, preview, refreshPreview } = useResourcePreview<SubscriptionPreview>(item ? `${item.kind}:${item.name}` : undefined, loadPreview, refreshPreviewLoader);

  if (subscriptions.loading) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("subscriptions.missing")} onBack={() => navigate("/subscriptions")} />;
  }

  return (
    <SubscriptionPreviewPage
      backLabel={t("actions.back")}
      failed={failed}
      item={item}
      key={`${item.kind}:${item.name}`}
      onBack={() => navigate(backToList ? "/subscriptions" : subscriptionEditPath(item.kind, item.name))}
      onRefresh={refreshPreview}
      onShare={() => shareDialog.open({ kind: "subscription", name: item.name })}
      pending={pending}
      preview={preview}
    />
  );
}
