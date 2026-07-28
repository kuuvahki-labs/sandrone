import { SettingsPageHeading } from "~/features/settings/components/settings-page-heading";
import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

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
    <section className="mx-auto grid w-full max-w-[760px] gap-4">
      <SettingsPageHeading title={t("settings.runtime.title")} onBack={onBack} />
      <RuntimeSettingsSection
        runtimeSettings={runtimeSettings}
        onSaveRuntimeSettings={onSaveRuntimeSettings}
      />
    </section>
  );
}
