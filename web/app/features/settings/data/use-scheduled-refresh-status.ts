import { useEffect, useState } from "react";

import type { ApiClient, ScheduledRefreshStatus } from "~/shared/api/client";

const statusPollIntervalMS = 30_000;

export function useScheduledRefreshStatus(client: ApiClient): ScheduledRefreshStatus | undefined {
  const [status, setStatus] = useState<ScheduledRefreshStatus>();

  useEffect(() => {
    let active = true;
    const poll = async () => {
      if (!active) return;
      try {
        const next = await client.getScheduledRefreshStatus();
        if (active) setStatus(next);
      } catch {
        // Status is auxiliary. Keep the last value and let the next poll retry.
      }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), statusPollIntervalMS);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [client]);

  return status;
}
