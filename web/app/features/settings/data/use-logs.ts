import { useCallback, useEffect, useRef, useState } from "react";

import type { ApiClient, LogSnapshot } from "~/shared/api/client";

export function useLogs(client: Pick<ApiClient, "getLogs">) {
  const [snapshot, setSnapshot] = useState<LogSnapshot>();
  const [error, setError] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const active = useRef<AbortController | undefined>(undefined);

  const refresh = useCallback(async () => {
    active.current?.abort();
    const controller = new AbortController();
    active.current = controller;
    setLoading(true);
    setError(undefined);
    try {
      const next = await client.getLogs(controller.signal);
      if (!controller.signal.aborted) {
        setSnapshot(next);
        setPage(1);
      }
    } catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : String(cause));
    } finally {
      if (!controller.signal.aborted) setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    setSnapshot(undefined);
    void refresh();
    return () => active.current?.abort();
  }, [refresh]);

  return { snapshot, error, loading, refresh, page, setPage };
}
