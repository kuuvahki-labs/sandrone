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
  const [version, setVersion] = useState<string>();
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

  useEffect(() => {
    let cancelled = false;

    void client.getVersion()
      .then((versionInfo) => {
        if (!cancelled) {
          setVersion(versionInfo.version);
        }
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [client]);

  const saveRuntimeSettings = useCallback(async (settings: RuntimeSettingsInput) => {
    await client.updateRuntimeSettings(settings);
    setRuntimeSettings(settings);
    showNotice(t("messages.settingsSaved"));
  }, [client, showNotice, t]);

  const downloadBackup = useCallback(async () => {
    let anchor: HTMLAnchorElement | undefined;
    let objectURL: string | undefined;
    try {
      const backup = await client.downloadBackup();
      objectURL = URL.createObjectURL(backup.blob);
      anchor = document.createElement("a");
      anchor.download = backup.filename;
      anchor.href = objectURL;
      document.body.append(anchor);
      anchor.click();
    } catch {
      showNotice(t("settings.data.downloadFailed"), "error");
    } finally {
      anchor?.remove();
      if (objectURL) {
        URL.revokeObjectURL(objectURL);
      }
    }
  }, [client, showNotice, t]);

  const restoreBackup = useCallback(async (file: Blob) => {
    try {
      await client.restoreBackup(file);
    } catch (error) {
      showNotice(error instanceof Error ? error.message : t("settings.data.restoreFailed"), "error");
      throw error;
    }

    const request = ++runtimeSettingsRequest.current;
    try {
      const settings = await client.getRuntimeSettings({ fresh: true });
      if (runtimeSettingsRequest.current === request) {
        setRuntimeSettings(settings);
      }
    } catch {
      if (runtimeSettingsRequest.current === request) {
        showNotice(t("errors.settingsLoadFailed"), "error");
      }
    }
    showNotice(t("settings.data.restoreSucceeded"));
  }, [client, showNotice, t]);

  return {
    runtimeSettings,
    version,
    saveRuntimeSettings,
    downloadBackup,
    restoreBackup,
  };
}
