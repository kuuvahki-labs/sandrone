import { describe, expect, it } from "vitest";

import {
  decodeResourceRouteParam,
  decodeRouteParam,
  fileNewPath,
  fileResourcePath,
  subscriptionNewPath,
  subscriptionResourcePath,
} from "./paths";

describe("route path helpers", () => {
  it("accepts only single-segment public resource names from route params", () => {
    expect(decodeResourceRouteParam("default.yaml")).toBe("default.yaml");
    expect(decodeResourceRouteParam("name%20with%20space.yaml")).toBe("name with space.yaml");
    expect(decodeResourceRouteParam("files%2Fdefault.txt")).toBeNull();
    expect(decodeResourceRouteParam("files/default.txt")).toBeNull();
    expect(decodeResourceRouteParam("foo%5Cbar")).toBeNull();
    expect(decodeResourceRouteParam(".")).toBeNull();
    expect(decodeResourceRouteParam("..")).toBeNull();
  });

  it("builds typed new resource paths", () => {
    expect(subscriptionNewPath("remote")).toBe("/subscriptions/new?type=remote");
    expect(subscriptionNewPath("local")).toBe("/subscriptions/new?type=local");
    expect(subscriptionNewPath("collection")).toBe("/subscriptions/new?type=collection");
    expect(fileNewPath("local")).toBe("/files/new?source=local");
    expect(fileNewPath("remote")).toBe("/files/new?source=remote");
    expect(fileNewPath("mihomo")).toBe("/files/new?source=mihomo");
    expect(fileNewPath("sing-box")).toBe("/files/new?source=sing-box");
  });

  it("encodes resource names and decodes route params", () => {
    expect(subscriptionResourcePath("remote", "default/sub")).toBe("/subscriptions/remote/default%2Fsub");
    expect(fileResourcePath("default.yaml")).toBe("/files/default.yaml");
    expect(decodeRouteParam("name%20with%20space")).toBe("name with space");
  });
});
