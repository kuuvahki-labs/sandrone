import { useEffect, useState } from "react";

import type { SubscriptionItem, SubscriptionTraffic } from "~/features/subscriptions/model/types";

export function useSubscriptionTrafficByKey(
  items: SubscriptionItem[],
  enabled: boolean,
  loadSubscriptionTraffic: (name: string, options?: { refresh?: boolean }) => Promise<SubscriptionTraffic | null>,
) {
  const [trafficByKey, setTrafficByKey] = useState<Record<string, SubscriptionTraffic | null>>({});

  useEffect(() => {
    let cancelled = false;
    if (!enabled) {
      setTrafficByKey({});
      return () => {
        cancelled = true;
      };
    }
    for (const item of items) {
      if (item.kind !== "remote") {
        continue;
      }
      const key = subscriptionTrafficKey(item);
      void loadSubscriptionTraffic(item.name).then((traffic) => {
        if (!cancelled) {
          setTrafficByKey((current) => ({ ...current, [key]: traffic }));
        }
      });
    }
    return () => {
      cancelled = true;
    };
  }, [enabled, items, loadSubscriptionTraffic]);

  return trafficByKey;
}

function subscriptionTrafficKey(item: SubscriptionItem): string {
  return `${item.kind}:${item.name}`;
}
