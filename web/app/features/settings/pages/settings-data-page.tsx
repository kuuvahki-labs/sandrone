import { CacheSettingsSection } from "~/features/settings/sections/cache-settings-section";
import { DataSettingsSection } from "~/features/settings/sections/data-settings-section";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsDataPageProps {
  cacheClearing: boolean;
  onBack: () => void;
  onClearCache: () => Promise<void>;
  onDownloadBackup: () => Promise<void>;
  onRestoreBackup: (file: Blob) => Promise<void>;
}

export function SettingsDataPage({
  cacheClearing,
  onBack,
  onClearCache,
  onDownloadBackup,
  onRestoreBackup,
}: SettingsDataPageProps) {
  const { t } = useI18n();

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        title={t("settings.data.title")}
      />
      <CacheSettingsSection
        clearing={cacheClearing}
        onClear={onClearCache}
      />
      <DataSettingsSection
        onDownloadBackup={onDownloadBackup}
        onRestoreBackup={onRestoreBackup}
      />
    </section>
  );
}
