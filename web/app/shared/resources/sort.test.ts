import { describe, expect, it } from "vitest";

import { defaultResourceSort, sortResourceItems } from "./sort";

interface TestResource {
  name: string;
  title?: string;
  createdAt?: string;
  updatedAt?: string;
}

describe("resource list sorting", () => {
  it("sorts resources by created time descending by default without mutating input", () => {
    const items: TestResource[] = [
      { name: "old", createdAt: "2026-06-25T01:00:00.000Z" },
      { name: "fallback", updatedAt: "2026-06-27T01:00:00.000Z" },
      { name: "new", createdAt: "2026-06-26T01:00:00.000Z" },
    ];

    expect(sortResourceItems(items)).toEqual([
      expect.objectContaining({ name: "fallback" }),
      expect.objectContaining({ name: "new" }),
      expect.objectContaining({ name: "old" }),
    ]);
    expect(items.map((item) => item.name)).toEqual(["old", "fallback", "new"]);
    expect(defaultResourceSort).toEqual({ key: "createdAt", direction: "desc" });
  });

  it("places invalid and missing timestamps after valid timestamps", () => {
    const items: TestResource[] = [
      { name: "missing" },
      { name: "valid", createdAt: "2026-06-27T01:00:00.000Z" },
      { name: "invalid", title: "aaa", createdAt: "not-a-date" },
    ];

    expect(sortResourceItems(items).map((item) => item.name)).toEqual(["valid", "invalid", "missing"]);
  });

  it("uses title or name as a stable tie breaker", () => {
    const items: TestResource[] = [
      { name: "zeta", createdAt: "2026-06-27T01:00:00.000Z" },
      { name: "alpha", createdAt: "2026-06-27T01:00:00.000Z" },
      { name: "middle", title: "Beta", createdAt: "2026-06-27T01:00:00.000Z" },
    ];

    expect(sortResourceItems(items).map((item) => item.name)).toEqual(["alpha", "middle", "zeta"]);
  });

  it("supports future explicit name sort descriptors", () => {
    const items: TestResource[] = [
      { name: "alpha" },
      { name: "zeta" },
      { name: "middle", title: "Beta" },
    ];

    expect(sortResourceItems(items, { key: "name", direction: "desc" }).map((item) => item.name)).toEqual(["zeta", "middle", "alpha"]);
  });
});
