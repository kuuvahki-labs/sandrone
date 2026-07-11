import type { ComponentType } from "react";
import { describe, expect, it } from "vitest";

import productionRoutes from "~/routes";

import { integrationRouteEntries } from "./app-routing.test-data";

interface RouteContractEntry {
  readonly Component: ComponentType;
  readonly file: string;
  readonly id: string;
  readonly index?: true;
  readonly path?: string;
}

interface ProductionRouteEntry {
  readonly children?: readonly unknown[];
  readonly file?: string;
  readonly id?: string;
  readonly index?: boolean;
  readonly path?: string;
}

function normalize(entry: ProductionRouteEntry | RouteContractEntry) {
  return {
    id: entry.id,
    index: entry.index === true,
    path: entry.path ?? null,
    file: entry.file,
  };
}

describe("real-route integration harness", () => {
  it("uses the same component identity for the root and subscriptions aliases", () => {
    expect(integrationRouteEntries[0]?.Component).toBe(integrationRouteEntries[1]?.Component);
  });

  it("covers the exact production route contract without deriving its component table", async () => {
    const configuredRoutes = await Promise.resolve(productionRoutes) as readonly ProductionRouteEntry[];

    expect(configuredRoutes.map(normalize)).toEqual(
      (integrationRouteEntries as readonly RouteContractEntry[]).map(normalize),
    );
    expect(configuredRoutes.every((entry) => !entry.children?.length)).toBe(true);
  });
});
