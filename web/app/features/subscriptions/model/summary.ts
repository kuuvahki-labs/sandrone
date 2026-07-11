import type { SubscriptionItem } from "./types";

export function subscriptionSummary(items: SubscriptionItem[]) {
  return items.reduce(
    (summary, item) => ({
      total: summary.total + 1,
      remote: summary.remote + (item.kind === "remote" ? 1 : 0),
      local: summary.local + (item.kind === "local" ? 1 : 0),
      collections: summary.collections + (item.kind === "collection" ? 1 : 0),
      warnings: summary.warnings + (item.status === "warning" ? 1 : 0),
    }),
    { total: 0, remote: 0, local: 0, collections: 0, warnings: 0 },
  );
}
