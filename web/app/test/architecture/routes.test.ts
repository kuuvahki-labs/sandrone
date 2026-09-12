import { describe, expect, it } from "vitest";

import routes from "~/routes";

const expectedRoutes = [
  { id: "home", index: true, path: null },
  { id: "subscriptions", index: false, path: "subscriptions" },
  { id: "subscriptions-new", index: false, path: "subscriptions/new" },
  { id: "subscriptions-edit", index: false, path: "subscriptions/:kind/:name/edit" },
  { id: "subscriptions-preview", index: false, path: "subscriptions/:kind/:name/preview" },
  { id: "files", index: false, path: "files" },
  { id: "files-new", index: false, path: "files/new" },
  { id: "files-edit", index: false, path: "files/:name/edit" },
  { id: "files-preview", index: false, path: "files/:name/preview" },
  { id: "shares", index: false, path: "shares" },
  { id: "settings", index: false, path: "settings" },
  { id: "settings-service", index: false, path: "settings/service" },
  { id: "settings-data", index: false, path: "settings/data" },
  { id: "settings-logs", index: false, path: "settings/logs" },
] as const;

interface RouteConfigRecord {
  readonly children?: readonly unknown[];
  readonly id?: string;
  readonly index?: boolean;
  readonly path?: string;
}

function normalizeRoute(entry: RouteConfigRecord) {
  return {
    id: entry.id,
    index: entry.index === true,
    path: entry.path ?? null,
  };
}

describe("public React Router modules", () => {
  it("preserves the public route IDs and URL patterns", async () => {
    const configuredRoutes = await Promise.resolve(routes) as readonly RouteConfigRecord[];

    expect(configuredRoutes.map(normalizeRoute)).toEqual(expectedRoutes);
    expect(configuredRoutes.every((entry) => !entry.children?.length)).toBe(true);
  });
});
