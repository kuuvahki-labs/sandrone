import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useFileResources } from "~/features/files/data/use-file-resources";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { createSubscriptionActions } from "~/features/subscriptions/data/create-subscription-actions";
import { useSubscriptionDetailsResource, useSubscriptionResources } from "~/features/subscriptions/data/use-subscription-resources";
import type { SubscriptionDefinition, SubscriptionItem } from "~/features/subscriptions/model/types";
import { SubscriptionEditPage } from "~/features/subscriptions/pages/subscription-edit-page";
import { useI18n } from "~/shared/i18n/context";
import { remoteInputDefaultsFromSettings } from "~/shared/resources/remote-input-defaults";
import { decodeResourceRouteParam, subscriptionKind, subscriptionPreviewPath } from "~/shared/routing/paths";
import { ResourceLoadingCard } from "~/shared/ui/feedback";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function SubscriptionEditRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const subscriptions = useSubscriptionResources(resourcePorts);
  const files = useFileResources(resourcePorts);
  const { loadSubscriptionDefinition } = useSubscriptionDetailsResource(resourcePorts);
  const subscriptionActions = createSubscriptionActions({
    client: app.client,
    closeSheet: shareDialog.close,
    navigate,
    refreshResources: subscriptions.reload,
    showNotice: app.showNotice,
    t,
  });
  const kind = subscriptionKind(params.kind);
  const name = decodeResourceRouteParam(params.name);
  const listedItem = kind && name ? subscriptions.items.find((candidate) => candidate.kind === kind && candidate.name === name) : undefined;
  const [editSession, setEditSession] = useState<{ item: SubscriptionItem; sources: SubscriptionItem[] } | null>(null);
  const activeSession = kind && editSession?.item.name === name ? editSession : null;
  const item = activeSession?.item ?? listedItem;
  const sourceItems = activeSession?.sources ?? subscriptions.items;
  const [definition, setDefinition] = useState<SubscriptionDefinition | null>(null);
  const [definitionPending, setDefinitionPending] = useState(false);
  const [definitionFailed, setDefinitionFailed] = useState(false);

  useEffect(() => {
    if (!listedItem || !name) return;
    setEditSession((current) => current?.item.name === name ? current : {
      item: listedItem,
      sources: subscriptions.items,
    });
  }, [listedItem, name, subscriptions.items]);

  useEffect(() => {
    if (!item) {
      setDefinition(null);
      setDefinitionPending(false);
      setDefinitionFailed(false);
      return;
    }
    let active = true;
    setDefinition(null);
    setDefinitionPending(true);
    setDefinitionFailed(false);
    void loadSubscriptionDefinition(item.name).then((nextDefinition) => {
      if (active) {
        setDefinition(nextDefinition);
        setDefinitionFailed(nextDefinition === null);
      }
    }).finally(() => {
      if (active) {
        setDefinitionPending(false);
      }
    });
    return () => {
      active = false;
    };
  }, [item, loadSubscriptionDefinition]);

  if (
    (subscriptions.loading && subscriptions.items.length === 0)
    || (files.loading && files.items.length === 0)
  ) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("subscriptions.missing")} onBack={() => navigate("/subscriptions")} />;
  }

  if (!definition && !definitionFailed) {
    return <ResourceLoadingCard title={t("subscriptions.edit.loadingTitle")} />;
  }

  return (
    <SubscriptionEditPage
      item={item}
      key={`${item.kind}:${item.name}:${definition ? "definition" : "summary"}`}
      definition={definition}
      definitionPending={definitionPending}
      onBack={() => navigate("/subscriptions")}
      onCopySource={subscriptionActions.copySubscriptionSource}
      onPreview={() => navigate(subscriptionPreviewPath(kind || item.kind, item.name, "edit"))}
      onSave={(form) => subscriptionActions.saveSubscriptionEdit(item, form, definition)}
      onShare={() => shareDialog.open({ kind: "subscription", name: item.name })}
      probeCacheTTLSeconds={app.effectiveSettings.cache_defaults.probe_ttl_seconds}
      probeDefaults={app.effectiveSettings.probe_defaults}
      remoteDefaults={remoteInputDefaultsFromSettings(app.effectiveSettings)}
      scriptFiles={files.items.map(({ name, title }) => ({ name, title }))}
      scriptTimeoutMS={app.effectiveSettings.script_defaults.timeout_ms}
      sources={sourceItems}
    />
  );
}
