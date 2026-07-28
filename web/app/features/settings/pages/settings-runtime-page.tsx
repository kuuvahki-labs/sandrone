import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsRuntimePageProps {
  runtimeSettings: RuntimeSettingsInput;
  onBack: () => void;
  onSaveRuntimeSettings: (value: RuntimeSettingsInput) => void;
}

export function SettingsRuntimePage({
  runtimeSettings,
  onBack,
  onSaveRuntimeSettings,
}: SettingsRuntimePageProps) {
  const { t } = useI18n();

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        title={t("settings.runtime.title")}
      />
      <RuntimeSettingsSection
        runtimeSettings={runtimeSettings}
        onSaveRuntimeSettings={onSaveRuntimeSettings}
      />
    </section>
  );
}
