import { useCallback } from "react";
import { useNavigate, useSearchParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { createFileActions } from "~/features/files/data/create-file-actions";
import { useFileResources } from "~/features/files/data/use-file-resources";
import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";
import { ruleSetCatalogFromAPI } from "~/features/files/model/codec";
import type { RuleSetCatalogTarget } from "~/features/files/model/types";
import { FileNewPage } from "~/features/files/pages/file-new-page";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import { subscriptionPreviewFromAPI } from "~/features/subscriptions/model/codec";
import { useI18n } from "~/shared/i18n/context";
import type { FileCreateSource } from "~/shared/routing/paths";

export default function NewFileRoute() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const source = normalizeFileCreateSource(searchParams.get("source"));
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const files = useFileResources(resourcePorts);
  const subscriptions = useSubscriptionResources(resourcePorts);
  const fileActions = createFileActions({
    client: app.client,
    closeSheet: shareDialog.close,
    navigate,
    refreshResources: noopRefresh,
    showNotice: app.showNotice,
    t,
  });
  const loadSubscriptionPreview = useCallback(async (name: string) => (
    subscriptionPreviewFromAPI(await app.client.previewSubscription(name))
  ), [app.client]);
  const loadRuleSetCatalog = useCallback(async (target: RuleSetCatalogTarget) => ruleSetCatalogFromAPI(await app.client.listRuleSetCatalog(target)), [app.client]);

  if (files.loading || subscriptions.loading) return <LoadingScreen />;

  return (
    <FileNewPage
      loadSubscriptionPreview={loadSubscriptionPreview}
      loadRuleSetCatalog={loadRuleSetCatalog}
      source={source}
      onBack={() => navigate("/files")}
      onSave={fileActions.createFile}
      scriptFiles={files.items.map(({ name, title }) => ({ name, title }))}
      subscriptions={subscriptions.items.map(({ name, title }) => ({ name, title }))}
    />
  );
}

export function normalizeFileCreateSource(value: string | null | undefined): FileCreateSource {
  return FILE_DRIVER_REGISTRY.resolveCreatePreset(value)?.source ?? "local";
}

async function noopRefresh() {}
