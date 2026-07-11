import { useCallback } from "react";
import { useNavigate, useParams } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useFileDetailsResource, useFileResources } from "~/features/files/data/use-file-resources";
import type { FilePreview } from "~/features/files/model/types";
import { FilePreviewPage } from "~/features/files/pages/file-preview-page";
import { useI18n } from "~/shared/i18n/context";
import { useResourcePreview } from "~/shared/preview/use-resource-preview";
import { decodeResourceRouteParam, fileEditPath } from "~/shared/routing/paths";
import { MissingResource } from "~/shared/ui/missing-resource";

export default function FilePreviewRoute() {
  const navigate = useNavigate();
  const params = useParams();
  const app = useSandrone();
  const { t } = useI18n();
  const resourcePorts = { client: app.client, showNotice: app.showNotice, t };
  const files = useFileResources(resourcePorts);
  const { loadFilePreview } = useFileDetailsResource(resourcePorts);
  const name = decodeResourceRouteParam(params.name);
  const item = name ? files.items.find((candidate) => candidate.name === name) : undefined;
  const loadPreview = useCallback((): Promise<FilePreview | null> => item ? loadFilePreview(item.name) : Promise.resolve(null), [item, loadFilePreview]);
  const { failed, pending, preview, refreshPreview } = useResourcePreview<FilePreview>(item?.name, loadPreview);

  if (files.loading) return <LoadingScreen />;

  if (!item) {
    return <MissingResource title={t("files.missing")} onBack={() => navigate("/files")} />;
  }

  return (
    <FilePreviewPage
      failed={failed}
      fileName={item.name}
      key={item.name}
      onBack={() => navigate(fileEditPath(item.name))}
      onRefresh={refreshPreview}
      pending={pending}
      preview={preview}
    />
  );
}
