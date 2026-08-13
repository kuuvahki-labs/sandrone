import { useEffect, useState } from "react";

import type { ApiClient, ScheduledRefreshStatus } from "~/shared/api/client";
import { useUICapabilities } from "~/shared/capabilities/context";

const statusPollIntervalMS = 30_000;

export function useScheduledRefreshStatus(client: ApiClient): ScheduledRefreshStatus | undefined {
  const { hasFeature } = useUICapabilities();
  const schedulerEnabled = hasFeature("scheduler.enabled");
  const [status, setStatus] = useState<ScheduledRefreshStatus>();

  useEffect(() => {
    if (!schedulerEnabled) {
      setStatus(undefined);
      return;
    }
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
  }, [client, schedulerEnabled]);

  return status;
}
