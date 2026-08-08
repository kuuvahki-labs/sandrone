import type { RuleSetCatalogItem } from "~/features/files/model/types";

export const CATALOG_PAGE_SIZE = 25;

export function deriveCatalogView({
  items,
  page,
  query,
}: {
  items: readonly RuleSetCatalogItem[];
  page: number;
  query: string;
}): {
  items: RuleSetCatalogItem[];
  pageCount: number;
  total: number;
} {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const matchingItems = items.filter((item) => (
    !normalizedQuery
    || item.name.toLocaleLowerCase().includes(normalizedQuery)
    || item.url.toLocaleLowerCase().includes(normalizedQuery)
  ));
  const pageStart = (Math.max(1, page) - 1) * CATALOG_PAGE_SIZE;

  return {
    items: matchingItems.slice(pageStart, pageStart + CATALOG_PAGE_SIZE),
    pageCount: Math.ceil(matchingItems.length / CATALOG_PAGE_SIZE),
    total: matchingItems.length,
  };
}
