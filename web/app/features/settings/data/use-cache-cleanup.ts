import { useCallback, useRef, useState } from "react";

import type { ApiClient } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

interface UseCacheCleanupOptions {
  client: ApiClient;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t: Translator;
}

export function useCacheCleanup({ client, showNotice, t }: UseCacheCleanupOptions) {
  const [clearing, setClearing] = useState(false);
  const clearingRef = useRef(false);

  const clearCache = useCallback(async () => {
    if (clearingRef.current) {
      return;
    }
    clearingRef.current = true;
    setClearing(true);
    try {
      await client.clearCache();
      showNotice(t("settings.cache.cleared"));
    } catch (error) {
      showNotice(error instanceof Error ? error.message : t("settings.cache.operationFailed"), "error");
      throw error;
    } finally {
      clearingRef.current = false;
      setClearing(false);
    }
  }, [client, showNotice, t]);

  return { clearCache, clearing };
}
