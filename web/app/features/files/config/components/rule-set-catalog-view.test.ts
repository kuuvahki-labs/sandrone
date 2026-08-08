import { expect, it } from "vitest";

import type { RuleSetCatalogItem } from "~/features/files/model/types";

import { deriveCatalogView } from "./rule-set-catalog-view";

it("searches the complete catalog before slicing a fixed 25-item page", () => {
  const items = Array.from({ length: 30 }, (_, index) => (
    item(`match-${String(index).padStart(2, "0")}`, `https://example.com/${index}.mrs`)
  ));
  items.push(item("other", "https://example.com/other.list"));

  const result = deriveCatalogView({
    items,
    page: 2,
    query: "MATCH",
  });

  expect(result.total).toBe(30);
  expect(result.pageCount).toBe(2);
  expect(result.items.map((entry) => entry.name)).toEqual([
    "match-25",
    "match-26",
    "match-27",
    "match-28",
    "match-29",
  ]);
});

function item(name: string, url: string): RuleSetCatalogItem {
  return { name, ruleKind: "domain", url };
}
