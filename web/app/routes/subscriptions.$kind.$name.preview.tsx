import { useCallback } from "react";
import { useNavigate, useParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useSubscriptionDetailsResource, useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import type { SubscriptionPreview } from "~/features/subscriptions/model/types";
import { SubscriptionPreviewPage } from "~/features/subscriptions/pages/subscription-preview-page";
import { useI18n } from "~/shared/i18n/context";
import { useResourcePreview } from "~/shared/preview/use-resource-preview";
import { decodeResourceRouteParam, subscriptionEditPath, subscriptionKind } from "~/shared/routing/paths";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function SubscriptionPreviewRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const app = useSandrone();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const { loadSubscriptionPreview } = useSubscriptionDetailsResource(resourcePorts);
  const kind = subscriptionKind(params.kind);
  const name = decodeResourceRouteParam(params.name);
  const item = kind && name ? subscriptions.items.find((candidate) => candidate.kind === kind && candidate.name === name) : undefined;
  const loadPreview = useCallback((): Promise<SubscriptionPreview | null> => item ? loadSubscriptionPreview(item.name) : Promise.resolve(null), [item, loadSubscriptionPreview]);
  const { failed, pending, preview, refreshPreview } = useResourcePreview<SubscriptionPreview>(item ? `${item.kind}:${item.name}` : undefined, loadPreview);

  if (subscriptions.loading) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("subscriptions.missing")} onBack={() => navigate("/subscriptions")} />;
  }

  return (
    <SubscriptionPreviewPage
      failed={failed}
      item={item}
      key={`${item.kind}:${item.name}`}
      onBack={() => navigate(subscriptionEditPath(item.kind, item.name))}
      onRefresh={refreshPreview}
      pending={pending}
      preview={preview}
    />
  );
}
