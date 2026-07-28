import { SettingsPageHeading } from "~/features/settings/components/settings-page-heading";
import { DataSettingsSection } from "~/features/settings/sections/data-settings-section";
import { useI18n } from "~/shared/i18n/context";

interface SettingsDataPageProps {
  onBack: () => void;
  onDownloadBackup: () => Promise<void>;
  onRestoreBackup: (file: Blob) => Promise<void>;
}

export function SettingsDataPage({
  onBack,
  onDownloadBackup,
  onRestoreBackup,
}: SettingsDataPageProps) {
  const { t } = useI18n();

  return (
    <section className="mx-auto grid w-full max-w-[760px] gap-4">
      <SettingsPageHeading title={t("settings.data.title")} onBack={onBack} />
      <DataSettingsSection
        onDownloadBackup={onDownloadBackup}
        onRestoreBackup={onRestoreBackup}
      />
    </section>
  );
}
