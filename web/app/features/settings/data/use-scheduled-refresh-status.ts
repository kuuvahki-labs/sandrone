import { useCallback, useEffect, useRef, useState } from "react";

import type { ApiClient, ScheduledRefreshStatus } from "~/shared/api/client";
import { useUICapabilities } from "~/shared/capabilities/context";

const statusPollIntervalMS = 30_000;

interface ScheduledRefreshStatusController {
  refresh: () => Promise<void>;
  status?: ScheduledRefreshStatus;
}

export function useScheduledRefreshStatus(client: ApiClient): ScheduledRefreshStatusController {
  const { hasFeature } = useUICapabilities();
  const schedulerEnabled = hasFeature("scheduler.enabled");
  const [status, setStatus] = useState<ScheduledRefreshStatus>();
  const requestGeneration = useRef(0);

  const refresh = useCallback(async () => {
    const generation = ++requestGeneration.current;
    if (!schedulerEnabled) {
      setStatus(undefined);
      return;
    }
    try {
      const next = await client.getScheduledRefreshStatus({ fresh: true });
      if (requestGeneration.current === generation) setStatus(next);
    } catch {
      // Status is auxiliary. Keep the last value and let the next poll retry.
    }
  }, [client, schedulerEnabled]);

  useEffect(() => {
    if (!schedulerEnabled) {
      requestGeneration.current += 1;
      setStatus(undefined);
      return;
    }
    let active = true;
    const poll = async () => {
      if (!active) return;
      const generation = ++requestGeneration.current;
      try {
        const next = await client.getScheduledRefreshStatus();
        if (active && requestGeneration.current === generation) setStatus(next);
      } catch {
        // Status is auxiliary. Keep the last value and let the next poll retry.
      }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), statusPollIntervalMS);
    return () => {
      active = false;
      requestGeneration.current += 1;
      window.clearInterval(timer);
    };
  }, [client, schedulerEnabled]);

  return { refresh, status };
}
