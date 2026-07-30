import { useNavigate } from "react-router";

import { useSandrone } from "~/core/provider/context";
import { useBackupOperations } from "~/features/settings/data/use-backup-operations";
import { SettingsDataPage } from "~/features/settings/pages/settings-data-page";
import { useI18n } from "~/shared/i18n/context";

export default function SettingsDataRoute() {
  const app = useSandrone();
  const navigate = useNavigate();
  const { t } = useI18n();
  const backup = useBackupOperations({
    client: app.client,
    reloadSettings: app.reloadSettings,
    showNotice: app.showNotice,
    t,
  });

  return (
    <SettingsDataPage
      onBack={() => navigate("/settings")}
      onDownloadBackup={backup.downloadBackup}
      onRestoreBackup={backup.restoreBackup}
    />
  );
}
