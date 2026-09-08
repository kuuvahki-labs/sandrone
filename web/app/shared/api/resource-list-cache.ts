import { onApiSessionReset } from "~/shared/storage/api-session";

export type ResourceListKind = "subscriptions" | "files" | "shares";
export const resourceListFreshMs = 5_000;

interface ListEntry {
  value?: unknown;
  updatedAt: number;
  pending?: Promise<unknown>;
  traffic: Map<string, Promise<unknown>>;
}

const lists = new Map<string, ListEntry>();
onApiSessionReset(() => lists.clear());

export function listCacheEntry(baseUrl: string, kind: ResourceListKind): ListEntry {
  const key = JSON.stringify([baseUrl, kind]);
  let entry = lists.get(key);
  if (!entry) {
    entry = { updatedAt: 0, traffic: new Map() };
    lists.set(key, entry);
  }
  return entry;
}

export function invalidateListCache(baseUrl: string, kind?: ResourceListKind): void {
  for (const resource of kind ? [kind] : ["subscriptions", "files", "shares"] as const) {
    lists.delete(JSON.stringify([baseUrl, resource]));
  }
}
