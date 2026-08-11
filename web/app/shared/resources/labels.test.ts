import { describe, expect, it } from "vitest";

import {
  resourceOptionText,
  resourceSecondaryName,
  resourceTitle,
} from "./labels";

describe("resource labels", () => {
  it("prefers the display title and keeps a differing canonical name", () => {
    const resource = { name: "provider", title: "Provider Main" };

    expect(resourceTitle(resource)).toBe("Provider Main");
    expect(resourceSecondaryName(resource)).toBe("provider");
    expect(resourceOptionText(resource)).toBe("Provider Main (provider)");
  });

  it("uses only the canonical name when no distinct display title exists", () => {
    const resource = { name: "provider", title: "provider" };

    expect(resourceTitle(resource)).toBe("provider");
    expect(resourceSecondaryName(resource)).toBeUndefined();
    expect(resourceOptionText(resource)).toBe("provider");
  });
});
