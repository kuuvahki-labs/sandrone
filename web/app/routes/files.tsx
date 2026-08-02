import { useNavigate } from "react-router";

import { LoadingScreen } from "~/core/components/loading-screen";
import { useSandrone } from "~/core/provider/context";
import { useFileResources } from "~/features/files/data/use-file-resources";
import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";
import { FileDriverIcon } from "~/features/files/editor/file-driver-icon";
import { FilesPage } from "~/features/files/pages/files-page";
import { useShareDialog } from "~/features/shares/components/share-dialog-context";
import { useI18n } from "~/shared/i18n/context";
import { fileEditPath, fileNewPath, filePreviewPath } from "~/shared/routing/paths";

export default function FilesRoute() {
  const navigate = useNavigate();
  const app = useSandrone();
  const shareDialog = useShareDialog();
  const { t } = useI18n();
  const files = useFileResources({ client: app.client, showNotice: app.showNotice, t });

  if (files.loading) return <LoadingScreen />;

  return (
    <FilesPage
      createActions={[
        ...FILE_DRIVER_REGISTRY.createPresets.map((preset) => {
          const driver = FILE_DRIVER_REGISTRY.get(preset.kind)!;
          return {
            ariaLabel: t(preset.accessibleLabelKey ?? driver.presentation.labelKey),
            icon: <FileDriverIcon icon={preset.icon ?? driver.presentation.icon} />,
            label: t(preset.labelKey ?? driver.presentation.labelKey),
            onSelect: () => navigate(fileNewPath(preset.source)),
          };
        }),
      ]}
      items={files.items}
      onDelete={(item) => app.requestDelete({ kind: "files", name: item.name, label: t("nav.files"), onDeleted: files.reload })}
      onEdit={(item) => navigate(fileEditPath(item.name))}
      onPreview={(item) => navigate(filePreviewPath(item.name, "list"))}
      onShare={(item) => shareDialog.open({ kind: "file", name: item.name })}
    />
  );
}
