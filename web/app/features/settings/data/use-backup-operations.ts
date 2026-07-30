import { useCallback } from "react";

import type { ApiClient } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

interface UseBackupOperationsOptions {
  client: ApiClient;
  reloadSettings: (fresh?: boolean) => Promise<unknown>;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t: Translator;
}

export function useBackupOperations({ client, reloadSettings, showNotice, t }: UseBackupOperationsOptions) {
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

    try {
      await reloadSettings(true);
    } catch {
      showNotice(t("errors.settingsLoadFailed"), "error");
    }
    showNotice(t("settings.data.restoreSucceeded"));
  }, [client, reloadSettings, showNotice, t]);

  return { downloadBackup, restoreBackup };
}
