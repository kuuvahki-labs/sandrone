import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { createShareActions } from "~/features/shares/data/create-share-actions";
import { useSharesResource } from "~/features/shares/data/use-shares-resource";
import { SharesPage } from "~/features/shares/pages/shares-page";
import { useI18n } from "~/shared/i18n/context";

export default function SharesRoute() {
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const shares = useSharesResource({
    client: app.client,
    publicBaseUrl: app.publicBaseUrl,
    showNotice: app.showNotice,
    t,
  });
  const shareActions = createShareActions({
    client: app.client,
    closeSheet: shareDialog.close,
    showNotice: app.showNotice,
    t,
  });

  if (shares.loading) return <LoadingScreen />;

  return (
    <SharesPage
      items={shares.items}
      onCopy={(item, format) => void shareActions.copyShare(item, format)}
      onDelete={(item) => app.requestDelete({ kind: "shares", name: item.id, label: t("shares.resourceLabel"), onDeleted: shares.reload })}
    />
  );
}
