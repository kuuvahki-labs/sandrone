import { RuntimeSettingsSection } from "~/features/settings/sections/runtime-settings-section";
import { SubscriptionTrafficSettingsSection } from "~/features/settings/sections/subscription-traffic-settings-section";
import type { RuntimeSettingsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

interface SettingsRuntimePageProps {
  autoLoadSubscriptionTraffic: boolean;
  runtimeSettings: RuntimeSettingsInput;
  onBack: () => void;
  onAutoLoadSubscriptionTraffic: (enabled: boolean) => void;
  onSaveRuntimeSettings: (value: RuntimeSettingsInput) => void;
}

export function SettingsRuntimePage({
  autoLoadSubscriptionTraffic,
  runtimeSettings,
  onBack,
  onAutoLoadSubscriptionTraffic,
  onSaveRuntimeSettings,
}: SettingsRuntimePageProps) {
  const { t } = useI18n();

  return (
    <section className="grid gap-6">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        title={t("settings.advanced.title")}
      />
      <SubscriptionTrafficSettingsSection
        autoLoadSubscriptionTraffic={autoLoadSubscriptionTraffic}
        onAutoLoadSubscriptionTraffic={onAutoLoadSubscriptionTraffic}
      />
      <RuntimeSettingsSection
        runtimeSettings={runtimeSettings}
        onSaveRuntimeSettings={onSaveRuntimeSettings}
      />
    </section>
  );
}
