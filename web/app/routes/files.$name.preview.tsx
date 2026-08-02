import { useCallback } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useFileDetailsResource, useFileResources } from "~/features/files/data/use-file-resources";
import type { FilePreview } from "~/features/files/model/types";
import { FilePreviewPage } from "~/features/files/pages/file-preview-page";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useI18n } from "~/shared/i18n/context";
import { useResourcePreview } from "~/shared/preview/use-resource-preview";
import { decodeResourceRouteParam, fileEditPath, resourcePreviewOrigin } from "~/shared/routing/paths";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function FilePreviewRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const [searchParams] = useSearchParams();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const files = useFileResources(resourcePorts);
  const { loadFilePreview } = useFileDetailsResource(resourcePorts);
  const name = decodeResourceRouteParam(params.name);
  const item = name ? files.items.find((candidate) => candidate.name === name) : undefined;
  const backToList = resourcePreviewOrigin(searchParams.get("from")) === "list";
  const loadPreview = useCallback((): Promise<FilePreview | null> => item ? loadFilePreview(item.name) : Promise.resolve(null), [item, loadFilePreview]);
  const { failed, pending, preview, refreshPreview } = useResourcePreview<FilePreview>(item?.name, loadPreview);

  if (files.loading) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("files.missing")} onBack={() => navigate("/files")} />;
  }

  return (
    <FilePreviewPage
      backLabel={t("actions.back")}
      failed={failed}
      fileName={item.name}
      key={item.name}
      onBack={() => navigate(backToList ? "/files" : fileEditPath(item.name))}
      onRefresh={refreshPreview}
      onShare={() => shareDialog.open({ kind: "file", name: item.name })}
      pending={pending}
      preview={preview}
    />
  );
}
