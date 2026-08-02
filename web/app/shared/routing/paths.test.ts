import { describe, expect, it } from "vitest";

import {
  decodeResourceRouteParam,
  decodeRouteParam,
  fileNewPath,
  filePreviewPath,
  fileResourcePath,
  resourcePreviewOrigin,
  subscriptionNewPath,
  subscriptionPreviewPath,
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

  it("builds and parses closed preview origins", () => {
    expect(filePreviewPath("default.yaml")).toBe("/files/default.yaml/preview");
    expect(filePreviewPath("default.yaml", "list")).toBe("/files/default.yaml/preview?from=list");
    expect(filePreviewPath("default.yaml", "edit")).toBe("/files/default.yaml/preview?from=edit");
    expect(subscriptionPreviewPath("remote", "provider", "list"))
      .toBe("/subscriptions/remote/provider/preview?from=list");
    expect(resourcePreviewOrigin("list")).toBe("list");
    expect(resourcePreviewOrigin("edit")).toBe("edit");
    expect(resourcePreviewOrigin(null)).toBe("edit");
    expect(resourcePreviewOrigin("https://example.com")).toBe("edit");
  });
});
