import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { createFileActions } from "~/features/files/data/create-file-actions";
import { useFileDetailsResource, useFileResources } from "~/features/files/data/use-file-resources";
import { ruleSetCatalogFromAPI } from "~/features/files/model/codec";
import type { RuleSetCatalogTarget } from "~/features/files/model/types";
import { FileEditPage } from "~/features/files/pages/file-edit-page";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import { subscriptionPreviewFromAPI } from "~/features/subscriptions/model/codec";
import { useI18n } from "~/shared/i18n/context";
import { decodeResourceRouteParam, filePreviewPath } from "~/shared/routing/paths";
import { ResourceLoadingCard } from "~/shared/ui/feedback";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function FileEditRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const files = useFileResources(resourcePorts);
  const subscriptions = useSubscriptionResources(resourcePorts);
  const { fileDetails, loadFileDetail } = useFileDetailsResource(resourcePorts);
  const fileActions = createFileActions({
    client: app.client,
    closeSheet: shareDialog.close,
    navigate,
    refreshResources: files.reload,
    showNotice: app.showNotice,
    t,
  });
  const loadSubscriptionPreview = useCallback(async (subscriptionName: string) => (
    subscriptionPreviewFromAPI(await app.client.previewSubscription(subscriptionName))
  ), [app.client]);
  const loadRuleSetCatalog = useCallback(async (target: RuleSetCatalogTarget) => ruleSetCatalogFromAPI(await app.client.listRuleSetCatalog(target)), [app.client]);
  const name = decodeResourceRouteParam(params.name);
  const item = name ? files.items.find((candidate) => candidate.name === name) : undefined;
  const detail = item ? fileDetails[item.name] : undefined;
  const [detailPending, setDetailPending] = useState(false);
  const [detailFailed, setDetailFailed] = useState(false);

  useEffect(() => {
    if (!item || detail) {
      setDetailPending(false);
      return;
    }
    let active = true;
    setDetailPending(true);
    setDetailFailed(false);
    void loadFileDetail(item.name).then((loaded) => {
      if (active) setDetailFailed(loaded === null);
    }).finally(() => {
      if (active) setDetailPending(false);
    });
    return () => { active = false; };
  }, [detail, item, loadFileDetail]);

  if (files.loading || subscriptions.loading) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("files.missing")} onBack={() => navigate("/files")} />;
  }

  if (!detail && detailFailed) {
    return <MissingResource title={t("errors.fileDefinitionLoadFailed")} onBack={() => navigate("/files")} />;
  }

  if (!detail && !detailFailed) {
    return <ResourceLoadingCard title={t("files.edit.loadingTitle")} />;
  }

  return (
    <FileEditPage
      detail={detail}
      detailPending={detailPending}
      item={item}
      key={`${item.name}:${detail ? "detail" : "summary"}`}
      loadSubscriptionPreview={loadSubscriptionPreview}
      loadRuleSetCatalog={loadRuleSetCatalog}
      onBack={() => navigate("/files")}
      onPreview={() => navigate(filePreviewPath(item.name))}
      onSave={(form) => fileActions.saveFileEdit(item, form, detail)}
      scriptFiles={files.items.map(({ name, title }) => ({ name, title }))}
      subscriptions={subscriptions.items.map(({ name, title }) => ({ name, title }))}
    />
  );
}
