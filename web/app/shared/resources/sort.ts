export type ResourceSortKey = "createdAt" | "updatedAt" | "name" | "title";
export type ResourceSortDirection = "asc" | "desc";

export interface ResourceSortDescriptor {
  key: ResourceSortKey;
  direction: ResourceSortDirection;
}

export interface SortableResourceItem {
  name: string;
  title?: string;
  createdAt?: string;
  updatedAt?: string;
}

export const defaultResourceSort: ResourceSortDescriptor = { key: "createdAt", direction: "desc" };

export function sortResourceItems<T extends SortableResourceItem>(
  items: readonly T[],
  descriptor: ResourceSortDescriptor = defaultResourceSort,
): T[] {
  return items
    .map((item, index) => ({ item, index }))
    .sort((left, right) => compareResourceItems(left.item, right.item, descriptor) || left.index - right.index)
    .map(({ item }) => item);
}

function compareResourceItems(left: SortableResourceItem, right: SortableResourceItem, descriptor: ResourceSortDescriptor): number {
  switch (descriptor.key) {
    case "createdAt":
    case "updatedAt":
      return compareResourceTimestamp(left, right, descriptor.key, descriptor.direction) || compareResourceLabel(left, right, { key: "title", direction: "asc" });
    case "name":
    case "title":
      return compareResourceLabel(left, right, descriptor);
  }
}

function compareResourceTimestamp(left: SortableResourceItem, right: SortableResourceItem, key: "createdAt" | "updatedAt", direction: ResourceSortDirection): number {
  const leftTime = resourceTimestamp(left, key);
  const rightTime = resourceTimestamp(right, key);
  if (leftTime === undefined && rightTime === undefined) return 0;
  if (leftTime === undefined) return 1;
  if (rightTime === undefined) return -1;
  if (leftTime === rightTime) return 0;
  return direction === "asc" ? leftTime - rightTime : rightTime - leftTime;
}

function resourceTimestamp(item: SortableResourceItem, key: "createdAt" | "updatedAt"): number | undefined {
  const primary = parseTimestamp(item[key]);
  if (primary !== undefined) return primary;
  return parseTimestamp(item[key === "createdAt" ? "updatedAt" : "createdAt"]);
}

function parseTimestamp(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? timestamp : undefined;
}

function compareResourceLabel(left: SortableResourceItem, right: SortableResourceItem, descriptor: ResourceSortDescriptor): number {
  const leftValue = resourceLabel(left, descriptor.key);
  const rightValue = resourceLabel(right, descriptor.key);
  const comparison = leftValue.localeCompare(rightValue, undefined, { numeric: true, sensitivity: "base" });
  return descriptor.direction === "asc" ? comparison : -comparison;
}

function resourceLabel(item: SortableResourceItem, key: ResourceSortKey): string {
  if (key === "name") return item.name;
  return item.title || item.name;
}
