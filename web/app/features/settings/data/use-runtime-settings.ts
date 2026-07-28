import { useCallback, useEffect, useRef, useState } from "react";

import { defaultRuntimeSettings } from "~/features/settings/model/runtime-settings";
import type { ApiClient, RuntimeSettingsInput } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

interface UseRuntimeSettingsOptions {
  client: ApiClient;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t: Translator;
}

export function useRuntimeSettings({ client, showNotice, t }: UseRuntimeSettingsOptions) {
  const [runtimeSettings, setRuntimeSettings] = useState<RuntimeSettingsInput>(defaultRuntimeSettings);
  const runtimeSettingsRequest = useRef(0);

  useEffect(() => {
    let cancelled = false;
    const request = ++runtimeSettingsRequest.current;

    void client.getRuntimeSettings()
      .then((settings) => {
        if (!cancelled && runtimeSettingsRequest.current === request) {
          setRuntimeSettings(settings);
        }
      })
      .catch(() => {
        if (!cancelled && runtimeSettingsRequest.current === request) {
          showNotice(t("errors.settingsLoadFailed"), "error");
        }
      });

    return () => {
      cancelled = true;
      if (runtimeSettingsRequest.current === request) {
        runtimeSettingsRequest.current += 1;
      }
    };
  }, [client, showNotice, t]);

  const saveRuntimeSettings = useCallback(async (settings: RuntimeSettingsInput) => {
    await client.updateRuntimeSettings(settings);
    setRuntimeSettings(settings);
    showNotice(t("messages.settingsSaved"));
  }, [client, showNotice, t]);

  return {
    runtimeSettings,
    saveRuntimeSettings,
  };
}
